package kuro

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	servicekuro "kurohelperservice/airuntime"
	kurosvc "kurohelperservice/kuro"
)

func OnMessageCreate(session *discordgo.Session, event *discordgo.MessageCreate) {
	if event == nil || event.Author == nil || event.Author.Bot {
		return
	}
	if !ChannelAllowed(event.ChannelID) {
		return
	}
	botID := session.State.User.ID
	mentioned := false
	for _, user := range event.Mentions {
		if user != nil && user.ID == botID {
			mentioned = true
			break
		}
	}
	settings := GetSettings()
	images := collectKuroImageAttachments(event.Message)
	if len(event.Attachments) > 0 {
		slog.Info("Kuro Discord attachments inspected",
			"requestID", event.ID,
			"attachmentCount", len(event.Attachments),
			"visionImageCount", len(images),
		)
	}
	content, accepted := prepareKuroTrigger(
		event.Content,
		settings.TriggerPrefix,
		botID,
		mentioned,
	)
	if !accepted || (content == "" && len(images) == 0) {
		return
	}
	if content == "" || (len(images) > 0 && strings.TrimSpace(content) == strings.TrimSpace(settings.TriggerPrefix)) {
		content = "請看看附加的圖片。"
	}
	if command, ok := parseKuroTextCommand(event.Content, settings.TriggerPrefix); ok {
		handleKuroTextCommand(session, event, command)
		return
	}

	client := Client()
	if client == nil {
		return
	}

	if !client.Connected() {
		sendKuroMessage(session, event.ChannelID, "Kuro AI Runtime 目前未連線，請稍後再試。")
		return
	}
	acceptedAt := time.Now()
	queueStartedAt := time.Now()
	LockGeneration()
	botQueueMs := elapsedMilliseconds(queueStartedAt)
	defer UnlockGeneration()
	if !client.Connected() {
		sendKuroMessage(session, event.ChannelID, "Kuro AI Runtime 已中斷連線，請稍後再試。")
		return
	}
	_ = session.ChannelTyping(event.ChannelID)

	historyStartedAt := time.Now()
	boundaryID, err := kurosvc.GetContextBoundary(event.ChannelID)
	if err != nil {
		slog.Warn("讀取 Kuro 頻道上下文邊界失敗", "error", err, "channelID", event.ChannelID)
	}

	recentMessages := fetchRecentMessages(session, event, settings.RecentMessageLimit, boundaryID, settings.TriggerPrefix)
	contextOptions := kuroContextOptions{
		MessageLimit: settings.RecentMessageLimit,
		MaxChars:     settings.RecentContextChars,
		BoundaryID:   boundaryID,
	}
	recentPrompt, retrievalText := buildKuroRecentContext(recentMessages, contextOptions)
	selectedRecentMessages := selectKuroRecentMessages(recentMessages, contextOptions)
	displayName := messageDisplayName(event.Message)
	for index := range images {
		images[index].MessageID = event.ID
		images[index].AuthorName = displayName
	}
	if remaining := kuroMaxVisionImages - len(images); remaining > 0 {
		images = append(images, collectKuroRecentImages(recentMessages, contextOptions, remaining)...)
	}
	mentionedParticipants := mentionedUsers(event.Message, botID)
	contextParticipants := collectKuroContextParticipants(
		recentMessages,
		servicekuro.MentionedUser{ID: event.Author.ID, DisplayName: displayName},
		mentionedParticipants,
	)
	discordHistoryMs := elapsedMilliseconds(historyStartedAt)

	runtimeStartedAt := time.Now()
	response, err := client.Generate(context.Background(), servicekuro.GenerateRequest{
		RequestID:           event.ID,
		ChannelID:           event.ChannelID,
		UserID:              event.Author.ID,
		DisplayName:         displayName,
		Text:                content,
		RecentContext:       recentPrompt,
		RecentMessages:      selectedRecentMessages,
		RetrievalText:       retrievalText,
		MentionedUsers:      mentionedParticipants,
		ContextParticipants: contextParticipants,
		Images:              images,
	})
	runtimeRoundTripMs := elapsedMilliseconds(runtimeStartedAt)
	if err != nil {
		slog.Error("Kuro 生成失敗", "error", err, "requestID", event.ID)
		sendStartedAt := time.Now()
		_, sendErr := sendKuroMessage(session, event.ChannelID, "Kuro 目前無法完成回覆，請稍後再試。")
		recordKuroGenerationMetric(event, acceptedAt, "error", nil, botQueueMs, discordHistoryMs, runtimeRoundTripMs, elapsedMilliseconds(sendStartedAt), sendErr)
		return
	}
	if strings.TrimSpace(response.Text) != "" {
		sendStartedAt := time.Now()
		_, sendErr := sendKuroMessage(session, event.ChannelID, response.Text)
		recordKuroGenerationMetric(event, acceptedAt, "success", response.Metrics, botQueueMs, discordHistoryMs, runtimeRoundTripMs, elapsedMilliseconds(sendStartedAt), sendErr)
	} else {
		recordKuroGenerationMetric(event, acceptedAt, "error", response.Metrics, botQueueMs, discordHistoryMs, runtimeRoundTripMs, 0, errors.New("empty AI response"))
	}
}

func fetchRecentMessages(session *discordgo.Session, current *discordgo.MessageCreate, limit int, boundaryID, triggerPrefix string) []servicekuro.RecentMessage {
	fetchLimit := limit * 3
	if fetchLimit < 15 {
		fetchLimit = 15
	}
	if fetchLimit > 100 {
		fetchLimit = 100
	}
	messages, err := session.ChannelMessages(current.ChannelID, fetchLimit, current.ID, "", "")
	if err != nil {
		slog.Warn("取得 Discord 近期訊息失敗", "error", err, "channelID", current.ChannelID)
		return nil
	}
	messageIDs := make([]string, 0, len(messages))
	for _, message := range messages {
		if message != nil && message.ID != "" {
			messageIDs = append(messageIDs, message.ID)
		}
	}
	contextMessageKinds, err := kurosvc.GetContextMessageKinds(current.ChannelID, messageIDs)
	if err != nil {
		slog.Warn("讀取 Kuro Discord 訊息上下文分類失敗", "error", err, "channelID", current.ChannelID)
		contextMessageKinds = nil
	}
	botID := session.State.User.ID
	result := make([]servicekuro.RecentMessage, 0, len(messages))
	for _, message := range messages {
		if message == nil || message.Author == nil {
			continue
		}
		if _, excluded := contextMessageKinds[message.ID]; excluded {
			continue
		}
		assistant := message.Author.ID == botID
		if message.Author.Bot && !assistant {
			continue
		}
		if assistant && strings.TrimSpace(message.Content) == kuroNewChatConfirmation {
			continue
		}
		if _, isCommand := parseKuroTextCommand(message.Content, triggerPrefix); isCommand {
			continue
		}
		result = append(result, servicekuro.RecentMessage{
			ID:     message.ID,
			UserID: message.Author.ID,
			DisplayName: func() string {
				if assistant {
					return "Kuro"
				}
				return messageDisplayName(message)
			}(),
			Content:   message.Content,
			Assistant: assistant,
			CreatedAt: message.Timestamp,
			Images:    collectKuroImageAttachments(message),
		})
	}
	return result
}

func messageDisplayName(message *discordgo.Message) string {
	if message == nil || message.Author == nil {
		return "Discord 使用者"
	}
	if message.Member != nil && strings.TrimSpace(message.Member.Nick) != "" {
		return message.Member.Nick
	}
	if strings.TrimSpace(message.Author.GlobalName) != "" {
		return message.Author.GlobalName
	}
	return message.Author.Username
}

func mentionedUsers(message *discordgo.Message, botID string) []servicekuro.MentionedUser {
	result := make([]servicekuro.MentionedUser, 0, len(message.Mentions))
	for _, user := range message.Mentions {
		if user == nil || user.ID == botID {
			continue
		}
		name := user.GlobalName
		if strings.TrimSpace(name) == "" {
			name = user.Username
		}
		result = append(result, servicekuro.MentionedUser{ID: user.ID, DisplayName: name})
	}
	return result
}

func sendKuroMessage(session *discordgo.Session, channelID, content string) (*discordgo.Message, error) {
	message, err := session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Content: content,
		AllowedMentions: &discordgo.MessageAllowedMentions{
			Parse: []discordgo.AllowedMentionType{},
		},
	})
	if err != nil {
		slog.Error("發送 Kuro Discord 訊息失敗", "error", err, "channelID", channelID)
	}
	return message, err
}

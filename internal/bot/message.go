package bot

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"

	botkuro "kurohelper/internal/kuro"
	"kurohelperservice/db"
	servicekuro "kurohelperservice/kuro"
)

func OnMessageCreate(session *discordgo.Session, event *discordgo.MessageCreate) {
	if event == nil || event.Author == nil || event.Author.Bot || event.Content == "" {
		return
	}
	if !botkuro.ChannelAllowed(event.ChannelID) {
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
	settings := botkuro.GetSettings()
	content, accepted := servicekuro.PrepareTrigger(
		event.Content,
		settings.TriggerPrefix,
		botID,
		mentioned,
	)
	if !accepted || content == "" {
		return
	}
	if command, ok := servicekuro.ParseTextCommand(event.Content, settings.TriggerPrefix); ok {
		handleKuroTextCommand(session, event, command)
		return
	}

	client := botkuro.Client()
	if client == nil {
		return
	}

	if !client.Connected() {
		sendKuroMessage(session, event.ChannelID, "Kuro AI Runtime 目前未連線，請稍後再試。")
		return
	}
	acceptedAt := time.Now()
	queueStartedAt := time.Now()
	botkuro.LockGeneration()
	botQueueMs := elapsedMilliseconds(queueStartedAt)
	defer botkuro.UnlockGeneration()
	if !client.Connected() {
		sendKuroMessage(session, event.ChannelID, "Kuro AI Runtime 已中斷連線，請稍後再試。")
		return
	}
	_ = session.ChannelTyping(event.ChannelID)

	historyStartedAt := time.Now()
	boundaryID := ""
	state, err := db.GetKuroChannelState(db.Dbs, event.ChannelID)
	if err == nil {
		boundaryID = state.ContextBoundary
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		slog.Warn("讀取 Kuro 頻道上下文邊界失敗", "error", err, "channelID", event.ChannelID)
	}

	recentMessages := fetchRecentMessages(session, event, settings.RecentMessageLimit, boundaryID, settings.TriggerPrefix)
	recentPrompt, retrievalText := servicekuro.BuildRecentContext(recentMessages, servicekuro.ContextOptions{
		MessageLimit: settings.RecentMessageLimit,
		MaxChars:     settings.RecentContextChars,
		BoundaryID:   boundaryID,
	})
	displayName := messageDisplayName(event.Message)
	mentionedParticipants := mentionedUsers(event.Message, botID)
	contextParticipants := servicekuro.CollectContextParticipants(
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
		RetrievalText:       retrievalText,
		MentionedUsers:      mentionedParticipants,
		ContextParticipants: contextParticipants,
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
	botID := session.State.User.ID
	result := make([]servicekuro.RecentMessage, 0, len(messages))
	for _, message := range messages {
		if message == nil || message.Author == nil {
			continue
		}
		assistant := message.Author.ID == botID
		if message.Author.Bot && !assistant {
			continue
		}
		if _, isCommand := servicekuro.ParseTextCommand(message.Content, triggerPrefix); isCommand {
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

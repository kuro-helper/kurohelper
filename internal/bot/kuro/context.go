package kuro

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	servicekuro "kurohelperservice/airuntime"
)

type kuroContextOptions struct {
	MessageLimit int
	MaxChars     int
	BoundaryID   string
}

func collectKuroContextParticipants(messages []servicekuro.RecentMessage, current servicekuro.MentionedUser, mentioned []servicekuro.MentionedUser) []servicekuro.MentionedUser {
	candidates := make([]servicekuro.MentionedUser, 0, len(messages)+len(mentioned)+1)
	candidates = append(candidates, current)
	candidates = append(candidates, mentioned...)
	for _, message := range messages {
		if !message.Assistant {
			candidates = append(candidates, servicekuro.MentionedUser{ID: message.UserID, DisplayName: message.DisplayName})
		}
	}

	participants := make([]servicekuro.MentionedUser, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		candidate.ID = cleanKuroContextText(candidate.ID, 100)
		candidate.DisplayName = cleanKuroContextText(candidate.DisplayName, 100)
		if candidate.ID == "" {
			continue
		}
		if _, exists := seen[candidate.ID]; exists {
			continue
		}
		seen[candidate.ID] = struct{}{}
		participants = append(participants, candidate)
		if len(participants) == 25 {
			break
		}
	}
	return participants
}

func buildKuroRetrievalText(messages []servicekuro.RecentMessage, options kuroContextOptions) string {
	filtered := selectKuroRecentMessages(messages, options)
	_, maxChars := filterKuroRecentMessages(nil, options)
	lines := make([]string, 0, len(filtered))
	used := 0
	for index := len(filtered) - 1; index >= 0; index-- {
		message := filtered[index]
		content := message.Content
		if len(message.Images) > 0 {
			marker := fmt.Sprintf("[附有 %d 張圖片；Discord 訊息 ID=%s]", len(message.Images), message.ID)
			if content == "" {
				content = marker
			} else {
				content += " " + marker
			}
		}
		speaker := message.DisplayName
		if replyLabel := formatKuroReplyLabel(message.ReplyTo); replyLabel != "" {
			speaker += "｜" + replyLabel
		}
		line := fmt.Sprintf("[%s] %s", speaker, content)
		lineLength := utf8.RuneCountInString(line)
		if len(lines) > 0 && used+lineLength+1 > maxChars {
			break
		}
		if lineLength > maxChars {
			line = truncateKuroContextRunes(line, maxChars)
			lineLength = maxChars
		}
		lines = append([]string{line}, lines...)
		used += lineLength + 1
	}
	if len(lines) == 0 {
		return ""
	}

	return strings.Join(lines, "\n")
}

func selectKuroRecentMessages(messages []servicekuro.RecentMessage, options kuroContextOptions) []servicekuro.RecentMessage {
	filtered, maxChars := filterKuroRecentMessages(messages, options)
	selected := make([]servicekuro.RecentMessage, 0, len(filtered))
	used := 0
	for index := len(filtered) - 1; index >= 0; index-- {
		message := filtered[index]
		markerLength := 0
		if len(message.Images) > 0 {
			markerLength = utf8.RuneCountInString(fmt.Sprintf(" [附有 %d 張圖片；Discord 訊息 ID=%s]", len(message.Images), message.ID))
		}
		replyLength := utf8.RuneCountInString(formatKuroReplyLabel(message.ReplyTo))
		if replyLength > 0 {
			replyLength += 1
		}
		fixedLength := utf8.RuneCountInString(message.DisplayName) + replyLength + 3 + markerLength
		lineLength := fixedLength + utf8.RuneCountInString(message.Content)
		if len(selected) > 0 && used+lineLength+1 > maxChars {
			break
		}
		if lineLength > maxChars {
			message.Content = truncateKuroContextRunes(message.Content, max(0, maxChars-fixedLength))
			lineLength = maxChars
		}
		selected = append([]servicekuro.RecentMessage{message}, selected...)
		used += lineLength + 1
	}
	return selected
}

func collectKuroRecentImages(messages []servicekuro.RecentMessage, options kuroContextOptions, maxImages int) []servicekuro.ImageAttachment {
	if maxImages <= 0 {
		return nil
	}
	filtered := selectKuroRecentMessages(messages, options)
	result := make([]servicekuro.ImageAttachment, 0, maxImages)
	seen := make(map[string]struct{})
	for messageIndex := len(filtered) - 1; messageIndex >= 0; messageIndex-- {
		message := filtered[messageIndex]
		for imageIndex := len(message.Images) - 1; imageIndex >= 0; imageIndex-- {
			image := message.Images[imageIndex]
			key := image.ID
			if key == "" {
				key = image.URL
			}
			if key == "" {
				continue
			}
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			image.MessageID = message.ID
			image.AuthorName = message.DisplayName
			image.SourceKind = kuroImageSourceRecent
			image.SourceMessageText = message.Content
			image.ContextOnly = true
			result = append(result, image)
			if len(result) == maxImages {
				return result
			}
		}
	}
	return result
}

func filterKuroRecentMessages(messages []servicekuro.RecentMessage, options kuroContextOptions) ([]servicekuro.RecentMessage, int) {
	limit := options.MessageLimit
	if limit <= 0 || limit > 50 {
		limit = 15
	}
	maxChars := options.MaxChars
	if maxChars < 500 || maxChars > 20000 {
		maxChars = 6000
	}

	filtered := make([]servicekuro.RecentMessage, 0, len(messages))
	for _, message := range messages {
		message.Content = cleanKuroContextText(message.Content, 1500)
		message.DisplayName = cleanKuroContextText(message.DisplayName, 64)
		message.ReplyTo = cleanKuroReplyReference(message.ReplyTo)
		if message.ID == "" || (message.Content == "" && len(message.Images) == 0) || strings.HasPrefix(message.Content, "/") {
			continue
		}
		if options.BoundaryID != "" && !kuroSnowflakeAfter(message.ID, options.BoundaryID) {
			continue
		}
		filtered = append(filtered, message)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].CreatedAt.Equal(filtered[j].CreatedAt) {
			return filtered[i].ID < filtered[j].ID
		}
		return filtered[i].CreatedAt.Before(filtered[j].CreatedAt)
	})
	if len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}
	return filtered, maxChars
}

func cleanKuroReplyReference(reply *servicekuro.ReplyReference) *servicekuro.ReplyReference {
	if reply == nil {
		return nil
	}
	cleaned := *reply
	cleaned.MessageID = cleanKuroContextText(cleaned.MessageID, 100)
	cleaned.UserID = cleanKuroContextText(cleaned.UserID, 100)
	cleaned.DisplayName = cleanKuroContextText(cleaned.DisplayName, 64)
	cleaned.Content = cleanKuroContextText(cleaned.Content, 300)
	if cleaned.ImageCount < 0 {
		cleaned.ImageCount = 0
	}
	if cleaned.ImageCount > kuroMaxVisionImages {
		cleaned.ImageCount = kuroMaxVisionImages
	}
	if cleaned.MessageID == "" && cleaned.DisplayName == "" && cleaned.Content == "" && cleaned.ImageCount == 0 {
		return nil
	}
	return &cleaned
}

func formatKuroReplyLabel(reply *servicekuro.ReplyReference) string {
	reply = cleanKuroReplyReference(reply)
	if reply == nil {
		return ""
	}
	if reply.Unavailable {
		return "回覆一則已無法取得的訊息"
	}
	name := reply.DisplayName
	if name == "" {
		name = "某位使用者"
	}
	if reply.Content != "" {
		return fmt.Sprintf("回覆 %s 的「%s」", name, truncateKuroContextRunes(reply.Content, 120))
	}
	if reply.ImageCount > 0 {
		return fmt.Sprintf("回覆 %s 的圖片訊息", name)
	}
	return fmt.Sprintf("回覆 %s 的訊息", name)
}

func cleanKuroContextText(value string, maxLength int) string {
	value = strings.Join(strings.Fields(value), " ")
	return truncateKuroContextRunes(strings.TrimSpace(value), maxLength)
}

func truncateKuroContextRunes(value string, maxLength int) string {
	runes := []rune(value)
	if len(runes) <= maxLength {
		return value
	}
	return string(runes[:maxLength])
}

func kuroSnowflakeAfter(messageID, boundaryID string) bool {
	if len(messageID) != len(boundaryID) {
		return len(messageID) > len(boundaryID)
	}
	return messageID > boundaryID
}

package kuro

import (
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/bwmarrin/discordgo"

	servicekuro "kurohelperservice/airuntime"
)

const (
	kuroImageSourceCurrent = "current"
	kuroImageSourceReply   = "reply"
	kuroImageSourceRecent  = "recent"
)

const (
	kuroMaxVisionImages    = 4
	kuroMaxVisionImageSize = 10 * 1024 * 1024
)

var kuroVisionExtensions = map[string]struct{}{
	".gif":  {},
	".jpeg": {},
	".jpg":  {},
	".png":  {},
	".webp": {},
}

func collectKuroImageAttachments(message *discordgo.Message) []servicekuro.ImageAttachment {
	if message == nil {
		return nil
	}
	images := make([]servicekuro.ImageAttachment, 0, kuroMaxVisionImages)
	for _, attachment := range message.Attachments {
		if attachment == nil || !isKuroVisionImage(attachment) {
			continue
		}
		if attachment.Size > kuroMaxVisionImageSize {
			continue
		}
		imageURL := strings.TrimSpace(attachment.URL)
		if imageURL == "" {
			imageURL = strings.TrimSpace(attachment.ProxyURL)
		}
		if imageURL == "" {
			continue
		}
		images = append(images, servicekuro.ImageAttachment{
			ID:          strings.TrimSpace(attachment.ID),
			URL:         imageURL,
			Filename:    strings.TrimSpace(attachment.Filename),
			ContentType: strings.TrimSpace(attachment.ContentType),
			Size:        attachment.Size,
		})
		if len(images) == kuroMaxVisionImages {
			break
		}
	}
	return images
}

func isKuroVisionImage(attachment *discordgo.MessageAttachment) bool {
	if attachment == nil {
		return false
	}
	contentType := strings.ToLower(strings.TrimSpace(attachment.ContentType))
	if strings.HasPrefix(contentType, "image/") {
		return true
	}
	_, ok := kuroVisionExtensions[strings.ToLower(filepath.Ext(attachment.Filename))]
	return ok
}

func resolveKuroReplyMessage(session *discordgo.Session, event *discordgo.MessageCreate) *discordgo.Message {
	if event == nil || event.Message == nil {
		return nil
	}
	if event.ReferencedMessage != nil {
		return event.ReferencedMessage
	}
	if session == nil {
		return nil
	}
	reference := event.MessageReference
	if reference == nil || strings.TrimSpace(reference.MessageID) == "" {
		return nil
	}
	channelID := strings.TrimSpace(reference.ChannelID)
	if channelID == "" {
		channelID = event.ChannelID
	}
	if channelID != event.ChannelID {
		slog.Warn("忽略跨頻道的 Kuro Discord 回覆引用", "requestID", event.ID)
		return nil
	}
	message, err := session.ChannelMessage(channelID, reference.MessageID)
	if err != nil {
		slog.Warn("取得 Kuro Discord 被回覆訊息失敗", "error", err, "requestID", event.ID)
		return nil
	}
	return message
}

func annotateKuroImages(
	images []servicekuro.ImageAttachment,
	message *discordgo.Message,
	sourceKind string,
	contextOnly bool,
) []servicekuro.ImageAttachment {
	if len(images) == 0 {
		return nil
	}
	result := make([]servicekuro.ImageAttachment, len(images))
	copy(result, images)
	messageID := ""
	authorName := ""
	sourceText := ""
	if message != nil {
		messageID = strings.TrimSpace(message.ID)
		authorName = messageDisplayName(message)
		sourceText = cleanKuroContextText(message.Content, 1500)
	}
	for index := range result {
		result[index].MessageID = messageID
		result[index].AuthorName = authorName
		result[index].SourceKind = sourceKind
		result[index].SourceMessageText = sourceText
		result[index].ContextOnly = contextOnly
	}
	return result
}

func appendUniqueKuroImages(
	destination []servicekuro.ImageAttachment,
	candidates []servicekuro.ImageAttachment,
	maximum int,
) []servicekuro.ImageAttachment {
	if maximum <= 0 {
		return nil
	}
	if len(destination) > maximum {
		destination = destination[:maximum]
	}
	seen := make(map[string]struct{}, maximum)
	for _, image := range destination {
		if key := kuroImageKey(image); key != "" {
			seen[key] = struct{}{}
		}
	}
	for _, image := range candidates {
		if len(destination) == maximum {
			break
		}
		key := kuroImageKey(image)
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		destination = append(destination, image)
	}
	return destination
}

func kuroImageKey(image servicekuro.ImageAttachment) string {
	if id := strings.TrimSpace(image.ID); id != "" {
		return "id:" + id
	}
	if url := strings.TrimSpace(image.URL); url != "" {
		return "url:" + url
	}
	return ""
}

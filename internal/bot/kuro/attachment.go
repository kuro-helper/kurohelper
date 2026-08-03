package kuro

import (
	"path/filepath"
	"strings"

	"github.com/bwmarrin/discordgo"

	servicekuro "kurohelperservice/airuntime"
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

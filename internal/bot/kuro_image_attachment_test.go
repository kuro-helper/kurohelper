package bot

import (
	"fmt"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestCollectKuroImageAttachmentsFiltersAndCapsImages(t *testing.T) {
	attachments := []*discordgo.MessageAttachment{
		{ID: "text", URL: "https://cdn.discordapp.com/attachments/a/b/file.txt", Filename: "file.txt", ContentType: "text/plain"},
		{ID: "large", URL: "https://cdn.discordapp.com/attachments/a/b/large.png", Filename: "large.png", ContentType: "image/png", Size: kuroMaxVisionImageSize + 1},
	}
	for index := 0; index < 5; index++ {
		attachments = append(attachments, &discordgo.MessageAttachment{
			ID:          fmt.Sprintf("image-%d", index),
			URL:         fmt.Sprintf("https://cdn.discordapp.com/attachments/a/b/%d.png", index),
			Filename:    fmt.Sprintf("%d.png", index),
			ContentType: "image/png",
			Size:        1024,
		})
	}

	images := collectKuroImageAttachments(&discordgo.Message{Attachments: attachments})
	if len(images) != kuroMaxVisionImages {
		t.Fatalf("got %d images, want %d", len(images), kuroMaxVisionImages)
	}
	if images[0].ID != "image-0" || images[3].ID != "image-3" {
		t.Fatalf("unexpected image selection: %#v", images)
	}
}

func TestCollectKuroImageAttachmentsAcceptsKnownExtensionWithoutMIME(t *testing.T) {
	images := collectKuroImageAttachments(&discordgo.Message{Attachments: []*discordgo.MessageAttachment{{
		ID: "image", ProxyURL: "https://media.discordapp.net/attachments/a/b/photo.webp", Filename: "photo.WEBP",
	}}})
	if len(images) != 1 || images[0].URL == "" {
		t.Fatalf("expected extension-based image, got %#v", images)
	}
}

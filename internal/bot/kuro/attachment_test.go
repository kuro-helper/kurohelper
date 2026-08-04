package kuro

import (
	"fmt"
	"testing"

	"github.com/bwmarrin/discordgo"

	servicekuro "kurohelperservice/airuntime"
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

func TestResolveKuroReplyMessageUsesEmbeddedDiscordReply(t *testing.T) {
	replied := &discordgo.Message{ID: "reply-message"}
	event := &discordgo.MessageCreate{Message: &discordgo.Message{
		ID:                "current-message",
		ReferencedMessage: replied,
	}}
	if got := resolveKuroReplyMessage(nil, event); got != replied {
		t.Fatalf("resolved reply = %#v, want embedded message", got)
	}
}

func TestKuroImagePriorityAndDeduplication(t *testing.T) {
	currentMessage := &discordgo.Message{
		ID:      "current-message",
		Content: "小黑 這張圖是什麼？",
		Author:  &discordgo.User{Username: "Current"},
	}
	replyMessage := &discordgo.Message{
		ID:      "reply-message",
		Content: "請看這張錯誤截圖",
		Author:  &discordgo.User{Username: "ReplyAuthor"},
	}
	current := annotateKuroImages([]servicekuro.ImageAttachment{{
		ID: "current-image", URL: "https://cdn.discordapp.com/attachments/a/b/current.png",
	}}, currentMessage, kuroImageSourceCurrent, false)
	reply := annotateKuroImages([]servicekuro.ImageAttachment{
		{ID: "reply-image", URL: "https://cdn.discordapp.com/attachments/a/b/reply.png"},
		{ID: "duplicate-recent", URL: "https://cdn.discordapp.com/attachments/a/b/duplicate.png"},
	}, replyMessage, kuroImageSourceReply, false)
	recent := []servicekuro.ImageAttachment{
		{ID: "duplicate-recent", URL: "https://cdn.discordapp.com/attachments/a/b/duplicate.png", SourceKind: kuroImageSourceRecent},
		{ID: "recent-image", URL: "https://cdn.discordapp.com/attachments/a/b/recent.png", SourceKind: kuroImageSourceRecent},
		{ID: "overflow-image", URL: "https://cdn.discordapp.com/attachments/a/b/overflow.png", SourceKind: kuroImageSourceRecent},
	}

	images := appendUniqueKuroImages(nil, current, kuroMaxVisionImages)
	images = appendUniqueKuroImages(images, reply, kuroMaxVisionImages)
	images = appendUniqueKuroImages(images, recent, kuroMaxVisionImages)

	if len(images) != 4 {
		t.Fatalf("got %d images, want 4: %#v", len(images), images)
	}
	wantIDs := []string{"current-image", "reply-image", "duplicate-recent", "recent-image"}
	for index, want := range wantIDs {
		if images[index].ID != want {
			t.Fatalf("images[%d].ID = %q, want %q", index, images[index].ID, want)
		}
	}
	if images[0].SourceKind != kuroImageSourceCurrent || images[1].SourceKind != kuroImageSourceReply {
		t.Fatalf("unexpected source ordering: %#v", images)
	}
	if images[1].SourceMessageText != "請看這張錯誤截圖" || images[1].AuthorName != "ReplyAuthor" {
		t.Fatalf("reply source metadata missing: %#v", images[1])
	}
}

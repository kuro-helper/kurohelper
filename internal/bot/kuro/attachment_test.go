package kuro

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"

	servicekuro "kurohelperservice/airuntime"
)

type kuroRoundTripFunc func(*http.Request) (*http.Response, error)

func (function kuroRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

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

func TestResolveKuroReplyMessageFailsOpenWhenRepliedMessageWasDeleted(t *testing.T) {
	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("discordgo.New() error = %v", err)
	}
	requests := 0
	session.Client = &http.Client{Transport: kuroRoundTripFunc(func(_ *http.Request) (*http.Response, error) {
		requests++
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Status:     "404 Not Found",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"message":"Unknown Message","code":10008}`)),
		}, nil
	})}
	event := &discordgo.MessageCreate{Message: &discordgo.Message{
		ID:        "current-message",
		ChannelID: "channel-a",
		MessageReference: &discordgo.MessageReference{
			MessageID: "deleted-message",
			ChannelID: "channel-a",
		},
	}}

	if got := resolveKuroReplyMessage(session, event); got != nil {
		t.Fatalf("resolved deleted reply = %#v, want nil", got)
	}
	if requests != 1 {
		t.Fatalf("Discord message fetches = %d, want 1", requests)
	}
}

func TestReplyImageAndCurrentMessageImageAreBothPreserved(t *testing.T) {
	replied := &discordgo.Message{
		ID:      "reply-message",
		Content: "被回覆圖片的問題",
		Author:  &discordgo.User{Username: "ReplyAuthor"},
		Attachments: []*discordgo.MessageAttachment{{
			ID: "reply-image", URL: "https://cdn.discordapp.com/attachments/a/b/reply.png",
			Filename: "reply.png", ContentType: "image/png",
		}},
	}
	current := &discordgo.Message{
		ID:      "current-message",
		Content: "小黑，比較這兩張圖",
		Author:  &discordgo.User{Username: "CurrentAuthor"},
		Attachments: []*discordgo.MessageAttachment{{
			ID: "current-image", URL: "https://cdn.discordapp.com/attachments/a/b/current.png",
			Filename: "current.png", ContentType: "image/png",
		}},
		ReferencedMessage: replied,
	}

	images := appendUniqueKuroImages(nil, annotateKuroImages(
		collectKuroImageAttachments(current), current, kuroImageSourceCurrent, false,
	), kuroMaxVisionImages)
	images = appendUniqueKuroImages(images, annotateKuroImages(
		collectKuroImageAttachments(replied), replied, kuroImageSourceReply, false,
	), kuroMaxVisionImages)

	if len(images) != 2 {
		t.Fatalf("images = %#v, want current and reply images", images)
	}
	if images[0].ID != "current-image" || images[0].SourceKind != kuroImageSourceCurrent {
		t.Fatalf("current image priority was lost: %#v", images[0])
	}
	if images[1].ID != "reply-image" || images[1].SourceKind != kuroImageSourceReply {
		t.Fatalf("reply image metadata was lost: %#v", images[1])
	}
	if images[1].MessageID != "reply-message" || images[1].SourceMessageText != "被回覆圖片的問題" {
		t.Fatalf("reply source context was lost: %#v", images[1])
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

package kuro

import (
	"strings"
	"testing"
	"time"

	servicekuro "kurohelperservice/airuntime"
)

func TestBuildKuroRecentContextKeepsSpeakersAndBoundary(t *testing.T) {
	now := time.Now()
	prompt, retrieval := buildKuroRecentContext([]servicekuro.RecentMessage{
		{ID: "100", DisplayName: "舊使用者", Content: "不應出現", CreatedAt: now},
		{ID: "102", DisplayName: "肉圓", Content: "早安", CreatedAt: now.Add(time.Second)},
		{ID: "103", DisplayName: "Kuro", Content: "……早安。", Assistant: true, CreatedAt: now.Add(2 * time.Second)},
	}, kuroContextOptions{BoundaryID: "101", MessageLimit: 15, MaxChars: 6000})
	if strings.Contains(prompt, "不應出現") {
		t.Fatal("context included a message before the new-chat boundary")
	}
	if !strings.Contains(retrieval, "[肉圓] 早安") || !strings.Contains(retrieval, "[Kuro] ……早安。") {
		t.Fatalf("unexpected context: %s", retrieval)
	}
}

func TestCollectKuroContextParticipantsUsesStableIDs(t *testing.T) {
	participants := collectKuroContextParticipants(
		[]servicekuro.RecentMessage{{UserID: "u2", DisplayName: "海獺"}, {UserID: "bot", Assistant: true}, {UserID: "u1", DisplayName: "舊名"}},
		servicekuro.MentionedUser{ID: "u1", DisplayName: "肉圓"},
		[]servicekuro.MentionedUser{{ID: "u3", DisplayName: "Tommy"}},
	)
	if len(participants) != 3 || participants[0].ID != "u1" || participants[1].ID != "u3" || participants[2].ID != "u2" {
		t.Fatalf("participants = %#v", participants)
	}
}

func TestKuroRecentContextIncludesImageOnlyMessages(t *testing.T) {
	now := time.Now()
	messages := []servicekuro.RecentMessage{
		{ID: "100", DisplayName: "Earlier", Images: []servicekuro.ImageAttachment{{ID: "old", URL: "https://cdn.discordapp.com/old.png"}}, CreatedAt: now},
		{ID: "102", DisplayName: "Alice", Content: "看這張", Images: []servicekuro.ImageAttachment{{ID: "image-1", URL: "https://cdn.discordapp.com/a.png"}}, CreatedAt: now.Add(time.Second)},
		{ID: "103", DisplayName: "Bob", Images: []servicekuro.ImageAttachment{{ID: "image-2", URL: "https://cdn.discordapp.com/b.png"}}, CreatedAt: now.Add(2 * time.Second)},
	}
	options := kuroContextOptions{BoundaryID: "101", MessageLimit: 15, MaxChars: 6000}
	_, retrieval := buildKuroRecentContext(messages, options)
	if !strings.Contains(retrieval, "[Alice] 看這張 [附有 1 張圖片；Discord 訊息 ID=102]") || !strings.Contains(retrieval, "[Bob] [附有 1 張圖片；Discord 訊息 ID=103]") {
		t.Fatalf("missing image context: %s", retrieval)
	}
	images := collectKuroRecentImages(messages, options, 4)
	if len(images) != 2 || images[0].ID != "image-2" || images[1].ID != "image-1" || !images[0].ContextOnly {
		t.Fatalf("unexpected recent images: %#v", images)
	}
}

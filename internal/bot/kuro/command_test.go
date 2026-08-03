package kuro

import (
	"strings"
	"testing"

	servicekuro "kurohelperservice/airuntime"
)

func TestKuroTextCommandHelpUsesEnglishCommandNames(t *testing.T) {
	for _, command := range []string{
		"/help",
		"/newchat",
		"/status",
		"/raw-responses",
		"/memory-list",
		"/memory-info",
		"/memory-trash",
		"/forget",
		"/restore",
		"/memory-clear confirm",
		"/memory-backups",
		"/memory-backup",
		"/memory-rollback",
	} {
		if !strings.Contains(kuroTextCommandHelp, command) {
			t.Fatalf("help is missing %q", command)
		}
	}
	for _, command := range []string{
		"/新對話",
		"/狀態",
		"/記憶列表",
		"/記憶垃圾桶",
		"/忘記",
		"/復原",
		"/清除記憶",
	} {
		if strings.Contains(kuroTextCommandHelp, command) {
			t.Fatalf("help still contains Chinese command name %q", command)
		}
	}
}

func TestFormatKuroRawRepliesShowsNewestFirstAndPreservesText(t *testing.T) {
	formatted := strings.Join(formatKuroRawReplies(servicekuro.RawRepliesResponse{
		Entries: []servicekuro.RawReply{
			{CachedAt: "2026-08-01T01:00:00Z", RawText: "第一則（低下頭）"},
			{CachedAt: "2026-08-01T02:00:00Z", RawText: "第二則（耳朵抖了一下）"},
		},
	}), "")
	if strings.Index(formatted, "第二則") > strings.Index(formatted, "第一則") {
		t.Fatalf("newest reply should be shown first: %s", formatted)
	}
	for _, expected := range []string{"（低下頭）", "（耳朵抖了一下）", "最近 2 則"} {
		if !strings.Contains(formatted, expected) {
			t.Fatalf("formatted raw replies are missing %q: %s", expected, formatted)
		}
	}
}

func TestFormatKuroRawRepliesSplitsDiscordMessages(t *testing.T) {
	formatted := formatKuroRawReplies(servicekuro.RawRepliesResponse{
		Entries: []servicekuro.RawReply{{RawText: strings.Repeat("原", 4000)}},
	})
	if len(formatted) < 2 {
		t.Fatal("long raw replies should be split into multiple Discord messages")
	}
	for _, message := range formatted {
		if len([]rune(message)) > 1900 {
			t.Fatalf("Discord message exceeds limit: %d", len([]rune(message)))
		}
	}
}

func TestTextCommandPage(t *testing.T) {
	tests := []struct {
		args  []string
		want  int
		valid bool
	}{
		{args: nil, want: 1, valid: true},
		{args: []string{"1"}, want: 1, valid: true},
		{args: []string{"50"}, want: 50, valid: true},
		{args: []string{"0"}, valid: false},
		{args: []string{"100001"}, valid: false},
		{args: []string{"abc"}, valid: false},
		{args: []string{"10", "20"}, valid: false},
	}
	for _, test := range tests {
		got, valid := textCommandPage(test.args)
		if got != test.want || valid != test.valid {
			t.Fatalf("textCommandPage(%v) = (%d, %t), want (%d, %t)", test.args, got, valid, test.want, test.valid)
		}
	}
}

func TestFormatKuroMemoriesShowsPagination(t *testing.T) {
	result := servicekuro.MemoryResponse{
		Count: 12,
		Memories: []servicekuro.Memory{{
			ID:       "abcdef123456",
			Category: "conversation_event",
			Value:    "大家決定星期六一起玩遊戲。",
		}},
	}
	formatted := formatKuroMemories(result, false, 2, 5)
	for _, expected := range []string{"第 2/3 頁", "共 12 條", "`abcdef12`", "小黑 /memory-list <頁碼>"} {
		if !strings.Contains(formatted, expected) {
			t.Fatalf("formatted page is missing %q: %s", expected, formatted)
		}
	}
}

func TestFormatKuroMemoriesRejectsOutOfRangePage(t *testing.T) {
	formatted := formatKuroMemories(servicekuro.MemoryResponse{Count: 12}, true, 4, 5)
	if !strings.Contains(formatted, "頁碼超出範圍") || !strings.Contains(formatted, "3 頁") {
		t.Fatalf("unexpected out-of-range message: %s", formatted)
	}
}

func TestFormatKuroMemoryDetail(t *testing.T) {
	formatted := formatKuroMemoryDetail(servicekuro.MemoryResponse{
		Status: "found",
		Memory: &servicekuro.Memory{
			ID: "abcdef12-3456", Key: "event.key", Value: "一段重要事件",
			Category: "conversation_event", Status: "active",
			Importance: 0.9, Confidence: 0.95, Scope: "channel", ScopeID: "channel-1",
			CreatedAt:      "2026-07-30T05:01:31.082197+00:00",
			UpdatedAt:      "2026-07-30T05:01:31.082197+00:00",
			LastAccessedAt: "2026-07-31T03:31:35.852803+00:00", AccessCount: 17,
			SourceChannelID: "kurohelper:channel-1", SourceRequestID: "request-1",
			Participants: []servicekuro.MemoryParticipant{{DisplayName: "肉圓", Role: "speaker"}},
		},
	})
	for _, expected := range []string{"abcdef12-3456", "一段重要事件", "0.90／0.95", "肉圓", "17 次", "request-1"} {
		if !strings.Contains(formatted, expected) {
			t.Fatalf("formatted detail is missing %q: %s", expected, formatted)
		}
	}
}

func TestFormatKuroMemoryDetailHandlesLookupErrors(t *testing.T) {
	if !strings.Contains(formatKuroMemoryDetail(servicekuro.MemoryResponse{Status: "ambiguous"}), "多條") {
		t.Fatal("ambiguous lookup should ask for a longer ID")
	}
	if !strings.Contains(formatKuroMemoryDetail(servicekuro.MemoryResponse{Status: "not_found"}), "找不到") {
		t.Fatal("missing memory should be reported")
	}
}

func TestTextCommandConfirmed(t *testing.T) {
	for _, value := range []string{"confirm", "CONFIRM"} {
		if !textCommandConfirmed(value) {
			t.Fatalf("expected %q to confirm", value)
		}
	}
	for _, value := range []string{"確認", "确认", "true", "yes"} {
		if textCommandConfirmed(value) {
			t.Fatalf("unexpected confirmation from %q", value)
		}
	}
}

func TestFormatKuroMemoryBackups(t *testing.T) {
	result := servicekuro.MemoryResponse{
		Count:                6,
		BackupRetentionCount: 30,
		Backups: []servicekuro.MemoryBackup{{
			ID:          "20260730T120000Z-auto-abcdef12",
			CreatedAt:   "2026-07-30T12:00:00+00:00",
			Reason:      "auto",
			SizeBytes:   4096,
			MemoryCount: 3,
		}},
	}
	formatted := formatKuroMemoryBackups(result, 1, 5)
	for _, expected := range []string{"第 1/2 頁", "最多保留 30 份", "定時", "3 條", "20260730T120000Z-auto-abcdef12"} {
		if !strings.Contains(formatted, expected) {
			t.Fatalf("formatted backup list is missing %q: %s", expected, formatted)
		}
	}
}

func TestFormatKuroBackupRestoreIncludesSafetyBackup(t *testing.T) {
	formatted := formatKuroBackupRestore(servicekuro.MemoryResponse{
		Status:              "restored_backup",
		RestoredActiveCount: 4,
		Backup:              &servicekuro.MemoryBackup{ID: "selected-backup"},
		SafetyBackup:        &servicekuro.MemoryBackup{ID: "safety-backup"},
	})
	for _, expected := range []string{"selected-backup", "4 條", "safety-backup"} {
		if !strings.Contains(formatted, expected) {
			t.Fatalf("formatted restore result is missing %q: %s", expected, formatted)
		}
	}
}

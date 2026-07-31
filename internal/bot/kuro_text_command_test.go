package bot

import (
	"strings"
	"testing"

	servicekuro "kurohelperservice/kuro"
)

func TestKuroTextCommandHelpUsesEnglishCommandNames(t *testing.T) {
	for _, command := range []string{
		"/help",
		"/newchat",
		"/status",
		"/memory-list",
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

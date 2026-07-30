package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	botkuro "kurohelper/internal/kuro"
	"kurohelperservice/db"
	servicekuro "kurohelperservice/kuro"
)

const kuroMemoryPageSize = 5

const kuroTextCommandHelp = `Kuro 可用指令：
小黑 /help — 列出這份指令說明
小黑 /newchat — 開始新的短期對話
小黑 /status — 查看 AI Runtime 狀態
小黑 /memory-list [頁碼] — 分頁列出有效記憶
小黑 /memory-trash [頁碼] — 分頁列出記憶垃圾桶
小黑 /forget <記憶ID> — 將記憶移入垃圾桶
小黑 /restore <記憶ID> — 復原記憶
小黑 /memory-clear confirm — 將所有有效記憶移入垃圾桶`

func handleKuroTextCommand(session *discordgo.Session, event *discordgo.MessageCreate, command servicekuro.TextCommand) {
	if !botkuro.CommandAllowed(event.Author.ID) {
		sendKuroMessage(session, event.ChannelID, "你沒有使用 Kuro 管理指令的權限。")
		return
	}

	if command.Name == "help" {
		sendKuroMessage(session, event.ChannelID, kuroTextCommandHelp)
		return
	}

	if command.Name == "newchat" {
		if len(command.Args) != 0 {
			sendKuroMessage(session, event.ChannelID, "用法：小黑 /newchat")
			return
		}
		botkuro.LockGeneration()
		defer botkuro.UnlockGeneration()
		if err := db.SetKuroContextBoundary(db.Dbs, event.ChannelID, event.ID); err != nil {
			sendKuroMessage(session, event.ChannelID, "建立新對話失敗，請稍後再試。")
			return
		}
		sendKuroMessage(session, event.ChannelID, "已開始新的短期對話；長期記憶不會被刪除。")
		return
	}

	client := botkuro.Client()
	if client == nil || !client.Connected() {
		sendKuroMessage(session, event.ChannelID, "Kuro AI Runtime 目前未連線。")
		return
	}
	if command.Name == "forget" || command.Name == "restore" || command.Name == "memory-clear" {
		botkuro.LockGeneration()
		defer botkuro.UnlockGeneration()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var content string
	var err error
	switch command.Name {
	case "status":
		if len(command.Args) != 0 {
			content = "用法：小黑 /status"
			break
		}
		var health servicekuro.HealthResponse
		health, err = client.Health(ctx)
		if err == nil {
			content = fmt.Sprintf("Runtime：%s\nSillyTavern：%t\n長期記憶：%t", health.Status, health.SillyTavernReady, health.MemoryEnabled)
		}
	case "memory-list", "memory-trash":
		page, valid := textCommandPage(command.Args)
		if !valid {
			content = "用法：小黑 /memory-list [頁碼]，或小黑 /memory-trash [頁碼]"
			break
		}
		status := "active"
		if command.Name == "memory-trash" {
			status = "deleted"
		}
		var result servicekuro.MemoryResponse
		result, err = client.ListMemories(ctx, status, kuroMemoryPageSize, (page-1)*kuroMemoryPageSize)
		if err == nil {
			content = formatKuroMemories(result, status == "deleted", page, kuroMemoryPageSize)
		}
	case "forget", "restore":
		if len(command.Args) != 1 || len(command.Args[0]) < 6 {
			content = "用法：小黑 /forget <記憶ID>，或小黑 /restore <記憶ID>"
			break
		}
		var result servicekuro.MemoryResponse
		if command.Name == "forget" {
			result, err = client.ForgetMemory(ctx, command.Args[0])
			if err == nil {
				content = formatKuroMemoryAction(result, "已移入垃圾桶")
			}
		} else {
			result, err = client.RestoreMemory(ctx, command.Args[0])
			if err == nil {
				content = formatKuroMemoryAction(result, "已復原")
			}
		}
	case "memory-clear":
		if len(command.Args) != 1 || !textCommandConfirmed(command.Args[0]) {
			content = "這會把所有有效記憶移入垃圾桶。若要執行，請輸入：小黑 /memory-clear confirm"
			break
		}
		var result servicekuro.MemoryResponse
		result, err = client.ClearMemories(ctx)
		if err == nil {
			content = fmt.Sprintf("已將 %d 條記憶移入垃圾桶，%d 天內可以復原。", result.Count, result.TrashRetentionDays)
		}
	default:
		content = "未知的 Kuro 指令。\n\n" + kuroTextCommandHelp
	}

	if err != nil {
		content = "操作失敗：" + err.Error()
	}
	sendKuroMessage(session, event.ChannelID, content)
}

func textCommandPage(args []string) (int, bool) {
	if len(args) == 0 {
		return 1, true
	}
	if len(args) != 1 {
		return 0, false
	}
	value, err := strconv.Atoi(args[0])
	if err != nil || value < 1 || value > 100000 {
		return 0, false
	}
	return value, true
}

func textCommandConfirmed(value string) bool {
	return strings.EqualFold(value, "confirm")
}

func formatKuroMemories(result servicekuro.MemoryResponse, trash bool, page, pageSize int) string {
	total := result.Count
	minimumTotal := (page-1)*pageSize + len(result.Memories)
	if total < minimumTotal {
		total = minimumTotal
	}
	if len(result.Memories) == 0 {
		if total > 0 {
			totalPages := (total + pageSize - 1) / pageSize
			return fmt.Sprintf("頁碼超出範圍；目前共有 %d 條記憶、%d 頁。", total, totalPages)
		}
		if trash {
			return "記憶垃圾桶是空的。"
		}
		return "目前沒有有效共享記憶。"
	}
	totalPages := (total + pageSize - 1) / pageSize
	label := "共享記憶"
	commandName := "memory-list"
	if trash {
		label = "記憶垃圾桶"
		commandName = "memory-trash"
	}
	lines := []string{fmt.Sprintf("%s（第 %d/%d 頁，共 %d 條）：", label, page, totalPages, total)}
	for _, memory := range result.Memories {
		id := memory.ID
		if len(id) > 8 {
			id = id[:8]
		}
		participantNames := make([]string, 0, len(memory.Participants))
		for _, participant := range memory.Participants {
			if name := strings.TrimSpace(participant.DisplayName); name != "" {
				participantNames = append(participantNames, name)
			}
		}
		participants := "未標記"
		if len(participantNames) > 0 {
			participants = truncateKuroText(strings.Join(participantNames, "、"), 60)
		}
		scope := "本頻道"
		if memory.Scope == "global" {
			scope = "跨頻道"
		}
		line := fmt.Sprintf("`%s` [%s｜%s｜%s] %s", id, kuroMemoryCategoryLabel(memory.Category), scope, participants, truncateKuroText(memory.Value, 220))
		if trash && len(memory.PurgeAfter) >= 10 {
			line += " · 永久清除：" + memory.PurgeAfter[:10]
		}
		lines = append(lines, line)
	}
	lines = append(lines, fmt.Sprintf("使用 `小黑 /%s <頁碼>` 切換頁面。", commandName))
	return truncateKuroText(strings.Join(lines, "\n"), 1900)
}

func kuroMemoryCategoryLabel(category string) string {
	labels := map[string]string{
		"conversation_event":     "對話事件",
		"user_preference":        "使用者偏好",
		"plan_task":              "計畫／待辦",
		"relationship_milestone": "關係里程碑",
		"decision":               "共同決定",
		"summary":                "階段摘要",
	}
	if label := labels[category]; label != "" {
		return label
	}
	return category
}

func formatKuroMemoryAction(result servicekuro.MemoryResponse, success string) string {
	if result.Status == "deleted" || result.Status == "restored" {
		id := ""
		if result.Memory != nil {
			id = result.Memory.ID
			if len(id) > 8 {
				id = id[:8]
			}
		}
		return fmt.Sprintf("記憶 %s %s。", id, success)
	}
	if result.Status == "conflict" {
		return "同一事件欄位已有有效記憶，目前無法復原。"
	}
	if result.Status == "ambiguous" {
		return "ID 前綴符合多條記憶，請輸入更多字元。"
	}
	return "找不到相符的記憶。"
}

func truncateKuroText(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	if max <= 1 {
		return string(runes[:max])
	}
	return string(runes[:max-1]) + "…"
}

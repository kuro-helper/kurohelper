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
小黑 /ai-stats [24h|7d|30d] — 查看 AI 延遲、Token 與費用統計
小黑 /memory-list [頁碼] — 分頁列出有效記憶
小黑 /memory-info <記憶ID> — 查看單筆記憶的詳細資訊
小黑 /memory-trash [頁碼] — 分頁列出記憶垃圾桶
小黑 /forget <記憶ID> — 將記憶移入垃圾桶
小黑 /restore <記憶ID> — 復原記憶
小黑 /memory-clear confirm — 將所有有效記憶移入垃圾桶
小黑 /memory-backups [頁碼] — 分頁列出整庫備份
小黑 /memory-backup — 立即建立整庫備份
小黑 /memory-rollback <備份ID> confirm — 將整個記憶庫復原到指定備份`

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

	if command.Name == "ai-stats" {
		period, label, valid := kuroAIStatsPeriod(command.Args)
		if !valid {
			sendKuroMessage(session, event.ChannelID, "用法：小黑 /ai-stats [24h|7d|30d]")
			return
		}
		since := time.Now().Add(-period)
		stats, err := db.GetKuroAIStats(db.Dbs, since)
		if err != nil {
			sendKuroMessage(session, event.ChannelID, "讀取 AI 統計失敗，請稍後再試。")
			return
		}
		providers, err := db.GetKuroAIProviderStats(db.Dbs, since)
		if err != nil {
			sendKuroMessage(session, event.ChannelID, "讀取 AI 供應商統計失敗，請稍後再試。")
			return
		}
		sendKuroMessage(session, event.ChannelID, formatKuroAIStats(stats, providers, label))
		return
	}

	client := botkuro.Client()
	if client == nil || !client.Connected() {
		sendKuroMessage(session, event.ChannelID, "Kuro AI Runtime 目前未連線。")
		return
	}
	if command.Name == "forget" || command.Name == "restore" || command.Name == "memory-clear" || command.Name == "memory-backup" || command.Name == "memory-rollback" {
		botkuro.LockGeneration()
		defer botkuro.UnlockGeneration()
	}

	timeout := 15 * time.Second
	if command.Name == "memory-rollback" {
		timeout = 2 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
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
	case "memory-info":
		if len(command.Args) != 1 || len(command.Args[0]) < 6 {
			content = "用法：小黑 /memory-info <記憶ID>"
			break
		}
		var result servicekuro.MemoryResponse
		result, err = client.GetMemory(ctx, command.Args[0])
		if err == nil {
			content = formatKuroMemoryDetail(result)
			if result.Status == "found" && result.Memory != nil {
				content = appendKuroMemorySource(content, loadKuroMemorySource(session, result.Memory))
			}
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
	case "memory-backups":
		page, valid := textCommandPage(command.Args)
		if !valid {
			content = "用法：小黑 /memory-backups [頁碼]"
			break
		}
		var result servicekuro.MemoryResponse
		result, err = client.ListMemoryBackups(ctx, kuroMemoryPageSize, (page-1)*kuroMemoryPageSize)
		if err == nil {
			if result.Status == "unavailable" || result.Status == "disabled" {
				content = "記憶備份服務目前不可用。"
			} else {
				content = formatKuroMemoryBackups(result, page, kuroMemoryPageSize)
			}
		}
	case "memory-backup":
		if len(command.Args) != 0 {
			content = "用法：小黑 /memory-backup"
			break
		}
		var result servicekuro.MemoryResponse
		result, err = client.CreateMemoryBackup(ctx)
		if err == nil {
			if result.Backup != nil {
				content = fmt.Sprintf("已建立記憶備份 `%s`（%d 條記憶）。", result.Backup.ID, result.Backup.MemoryCount)
			} else {
				content = "記憶備份服務目前不可用。"
			}
		}
	case "memory-rollback":
		if len(command.Args) != 2 || len(command.Args[0]) < 8 || !textCommandConfirmed(command.Args[1]) {
			content = "這會以備份取代整個記憶庫。若要執行，請輸入：小黑 /memory-rollback <備份ID> confirm"
			break
		}
		var result servicekuro.MemoryResponse
		result, err = client.RestoreMemoryBackup(ctx, command.Args[0])
		if err == nil {
			content = formatKuroBackupRestore(result)
		}
	default:
		content = "未知的 Kuro 指令。\n\n" + kuroTextCommandHelp
	}

	if err != nil {
		content = "操作失敗：" + err.Error()
	}
	sendKuroMessage(session, event.ChannelID, content)
}

func formatKuroMemoryBackups(result servicekuro.MemoryResponse, page, pageSize int) string {
	total := result.Count
	minimumTotal := (page-1)*pageSize + len(result.Backups)
	if total < minimumTotal {
		total = minimumTotal
	}
	if len(result.Backups) == 0 {
		if total > 0 {
			totalPages := (total + pageSize - 1) / pageSize
			return fmt.Sprintf("頁碼超出範圍；目前共有 %d 份備份、%d 頁。", total, totalPages)
		}
		return "目前沒有記憶備份。"
	}
	totalPages := (total + pageSize - 1) / pageSize
	lines := []string{fmt.Sprintf("記憶備份（第 %d/%d 頁，共 %d 份；最多保留 %d 份）：", page, totalPages, total, result.BackupRetentionCount)}
	for _, backup := range result.Backups {
		createdAt := backup.CreatedAt
		if parsed, parseErr := time.Parse(time.RFC3339, backup.CreatedAt); parseErr == nil {
			createdAt = parsed.Local().Format("2006-01-02 15:04")
		}
		lines = append(lines, fmt.Sprintf("`%s` [%s｜%s] %d 條、%.1f KiB", backup.ID, createdAt, kuroBackupReasonLabel(backup.Reason), backup.MemoryCount, float64(backup.SizeBytes)/1024))
	}
	lines = append(lines, "使用 `小黑 /memory-backups <頁碼>` 切換頁面。")
	return truncateKuroText(strings.Join(lines, "\n"), 1900)
}

func kuroBackupReasonLabel(reason string) string {
	labels := map[string]string{
		"auto":        "定時",
		"startup":     "啟動",
		"manual":      "手動",
		"pre-restore": "復原前安全備份",
	}
	if label := labels[reason]; label != "" {
		return label
	}
	return reason
}

func formatKuroBackupRestore(result servicekuro.MemoryResponse) string {
	if result.Status == "restored_backup" && result.Backup != nil {
		safetyID := ""
		if result.SafetyBackup != nil {
			safetyID = fmt.Sprintf("；操作前狀態另存為 `%s`", result.SafetyBackup.ID)
		}
		return fmt.Sprintf("已從備份 `%s` 復原，共重建 %d 條有效記憶%s。", result.Backup.ID, result.RestoredActiveCount, safetyID)
	}
	if result.Status == "ambiguous" {
		return "備份 ID 前綴符合多份備份，請輸入更多字元。"
	}
	if result.Status == "unavailable" || result.Status == "disabled" {
		return "記憶備份服務目前不可用。"
	}
	return "找不到相符的記憶備份。"
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

func kuroAIStatsPeriod(args []string) (time.Duration, string, bool) {
	if len(args) == 0 {
		return 24 * time.Hour, "最近 24 小時", true
	}
	if len(args) != 1 {
		return 0, "", false
	}
	switch strings.ToLower(args[0]) {
	case "24h":
		return 24 * time.Hour, "最近 24 小時", true
	case "7d":
		return 7 * 24 * time.Hour, "最近 7 天", true
	case "30d":
		return 30 * 24 * time.Hour, "最近 30 天", true
	default:
		return 0, "", false
	}
}

func formatKuroAIStats(stats db.KuroAIStats, providers []db.KuroAIProviderStats, label string) string {
	if stats.RequestCount == 0 {
		return fmt.Sprintf("AI 統計（%s）\n目前還沒有生成紀錄。", label)
	}
	usage := fmt.Sprintf("Token：尚未取得供應商用量（0/%d 筆）", stats.RequestCount)
	if stats.UsageCount > 0 {
		usage = fmt.Sprintf(
			"Token：輸入 %d／輸出 %d／合計 %d（推理 %d、快取 %d）\n費用：US$ %.6f（有精確用量 %d/%d 筆）",
			stats.PromptTokens, stats.CompletionTokens, stats.TotalTokens,
			stats.ReasoningTokens, stats.CachedTokens, stats.CostUSD,
			stats.UsageCount, stats.RequestCount,
		)
	}
	result := fmt.Sprintf(
		"AI 統計（%s）\n請求：%d（成功 %d、失敗 %d、重試 %d）\n成功回覆端到端：平均 %.2fs／P50 %.2fs／P95 %.2fs\n成功回覆 AI Runtime：平均 %.2fs；供應商：平均 %.2fs\n%s",
		label, stats.RequestCount, stats.SuccessCount, stats.FailureCount, stats.RetryCount,
		stats.AverageEndToEndMs/1000, stats.P50EndToEndMs/1000, stats.P95EndToEndMs/1000,
		stats.AverageRuntimeMs/1000, stats.AverageProviderMs/1000, usage,
	)
	if len(providers) == 0 {
		return result
	}
	lines := []string{"實際供應商："}
	for index, provider := range providers {
		if index >= 5 {
			lines = append(lines, fmt.Sprintf("另有 %d 家供應商。", len(providers)-index))
			break
		}
		lines = append(lines, fmt.Sprintf(
			"• %s：%d 次（成功 %d、失敗 %d），首 Token 平均 %.2fs，總耗時平均 %.2fs／P95 %.2fs",
			provider.Provider, provider.RequestCount, provider.SuccessCount, provider.FailureCount,
			provider.AverageFirstTokenMs/1000, provider.AverageDurationMs/1000, provider.P95DurationMs/1000,
		))
	}
	return truncateKuroText(result+"\n"+strings.Join(lines, "\n"), 1900)
}

func formatKuroMemoryDetail(result servicekuro.MemoryResponse) string {
	if result.Status == "ambiguous" {
		return "ID 前綴符合多條記憶，請輸入更多字元。"
	}
	if result.Status == "unavailable" || result.Status == "disabled" {
		return "記憶服務目前不可用。"
	}
	if result.Status != "found" || result.Memory == nil {
		return "找不到相符的記憶。"
	}

	memory := result.Memory
	statusLabels := map[string]string{
		"active":     "有效",
		"deleted":    "垃圾桶",
		"forgotten":  "已遺忘",
		"superseded": "已被新版取代",
	}
	status := statusLabels[memory.Status]
	if status == "" {
		status = memory.Status
	}
	scope := "頻道"
	if memory.Scope == "global" {
		scope = "跨頻道"
	}
	participantNames := make([]string, 0, len(memory.Participants))
	for _, participant := range memory.Participants {
		name := strings.TrimSpace(participant.DisplayName)
		if name == "" {
			name = strings.TrimSpace(participant.ID)
		}
		if participant.Role != "" {
			name += "（" + participant.Role + "）"
		}
		if name != "" {
			participantNames = append(participantNames, name)
		}
	}
	participants := "無"
	if len(participantNames) > 0 {
		participants = strings.Join(participantNames, "、")
	}

	lines := []string{
		"記憶詳細資訊",
		fmt.Sprintf("ID：`%s`", memory.ID),
		fmt.Sprintf("狀態：%s", status),
		fmt.Sprintf("分類：%s（%s）", kuroMemoryCategoryLabel(memory.Category), memory.Category),
		fmt.Sprintf("內容：%s", memory.Value),
		fmt.Sprintf("鍵：`%s`", memory.Key),
		fmt.Sprintf("重要性／信心：%.2f／%.2f", memory.Importance, memory.Confidence),
		fmt.Sprintf("作用域：%s `%s`", scope, memory.ScopeID),
		fmt.Sprintf("參與者：%s", participants),
		fmt.Sprintf("建立／更新：%s／%s", formatKuroMemoryTime(memory.CreatedAt), formatKuroMemoryTime(memory.UpdatedAt)),
		fmt.Sprintf("最後檢索：%s（%d 次）", formatKuroMemoryTime(memory.LastAccessedAt), memory.AccessCount),
	}
	if memory.SourceChannelID != "" || memory.SourceRequestID != "" {
		lines = append(lines, fmt.Sprintf("來源：頻道 `%s`／請求 `%s`", memory.SourceChannelID, memory.SourceRequestID))
	}
	if memory.SupersedesID != "" {
		lines = append(lines, fmt.Sprintf("取代舊記憶：`%s`", memory.SupersedesID))
	}
	if memory.PurgeAfter != "" {
		lines = append(lines, fmt.Sprintf("預計永久清除：%s", formatKuroMemoryTime(memory.PurgeAfter)))
	}
	return truncateKuroText(strings.Join(lines, "\n"), 1900)
}

func formatKuroMemoryTime(value string) string {
	if strings.TrimSpace(value) == "" {
		return "尚未發生"
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return value
	}
	return parsed.Local().Format("2006-01-02 15:04:05")
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

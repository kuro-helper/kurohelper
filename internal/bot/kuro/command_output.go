package kuro

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	servicekuro "kurohelperservice/airuntime"
	kurosvc "kurohelperservice/kuro"
)

func formatKuroRawReplies(result servicekuro.RawRepliesResponse) []string {
	if len(result.Entries) == 0 {
		return []string{"目前沒有快取的模型原始回覆。"}
	}
	entries := result.Entries
	if len(entries) > 5 {
		entries = entries[len(entries)-5:]
	}
	blocks := make([]string, 0, len(entries)+1)
	blocks = append(blocks, fmt.Sprintf("最近 %d 則模型原始回覆（最新在前）：", len(entries)))
	for index := len(entries) - 1; index >= 0; index-- {
		entry := entries[index]
		number := len(entries) - index
		cachedAt := formatKuroMemoryTime(entry.CachedAt)
		source := "主模型"
		if strings.EqualFold(strings.TrimSpace(entry.Source), "vision") {
			source = "Vision"
		}
		blocks = append(blocks, fmt.Sprintf("#%d｜%s｜%s\n%s", number, source, cachedAt, entry.RawText))
	}
	return splitKuroText(strings.Join(blocks, "\n\n──────────\n\n"), 1900)
}

func splitKuroText(value string, max int) []string {
	if max < 1 {
		return nil
	}
	runes := []rune(value)
	chunks := make([]string, 0, (len(runes)+max-1)/max)
	for len(runes) > 0 {
		length := max
		if len(runes) < length {
			length = len(runes)
		}
		chunks = append(chunks, string(runes[:length]))
		runes = runes[length:]
	}
	return chunks
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

func formatKuroRuntimeStatus(health servicekuro.HealthResponse) string {
	result := fmt.Sprintf(
		"Runtime：%s\nSillyTavern：%t\n長期記憶：%t",
		health.Status,
		health.SillyTavernReady,
		health.MemoryEnabled,
	)
	cache := health.VisionCache
	if cache == nil {
		return result
	}
	mode := "記憶體"
	if cache.Persistent {
		mode = "持久化"
	}
	return fmt.Sprintf(
		"%s\nVision 快取：%s／%s，%d/%d 張圖片、%d 筆（OCR %d、觀察 %d）\n快取命中：%d/%d（%.1f%%）；淘汰圖片 %d、項目 %d",
		result,
		map[bool]string{true: "啟用", false: "停用"}[cache.Enabled],
		mode,
		cache.Images,
		cache.MaxImages,
		cache.Entries,
		cache.OCREntries,
		cache.ObservationEntries,
		cache.Hits,
		cache.Hits+cache.Misses,
		cache.HitRate*100,
		cache.EvictedImages,
		cache.EvictedEntries,
	)
}

func formatKuroAIStats(stats kurosvc.AIStats, providers []kurosvc.AIProviderStats, label string) string {
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
	memoryUsage := fmt.Sprintf(
		"記憶擷取：%d 次，輸入 %d／輸出 %d／合計 %d Token，US$ %.6f",
		stats.MemoryExtractionCount,
		stats.MemoryExtractionPromptTokens,
		stats.MemoryExtractionCompletionTokens,
		stats.MemoryExtractionTotalTokens,
		stats.MemoryExtractionCostUSD,
	)
	visionUsage := fmt.Sprintf(
		"Vision：%d 次，輸入 %d／輸出 %d／合計 %d Token，US$ %.6f",
		stats.VisionGenerationCount,
		stats.VisionPromptTokens,
		stats.VisionCompletionTokens,
		stats.VisionTotalTokens,
		stats.VisionCostUSD,
	)
	result := fmt.Sprintf(
		"AI 統計（%s）\n請求：%d（成功 %d、失敗 %d、重試 %d）\n成功回覆端到端：平均 %.2fs／P50 %.2fs／P95 %.2fs\n成功回覆 AI Runtime：平均 %.2fs；供應商：平均 %.2fs\n%s\n%s\n%s",
		label, stats.RequestCount, stats.SuccessCount, stats.FailureCount, stats.RetryCount,
		stats.AverageEndToEndMs/1000, stats.P50EndToEndMs/1000, stats.P95EndToEndMs/1000,
		stats.AverageRuntimeMs/1000, stats.AverageProviderMs/1000, usage, memoryUsage, visionUsage,
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
		"pending":    "等待衝突確認",
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
	if memory.ConflictMemoryID != "" {
		lines = append(lines,
			fmt.Sprintf("衝突記憶：`%s`（%s，相似度 %.2f，%d 份證據）",
				memory.ConflictMemoryID, memory.ConflictType, memory.ConflictSimilarity, memory.EvidenceCount))
	}
	if memory.ResolutionNote != "" {
		lines = append(lines, fmt.Sprintf("衝突處理：%s", memory.ResolutionNote))
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

func formatKuroPendingMemories(result servicekuro.MemoryResponse, page, pageSize int) string {
	total := result.Count
	minimumTotal := (page-1)*pageSize + len(result.Memories)
	if total < minimumTotal {
		total = minimumTotal
	}
	if len(result.Memories) == 0 {
		if total > 0 {
			return fmt.Sprintf("頁碼超出範圍；目前共有 %d 條待確認衝突。", total)
		}
		return "目前沒有等待確認的記憶衝突。"
	}
	totalPages := (total + pageSize - 1) / pageSize
	lines := []string{fmt.Sprintf("待確認記憶衝突（第 %d/%d 頁，共 %d 條）：", page, totalPages, total)}
	for _, memory := range result.Memories {
		id := memory.ID
		if len(id) > 8 {
			id = id[:8]
		}
		conflictID := memory.ConflictMemoryID
		if len(conflictID) > 8 {
			conflictID = conflictID[:8]
		}
		lines = append(lines, fmt.Sprintf(
			"`%s` ↔ `%s` [相似度 %.2f｜證據 %d] %s",
			id, conflictID, memory.ConflictSimilarity, memory.EvidenceCount,
			truncateKuroText(memory.Value, 190),
		))
	}
	lines = append(lines,
		"使用 `小黑 /memory-info <記憶ID>` 查看內容；",
		"再用 `小黑 /memory-resolve <記憶ID> <keep-new|keep-old|coexist>` 處理。",
	)
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

func formatKuroMemoryResolution(result servicekuro.MemoryResponse) string {
	if result.Status == "resolved" {
		labels := map[string]string{
			"keep_new": "已採用新記憶，舊記憶標為已取代",
			"keep_old": "已保留舊記憶，候選記憶不再生效",
			"coexist":  "已允許兩條記憶共存",
		}
		return labels[result.Resolution] + "。"
	}
	if result.Status == "stale_conflict" {
		return "原本衝突的有效記憶已變更，請重新檢查這筆候選記憶。"
	}
	if result.Status == "ambiguous" {
		return "ID 前綴符合多條候選記憶，請輸入更多字元。"
	}
	return "找不到相符的待確認記憶。"
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

package kuro

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	servicekuro "kurohelperservice/airuntime"
	kurosvc "kurohelperservice/kuro"
)

const kuroMemoryPageSize = 5

const kuroNewChatConfirmation = "已開始新的短期對話；長期記憶不會被刪除。"

const kuroUnknownCommandMessage = "未知的 Kuro 指令。"

const kuroTextCommandHelp = `Kuro 可用指令：
小黑 /help — 列出這份指令說明
小黑 /newchat — 開始新的短期對話
小黑 /status — 查看 AI Runtime 狀態
小黑 /ai-stats [24h|7d|30d] — 查看 AI 延遲、Token 與費用統計
小黑 /raw-responses [頻道ID] — 查看指定頻道最近五則模型原始回覆
小黑 /channel-list [群組ID] — 列出群組名稱、群組ID、頻道名稱與頻道ID
小黑 /channel-add <頻道ID> — 啟用指定頻道的 Kuro 對話
小黑 /channel-remove <頻道ID> — 停用指定頻道的 Kuro 對話
小黑 /guild-disable <群組ID> — 停用指定群組的 Kuro 對話
小黑 /guild-enable <群組ID> — 恢復指定群組的 Kuro 對話
小黑 /memory-list [頁碼] — 分頁列出有效記憶
小黑 /memory-pending [頁碼] — 列出等待確認的衝突記憶
小黑 /memory-resolve <記憶ID> <keep-new|keep-old|coexist> — 解決記憶衝突
小黑 /memory-info <記憶ID> — 查看單筆記憶的詳細資訊
小黑 /memory-trash [頁碼] — 分頁列出記憶垃圾桶
小黑 /forget <記憶ID> — 將記憶移入垃圾桶
小黑 /restore <記憶ID> — 復原記憶
小黑 /memory-clear confirm — 將所有有效記憶移入垃圾桶
小黑 /memory-backups [頁碼] — 分頁列出整庫備份
小黑 /memory-backup — 立即建立整庫備份
小黑 /memory-rollback <備份ID> confirm — 將整個記憶庫復原到指定備份`

func handleKuroTextCommand(session *discordgo.Session, event *discordgo.MessageCreate, command kuroTextCommand) {
	if !CommandAllowed(event.Author.ID) {
		sendKuroCommandMessage(session, event.ChannelID, "你沒有使用 Kuro 管理指令的權限。")
		return
	}

	if command.Name == "help" {
		sendKuroCommandMessage(session, event.ChannelID, kuroTextCommandHelp)
		return
	}

	if command.Name == "newchat" {
		if len(command.Args) != 0 {
			sendKuroCommandMessage(session, event.ChannelID, "用法：小黑 /newchat")
			return
		}
		unlockGeneration := LockChannelGeneration(event.ChannelID)
		defer unlockGeneration()
		if err := kurosvc.SetContextBoundary(event.ChannelID, event.ID); err != nil {
			sendKuroCommandMessage(session, event.ChannelID, "建立新對話失敗，請稍後再試。")
			return
		}
		confirmation, sendErr := sendKuroCommandMessage(session, event.ChannelID, kuroNewChatConfirmation)
		if sendErr == nil && confirmation != nil {
			if err := kurosvc.SetContextBoundary(event.ChannelID, confirmation.ID); err != nil {
				slog.Warn("更新 Kuro 新對話確認訊息邊界失敗", "error", err, "channelID", event.ChannelID)
			}
		}
		return
	}

	if command.Name == "ai-stats" {
		period, label, valid := kuroAIStatsPeriod(command.Args)
		if !valid {
			sendKuroCommandMessage(session, event.ChannelID, "用法：小黑 /ai-stats [24h|7d|30d]")
			return
		}
		since := time.Now().Add(-period)
		stats, providers, err := kurosvc.GetAIStats(since)
		if err != nil {
			sendKuroCommandMessage(session, event.ChannelID, "讀取 AI 統計失敗，請稍後再試。")
			return
		}
		sendKuroCommandMessage(session, event.ChannelID, formatKuroAIStats(stats, providers, label))
		return
	}

	if handleKuroAccessCommand(session, event, command) {
		return
	}

	client := Client()
	if client == nil || !client.Connected() {
		sendKuroCommandMessage(session, event.ChannelID, "Kuro AI Runtime 目前未連線。")
		return
	}
	if command.Name == "forget" || command.Name == "restore" || command.Name == "memory-resolve" || command.Name == "memory-clear" || command.Name == "memory-backup" || command.Name == "memory-rollback" {
		unlockGeneration := LockAllGenerations()
		defer unlockGeneration()
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
			content = formatKuroRuntimeStatus(health)
		}
	case "raw-responses":
		if len(command.Args) > 1 || (len(command.Args) == 1 && !validDiscordID(command.Args[0])) {
			content = "用法：小黑 /raw-responses [頻道ID]"
			break
		}
		targetChannelID := event.ChannelID
		if len(command.Args) == 1 {
			targetChannelID = command.Args[0]
			if channel, channelErr := session.Channel(targetChannelID); channelErr != nil || channel == nil {
				content = "找不到 Bot 可存取的 Discord 頻道。"
				break
			}
		}
		var result servicekuro.RawRepliesResponse
		result, err = client.ListRawReplies(ctx, targetChannelID)
		if err == nil {
			for _, message := range formatKuroRawReplies(result) {
				sendKuroCommandMessage(session, event.ChannelID, message)
			}
			return
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
	case "memory-pending":
		page, valid := textCommandPage(command.Args)
		if !valid {
			content = "用法：小黑 /memory-pending [頁碼]"
			break
		}
		var result servicekuro.MemoryResponse
		result, err = client.ListMemories(ctx, "pending", kuroMemoryPageSize, (page-1)*kuroMemoryPageSize)
		if err == nil {
			content = formatKuroPendingMemories(result, page, kuroMemoryPageSize)
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
	case "memory-resolve":
		if len(command.Args) != 2 || len(command.Args[0]) < 6 {
			content = "用法：小黑 /memory-resolve <記憶ID> <keep-new|keep-old|coexist>"
			break
		}
		resolution := strings.ReplaceAll(strings.ToLower(command.Args[1]), "-", "_")
		if resolution != "keep_new" && resolution != "keep_old" && resolution != "coexist" {
			content = "處理方式只能是 keep-new、keep-old 或 coexist。"
			break
		}
		var result servicekuro.MemoryResponse
		result, err = client.ResolveMemory(ctx, command.Args[0], resolution)
		if err == nil {
			content = formatKuroMemoryResolution(result)
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
		content = kuroUnknownCommandMessage
	}

	if err != nil {
		content = "操作失敗：" + err.Error()
	}
	sendKuroCommandMessage(session, event.ChannelID, content)
}

func sendKuroCommandMessage(session *discordgo.Session, channelID, content string) (*discordgo.Message, error) {
	message, err := sendKuroMessage(session, channelID, content)
	if err != nil || message == nil {
		return message, err
	}
	if recordErr := kurosvc.RecordCommandResponse(channelID, message.ID); recordErr != nil {
		slog.Warn(
			"記錄 Kuro 指令回覆的上下文分類失敗",
			"error", recordErr,
			"channelID", channelID,
			"messageID", message.ID,
		)
	}
	return message, nil
}

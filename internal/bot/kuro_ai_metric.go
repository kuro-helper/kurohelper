package bot

import (
	"log/slog"
	"time"

	"github.com/bwmarrin/discordgo"

	"kurohelperservice/db"
	servicekuro "kurohelperservice/kuro"
)

func elapsedMilliseconds(since time.Time) int64 {
	value := time.Since(since).Milliseconds()
	if value < 0 {
		return 0
	}
	return value
}

func recordKuroGenerationMetric(
	event *discordgo.MessageCreate,
	acceptedAt time.Time,
	status string,
	metrics *servicekuro.GenerationMetrics,
	botQueueMs, discordHistoryMs, runtimeRoundTripMs, discordSendMs int64,
	sendErr error,
) {
	if event == nil || event.Message == nil {
		return
	}
	if metrics != nil && metrics.Status != "" {
		status = metrics.Status
	}
	if sendErr != nil {
		status = "error"
	}
	record := &db.KuroAIMetric{
		RequestID: event.ID, ChannelID: event.ChannelID, CreatedAt: acceptedAt,
		Status: status, BotQueueMs: botQueueMs, DiscordHistoryMs: discordHistoryMs,
		RuntimeRoundTripMs: runtimeRoundTripMs, DiscordSendMs: discordSendMs,
		EndToEndMs: time.Since(acceptedAt).Milliseconds(),
	}
	if metrics != nil {
		record.Model = metrics.Model
		record.Provider = metrics.Provider
		record.ProviderModel = metrics.ProviderModel
		record.RoutingStrategy = metrics.RoutingStrategy
		record.RoutingRegion = metrics.RoutingRegion
		record.RoutingAttempt = metrics.RoutingAttempt
		record.ProviderStatusCode = metrics.ProviderStatusCode
		record.UsageAvailable = metrics.UsageAvailable
		record.PromptTokens = metrics.PromptTokens
		record.CompletionTokens = metrics.CompletionTokens
		record.TotalTokens = metrics.TotalTokens
		record.ReasoningTokens = metrics.ReasoningTokens
		record.CachedTokens = metrics.CachedTokens
		record.CostUSD = metrics.CostUSD
		record.GenerationCount = metrics.GenerationCount
		record.RetryCount = metrics.RetryCount
		record.MemoryRecallMs = metrics.MemoryRecallMs
		record.BrowserQueueMs = metrics.BrowserQueueMs
		record.LocaleMs = metrics.LocaleMs
		record.PersonaMs = metrics.PersonaMs
		record.InjectUserMs = metrics.InjectUserMs
		record.GenerateMs = metrics.GenerateMs
		record.ValidateMs = metrics.ValidateMs
		record.RetryGenerateMs = metrics.RetryGenerateMs
		record.RetryValidateMs = metrics.RetryValidateMs
		record.BrowserTotalMs = metrics.BrowserTotalMs
		record.ProviderHeadersMs = metrics.ProviderHeadersMs
		record.ProviderFirstTokenMs = metrics.ProviderFirstTokenMs
		record.ProviderDurationMs = metrics.ProviderDurationMs
	}
	if record.EndToEndMs < 0 {
		record.EndToEndMs = 0
	}
	if err := db.RecordKuroAIMetric(db.Dbs, record); err != nil {
		slog.Warn("儲存 Kuro AI 指標失敗", "error", err, "requestID", event.ID)
	}
}

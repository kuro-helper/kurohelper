package kuro

import (
	"log/slog"
	"time"

	"github.com/bwmarrin/discordgo"

	"kurohelperservice/airuntime"
	kurosvc "kurohelperservice/kuro"
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
	metrics *airuntime.GenerationMetrics,
	botQueueMs, discordHistoryMs, runtimeRoundTripMs, discordSendMs int64,
	sendErr error,
) {
	if event == nil || event.Message == nil {
		return
	}
	if sendErr != nil {
		status = "error"
	}
	if err := kurosvc.RecordGenerationMetric(kurosvc.GenerationMetricRecord{
		RequestID: event.ID, ChannelID: event.ChannelID, CreatedAt: acceptedAt,
		Status: status, Metrics: metrics, BotQueueMs: botQueueMs, DiscordHistoryMs: discordHistoryMs,
		RuntimeRoundTripMs: runtimeRoundTripMs, DiscordSendMs: discordSendMs,
		EndToEndMs: time.Since(acceptedAt).Milliseconds(),
	}); err != nil {
		slog.Warn("儲存 Kuro AI 指標失敗", "error", err, "requestID", event.ID)
	}
}

// RecordKuroRuntimeMetric persists background AI operations that finish after
// the Discord generation response, currently memory extraction.
func RecordKuroRuntimeMetric(event airuntime.MetricEvent) {
	if err := kurosvc.RecordRuntimeMetric(event); err != nil {
		slog.Warn("記錄 Kuro 背景 AI 統計失敗", "error", err, "requestID", event.RequestID, "operation", event.Operation)
	}
}

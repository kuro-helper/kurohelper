package bot

import (
	"strings"
	"testing"
	"time"

	"kurohelperservice/db"
)

func TestKuroAIStatsPeriod(t *testing.T) {
	duration, label, valid := kuroAIStatsPeriod([]string{"7d"})
	if !valid || duration != 7*24*time.Hour || label != "最近 7 天" {
		t.Fatalf("unexpected period: %v %q %v", duration, label, valid)
	}
	if _, _, valid := kuroAIStatsPeriod([]string{"31d"}); valid {
		t.Fatal("31d should be rejected")
	}
}

func TestFormatKuroAIStats(t *testing.T) {
	result := formatKuroAIStats(db.KuroAIStats{
		RequestCount: 10, SuccessCount: 9, FailureCount: 1, RetryCount: 2,
		UsageCount: 8, PromptTokens: 1000, CompletionTokens: 200, TotalTokens: 1200,
		CostUSD: 0.0123, AverageEndToEndMs: 2500, P50EndToEndMs: 2000,
		P95EndToEndMs: 5000, AverageRuntimeMs: 2200, AverageProviderMs: 1800,
	}, []db.KuroAIProviderStats{{
		Provider: "Provider B", RequestCount: 7, SuccessCount: 6, FailureCount: 1,
		AverageFirstTokenMs: 1200, AverageDurationMs: 2500, P95DurationMs: 5000,
	}}, "最近 24 小時")
	for _, expected := range []string{"成功 9", "成功回覆端到端", "P95 5.00s", "US$ 0.012300", "合計 1200", "Provider B", "首 Token 平均 1.20s"} {
		if !strings.Contains(result, expected) {
			t.Fatalf("expected %q in %q", expected, result)
		}
	}
}

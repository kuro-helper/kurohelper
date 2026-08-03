package kuro

import (
	"log/slog"
	"time"

	kurosvc "kurohelperservice/kuro"
)

const (
	kuroContextMessageCleanupInterval = 7 * 24 * time.Hour
	kuroContextMessageRetention       = 7 * 24 * time.Hour
)

// CleanKuroContextMessagesJob removes expired operational Discord message
// markers on startup and then once every seven days.
func CleanKuroContextMessagesJob(stopChan <-chan struct{}) {
	cleanup := func() {
		deleted, err := kurosvc.DeleteContextMessagesCreatedBefore(time.Now().Add(-kuroContextMessageRetention))
		if err != nil {
			slog.Warn("清理 Kuro 上下文訊息標記失敗", "error", err)
			return
		}
		slog.Info("Kuro 上下文訊息標記清理完成", "deleted", deleted)
	}

	cleanup()
	ticker := time.NewTicker(kuroContextMessageCleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			cleanup()
		case <-stopChan:
			return
		}
	}
}

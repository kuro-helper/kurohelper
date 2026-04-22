package cache

import (
	"fmt"
	"log/slog"
	"time"
)

// 清除Cache排程
//
// 先不檢查快取存活時間，統一全部清除
func CleanCacheJob(minute time.Duration, stopChan <-chan struct{}) {
	slog.Info("CleanCacheJob 正在啟動...")
	ticker := time.NewTicker(minute * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			// 清除Cache
			egsDC, egsC := UserInfoCache.Clean() // 混和型態快取
			slog.Info(fmt.Sprintf("UserInfoCache          快取資料: %d筆/%d筆 (清理/總數)", egsDC, egsC))
			egsDC, egsC = CIDStore.Clean()
			slog.Info(fmt.Sprintf("CIDStore               快取資料: %d筆/%d筆 (清理/總數)", egsDC, egsC))
			egsDC, egsC = ErogsGameListStore.Clean()
			slog.Info(fmt.Sprintf("ErogsGameListStore     快取資料: %d筆/%d筆 (清理/總數)", egsDC, egsC))
			egsDC, egsC = ErogsGameStore.Clean()
			slog.Info(fmt.Sprintf("ErogsGameStore         快取資料: %d筆/%d筆 (清理/總數)", egsDC, egsC))
			egsDC, egsC = ErogsMusicListStore.Clean()
			slog.Info(fmt.Sprintf("ErogsMusicListStore    快取資料: %d筆/%d筆 (清理/總數)", egsDC, egsC))
			egsDC, egsC = ErogsMusicStore.Clean()
			slog.Info(fmt.Sprintf("ErogsMusicStore        快取資料: %d筆/%d筆 (清理/總數)", egsDC, egsC))
			egsDC, egsC = ErogsCreatorListStore.Clean()
			slog.Info(fmt.Sprintf("ErogsCreatorListStore  快取資料: %d筆/%d筆 (清理/總數)", egsDC, egsC))
			egsDC, egsC = ErogsCreatorStore.Clean()
			slog.Info(fmt.Sprintf("ErogsCreatorStore      快取資料: %d筆/%d筆 (清理/總數)", egsDC, egsC))
			egsDC, egsC = ErogsSingerListStore.Clean()
			slog.Info(fmt.Sprintf("ErogsSingerListStore   快取資料: %d筆/%d筆 (清理/總數)", egsDC, egsC))
			egsDC, egsC = ErogsSingerStore.Clean()
			slog.Info(fmt.Sprintf("ErogsSingerStore       快取資料: %d筆/%d筆 (清理/總數)", egsDC, egsC))
			egsDC, egsC = VndbCharacterListStore.Clean()
			slog.Info(fmt.Sprintf("VndbCharacterListStore 快取資料: %d筆/%d筆 (清理/總數)", egsDC, egsC))
			egsDC, egsC = VndbCharacterStore.Clean()
			slog.Info(fmt.Sprintf("VndbCharacterStore     快取資料: %d筆/%d筆 (清理/總數)", egsDC, egsC))
			egsDC, egsC = BangumiCharacterStore.Clean()
			slog.Info(fmt.Sprintf("BangumiCharacterStore  快取資料: %d筆/%d筆 (清理/總數)", egsDC, egsC))

		case <-stopChan:
			slog.Info("CleanCacheJob 正在關閉...")
			return
		}
	}
}

package cache

import (
	"time"

	"github.com/sirupsen/logrus"
)

// 清除Cache排程
//
// 先不檢查快取存活時間，統一全部清除
func CleanCacheJob(minute time.Duration, stopChan <-chan struct{}) {
	logrus.Print("CleanCacheJob 正在啟動...")
	ticker := time.NewTicker(minute * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			// 清除Cache
			logrus.Printf("%d筆SearchCache快取已被清除", SearchCache.Clean())
			logrus.Printf("%d筆UserInfoCache快取已被清除", UserInfoCache.Clean())
			// logrus.Printf("%d筆SubmitDataCache快取已被清除", SubmitDataCache.Clean())
			logrus.Printf("%d筆SearchBrandCache快取已被清除", SearchBrandCache.Clean())

			// 新快取(V2)
			egsDC, egsC := CIDStore.Clean()
			logrus.Printf("CIDStore            快取資料: %d筆/%d筆 (清理/總數)", egsC, egsDC)
			egsDC, egsC = ErogsGameListStore.Clean()
			logrus.Printf("ErogsGameListStore  快取資料: %d筆/%d筆 (清理/總數)", egsC, egsDC)
			egsDC, egsC = ErogsGameStore.Clean()
			logrus.Printf("ErogsGameStore      快取資料: %d筆/%d筆 (清理/總數)", egsC, egsDC)
			egsDC, egsC = ErogsMusicListStore.Clean()
			logrus.Printf("ErogsMusicListStore 快取資料: %d筆/%d筆 (清理/總數)", egsC, egsDC)
			egsDC, egsC = ErogsMusicStore.Clean()
			logrus.Printf("ErogsMusicStore     快取資料: %d筆/%d筆 (清理/總數)", egsC, egsDC)
			egsDC, egsC = ErogsCreatorListStore.Clean()
			logrus.Printf("ErogsCreatorListStore 快取資料: %d筆/%d筆 (清理/總數)", egsC, egsDC)
			egsDC, egsC = ErogsCreatorStore.Clean()
			logrus.Printf("ErogsCreatorStore   快取資料: %d筆/%d筆 (清理/總數)", egsC, egsDC)
		case <-stopChan:
			logrus.Println("CleanCacheJob 正在關閉...")
			return
		}
	}
}

package cache

import "time"

// default
var cacheLostTime = 4 * time.Hour

func InitCacheLostTime(hours int) {
	cacheLostTime = time.Duration(hours) * time.Hour
}

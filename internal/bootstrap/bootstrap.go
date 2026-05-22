package bootstrap

import (
	"kurohelper/internal/cache"
	"kurohelper/internal/store"
	"kurohelper/internal/utils"
	"log/slog"
	"os"
	"strings"
	"time"

	service "kurohelperservice"
	"kurohelperservice/db"
	"kurohelperservice/provider/erogs"
	"kurohelperservice/provider/seiya"
	"kurohelperservice/provider/ymgal"
)

// 基本啟動函式
func BasicInit(stopChan <-chan struct{}) {

	config := db.Config{
		DBOwner:    os.Getenv("DB_OWNER"),
		DBPassword: os.Getenv("DB_PASSWORD"),
		DBName:     os.Getenv("DB_NAME"),
		DBPort:     os.Getenv("DB_PORT"),
	}

	err := db.InitDsn(config)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	db.Migration(db.Dbs) // 選填

	// 將白名單存成快取
	store.InitAllowList()

	// init ZhtwToJp var
	service.InitZhtwToJp()

	seiya.InitSeiyaCorrespond()

	// erogs init
	erogs.InitRateLimit(time.Duration(utils.GetEnvInt("EROGS_RATE_LIMIT_RESET_TIME", 10)))
	erogs.InitErogsGameAutoComplete(os.Getenv("EROGS_GAME_AUTOCOMPLETE_FILE"))
	erogs.InitErogsBrandAutoComplete(os.Getenv("EROGS_BRAND_AUTOCOMPLETE_FILE"))
	erogs.InitErogsMusicAutoComplete(os.Getenv("EROGS_MUSIC_AUTOCOMPLETE_FILE"))

	// seiya init
	err = seiya.Init()
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	// ymgal init
	if strings.EqualFold(os.Getenv("INIT_YMGAL"), "true") {
		err = ymgalInit()
		if err != nil {
			slog.Error(err.Error())
			os.Exit(1)
		}
	}

	store.InitUser()
	// 掛載自動清除快取job
	go cache.CleanCacheJob(time.Duration(utils.GetEnvInt("CLEAN_CACHE_JOB_TIME", 720)), stopChan)
}

// ymgal init
func ymgalInit() error {
	// init config
	ymgal.Init(os.Getenv("YMGAL_ENDPOINT"), os.Getenv("YMGAL_CLIENT_ID"), os.Getenv("YMGAL_CLIENT_SECRET"))

	// init token
	// ymgal init token
	err := ymgal.GetToken()
	if err != nil {
		return err
	}
	return nil
}

package bootstrap

import (
	"kurohelper/cache"
	"kurohelper/store"
	"kurohelper/utils"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/lmittmann/tint"

	"kurohelper-core/erogs"

	"kurohelper-core/seiya"

	corestore "kurohelper-core/store"

	"kurohelper-core/ymgal"

	kurohelperdb "kurohelper-db"
)

func init() {
	// log settings
	w := os.Stderr
	logger := slog.New(
		tint.NewHandler(w, &tint.Options{
			Level:      slog.LevelDebug,
			TimeFormat: time.Stamp,
		}),
	)
	slog.SetDefault(logger)
}

// 基本啟動函式
func BasicInit(stopChan <-chan struct{}) {
	// load .env
	err := godotenv.Load(".env")
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	config := kurohelperdb.Config{
		DBOwner:    os.Getenv("DB_OWNER"),
		DBPassword: os.Getenv("DB_PASSWORD"),
		DBName:     os.Getenv("DB_NAME"),
		DBPort:     os.Getenv("DB_PORT"),
	}

	err = kurohelperdb.InitDsn(config)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	kurohelperdb.Migration(kurohelperdb.Dbs) // 選填

	// 將白名單存成快取
	store.InitAllowList()

	// init ZhtwToJp var
	corestore.InitZhtwToJp()

	corestore.InitSeiyaCorrespond()

	// erogs rate limit init
	erogs.InitRateLimit(time.Duration(utils.GetEnvInt("EROGS_RATE_LIMIT_RESET_TIME", 10)))

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

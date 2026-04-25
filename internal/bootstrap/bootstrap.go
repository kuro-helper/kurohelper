package bootstrap

import (
	"kurohelper/internal/cache"
	"kurohelper/internal/store"
	"kurohelper/internal/utils"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/lmittmann/tint"
	slogmulti "github.com/samber/slog-multi"

	service "kurohelperservice"
	"kurohelperservice/db"
	"kurohelperservice/provider/erogs"
	"kurohelperservice/provider/seiya"
	"kurohelperservice/provider/ymgal"
)

func init() {
	// load .env
	err := godotenv.Load(".env")
	if err != nil {
		panic(err)
	}

	// log settings
	logDir := os.Getenv("LOG_PATH")
	info, err := os.Stat(logDir)
	if os.IsNotExist(err) {
		panic(err)
	}

	if !info.IsDir() {
		panic("path is not a directory")
	}

	// make a no color log
	logFile, err := os.OpenFile(filepath.Join(logDir, "kurohelper-nocolor.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		panic(err)
	}

	standardLogHandler := tint.NewHandler(os.Stdout, &tint.Options{
		Level:      slog.LevelDebug,
		TimeFormat: time.Stamp,
	})

	noColorLogHandler := slog.NewTextHandler(logFile, &slog.HandlerOptions{
		Level: slog.LevelDebug,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.String(slog.TimeKey, a.Value.Time().Format(time.Stamp))
			}
			return a
		},
	})

	logger := slog.New(slogmulti.Fanout(
		standardLogHandler,
		noColorLogHandler,
	))

	slog.SetDefault(logger)
}

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

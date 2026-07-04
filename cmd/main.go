package main

import (
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
	"github.com/lmittmann/tint"
	slogmulti "github.com/samber/slog-multi"

	"kurohelper/internal/bot"
	"kurohelper/internal/cache"
	"kurohelper/internal/store"
	"kurohelper/internal/utils"
	service "kurohelperservice"
	"kurohelperservice/db"
	"kurohelperservice/provider/erogs"
	"kurohelperservice/provider/seiya"
	"kurohelperservice/provider/ymgal"
)

// 專案前置初始化
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

func main() {
	// ----初始化專案作業開始----

	// 資料庫初始化
	dbInit()
	// 初始化白名單存成快取
	store.InitAllowList()
	// init ZhtwToJp var
	service.InitZhtwToJp()
	// 使用者快取初始化
	store.InitUser()
	// 初始化快取時間
	cache.InitCacheLostTime(utils.GetEnvInt("COMMAND_CACHE_LOST_HOURS", 4))
	// Seiya初始化
	seiya.InitSeiyaCorrespond()
	err := seiya.Init()
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	// erogs init
	erogs.InitRateLimit(time.Duration(utils.GetEnvInt("EROGS_RATE_LIMIT_RESET_TIME", 10)))
	erogs.InitErogsGameAutoComplete(os.Getenv("EROGS_GAME_AUTOCOMPLETE_FILE"))
	erogs.InitErogsBrandAutoComplete(os.Getenv("EROGS_BRAND_AUTOCOMPLETE_FILE"))
	erogs.InitErogsMusicAutoComplete(os.Getenv("EROGS_MUSIC_AUTOCOMPLETE_FILE"))
	// ymgal init
	if strings.EqualFold(os.Getenv("INIT_YMGAL"), "true") {
		err = ymgalInit()
		if err != nil {
			slog.Error(err.Error())
			os.Exit(1)
		}
	}

	// ----初始化專案作業結束----

	// 掛載自動清除快取job
	stopChan := make(chan struct{})
	go cache.CleanCacheJob(time.Duration(utils.GetEnvInt("COMMAND_CLEAN_CACHE_JOB_HOURS", 12)), stopChan)

	token := os.Getenv("BOT_TOKEN")
	kuroHelper, err := discordgo.New("Bot " + token)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	kuroHelper.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMembers | discordgo.IntentsGuildMessages | discordgo.IntentsMessageContent

	slog.Info("KuroHelper is now running. Press CTRL+C to exit.")

	kuroHelper.AddHandler(bot.Ready)
	kuroHelper.AddHandler(bot.OnInteraction)

	err = kuroHelper.Open() // websocket connect
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	interruptSignal := <-c
	slog.Debug(interruptSignal.String())

	// 關閉 jobs
	close(stopChan)

	kuroHelper.Close() // websocket disconnect
}

// db init
func dbInit() {
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
	if err := db.Migration(db.Dbs); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
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

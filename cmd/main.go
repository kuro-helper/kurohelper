package main

import (
	"context"
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
	botkuro "kurohelper/internal/kuro"
	"kurohelper/internal/store"
	"kurohelper/internal/utils"
	service "kurohelperservice"
	"kurohelperservice/db"
	servicekuro "kurohelperservice/kuro"
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
	if err := store.InitAllowList(); err != nil {
		slog.Error("初始化 Discord 白名單失敗", "error", err)
		os.Exit(1)
	}
	// init ZhtwToJp var
	if err := service.InitZhtwToJp(); err != nil {
		slog.Error("初始化繁日漢字轉換表失敗", "error", err)
		os.Exit(1)
	}
	// 使用者快取初始化
	if err := store.InitUser(); err != nil {
		slog.Error("初始化使用者快取失敗", "error", err)
		os.Exit(1)
	}
	// 初始化快取時間
	cache.InitCacheLostTime(utils.GetEnvInt("COMMAND_CACHE_LOST_HOURS", 4))
	// Seiya初始化
	if err := seiya.InitSeiyaCorrespond(); err != nil {
		slog.Error("初始化 Seiya 對應表失敗", "error", err)
		os.Exit(1)
	}
	var err error
	if !strings.EqualFold(os.Getenv("INIT_SEIYA"), "false") {
		err = seiya.Init()
		if err != nil {
			slog.Warn("Seiya 資料初始化失敗，相關查詢功能暫時不可用", "error", err)
		}
	} else {
		slog.Warn("Seiya 資料初始化已停用")
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

	runtimeContext, stopRuntime := context.WithCancel(context.Background())
	defer stopRuntime()
	runtimeSecret := strings.TrimSpace(os.Getenv("KURO_RUNTIME_SECRET"))
	if runtimeSecret != "" {
		runtimeClient, runtimeErr := servicekuro.NewClient(servicekuro.Config{
			URL:            envOrDefault("KURO_RUNTIME_URL", "ws://127.0.0.1:2334"),
			Secret:         runtimeSecret,
			RequestTimeout: time.Duration(utils.GetEnvInt("KURO_REQUEST_TIMEOUT_SECONDS", 180)) * time.Second,
		})
		if runtimeErr != nil {
			slog.Error("Kuro AI Runtime 設定錯誤", "error", runtimeErr)
			os.Exit(1)
		}
		botkuro.Init(runtimeClient, botkuro.Settings{
			TriggerPrefix:      envOrDefault("KURO_TRIGGER_PREFIX", "小黑"),
			ChannelIDs:         servicekuro.ParseIDSet(os.Getenv("KURO_CHANNEL_IDS")),
			CommandUserIDs:     servicekuro.ParseIDSet(os.Getenv("KURO_COMMAND_USER_IDS")),
			RecentMessageLimit: utils.GetEnvInt("KURO_RECENT_MESSAGE_LIMIT", 15),
			RecentContextChars: utils.GetEnvInt("KURO_RECENT_CONTEXT_CHARS", 6000),
		})
		runtimeClient.Start(runtimeContext)
		defer runtimeClient.Close()
	} else {
		slog.Warn("KURO_RUNTIME_SECRET 未設定，Kuro 對話功能不會啟用")
	}

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
	kuroHelper.AddHandler(bot.OnMessageCreate)

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

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

// db init
func dbInit() {
	config := db.Config{
		DBHost:     os.Getenv("DB_HOST"),
		DBOwner:    os.Getenv("DB_OWNER"),
		DBPassword: os.Getenv("DB_PASSWORD"),
		DBName:     os.Getenv("DB_NAME"),
		DBPort:     os.Getenv("DB_PORT"),
		SSLMode:    os.Getenv("DB_SSLMODE"),
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

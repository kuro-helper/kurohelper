package main

import (
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
	"github.com/lmittmann/tint"
	slogmulti "github.com/samber/slog-multi"

	"kurohelper/internal/bootstrap"
	"kurohelper/internal/bot"
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

func main() {
	// 初始化專案作業
	stopChan := make(chan struct{})
	bootstrap.BasicInit(stopChan)

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

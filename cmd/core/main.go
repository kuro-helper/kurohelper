package main

import (
	"log/slog"
	"os"
	"os/signal"

	"github.com/bwmarrin/discordgo"

	"kurohelper/internal/bootstrap"
	"kurohelper/internal/bot"
)

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

package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/gofiber/fiber/v3"

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
	defer kuroHelper.Close() // defer websocket disconnect

	// Fiber server
	app := fiber.New()
	app.Post("/github-actions", func(c fiber.Ctx) error {
		auth := c.Get("Authorization")
		if auth != "Bearer "+token {
			return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
		}
		eventName := c.FormValue("event_name")
		if eventName == "push" {
			_ = PushSend(kuroHelper, c)
			// if err != nil {
			// 	return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			// }
		}

		return c.JSON(fiber.Map{"status": "ok"})
	})

	fiberDone := make(chan struct{})

	go func() {
		if err := app.Listen(fmt.Sprintf("127.0.0.1:%s", os.Getenv("PRODUCTION_PORT"))); err != nil {
			slog.Info("Fiber shutdown: " + err.Error())
		}
		close(fiberDone)
		slog.Info("Fiber close success")
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	interruptSignal := <-c
	slog.Debug(interruptSignal.String())

	// 關閉 jobs
	close(stopChan)

	// 優雅關閉 Fiber server
	if err := app.Shutdown(); err != nil {
		slog.Info("Fiber shutdown error: " + err.Error())
	}

	// 等 Fiber goroutine 關閉
	<-fiberDone
}

func PushSend(kuroHelper *discordgo.Session, c fiber.Ctx) error {
	branch := c.FormValue("branch")
	hash := c.FormValue("hash")
	fullHash := c.FormValue("full_hash")
	authorEmail := c.FormValue("author_email")
	authorName := c.FormValue("author_name")
	date := c.FormValue("date")
	subject := c.FormValue("subject")
	body := c.FormValue("body")
	repoName := c.FormValue("repo_name")

	color := 0xF8C3CD

	switch strings.TrimSpace(repoName) {
	case "kurohelper":
		color = 0xF8C3CD
	case "kurohelper-service":
		color = 0x373C38
	case "kurohelper-docs":
		color = 0x268785
	case "kurohelper-api":
		color = 0xFFBA84
	case "kurohelper-web-nextjs":
		color = 0xB5495B
	case "kurohelper-web-nuxt3":
		color = 0x42D392
	}

	embed := &discordgo.MessageEmbed{
		Title:       repoName + " Push Event",
		Color:       color,
		Description: fmt.Sprintf("[%s](https://github.com/kuro-helper/%s/commit/%s)  %s\n%s", hash, repoName, fullHash, branch, date),
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "發送人",
				Value:  authorName + "/" + authorEmail,
				Inline: false,
			},
			{
				Name:   "主旨",
				Value:  subject,
				Inline: false,
			},
			{
				Name:   "內容",
				Value:  body,
				Inline: false,
			},
		},
	}

	_, err := kuroHelper.ChannelMessageSendEmbed(os.Getenv("BOT_CHANNEL_ID"), embed)
	if err != nil {
		return err
	}
	return nil
}

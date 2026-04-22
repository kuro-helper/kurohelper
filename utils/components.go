package utils

import (
	"errors"
	"fmt"

	"github.com/bwmarrin/discordgo"
)

type SelectMenuItem struct {
	Title string
	ID    string
}

var (
	ErrMakeChangePageComponentIndexZero = errors.New("utils: make change page component page index parameters can not be zero")
)

func MakeSelectMenuComponent(gameData []SelectMenuItem, commandName, routeKey, cacheID, placeholder string) *discordgo.ActionsRow {
	menuOptions := []discordgo.SelectMenuOption{}

	for _, gd := range gameData {
		menuOptions = append(menuOptions, discordgo.SelectMenuOption{
			Label: gd.Title,
			Value: gd.ID,
		})
	}

	return &discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.SelectMenu{
				CustomID:    MakeSelectMenuCIDV2(commandName, routeKey, cacheID),
				Placeholder: placeholder,
				Options:     menuOptions,
			},
		},
	}
}

// 製作回到主頁的Component
func MakeBackToHomeComponent(commandName, routeKey, cacheID string) *discordgo.ActionsRow {
	return &discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.Button{
				Label:    "🏠回到主頁",
				Style:    discordgo.PrimaryButton,
				CustomID: MakeBackToHomeCIDV2(commandName, routeKey, cacheID),
			},
		},
	}
}

// 製作翻頁Component
func MakeChangePageComponent(commandName, routeKey string, currentPage int, totalPage int, cacheID string) (*discordgo.ActionsRow, error) {
	if currentPage == 0 || totalPage == 0 {
		return nil, ErrMakeChangePageComponentIndexZero
	}

	// 中間的顯示頁數按鈕(不可點擊)
	tabButton := discordgo.Button{
		Label:    fmt.Sprintf("%d/%d", currentPage, totalPage),
		Style:    discordgo.SecondaryButton,
		Disabled: true,
		CustomID: MakePageCIDV2(commandName, routeKey, currentPage, cacheID, true),
	}

	previousDisabled := false
	nextDisabled := false

	if currentPage == totalPage {
		nextDisabled = true
	}

	if currentPage == 1 {
		previousDisabled = true
	}

	// 上一頁按鈕
	previousButton := discordgo.Button{
		Label:    "◀️",
		Style:    discordgo.SecondaryButton,
		Disabled: previousDisabled,
		CustomID: MakePageCIDV2(commandName, routeKey, currentPage-1, cacheID, false),
	}

	// 下一頁按鈕
	nextButton := discordgo.Button{
		Label:    "▶️",
		Style:    discordgo.SecondaryButton,
		Disabled: nextDisabled,
		CustomID: MakePageCIDV2(commandName, routeKey, currentPage+1, cacheID, false),
	}

	return &discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			previousButton,
			tabButton,
			nextButton,
		},
	}, nil
}

func MakeErrorComponentV2(errMsg string) []discordgo.MessageComponent {
	color := 0xcc543a
	divider := true
	containerComponents := []discordgo.MessageComponent{
		discordgo.TextDisplay{
			Content: "# ❌錯誤 \n## " + errMsg,
		},
		discordgo.Separator{Divider: &divider},
		discordgo.TextDisplay{
			Content: "聯絡我們: https://discord.gg/6rkrm7tsXr",
		},
	}

	return []discordgo.MessageComponent{
		discordgo.Container{
			AccentColor: &color,
			Components:  containerComponents,
		},
	}
}

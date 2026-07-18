package commands

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	kurohelpererrors "kurohelper/internal/errors"
	"kurohelper/internal/utils"
	kurohelperdb "kurohelperservice/db"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
)

const (
	announcementCommandName = "公告"
	announcementListRoute   = "l"
	announcementDetailRoute = "d"
	// 不使用快取；CID 仍需佔位
	announcementNoCacheID = "-"
	announcementMaxMenu   = 5
)

var announcementColor = 0xF19483

type Announcement struct{}

func (a *Announcement) Definition() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        announcementCommandName,
		Description: "查看公告與詳細內容",
	}
}

func (a *Announcement) Handler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	a.HandleComponent(s, i, nil)
}

func (a *Announcement) HandleComponent(s *discordgo.Session, i *discordgo.InteractionCreate, cid *utils.CIDV2) {
	if cid == nil {
		respondAnnouncementList(s, i, false)
		return
	}

	switch {
	case cid.GetRouteKey() == announcementDetailRoute && cid.GetBehaviorID() == utils.SelectMenuBehavior:
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
		})
		respondAnnouncementDetail(s, i, cid)
	case cid.GetRouteKey() == announcementListRoute && cid.GetBehaviorID() == utils.BackToHomeBehavior:
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
		})
		respondAnnouncementList(s, i, true)
	default:
		utils.HandleErrorV2(kurohelpererrors.ErrCIDBehaviorMismatch, s, i, utils.InteractionRespondEditComplex)
	}
}

func respondAnnouncementList(s *discordgo.Session, i *discordgo.InteractionCreate, editExisting bool) {
	if !editExisting {
		if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		}); err != nil {
			return
		}
	}

	list, err := kurohelperdb.GetAllAnnouncements(kurohelperdb.Dbs)
	if err != nil {
		respondErr := utils.WebhookEditRespond
		if editExisting {
			respondErr = utils.InteractionRespondEditComplex
		}
		utils.HandleErrorV2(err, s, i, respondErr)
		return
	}

	components, err := buildAnnouncementListComponents(list)
	if err != nil {
		respondErr := utils.WebhookEditRespond
		if editExisting {
			respondErr = utils.InteractionRespondEditComplex
		}
		utils.HandleErrorV2(err, s, i, respondErr)
		return
	}

	if editExisting {
		utils.InteractionRespondEditComplex(s, i, components)
		return
	}
	utils.WebhookEditRespond(s, i, components)
}

func respondAnnouncementDetail(s *discordgo.Session, i *discordgo.InteractionCreate, cid *utils.CIDV2) {
	id, err := strconv.Atoi(cid.ToSelectMenuCIDV2().Value)
	if err != nil || id <= 0 {
		utils.HandleErrorV2(fmt.Errorf("announcement: invalid id"), s, i, utils.InteractionRespondEditComplex)
		return
	}

	utils.WebhookEditRespond(s, i, []discordgo.MessageComponent{
		discordgo.Container{
			Components: []discordgo.MessageComponent{
				discordgo.TextDisplay{Content: "# ⌛ 正在載入,請稍候..."},
			},
		},
	})

	item, err := kurohelperdb.GetAnnouncementByID(kurohelperdb.Dbs, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.InteractionRespondEditComplex(s, i, utils.MakeErrorComponentV2("找不到該公告（可能已被刪除）"))
			return
		}
		utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
		return
	}

	components, err := buildAnnouncementDetailComponents(item)
	if err != nil {
		utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
		return
	}
	utils.InteractionRespondEditComplex(s, i, components)
}

func buildAnnouncementListComponents(list []kurohelperdb.Announcement) ([]discordgo.MessageComponent, error) {
	if len(list) == 0 {
		return []discordgo.MessageComponent{
			discordgo.Container{
				AccentColor: &announcementColor,
				Components: []discordgo.MessageComponent{
					discordgo.TextDisplay{Content: "# 公告列表\n目前沒有公告"},
				},
			},
		}, nil
	}

	shown := list
	if len(shown) > announcementMaxMenu {
		shown = shown[:announcementMaxMenu]
	}

	header := fmt.Sprintf("# 公告列表\n共 **%d** 則", len(list))
	if len(list) > announcementMaxMenu {
		header += fmt.Sprintf("（選單最多顯示 %d 則）", announcementMaxMenu)
	}

	divider := true
	containerComponents := []discordgo.MessageComponent{
		discordgo.TextDisplay{Content: header},
		discordgo.Separator{Divider: &divider},
	}

	menuItems := make([]utils.SelectMenuItem, 0, len(shown))
	for idx, item := range shown {
		itemContent := fmt.Sprintf("**%d. [%s]** %s\n📅 %s",
			idx+1,
			item.Category,
			item.Title,
			item.CreatedAt.Local().Format("2006/01/02"),
		)

		thumbURL := optionalURL(item.Thumbnail)
		if thumbURL == "" {
			thumbURL = utils.PlaceholderImageURL
		}
		containerComponents = append(containerComponents, discordgo.Section{
			Components: []discordgo.MessageComponent{
				discordgo.TextDisplay{Content: itemContent},
			},
			Accessory: &discordgo.Thumbnail{
				Media: discordgo.UnfurledMediaItem{URL: thumbURL},
			},
		})

		menuItems = append(menuItems, utils.SelectMenuItem{
			Title: truncateRunes(fmt.Sprintf("[%s] %s", item.Category, item.Title), 100),
			ID:    strconv.Itoa(item.ID),
		})
	}

	containerComponents = append(containerComponents,
		discordgo.Separator{Divider: &divider},
		utils.MakeSelectMenuComponent(
			menuItems,
			announcementCommandName,
			announcementDetailRoute,
			announcementNoCacheID,
			"選擇公告查看詳細",
		),
	)

	return []discordgo.MessageComponent{
		discordgo.Container{
			AccentColor: &announcementColor,
			Components:  containerComponents,
		},
	}, nil
}

func buildAnnouncementDetailComponents(item kurohelperdb.Announcement) ([]discordgo.MessageComponent, error) {
	content := strings.TrimSpace(item.Content)
	if utf8.RuneCountInString(content) > 3500 {
		content = string([]rune(content)[:3500]) + "…"
	}

	fullContent := fmt.Sprintf("**分類**\n%s\n\n**發布日期**\n%s\n\n**內容**\n%s",
		item.Category,
		item.CreatedAt.Local().Format("2006/01/02"),
		content,
	)

	divider := true
	thumbURL := optionalURL(item.Thumbnail)
	if thumbURL == "" {
		thumbURL = utils.PlaceholderImageURL
	}

	containerComponents := []discordgo.MessageComponent{
		discordgo.TextDisplay{Content: fmt.Sprintf("# %s", item.Title)},
		discordgo.Separator{Divider: &divider},
		discordgo.Section{
			Components: []discordgo.MessageComponent{
				discordgo.TextDisplay{Content: fullContent},
			},
			Accessory: &discordgo.Thumbnail{
				Media: discordgo.UnfurledMediaItem{URL: thumbURL},
			},
		},
	}

	if imageURL := optionalURL(item.Image); imageURL != "" {
		containerComponents = append(containerComponents,
			discordgo.Separator{Divider: &divider},
			discordgo.MediaGallery{
				Items: []discordgo.MediaGalleryItem{
					{Media: discordgo.UnfurledMediaItem{URL: imageURL}},
				},
			},
		)
	}

	containerComponents = append(containerComponents,
		discordgo.Separator{Divider: &divider},
		utils.MakeBackToHomeComponent(announcementCommandName, announcementListRoute, announcementNoCacheID),
	)

	return []discordgo.MessageComponent{
		discordgo.Container{
			AccentColor: &announcementColor,
			Components:  containerComponents,
		},
	}, nil
}

func optionalURL(s *string) string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(*s)
}

func truncateRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max-1]) + "…"
}

package randomcmd

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	kurohelpererrors "kurohelper/errors"
	"kurohelper/utils"

	"kurohelper-core/vndb"
	"kurohelper-core/ymgal"

	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
)

// 隨機遊戲Handler
func RandomGame(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// 長時間查詢
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
	opt, err := utils.GetOptions(i, "查詢資料庫選項")
	if err != nil && errors.Is(err, kurohelpererrors.ErrOptionTranslateFail) {
		utils.HandleError(err, s, i)
		return
	}
	if opt == "" || opt == "1" {
		vndbRandomGame(s, i)
	} else {
		ymgalRandomGame(s, i)
	}

}

func ymgalRandomGame(s *discordgo.Session, i *discordgo.InteractionCreate) {
	game, err := ymgal.GetRandomGame()
	if err != nil {
		utils.HandleError(err, s, i)
		return
	}

	title := game[0].Name
	if game[0].HaveChinese {
		title += "/" + game[0].ChineseName
	}

	image := utils.GenerateImage(i, "https://store.ymgal.games/"+game[0].MainImg)

	embed := &discordgo.MessageEmbed{
		Title: title,
		Color: 0x261E47,
		Image: image,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "發售日",
				Value:  game[0].ReleaseDate,
				Inline: false,
			},
		},
	}

	utils.InteractionEmbedRespond(s, i, embed, nil, true)
}

func vndbRandomGame(s *discordgo.Session, i *discordgo.InteractionCreate) {
	res, err := vndb.GetRandomVN()
	if err != nil {
		utils.HandleError(err, s, i)
		return
	}
	/* 處理回傳結構 */

	gameTitle := res.Results[0].Alttitle
	if strings.TrimSpace(gameTitle) == "" {
		gameTitle = res.Results[0].Title
	}
	brandTitle := res.Results[0].Developers[0].Original
	if strings.TrimSpace(brandTitle) != "" {
		brandTitle += fmt.Sprintf("(%s)", res.Results[0].Developers[0].Name)
	} else {
		brandTitle = res.Results[0].Developers[0].Name
	}
	logrus.WithField("guildID", i.GuildID).Infof("隨機遊戲: %s", gameTitle)
	// staff block
	var scenario string
	var art string
	var songs string
	var tmpAlias string
	for _, staff := range res.Results[0].Staff {
		staffName := staff.Original
		if staffName == "" {
			staffName = staff.Name
		}
		if len(staff.Aliases) > 0 {
			aliases := make([]string, 0, len(staff.Aliases))
			for _, alias := range staff.Aliases {
				if alias.IsMain {
					staffName = alias.Name
				} else {
					aliases = append(aliases, alias.Name)
				}
			}
			tmpAlias = "(" + strings.Join(aliases, ", ") + ")"
			if len(aliases) == 0 {
				tmpAlias = ""
			}
		}

		switch staff.Role {
		case "scenario":
			scenario += fmt.Sprintf("%s %s\n", staffName, tmpAlias)
		case "art":
			art += fmt.Sprintf("%s %s\n", staffName, tmpAlias)
		case "songs":
			songs += fmt.Sprintf("%s %s\n", staffName, tmpAlias)
		}
	}

	// character block

	characterMap := make(map[string]utils.CharacterData) // map[characterID]CharacterData
	for _, va := range res.Results[0].Va {
		characterName := va.Character.Original
		if characterName == "" {
			characterName = va.Character.Name
		}
		for _, vn := range va.Character.Vns {
			if vn.ID == res.Results[0].ID {
				characterMap[va.Character.ID] = utils.CharacterData{
					Name: characterName,
					Role: vn.Role,
				}
				break
			}
		}
	}

	// 將 map 轉為 slice 並排序
	characterList := make([]utils.CharacterData, 0, len(characterMap))
	for _, character := range characterMap {
		characterList = append(characterList, character)
	}
	sort.Slice(characterList, func(i, j int) bool {
		return characterList[i].Role < characterList[j].Role
	})

	// 格式化輸出
	characters := make([]string, 0, len(characterList))
	for _, character := range characterList {
		characters = append(characters, fmt.Sprintf("**%s** (%s)", character.Name, vndb.Role[character.Role]))
	}

	// relations block
	relationsGame := make([]string, 0, len(res.Results[0].Relations))
	for _, rg := range res.Results[0].Relations {
		titleName := ""
		for _, title := range rg.Titles {
			if title.Main {
				titleName = title.Title
			}
		}
		relationsGame = append(relationsGame, fmt.Sprintf("%s(%s)", titleName, rg.ID))
	}
	relationsGameDisplay := strings.Join(relationsGame, ", ")
	if strings.TrimSpace(relationsGameDisplay) == "" {
		relationsGameDisplay = "無"
	}

	// 過濾色情/暴力圖片
	image := utils.GenerateImage(i, res.Results[0].Image.Url)
	if res.Results[0].Image.Sexual >= 1 || res.Results[0].Image.Violence >= 1 {
		image = nil
		logrus.Debugf("%s 封面已過濾圖片顯示", gameTitle)
	}

	embed := &discordgo.MessageEmbed{
		Title: gameTitle,
		Color: 0x04108e,
		Image: image,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "品牌(公司)名稱",
				Value:  brandTitle,
				Inline: false,
			},
			{
				Name:   "劇本",
				Value:  scenario,
				Inline: false,
			},
			{
				Name:   "美術",
				Value:  art,
				Inline: false,
			},
			{
				Name:   "音樂",
				Value:  songs,
				Inline: false,
			},
			{
				Name:   "評價(平均/貝式平均/樣本數)",
				Value:  fmt.Sprintf("%.1f/%.1f/%d", res.Results[0].Average, res.Results[0].Rating, res.Results[0].Votecount),
				Inline: true,
			},
			{
				Name:   "平均遊玩時數/樣本數",
				Value:  fmt.Sprintf("%d(H)/%d", res.Results[0].LengthMinutes/60, res.Results[0].LengthVotes),
				Inline: true,
			},
			{
				Name:   "角色列表",
				Value:  strings.Join(characters, " / "),
				Inline: false,
			},
			{
				Name:   "ID",
				Value:  res.Results[0].ID,
				Inline: false,
			},
			{
				Name:   "相關遊戲",
				Value:  relationsGameDisplay,
				Inline: false,
			},
		},
	}
	utils.InteractionEmbedRespond(s, i, embed, nil, true)
}

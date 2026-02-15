package randomcmd

import (
	"errors"
	"fmt"
	kurohelpererrors "kurohelper/errors"
	"kurohelper/utils"
	"sort"
	"strconv"
	"strings"

	"kurohelper-core/vndb"

	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
)

// 隨機角色Handler
func RandomCharacter(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// 長時間查詢
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
	opt, err := utils.GetOptions(i, "隨機角色的身分")
	if err != nil && errors.Is(err, kurohelpererrors.ErrOptionTranslateFail) {
		utils.HandleError(err, s, i)
		return
	}
	vndbRandomCharacter(s, i, opt)
}

func vndbRandomCharacter(s *discordgo.Session, i *discordgo.InteractionCreate, opt string) {
	res, err := vndb.GetRandomCharacter(opt)
	if err != nil {
		utils.HandleError(err, s, i)
		return
	}
	nameData := res.Name
	heightData := "未收錄"
	weightData := "未收錄"
	BWHData := "未收錄"
	ageData := "未收錄"
	birthDayData := "未收錄"
	sexData := ""
	genderData := ""
	vnData := make([]string, 0, len(res.VNs))
	if res.Original != "" {
		nameData = fmt.Sprintf("%s (%s)", res.Original, res.Name)
	}
	logrus.WithField("guildID", i.GuildID).Infof("隨機角色: %s", nameData)
	if len(res.Aliases) == 0 {
		res.Aliases = []string{"未收錄"}
	}
	if res.Description == "" {
		res.Description = "無角色敘述"
	}
	if res.BloodType == "" {
		res.BloodType = "未收錄"
	}
	if res.Height != 0 {
		heightData = strconv.Itoa(res.Height) + "cm"
	}
	if res.Weight != 0 {
		weightData = strconv.Itoa(res.Weight) + "kg"
	}
	if res.Bust != 0 && res.Waist != 0 && res.Hips != 0 {
		BWHData = fmt.Sprintf("%d/%d/%d", res.Bust, res.Waist, res.Hips)
	}
	if res.Cup == "" {
		res.Cup = "未收錄"
	}
	if res.Age != nil {
		ageData = strconv.Itoa(*res.Age)
	}
	if res.Birthday != [2]int{} {
		birthDayData = fmt.Sprintf("%d月%d號", res.Birthday[0], res.Birthday[1])
	}
	if res.Sex == [2]string{} {
		sexData = "未收錄"
	} else if res.Sex[0] != res.Sex[1] {
		sexData = fmt.Sprintf("%s/||%s||", vndb.Sex[res.Sex[0]], vndb.Sex[res.Sex[1]])
	} else {
		sexData = vndb.Sex[res.Sex[0]]
	}
	if res.Gender == [2]string{} {
		genderData = "未收錄"
	} else if res.Gender[0] != res.Gender[1] {
		genderData = fmt.Sprintf("%s/||%s||", vndb.Gender[res.Gender[0]], vndb.Gender[res.Gender[1]])
	} else {
		genderData = vndb.Gender[res.Gender[0]]
	}
	if len(res.VNs) == 0 {
		vnData = append(vnData, "未收錄")
	} else {
		sort.Slice(res.VNs, func(i, j int) bool { // 依照角色定位排序
			return vndb.RolePriority[res.VNs[i].Role] < vndb.RolePriority[res.VNs[j].Role]
		})
		for _, vn := range res.VNs {
			titleData := vn.Title
			for _, title := range vn.Titles {
				if title.Main { // 抓取原文標題
					titleData = title.Title
					break
				}
			}
			if vn.Spoiler == 0 {
				vnData = append(vnData, fmt.Sprintf("%s (%s)", titleData, vndb.Role[vn.Role]))
			} else {
				vnData = append(vnData, fmt.Sprintf("||%s (%s)||", titleData, vndb.Role[vn.Role]))
			}
		}
	}

	res.Description = vndb.ConvertBBCodeToMarkdown(res.Description)
	image := utils.GenerateImage(i, res.Image.URL)
	embed := &discordgo.MessageEmbed{
		Title:       nameData,
		Description: res.Description, // 敘述放在Description欄位以避免超過字數限制
		Color:       0xF8F8DF,
		Image:       image,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "別名",
				Value:  strings.Join(res.Aliases, "/"),
				Inline: true,
			},
			{
				Name:   "CV",
				Value:  strings.Join(res.Vas, "/"),
				Inline: true,
			},
			{
				Name:   "生日",
				Value:  birthDayData,
				Inline: true,
			},
			{
				Name:   "生理性別",
				Value:  sexData,
				Inline: true,
			},
			{
				Name:   "性別認同",
				Value:  genderData,
				Inline: true,
			},
			{
				Name:   "身高/體重",
				Value:  fmt.Sprintf("%s/%s", heightData, weightData),
				Inline: true,
			},
			{
				Name:   "年齡",
				Value:  ageData,
				Inline: true,
			},
			{
				Name:   "血型",
				Value:  res.BloodType,
				Inline: true,
			},
			{
				Name:   "三圍",
				Value:  BWHData,
				Inline: true,
			},
			{
				Name:   "登場於",
				Value:  strings.Join(vnData, "\n"),
				Inline: true,
			},
			{
				Name:   "ID",
				Value:  res.ID,
				Inline: true,
			},
		},
	}
	utils.InteractionEmbedRespond(s, i, embed, nil, true)
}

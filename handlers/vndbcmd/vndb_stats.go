package vndbcmd

import (
	"fmt"

	"github.com/bwmarrin/discordgo"

	"kurohelper/utils"

	"kurohelper-core/vndb"
)

// vndb統計資料Handler
func VndbStats(s *discordgo.Session, i *discordgo.InteractionCreate) {
	r, err := vndb.GetStats()
	if err != nil {
		utils.HandleError(err, s, i)
		return
	}

	embed := &discordgo.MessageEmbed{
		Title: "VNDB統計資料",
		Color: 0x04108e,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "角色收錄數量",
				Value:  fmt.Sprintf("%d筆", r.Chars),
				Inline: true,
			},
			{
				Name:   "公司/品牌收錄數量",
				Value:  fmt.Sprintf("%d筆", r.Producers),
				Inline: true,
			},
			{
				Name:   "發行版本收錄數量",
				Value:  fmt.Sprintf("%d筆", r.Releases),
				Inline: true,
			},
			{
				Name:   "標籤收錄數量",
				Value:  fmt.Sprintf("%d筆", r.Tags),
				Inline: true,
			},
			{
				Name:   "角色特徵收錄數量",
				Value:  fmt.Sprintf("%d筆", r.Traits),
				Inline: true,
			},
			{
				Name:   "視覺小說收錄數量",
				Value:  fmt.Sprintf("%d筆", r.VN),
				Inline: true,
			},
		},
	}
	utils.InteractionEmbedRespond(s, i, embed, nil, false)
}

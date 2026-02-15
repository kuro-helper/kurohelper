package usercmd

import (
	"errors"
	"fmt"
	kurohelpercore "kurohelper-core"
	"regexp"
	"strconv"
	"strings"
	"time"

	kurohelperdb "kurohelper-db"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"gorm.io/gorm"

	"kurohelper/cache"
	kurohelpererrors "kurohelper/errors"
	"kurohelper/store"
	"kurohelper/utils"

	"kurohelper-core/erogs"
)

// 加已玩Handler
func AddHasPlayed(s *discordgo.Session, i *discordgo.InteractionCreate, cid *utils.NewCID) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})

	if cid != nil {
		addHasPlayedCID := utils.AddHasPlayedCID{
			NewCID: *cid,
		}

		completeDate, err := addHasPlayedCID.GetCompleteDate()
		if err != nil {
			utils.HandleError(err, s, i)
			return
		}

		// get cache
		cacheValue, err := cache.UserInfoCache.Get(addHasPlayedCID.GetCacheID())
		if err != nil {
			utils.HandleError(err, s, i)
			return
		}
		resValue := cacheValue.(erogs.Game)
		res := &resValue

		userID := utils.GetUserID(i)
		userName := utils.GetUsername(i)
		if strings.TrimSpace(userID) != "" && strings.TrimSpace(userName) != "" {
			err := kurohelperdb.Dbs.Transaction(func(tx *gorm.DB) error {
				// 1. 確保 User 存在
				if _, err := kurohelperdb.EnsureUserTx(tx, userID, userName); err != nil {
					return err
				}

				// 2. 確保 Brand 存在
				if _, err := kurohelperdb.EnsureBrandErogsTx(tx, res.BrandID, res.BrandName); err != nil {
					return err
				}

				// 3. 確保 Game 存在
				if _, err := kurohelperdb.EnsureGameErogsTx(tx, res.ID, res.Gamename, res.BrandID); err != nil {
					return err
				}

				// 4. 建立資料
				if err := kurohelperdb.CreateUserHasPlayedTx(tx, userID, res.ID, completeDate); err != nil {
					return err
				}

				return nil // commit
			})
			if err != nil {
				utils.HandleError(err, s, i)
				return
			}

			// 確保新建立的使用者有加入快取
			if _, ok := store.UserStore[userID]; !ok {
				store.UserStore[userID] = struct{}{}
			}

			embed := &discordgo.MessageEmbed{
				Title: "加入成功！",
				Color: 0x7BA23F,
			}
			utils.InteractionEmbedRespondForSelf(s, i, embed, nil, true)
		} else { // 找不到使用者，此狀況應該會是Discord官方問題或是程式碼邏輯問題
			embed := &discordgo.MessageEmbed{
				Title: "找不到使用者！",
				Color: 0x7BA23F,
			}
			utils.InteractionEmbedRespondForSelf(s, i, embed, nil, true)
		}
	} else {
		var res *erogs.Game

		keyword, err := utils.GetOptions(i, "keyword")
		if err != nil {
			utils.HandleError(err, s, i)
			return
		}

		completeDate, err := utils.GetOptions(i, "complete_date")
		if err != nil && !errors.Is(err, kurohelpererrors.ErrOptionNotFound) {
			utils.HandleError(err, s, i)
			return
		}

		var t time.Time
		if completeDate != "" {
			t, err = utils.ParseYYYYMMDD(completeDate)
			if err != nil {
				utils.HandleError(err, s, i)
				return
			}

			if t.After(time.Now().AddDate(0, 0, 1)) {
				utils.HandleError(err, s, i)
				return
			}
		}

		idSearch, _ := regexp.MatchString(`^e\d+$`, keyword)
		if idSearch {
			num, _ := strconv.Atoi(keyword[1:])
			res, err = erogs.SearchGameByID(num)
		} else {
			res, err = erogs.SearchGameByKeyword([]string{keyword, kurohelpercore.ZhTwToJp(keyword)})
		}
		if err != nil {
			utils.HandleError(err, s, i)
			return
		}

		idStr := uuid.New().String()
		cache.UserInfoCache.Set(idStr, *res)

		cidCommandName := utils.MakeCIDCommandName(i.ApplicationCommandData().Name, false, "")
		messageComponent := []discordgo.MessageComponent{utils.MakeCIDAddHasPlayedComponent("✅", idStr, t, cidCommandName)}
		actionsRow := utils.MakeActionsRow(messageComponent)

		image := utils.GenerateImage(i, res.BannerUrl)

		embed := &discordgo.MessageEmbed{
			Author: &discordgo.MessageEmbedAuthor{
				Name: res.BrandName,
			},
			Title: fmt.Sprintf("**%s(%s)**", res.Gamename, res.SellDay),
			URL:   res.Shoukai,
			Color: 0x7BA23F,
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:   "發行機種",
					Value:  res.Model,
					Inline: false,
				},
				{
					Name:   "確認",
					Value:  "你確定要加入已玩嗎?",
					Inline: false,
				},
			},
			Image: image,
		}
		utils.InteractionEmbedRespondForSelf(s, i, embed, actionsRow, true)
	}
}

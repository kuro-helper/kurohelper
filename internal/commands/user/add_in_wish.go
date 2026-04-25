package user

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"

	"kurohelperservice"
	kurohelperdb "kurohelperservice/db"
	"kurohelperservice/provider/erogs"

	"kurohelper/internal/cache"
	kurohelpererrors "kurohelper/internal/errors"
	"kurohelper/internal/store"
	"kurohelper/internal/utils"
)

type AddInWish struct{}

const addInWishCommandName = "加收藏"

func (a *AddInWish) Definition() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "加收藏",
		Description: "把遊戲加到收藏(ErogameScape)",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "keyword",
				Description: "關鍵字",
				Required:    true,
			},
		},
	}
}

func (a *AddInWish) Handler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	a.HandleComponent(s, i, nil)
}

// 加收藏Handler
func (a *AddInWish) HandleComponent(s *discordgo.Session, i *discordgo.InteractionCreate, cid *utils.CIDV2) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})

	if cid != nil {
		if cid.GetBehaviorID() != utils.UserDataOperationBehavior {
			utils.HandleError(kurohelpererrors.ErrCIDBehaviorMismatch, s, i)
			return
		}
		userDataCID, err := cid.ToUserDataOperationCIDV2()
		if err != nil {
			utils.HandleError(err, s, i)
			return
		}

		// get cache
		cacheValue, err := cache.UserInfoCache.Get(userDataCID.CacheID)
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
				if _, err := kurohelperdb.EnsureUser(tx, userID, userName); err != nil {
					return err
				}

				// 2. 確保 Brand 存在
				// 新增欄位資料先用預設值
				if _, err := kurohelperdb.EnsureBrandErogs(tx, res.BrandID, res.BrandName, false, 0); err != nil {
					return err
				}

				// 3. 確保 Game 存在
				image := erogs.MakeDMMImageURL(res.DMM)
				if strings.TrimSpace(res.DMM) == "" {
					image = ""
				}
				if _, err := kurohelperdb.EnsureGameErogs(tx, res.ID, res.Gamename, image, res.BrandID); err != nil {
					return err
				}

				// 4. 建立資料
				if err := kurohelperdb.CreateUserInWish(tx, userID, res.ID); err != nil {
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
				Color: 0x90B44B,
			}
			utils.InteractionEmbedRespondForSelf(s, i, embed, nil, true)
		} else { // 找不到使用者，此狀況應該會是Discord官方問題或是程式碼邏輯問題
			embed := &discordgo.MessageEmbed{
				Title: "找不到使用者！",
				Color: 0x90B44B,
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

		idSearch, _ := regexp.MatchString(`^e\d+$`, keyword)
		if idSearch {
			num, _ := strconv.Atoi(keyword[1:])
			res, err = erogs.SearchGameByID(num)
		} else {
			res, err = erogs.SearchGameByKeyword([]string{keyword, kurohelperservice.ZhTwToJp(keyword)})
		}
		if err != nil {
			utils.HandleError(err, s, i)
			return
		}
		if res == nil {
			utils.HandleError(kurohelperservice.ErrSearchNoContent, s, i)
			return
		}

		idStr := uuid.New().String()
		cache.UserInfoCache.Set(idStr, *res)

		messageComponent := []discordgo.MessageComponent{
			discordgo.Button{
				Label:    "✅",
				Style:    discordgo.PrimaryButton,
				CustomID: utils.MakeUserDataOperationCIDV2(addInWishCommandName, idStr, res.ID),
			},
		}
		actionsRow := utils.MakeActionsRow(messageComponent)

		image := utils.GenerateImage(i, res.BannerUrl)

		embed := &discordgo.MessageEmbed{
			Author: &discordgo.MessageEmbedAuthor{
				Name: res.BrandName,
			},
			Title: fmt.Sprintf("**%s(%s)**", res.Gamename, res.SellDay),
			URL:   res.Shoukai,
			Color: 0x90B44B,
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:   "發行機種",
					Value:  res.Model,
					Inline: false,
				},
				{
					Name:   "確認",
					Value:  "你確定要加入收藏嗎?",
					Inline: false,
				},
			},
			Image: image,
		}
		utils.InteractionEmbedRespondForSelf(s, i, embed, actionsRow, true)
	}
}

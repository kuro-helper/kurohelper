package user

import (
	"errors"
	"fmt"
	"log/slog"
	"regexp"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"kurohelper/internal/cache"
	kurohelpererrors "kurohelper/internal/errors"
	"kurohelper/internal/utils"

	kurohelperdb "kurohelperservice/db"
)

type ForgotPassword struct{}

type forgotPasswordCacheData struct {
	OwnerDiscordID string
	UserID         int
	HashedPassword string
}

const forgotPasswordCommandName = "忘記密碼"

// 與網頁版註冊相同：僅英文與數字
var websitePasswordPattern = regexp.MustCompile(`^[A-Za-z0-9]+$`)

func validateWebsitePassword(password string) string {
	if password == "" {
		return "請輸入密碼。"
	}
	if !websitePasswordPattern.MatchString(password) {
		return "密碼只能使用英文與數字。"
	}
	if len(password) <= 10 {
		return "密碼長度需要大於 10 碼。"
	}
	return ""
}

func (f *ForgotPassword) Definition() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        forgotPasswordCommandName,
		Description: "重設 KuroHelper 網站版密碼",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "password",
				Description: "新密碼（僅英文與數字，長度需大於 10 碼）",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "password_confirm",
				Description: "確認新密碼",
				Required:    true,
			},
		},
	}
}

func (f *ForgotPassword) Handler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})

	password, err := utils.GetOptions(i, "password")
	if err != nil {
		utils.HandleError(err, s, i)
		return
	}
	passwordConfirm, err := utils.GetOptions(i, "password_confirm")
	if err != nil {
		utils.HandleError(err, s, i)
		return
	}

	if msg := validateWebsitePassword(password); msg != "" {
		embed := &discordgo.MessageEmbed{
			Title:       "密碼格式錯誤",
			Color:       0xcc543a,
			Description: msg,
		}
		utils.InteractionEmbedRespondForSelf(s, i, embed, nil, true)
		return
	}
	if password != passwordConfirm {
		embed := &discordgo.MessageEmbed{
			Title:       "密碼不一致",
			Color:       0xcc543a,
			Description: "密碼與確認密碼不一致。",
		}
		utils.InteractionEmbedRespondForSelf(s, i, embed, nil, true)
		return
	}

	discordID := utils.GetUserID(i)
	user, err := kurohelperdb.GetUserByDiscordID(kurohelperdb.Dbs, discordID)
	if err != nil {
		utils.HandleError(err, s, i)
		return
	}

	_, err = kurohelperdb.GetUserAuthByUserID(kurohelperdb.Dbs, user.ID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		embed := &discordgo.MessageEmbed{
			Title:       "尚未註冊網頁版帳號",
			Color:       0xcc543a,
			Description: "尚未註冊網站版帳號，要註冊帳號請使用「註冊帳號」指令",
		}
		utils.InteractionEmbedRespondForSelf(s, i, embed, nil, true)
		return
	}
	if err != nil {
		utils.HandleError(err, s, i)
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		utils.HandleError(err, s, i)
		return
	}

	cacheID := uuid.New().String()
	cache.UserInfoCache.Set(cacheID, forgotPasswordCacheData{
		OwnerDiscordID: discordID,
		UserID:         user.ID,
		HashedPassword: string(hashedPassword),
	})

	messageComponent := []discordgo.MessageComponent{
		discordgo.Button{
			Label:    "✅",
			Style:    discordgo.PrimaryButton,
			CustomID: utils.MakeUserDataOperationCIDV2(forgotPasswordCommandName, cacheID, user.ID),
		},
	}
	actionsRow := utils.MakeActionsRow(messageComponent)

	embed := &discordgo.MessageEmbed{
		Title: "確認重設密碼",
		Color: 0x90B44B,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "確認",
				Value:  "確定要將網站帳號密碼重設為剛才輸入的新密碼嗎？",
				Inline: false,
			},
		},
	}
	utils.InteractionEmbedRespondForSelf(s, i, embed, actionsRow, true)
}

func (f *ForgotPassword) HandleComponent(s *discordgo.Session, i *discordgo.InteractionCreate, cid *utils.CIDV2) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})

	if cid == nil {
		utils.HandleError(gorm.ErrRecordNotFound, s, i)
		return
	}

	userID := utils.GetUserID(i)
	if cid.GetBehaviorID() != utils.UserDataOperationBehavior {
		utils.HandleError(kurohelpererrors.ErrCIDBehaviorMismatch, s, i)
		return
	}
	userDataCID, err := cid.ToUserDataOperationCIDV2()
	if err != nil {
		utils.HandleError(err, s, i)
		return
	}

	cacheValue, err := cache.UserInfoCache.Get(userDataCID.CacheID)
	if err != nil {
		utils.HandleError(err, s, i)
		return
	}
	cacheData, ok := cacheValue.(forgotPasswordCacheData)
	if !ok {
		utils.HandleError(fmt.Errorf("forgot password cache data type mismatch"), s, i)
		return
	}
	if cacheData.OwnerDiscordID != userID {
		utils.HandleError(fmt.Errorf("only the original user can confirm this password reset"), s, i)
		return
	}

	if err := kurohelperdb.UpdateUserAuthPassword(kurohelperdb.Dbs, cacheData.UserID, cacheData.HashedPassword); err != nil {
		utils.HandleError(err, s, i)
		return
	}

	embed := &discordgo.MessageEmbed{
		Title:       "密碼重設成功",
		Color:       0x7BA23F,
		Description: "網站帳號密碼已更新，請使用新密碼登入(暫時不會全站登出，請手動登出)",
	}
	utils.InteractionEmbedRespondForSelf(s, i, embed, nil, true)
	slog.Info("忘記密碼重設成功", "使用者ID", userID, "userID", cacheData.UserID)
}

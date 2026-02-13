package utils

import (
	"errors"

	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	kurohelpererrors "kurohelper/errors"

	kurohelperdb "kurohelper-db"

	kurohelpercore "kurohelper-core"
)

// 錯誤統一處理方法
func HandleError(err error, s *discordgo.Session, i *discordgo.InteractionCreate) {
	logrus.WithField("guildID", i.GuildID).Error(err)
	switch {
	case errors.Is(err, kurohelperdb.ErrUniqueViolation):
		InteractionEmbedRespond(s, i, MakeErrorEmbedMsg("資料已存在，此次操作無效"), nil, true)
	case errors.Is(err, gorm.ErrRecordNotFound):
		InteractionEmbedRespond(s, i, MakeErrorEmbedMsg("找不到資料或使用者尚未建檔"), nil, true)
	case errors.Is(err, kurohelpercore.ErrRateLimit):
		InteractionEmbedRespond(s, i, MakeErrorEmbedMsg("速率限制，請過約1分鐘後再試"), nil, true)
	case errors.Is(err, kurohelpercore.ErrSearchNoContent):
		InteractionEmbedRespond(s, i, MakeErrorEmbedMsg("找不到任何結果喔"), nil, true)
	case errors.Is(err, kurohelpererrors.ErrTimeWrongFormat):
		InteractionEmbedRespond(s, i, MakeErrorEmbedMsg("日期格式錯誤，格式為YYYYMMDD"), nil, true)
	case errors.Is(err, kurohelpererrors.ErrDateExceedsTomorrow):
		InteractionEmbedRespond(s, i, MakeErrorEmbedMsg("日期格式錯誤，完成日期不得超過今日加一天"), nil, true)
	case errors.Is(err, kurohelpercore.ErrBangumiCharacterListSearchNotSupported):
		InteractionEmbedRespond(s, i, MakeErrorEmbedMsg("目前不支援對Bangumi使用角色列表搜尋"), nil, true)
	case errors.Is(err, kurohelpererrors.ErrCIDGetParameterFailed):
		fallthrough
	case errors.Is(err, kurohelpercore.ErrCacheLost):
		EditEmbedRespond(s, i, MakeErrorEmbedMsg("快取過期，請重新查詢"), nil)
	case errors.Is(err, kurohelpercore.ErrStatusCodeAbnormal):
		fallthrough
	case errors.Is(err, kurohelpererrors.ErrOptionTranslateFail):
		fallthrough
	case errors.Is(err, kurohelpererrors.ErrOptionNotFound):
		fallthrough
	default:
		InteractionEmbedRespond(s, i, MakeErrorEmbedMsg("該功能目前異常，請稍後再嘗試"), nil, true)
	}
}

// 錯誤統一處理方法(新版V2 API)
//
// 符合簽名的有三種函式
//
// - InteractionRespondV2
// - InteractionRespondEditComplex
// - WebhookEditRespond
func HandleErrorV2(
	err error,
	s *discordgo.Session,
	i *discordgo.InteractionCreate,
	responder func(*discordgo.Session, *discordgo.InteractionCreate, []discordgo.MessageComponent)) {
	logrus.WithField("guildID", i.GuildID).Error(err)

	errMsg := "該功能目前異常，請稍後再嘗試"
	switch {
	case errors.Is(err, kurohelperdb.ErrUniqueViolation):
		errMsg = "資料已存在，此次操作無效"
	case errors.Is(err, gorm.ErrRecordNotFound):
		errMsg = "找不到資料或使用者尚未建檔"
	case errors.Is(err, kurohelpercore.ErrRateLimit):
		errMsg = "速率限制，請過約1分鐘後再試"
	case errors.Is(err, kurohelpercore.ErrSearchNoContent):
		errMsg = "找不到任何結果喔"
	case errors.Is(err, kurohelpererrors.ErrTimeWrongFormat):
		errMsg = "日期格式錯誤，格式為YYYYMMDD"
	case errors.Is(err, kurohelpererrors.ErrDateExceedsTomorrow):
		errMsg = "日期格式錯誤，完成日期不得超過今日加一天"
	case errors.Is(err, kurohelpercore.ErrBangumiCharacterListSearchNotSupported):
		errMsg = "目前不支援對Bangumi使用角色列表搜尋"
	case errors.Is(err, kurohelpercore.ErrCacheLost):
		errMsg = "快取過期，請重新查詢"
	default:
		errMsg = "該功能目前異常，請稍後再嘗試"
	}

	responder(s, i, MakeErrorComponentV2(errMsg))
}

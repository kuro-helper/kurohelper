package navigator

import (
	"encoding/base64"
	"kurohelper/cache"
	"kurohelper/utils"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// 處理「關鍵字搜尋列表」的完整流程：檢查快取、無快取則查詢並延遲回應。
//
// 參數：
//   - s: 用於發送回應的 Discord session
//   - i: 觸發搜尋的 interaction（需含 "keyword" 選項）
//   - store: 存放查詢結果的快取儲存
//   - logPrefix: 日誌前綴，如 "vndb查詢角色列表"
//   - searcher: 執行實際查詢的函數，快取未命中時呼叫
//   - builder: 從查詢結果建構訊息元件的函數。接收 (cacheValue, pageNumber, cacheID)，回傳列表第一頁的元件
func SearchList[T any](
	s *discordgo.Session,
	i *discordgo.InteractionCreate,
	store *cache.CacheStoreV2[T],
	logPrefix string,
	searcher func() (T, error),
	builder func(T, int, string) ([]discordgo.MessageComponent, error),
) {
	keyword, err := utils.GetOptions(i, "keyword")
	if err != nil {
		utils.HandleErrorV2(err, s, i, utils.InteractionRespondV2)
		return
	}

	idStr := uuid.New().String()

	// 將 keyword 轉成 base64 作為快取鍵
	cacheKey := base64.RawURLEncoding.EncodeToString([]byte(keyword))

	// 檢查快取是否存在
	cacheValue, err := store.Get(cacheKey)
	if err == nil {
		// 存入CID與關鍵字的對應快取
		cache.CIDStore.Set(idStr, cacheKey)

		// 快取存在，直接使用，不需要延遲傳送
		components, err := builder(cacheValue, 1, idStr)
		if err != nil {
			utils.HandleErrorV2(err, s, i, utils.InteractionRespondV2)
			return
		}
		utils.InteractionRespondV2(s, i, components)
		return
	}

	// 快取不存在，需要查詢資料
	// 先發送延遲回應
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	logrus.WithField("interaction", i).Infof("%s: %s", logPrefix, keyword)

	res, err := searcher()
	if err != nil {
		utils.HandleErrorV2(err, s, i, utils.WebhookEditRespond)
		return
	}

	// 將查詢結果存入快取
	store.Set(cacheKey, res)

	// 存入CID與關鍵字的對應快取
	cache.CIDStore.Set(idStr, cacheKey)

	components, err := builder(res, 1, idStr)
	if err != nil {
		utils.HandleErrorV2(err, s, i, utils.WebhookEditRespond)
		return
	}

	utils.WebhookEditRespond(s, i, components)
}

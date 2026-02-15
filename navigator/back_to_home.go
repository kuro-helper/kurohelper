package navigator

import (
	"kurohelper/cache"
	"kurohelper/utils"

	"github.com/bwmarrin/discordgo"
)

// 處理「回到首頁」的導覽邏輯。
//
// 參數：
//   - s: 用於發送回應的 Discord session
//   - i: 觸發導覽的 interaction
//   - backToHomeCID: 由呼叫端透過 cid.ToBackToHomeCIDV2() 取得
//   - store: 存放列表資料的快取儲存
//   - builder: 從快取資料建構訊息元件的函數。接收 (cacheValue, pageNumber, cacheID)，回傳列表第一頁的元件
func BackToHome[T any](
	s *discordgo.Session,
	i *discordgo.InteractionCreate,
	backToHomeCID *utils.BackToHomeCIDV2,
	store *cache.CacheStoreV2[T],
	builder func(T, int, string) ([]discordgo.MessageComponent, error),
) {
	// 先發送延遲回應
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})

	cidCacheValue, err := cache.CIDStore.Get(backToHomeCID.CacheID)
	if err != nil {
		utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
		return
	}

	cacheValue, err := store.Get(cidCacheValue)
	if err != nil {
		utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
		return
	}

	components, err := builder(cacheValue, 1, backToHomeCID.CacheID)
	if err != nil {
		utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
		return
	}
	utils.InteractionRespondEditComplex(s, i, components)
}

package navigator

import (
	"errors"
	"kurohelper/cache"
	"kurohelper/utils"

	"github.com/bwmarrin/discordgo"
)

// 處理「回到首頁」的導覽邏輯。
//
// 參數：
//   - s: 用於發送回應的 Discord session
//   - i: 觸發導覽的 interaction
//   - cid: CIDV2
//   - store: 存放列表資料的快取儲存（如 VndbCharacterListStore）
//   - builder: 從快取資料建構訊息元件的函數。接收 (cacheValue, pageNumber, cacheID)，回傳列表第一頁的元件
func BackToHome[T any](
	s *discordgo.Session,
	i *discordgo.InteractionCreate,
	cid *utils.CIDV2,
	store *cache.CacheStoreV2[T],
	builder func(T, int, string) ([]discordgo.MessageComponent, error),
) {
	if cid.GetBehaviorID() != utils.BackToHomeBehavior {
		utils.HandleErrorV2(errors.New("handlers: cid behavior id error"), s, i, utils.InteractionRespondEditComplex)
		return
	}

	backToHomeCID := cid.ToBackToHomeCIDV2()

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

package navigator

import (
	"kurohelper/cache"
	"kurohelper/utils"

	"github.com/bwmarrin/discordgo"
)

// 處理「列表翻頁」的導覽邏輯。
//
// 參數：
//   - s: 用於發送回應的 Discord session
//   - i: 觸發翻頁的 interaction
//   - pageCID: 由呼叫端透過 cid.ToPageCIDV2() 取得
//   - store: 存放列表資料的快取儲存
//   - builder: 從快取資料建構訊息元件的函數。接收 (cacheValue, pageNumber, cacheID)，回傳該頁的元件
func ChangePage[T any](
	s *discordgo.Session,
	i *discordgo.InteractionCreate,
	pageCID *utils.PageCIDV2,
	store *cache.CacheStoreV2[T],
	builder func(T, int, string) ([]discordgo.MessageComponent, error),
) {
	// 先發送延遲回應
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})

	cidCacheValue, err := cache.CIDStore.Get(pageCID.CacheID)
	if err != nil {
		utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
		return
	}

	cacheValue, err := store.Get(cidCacheValue)
	if err != nil {
		utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
		return
	}

	components, err := builder(cacheValue, pageCID.Value, pageCID.CacheID)
	if err != nil {
		utils.HandleErrorV2(err, s, i, utils.InteractionRespondEditComplex)
		return
	}

	utils.WebhookEditRespond(s, i, components)
}

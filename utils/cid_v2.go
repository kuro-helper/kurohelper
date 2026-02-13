package utils

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type (
	// CID原型
	//
	// 這個原型型別只是方便後續轉換，直接拿本體使用會有型別不安全問題
	CIDV2 struct {
		behaviorID BehaviorID
		commandID  string
		cacheID    string
		value      string
	}

	BehaviorID string

	// 翻頁CID
	PageCIDV2 struct {
		BehaviorID BehaviorID
		CacheID    string
		Value      int
	}

	// 選單CID
	SelectMenuCIDV2 struct {
		BehaviorID BehaviorID
		CacheID    string
		Value      string
	}

	DetailBtnCIDV2 struct {
		BehaviorID BehaviorID
		CacheID    string
		Value      string
	}

	// 切換來源CID
	SwitchSourceCIDV2 struct {
		BehaviorID BehaviorID
		CacheID    string
		Value      string
	}

	// 回到主頁CID
	BackToHomeCIDV2 struct {
		BehaviorID BehaviorID
		CacheID    string
		// 回到主頁CID不需要Value
	}
)

const (
	// PageBehavior Value會是int
	PageBehavior BehaviorID = "P"
	// SelectMenuBehavior Value會是string(選擇後從Discord API獲得)
	SelectMenuBehavior BehaviorID = "S"
	// BackToHomeBehavior 不會有Value
	BackToHomeBehavior BehaviorID = "H"

	DetailBtnBehavior BehaviorID = "D"

	SwitchSourceBehavior BehaviorID = "W"
)

var (
	ErrCIDV2ParseFailed      = errors.New("utils: cidv2 parse failed")
	ErrCIDV2ParseValueFailed = errors.New("utils: cidv2 parse value failed")
)

// 將字串嘗試轉型成CIDV2原型格式
//
// 檢查CIDV2的格式是否正確
func ParseCIDV2(target string) (*CIDV2, error) {
	parts := strings.Split(target, ":")
	if len(parts) != 4 {
		return nil, ErrCIDV2ParseFailed
	}

	return &CIDV2{
		commandID:  parts[0],
		cacheID:    parts[1],
		behaviorID: BehaviorID(parts[2]),
		value:      parts[3],
	}, nil
}

// 從CIDV2獲取behaviorID
func (c CIDV2) GetBehaviorID() BehaviorID {
	return c.behaviorID
}

// 從CIDV2獲取commandID
func (c CIDV2) GetCommandID() string {
	return c.commandID
}

func (c CIDV2) ToPageCIDV2() (*PageCIDV2, error) {
	v, err := strconv.Atoi(c.value)
	if err != nil {
		return nil, ErrCIDV2ParseValueFailed
	}

	return &PageCIDV2{
		CacheID:    c.cacheID,
		BehaviorID: c.behaviorID,
		Value:      v,
	}, nil
}

func (c CIDV2) ToSelectMenuCIDV2() *SelectMenuCIDV2 {
	return &SelectMenuCIDV2{
		CacheID:    c.cacheID,
		BehaviorID: c.behaviorID,
		Value:      c.value,
	}
}

func (c CIDV2) ToDetailBtnCIDV2() *DetailBtnCIDV2 {
	return &DetailBtnCIDV2{
		CacheID:    c.cacheID,
		BehaviorID: c.behaviorID,
		Value:      c.value,
	}
}

func (c CIDV2) ToSwitchSourceCIDV2() *SwitchSourceCIDV2 {
	return &SwitchSourceCIDV2{
		CacheID:    c.cacheID,
		BehaviorID: c.behaviorID,
		Value:      c.value,
	}
}

func (c CIDV2) ToBackToHomeCIDV2() *BackToHomeCIDV2 {
	return &BackToHomeCIDV2{
		CacheID:    c.cacheID,
		BehaviorID: c.behaviorID,
	}
}

// 修改Value值(SelectMenuBehavior時使用)
func (c *CIDV2) ChangeValue(value string) {
	c.value = value
}

/*
 * CID產生相關
 */

// 產生page的CID
//
// CID標示符是P
func MakePageCIDV2(commandID string, index int, cacheID string, disable bool) string {
	if disable {
		return fmt.Sprintf("%s:%s:P:99", commandID, cacheID)
	}
	return fmt.Sprintf("%s:%s:P:%d", commandID, cacheID, index)
}

// 產生select menu的CID
//
// 產生select menu的CID時不需要先預留Value，Value會在選單選擇時才設定(Discord會自動設定)
//
// CID標示符是S
func MakeSelectMenuCIDV2(commandID, cacheID string) string {
	return fmt.Sprintf("%s:%s:S:", commandID, cacheID)
}

func MakeDetailBtnCIDV2(commandID, cacheID, searchID string) string {
	return fmt.Sprintf("%s:%s:D:%s", commandID, cacheID, searchID)
}

// 產生回到主頁的CID
//
// CID標示符是H
func MakeBackToHomeCIDV2(commandID, cacheID string) string {
	return fmt.Sprintf("%s:%s:H:", commandID, cacheID)
}

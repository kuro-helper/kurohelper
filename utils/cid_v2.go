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
		commandName string
		routeKey    string
		behaviorID  BehaviorID
		cacheID     string
		value       string
	}

	BehaviorID string

	// 翻頁CID
	PageCIDV2 struct {
		CommandName string
		RouteKey    string
		BehaviorID  BehaviorID
		CacheID     string
		Value       int
	}

	// 選單CID
	SelectMenuCIDV2 struct {
		CommandName string
		RouteKey    string
		BehaviorID  BehaviorID
		CacheID     string
		Value       string
	}

	DetailBtnCIDV2 struct {
		CommandName string
		RouteKey    string
		BehaviorID  BehaviorID
		CacheID     string
		Value       string
	}

	// 切換來源CID
	SwitchSourceCIDV2 struct {
		CommandName string
		RouteKey    string
		BehaviorID  BehaviorID
		CacheID     string
		Value       string
	}

	// 回到主頁CID
	BackToHomeCIDV2 struct {
		CommandName string
		RouteKey    string
		BehaviorID  BehaviorID
		CacheID     string
		// 回到主頁CID不需要Value
	}

	// 使用者資料操作CID
	// Value 會存放目標資料ID(例如 gameID)
	UserDataOperationCIDV2 struct {
		CommandName string
		// 不需要RouteKey
		BehaviorID BehaviorID
		CacheID    string
		Value      int
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

	UserDataOperationBehavior BehaviorID = "U"
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
	if len(parts) != 5 {
		return nil, ErrCIDV2ParseFailed
	}

	return &CIDV2{
		commandName: parts[0],
		routeKey:    parts[1],
		cacheID:     parts[2],
		behaviorID:  BehaviorID(parts[3]),
		value:       parts[4],
	}, nil
}

// 從CIDV2獲取behaviorID
func (c CIDV2) GetBehaviorID() BehaviorID {
	return c.behaviorID
}

// 從CIDV2獲取commandName
func (c CIDV2) GetCommandName() string {
	return c.commandName
}

// 從CIDV2獲取routeKey
func (c CIDV2) GetRouteKey() string {
	return c.routeKey
}

func (c CIDV2) ToPageCIDV2() (*PageCIDV2, error) {
	v, err := strconv.Atoi(c.value)
	if err != nil {
		return nil, ErrCIDV2ParseValueFailed
	}

	return &PageCIDV2{
		CommandName: c.commandName,
		RouteKey:    c.routeKey,
		CacheID:     c.cacheID,
		BehaviorID:  c.behaviorID,
		Value:       v,
	}, nil
}

func (c CIDV2) ToSelectMenuCIDV2() *SelectMenuCIDV2 {
	return &SelectMenuCIDV2{
		CommandName: c.commandName,
		RouteKey:    c.routeKey,
		CacheID:     c.cacheID,
		BehaviorID:  c.behaviorID,
		Value:       c.value,
	}
}

func (c CIDV2) ToDetailBtnCIDV2() *DetailBtnCIDV2 {
	return &DetailBtnCIDV2{
		CommandName: c.commandName,
		RouteKey:    c.routeKey,
		CacheID:     c.cacheID,
		BehaviorID:  c.behaviorID,
		Value:       c.value,
	}
}

func (c CIDV2) ToSwitchSourceCIDV2() *SwitchSourceCIDV2 {
	return &SwitchSourceCIDV2{
		CommandName: c.commandName,
		RouteKey:    c.routeKey,
		CacheID:     c.cacheID,
		BehaviorID:  c.behaviorID,
		Value:       c.value,
	}
}

func (c CIDV2) ToBackToHomeCIDV2() *BackToHomeCIDV2 {
	return &BackToHomeCIDV2{
		CommandName: c.commandName,
		RouteKey:    c.routeKey,
		CacheID:     c.cacheID,
		BehaviorID:  c.behaviorID,
	}
}

func (c CIDV2) ToUserDataOperationCIDV2() (*UserDataOperationCIDV2, error) {
	v, err := strconv.Atoi(c.value)
	if err != nil {
		return nil, ErrCIDV2ParseValueFailed
	}

	return &UserDataOperationCIDV2{
		CommandName: c.commandName,
		CacheID:     c.cacheID,
		BehaviorID:  c.behaviorID,
		Value:       v,
	}, nil
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
func MakePageCIDV2(commandName, routeKey string, index int, cacheID string, disable bool) string {
	if disable {
		return fmt.Sprintf("%s:%s:%s:P:99", commandName, routeKey, cacheID)
	}
	return fmt.Sprintf("%s:%s:%s:P:%d", commandName, routeKey, cacheID, index)
}

// 產生select menu的CID
//
// 產生select menu的CID時不需要先預留Value，Value會在選單選擇時才設定(Discord會自動設定)
//
// CID標示符是S
func MakeSelectMenuCIDV2(commandName, routeKey, cacheID string) string {
	return fmt.Sprintf("%s:%s:%s:S:", commandName, routeKey, cacheID)
}

func MakeDetailBtnCIDV2(commandName, routeKey, cacheID, searchID string) string {
	return fmt.Sprintf("%s:%s:%s:D:%s", commandName, routeKey, cacheID, searchID)
}

// 產生回到主頁的CID
//
// CID標示符是H
func MakeBackToHomeCIDV2(commandName, routeKey, cacheID string) string {
	return fmt.Sprintf("%s:%s:%s:H:", commandName, routeKey, cacheID)
}

// 產生使用者資料操作CID
//
// CID標示符是U，Value固定放目標資料ID(例如 gameID)
func MakeUserDataOperationCIDV2(commandName, cacheID string, targetID int) string {
	return fmt.Sprintf("%s::%s:U:%d", commandName, cacheID, targetID)
}

package utils

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	kurohelpererrors "kurohelper/errors"
)

type (
	NewCID []string

	CustomIDType int

	CustomIDCommandName string
)

type (
	PageCID struct {
		NewCID
	}

	AddHasPlayedCID struct {
		NewCID
	}
)

const (
	CustomIDTypePage CustomIDType = iota + 1
	CustomIDTypeSort
	CustomIDTypeAddCommon
	CustomIDTypeAddHasPlayed
)

// 獲取CID中的CommandName欄位
func (cid NewCID) GetCommandName() string {
	return strings.Split([]string(cid)[0], "/")[0]
}

// 獲取CID類型
func (cid NewCID) GetCIDType() (CustomIDType, error) {
	value, err := strconv.Atoi([]string(cid)[1])
	if err != nil {
		return 0, kurohelpererrors.ErrCIDGetParameterFailed
	}
	return CustomIDType(value), nil
}

// 獲取CID中的CacheID欄位，之後用於查找Cache
func (cid NewCID) GetCacheID() string {
	return cid[2]
}

// 取得PageCID的頁面索引
func (cid PageCID) GetPageIndex() (int, error) {
	value, err := strconv.Atoi([]string(cid.NewCID)[3])
	if err != nil {
		return 0, kurohelpererrors.ErrCIDGetParameterFailed
	}
	return value, nil
}

// 取得AddHasPlayedCID的遊玩結束時間(非必填，沒有的話會是nil)
func (cid AddHasPlayedCID) GetCompleteDate() (*time.Time, error) {
	if strings.TrimSpace([]string(cid.NewCID)[3]) == "" {
		return nil, nil
	}

	t, err := time.Parse("20060102", []string(cid.NewCID)[3])
	if err != nil {
		return nil, kurohelpererrors.ErrCIDGetParameterFailed
	}
	return &t, nil
}

// 建立CID索引為0的字串(CommandName)
func MakeCIDCommandName(commandName string, isList bool, provider string) CustomIDCommandName {
	if isList {
		return CustomIDCommandName(commandName + "/list/" + provider)
	} else {
		return CustomIDCommandName(commandName + "//" + provider)
	}
}

// 建立事件為Page的CID
func MakeCIDPageComponent(label string, id string, value int, commandName CustomIDCommandName) *discordgo.Button {
	return &discordgo.Button{
		Label:    label,
		Style:    discordgo.PrimaryButton,
		CustomID: fmt.Sprintf("%s|%d|%s|%d", string(commandName), CustomIDTypePage, id, value),
	}
}

// 建立事件為AddHasPlayed的CID
func MakeCIDAddHasPlayedComponent(label string, id string, completDate time.Time, commandName CustomIDCommandName) *discordgo.Button {
	var t string
	if !completDate.IsZero() {
		t = completDate.Format("20060102")
	}
	return &discordgo.Button{
		Label:    label,
		Style:    discordgo.PrimaryButton,
		CustomID: fmt.Sprintf("%s|%d|%s|%s", string(commandName), CustomIDTypeAddHasPlayed, id, t),
	}
}

func MakeCIDCommonComponent(label string, id string, commandName CustomIDCommandName) *discordgo.Button {
	return &discordgo.Button{
		Label:    label,
		Style:    discordgo.PrimaryButton,
		CustomID: fmt.Sprintf("%s|%d|%s", string(commandName), CustomIDTypeAddCommon, id),
	}
}

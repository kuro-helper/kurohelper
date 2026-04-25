package executor

import (
	"errors"
	"log/slog"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var (
	ErrFocusedOptionNotFound     = errors.New("focused option not found")
	ErrAutocompleteQueryTooShort = errors.New("autocomplete query too short")
	ErrSearchListNotInitialized  = errors.New("searchList has not been initialized")
	ErrInvertedIndexNotFound     = errors.New("inverted index not found for first query rune")
)

// Autocomplete 共用邏輯
func GetAutocomplete(
	s *discordgo.Session,
	i *discordgo.InteractionCreate,
	searchList []string,
	invertedIndex map[rune][]int) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	data := i.ApplicationCommandData()

	// 找出目前使用者正在打字的那個選項
	var focusedOption *discordgo.ApplicationCommandInteractionDataOption
	for _, opt := range data.Options {
		if opt.Focused {
			focusedOption = opt
			break
		}
	}

	if focusedOption == nil {
		return nil, ErrFocusedOptionNotFound
	}

	// 取得目前輸入的文字
	userInput := focusedOption.StringValue()
	userInput = strings.ToLower(userInput)
	queryRunes := []rune(userInput)

	if len(queryRunes) < 2 {
		return nil, ErrAutocompleteQueryTooShort
	}

	var choices []*discordgo.ApplicationCommandOptionChoice
	if len(searchList) == 0 {
		slog.Warn("searchList has not been initialized...")
		return nil, ErrSearchListNotInitialized
	}

	targetIndices, ok := invertedIndex[queryRunes[0]]
	if !ok {
		return nil, ErrInvertedIndexNotFound
	}

	limit := 20

	for _, idx := range targetIndices {
		name := searchList[idx]
		if strings.Contains(strings.ToLower(name), userInput) {
			choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
				Name:  name,
				Value: name,
			})
		}

		if len(choices) >= limit {
			break
		}
	}

	return choices, nil
}

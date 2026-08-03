package kuro

import "strings"

type kuroTextCommand struct {
	Name string
	Args []string
}

func prepareKuroTrigger(content, prefix, botUserID string, mentioned bool) (string, bool) {
	content = strings.TrimSpace(content)
	hasPrefix := prefix != "" && strings.HasPrefix(content, prefix)
	if prefix != "" && !hasPrefix && !mentioned {
		return content, false
	}
	if mentioned && botUserID != "" {
		content = strings.ReplaceAll(content, "<@"+botUserID+">", " ")
		content = strings.ReplaceAll(content, "<@!"+botUserID+">", " ")
		content = strings.Join(strings.Fields(content), " ")
	}
	return strings.TrimSpace(content), true
}

// parseKuroTextCommand parses Discord management commands in the form
// "<trigger prefix> /<command> [arguments]".
func parseKuroTextCommand(content, prefix string) (kuroTextCommand, bool) {
	content = strings.TrimSpace(content)
	prefix = strings.TrimSpace(prefix)
	if prefix == "" || !strings.HasPrefix(content, prefix) {
		return kuroTextCommand{}, false
	}

	remainder := strings.TrimPrefix(content, prefix)
	if remainder != "" {
		first := []rune(remainder)[0]
		if first != '/' && first != ' ' && first != '\t' && first != '\r' && first != '\n' {
			return kuroTextCommand{}, false
		}
	}
	remainder = strings.TrimSpace(remainder)
	if !strings.HasPrefix(remainder, "/") {
		return kuroTextCommand{}, false
	}

	fields := strings.Fields(strings.TrimPrefix(remainder, "/"))
	if len(fields) == 0 {
		return kuroTextCommand{Name: "help"}, true
	}
	command := kuroTextCommand{Name: strings.ToLower(strings.TrimSpace(fields[0]))}
	if len(fields) > 1 {
		command.Args = fields[1:]
	}
	return command, true
}

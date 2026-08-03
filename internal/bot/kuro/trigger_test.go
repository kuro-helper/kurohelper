package kuro

import (
	"reflect"
	"testing"
)

func TestParseKuroTextCommand(t *testing.T) {
	tests := []struct {
		name    string
		content string
		prefix  string
		want    kuroTextCommand
		ok      bool
	}{
		{name: "English new chat", content: "小黑 /newchat", prefix: "小黑", want: kuroTextCommand{Name: "newchat"}, ok: true},
		{name: "Memory list page", content: "小黑 /memory-list 30", prefix: "小黑", want: kuroTextCommand{Name: "memory-list", Args: []string{"30"}}, ok: true},
		{name: "Compact form", content: "小黑/status", prefix: "小黑", want: kuroTextCommand{Name: "status"}, ok: true},
		{name: "Empty command is help", content: "小黑 /", prefix: "小黑", want: kuroTextCommand{Name: "help"}, ok: true},
		{name: "Ordinary chat", content: "小黑 今天好嗎", prefix: "小黑", ok: false},
		{name: "Bare slash command", content: "/newchat", prefix: "小黑", ok: false},
		{name: "Prefix collision", content: "小黑貓 /newchat", prefix: "小黑", ok: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := parseKuroTextCommand(test.content, test.prefix)
			if ok != test.ok || !reflect.DeepEqual(got, test.want) {
				t.Fatalf("parseKuroTextCommand() = (%#v, %t), want (%#v, %t)", got, ok, test.want, test.ok)
			}
		})
	}
}

func TestPrepareKuroTriggerSupportsPrefixAndMention(t *testing.T) {
	if text, ok := prepareKuroTrigger("小黑早安", "小黑", "bot", false); !ok || text != "小黑早安" {
		t.Fatalf("prefix was not preserved: %q %v", text, ok)
	}
	if text, ok := prepareKuroTrigger("<@bot> 早安", "小黑", "bot", true); !ok || text != "早安" {
		t.Fatalf("mention was not removed: %q %v", text, ok)
	}
	if _, ok := prepareKuroTrigger("大家早安", "小黑", "bot", false); ok {
		t.Fatal("unaddressed message should not trigger")
	}
}

package event

import (
	"encoding/json"
	"fmt"
	"unicode/utf8"
)

// Decision は hook が決めた停止またはツールの扱いである。空値は介入しない。
type Decision string

const (
	Allow Decision = "allow"
	Block Decision = "block"
	Deny  Decision = "deny"
)

// Result は hook の処理結果である。判断や要約の作成は hook が担う。
// AdditionalContext は SessionStart・PreToolUse・UserPromptSubmit で使う。
// Stop の問いなど、人向けの通知は SystemMessage で渡す。
type Result struct {
	Decision          Decision
	Reason            string
	AdditionalContext string
	Summary           string
	SystemMessage     string
}

type output struct {
	Decision      string          `json:"decision,omitempty"`
	Reason        string          `json:"reason,omitempty"`
	SystemMessage string          `json:"systemMessage,omitempty"`
	Specific      *specificOutput `json:"hookSpecificOutput,omitempty"`
}

type specificOutput struct {
	Name       Name   `json:"hookEventName"`
	Permission string `json:"permissionDecision,omitempty"`
	Reason     string `json:"permissionDecisionReason,omitempty"`
	Context    string `json:"additionalContext,omitempty"`
}

// Format は処理結果をホストの出力 JSON にする。何もしない場合も {} を返す。
// 本文が上限を超えると Summary と全文への案内を使う。
func Format(host Host, name Name, result Result) ([]byte, error) {
	if err := validHost(host); err != nil {
		return nil, err
	}
	if err := validName(name); err != nil {
		return nil, err
	}
	out := output{SystemMessage: limited(host, result.SystemMessage)}
	var specific specificOutput
	specific.Name = name
	switch result.Decision {
	case "":
	case Block:
		if name != Stop {
			return nil, fmt.Errorf("%s では block を出力できません", name)
		}
		out.Decision = "block"
		out.Reason = limited(host, result.Reason)
	case Allow, Deny:
		if name != PreToolUse {
			return nil, fmt.Errorf("%s ではツールの許可・拒否を出力できません", name)
		}
		specific.Permission = string(result.Decision)
		specific.Reason = limited(host, result.Reason)
	default:
		return nil, fmt.Errorf("未対応の処理結果です: %q", result.Decision)
	}
	if result.AdditionalContext != "" {
		switch name {
		case SessionStart, PreToolUse, UserPromptSubmit:
		default:
			return nil, fmt.Errorf("%s では additionalContext を出力できません", name)
		}
		context := result.AdditionalContext
		if !fits(host, context) {
			const guide = "\n`devkit status` で全文"
			context = limitedWithSuffix(host, result.Summary, guide)
		}
		specific.Context = context
	}
	if specific.Permission != "" || specific.Context != "" {
		out.Specific = &specific
	}
	return json.Marshal(out)
}

func fits(host Host, text string) bool {
	if host == ClaudeCode {
		return utf8.RuneCountInString(text) <= 10000
	}
	// トークナイザーは持たず、約 2500 トークンに対して UTF-8 のバイト数で抑える。
	return len(text) <= 2500
}

func limited(host Host, text string) string {
	return limitedWithSuffix(host, text, "")
}

func limitedWithSuffix(host Host, text, suffix string) string {
	if host == ClaudeCode {
		runes := []rune(text)
		limit := 10000 - utf8.RuneCountInString(suffix)
		if len(runes) > limit {
			text = string(runes[:limit])
		}
	} else {
		limit := 2500 - len(suffix)
		if len(text) > limit {
			for !utf8.RuneStart(text[limit]) {
				limit--
			}
			text = text[:limit]
		}
	}
	return text + suffix
}

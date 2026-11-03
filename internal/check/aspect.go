package check

import (
	"fmt"
	"strings"

	"github.com/ildjenty/dev-kit/internal/form"
)

// Aspect は form の観点テンプレートを解析した値である。
type Aspect = form.AspectTemplate

// ParseAspect は観点の見出しと4つの項目を解析する。ファイルは読まない。
func ParseAspect(source string) (Aspect, error) {
	var aspect Aspect
	fields := map[string]*string{
		"確認事項": &aspect.Checks, "blocking": &aspect.Blocking,
		"報告しない事項": &aspect.DoNotReport, "出力形式": &aspect.OutputFormat,
	}
	seen := make(map[string]bool)
	var current *string
	for _, line := range strings.Split(source, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") && aspect.ID == "" {
			aspect.ID = strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
			continue
		}
		if strings.HasPrefix(line, "- ") {
			label, value, found := strings.Cut(strings.TrimPrefix(line, "- "), ":")
			if field, ok := fields[label]; found && ok {
				if seen[label] {
					return Aspect{}, fmt.Errorf("観点の項目が重複: %s", label)
				}
				seen[label] = true
				*field = strings.TrimSpace(value)
				current = field
				continue
			}
		}
		if current != nil {
			*current += "\n" + line
		} else if trimmed != "" {
			return Aspect{}, fmt.Errorf("観点の形式にない行: %s", line)
		}
	}
	if aspect.ID == "" {
		return Aspect{}, fmt.Errorf("観点 ID が必要です")
	}
	for _, label := range []string{"確認事項", "blocking", "報告しない事項", "出力形式"} {
		field := fields[label]
		*field = strings.TrimSpace(*field)
		if *field == "" {
			return Aspect{}, fmt.Errorf("観点の項目が必要です: %s", label)
		}
	}
	return aspect, nil
}

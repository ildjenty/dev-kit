package check

import (
	"fmt"
	"strings"

	"github.com/ildjenty/dev-kit/internal/form"
)

// Aspect is a parsed form aspect template.
type Aspect = form.AspectTemplate

// ParseAspect parses the aspect heading and four fields without reading files.
func ParseAspect(source string) (Aspect, error) {
	var aspect Aspect
	fields := map[string]*string{
		"checks": &aspect.Checks, "blocking": &aspect.Blocking,
		"do_not_report": &aspect.DoNotReport, "output_format": &aspect.OutputFormat,
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
			// Preserve existing field keys alongside the English kit labels.
			switch label {
			case "Checks", "\u78ba\u8a8d\u4e8b\u9805":
				label = "checks"
			case "Do not report", "\u5831\u544a\u3057\u306a\u3044\u4e8b\u9805":
				label = "do_not_report"
			case "Output format", "\u51fa\u529b\u5f62\u5f0f":
				label = "output_format"
			}
			if field, ok := fields[label]; found && ok {
				if seen[label] {
					return Aspect{}, fmt.Errorf("duplicate aspect field: %s", label)
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
			return Aspect{}, fmt.Errorf("invalid aspect line: %s", line)
		}
	}
	if aspect.ID == "" {
		return Aspect{}, fmt.Errorf("aspect ID is required")
	}
	for _, label := range []string{"checks", "blocking", "do_not_report", "output_format"} {
		field := fields[label]
		*field = strings.TrimSpace(*field)
		if *field == "" {
			return Aspect{}, fmt.Errorf("aspect field is required: %s", label)
		}
	}
	return aspect, nil
}

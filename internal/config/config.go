package config

import (
	"bytes"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode"

	"github.com/BurntSushi/toml"
)

// Config represents validated devkit.toml settings.
type Config struct {
	KitVersion string                       `toml:"kit_version"`
	Judges     map[string]Judge             `toml:"judges"`
	Commands   map[string]Command           `toml:"commands"`
	External   map[string]ExternalOperation `toml:"external"`
	Deny       Deny                         `toml:"deny"`
	Router     Router                       `toml:"router"`
}

// Command registers a command to execute without shell expansion.
type Command struct {
	Executable     string   `toml:"executable"`
	Args           []string `toml:"args"`
	TimeoutSeconds int64    `toml:"timeout_seconds"`
}

// Timeout returns the registered time limit.
func (c Command) Timeout() time.Duration {
	return time.Duration(c.TimeoutSeconds) * time.Second
}

// Judge registers an evaluation command with an output schema.
type Judge struct {
	Command
	OutputSchema string `toml:"output_schema"`
}

// ExternalOperation registers an external operation and an optional query command.
// occurrence_field specifies the input field identifying an occurrence of an operation repeated with the same input.
type ExternalOperation struct {
	Command
	Query           *Command `toml:"query"`
	OccurrenceField string   `toml:"occurrence_field"`
}

// Deny holds rules for all bound sessions and for devkit judge and planner subagents.
type Deny struct {
	Rules []string `toml:"rules"`
	Judge []string `toml:"judge"`
}

// Router holds additions to the default classification table.
type Router struct {
	HumanConfirmation []string `toml:"human_confirmation"`
}

// Parse parses and validates supplied TOML without reading files.
func Parse(data []byte) (Config, error) {
	var c Config
	metadata, err := toml.NewDecoder(bytes.NewReader(data)).Decode(&c)
	if err != nil {
		return Config{}, fmt.Errorf("parse configuration: %w", err)
	}
	if keys := metadata.Undecoded(); len(keys) != 0 {
		return Config{}, fmt.Errorf("unknown configuration key: %s", keys[0])
	}
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

// Validate checks required settings and formats without checking path existence or kit compatibility.
func (c Config) Validate() error {
	if strings.TrimSpace(c.KitVersion) == "" {
		return fmt.Errorf("kit_version is required")
	}
	for name, command := range c.Commands {
		if err := validateCommand("commands", name, command); err != nil {
			return err
		}
	}
	for name, judge := range c.Judges {
		if err := validateCommand("judges", name, judge.Command); err != nil {
			return err
		}
		if strings.TrimSpace(judge.OutputSchema) == "" {
			return fmt.Errorf("judges.%s.output_schema is required", name)
		}
	}
	for name, operation := range c.External {
		if err := validateCommand("external", name, operation.Command); err != nil {
			return err
		}
		if operation.Query != nil {
			if err := validateCommand("external", name+".query", *operation.Query); err != nil {
				return err
			}
		}
		if operation.OccurrenceField != "" && strings.TrimSpace(operation.OccurrenceField) == "" {
			return fmt.Errorf("external.%s.occurrence_field must not contain only whitespace", name)
		}
	}
	for _, rule := range c.Deny.Rules {
		if !validDenyRule(rule) {
			return fmt.Errorf("invalid deny.rules format: %q", rule)
		}
	}
	for _, tool := range c.Deny.Judge {
		if tool == "" || strings.ContainsAny(tool, "()") || strings.IndexFunc(tool, unicode.IsSpace) >= 0 {
			return fmt.Errorf("invalid deny.judge tool name: %q", tool)
		}
	}
	for _, item := range c.Router.HumanConfirmation {
		if strings.TrimSpace(item) == "" {
			return fmt.Errorf("router.human_confirmation entries must not be empty")
		}
	}
	return nil
}

func validateCommand(section, name string, command Command) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("%s registration name must not be empty", section)
	}
	if strings.TrimSpace(command.Executable) == "" {
		return fmt.Errorf("%s.%s.executable is required", section, name)
	}
	if command.TimeoutSeconds <= 0 || command.TimeoutSeconds > math.MaxInt64/int64(time.Second) {
		return fmt.Errorf("%s.%s.timeout_seconds must be a positive number of seconds that fits in time.Duration", section, name)
	}
	return nil
}

func validDenyRule(rule string) bool {
	if strings.HasPrefix(rule, "Bash(") {
		return strings.HasSuffix(rule, " *)") && strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(rule, "Bash("), " *)")) != ""
	}
	if strings.HasPrefix(rule, "Agent(") {
		attribute := strings.TrimSuffix(strings.TrimPrefix(rule, "Agent("), ")")
		return strings.HasSuffix(rule, ")") && strings.TrimSpace(attribute) != "" && !strings.ContainsAny(attribute, "()")
	}
	return rule != "" && !strings.ContainsAny(rule, "()") && strings.IndexFunc(rule, unicode.IsSpace) < 0
}

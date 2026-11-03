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

// Config は devkit.toml の検証済みの設定を表す。
type Config struct {
	KitVersion string                       `toml:"kit_version"`
	Judges     map[string]Judge             `toml:"judges"`
	Commands   map[string]Command           `toml:"commands"`
	External   map[string]ExternalOperation `toml:"external"`
	Deny       Deny                         `toml:"deny"`
	Router     Router                       `toml:"router"`
}

// Command はシェル展開せずに実行するコマンドの登録である。
type Command struct {
	Executable     string   `toml:"executable"`
	Args           []string `toml:"args"`
	TimeoutSeconds int64    `toml:"timeout_seconds"`
}

// Timeout は登録した期限を返す。
func (c Command) Timeout() time.Duration {
	return time.Duration(c.TimeoutSeconds) * time.Second
}

// Judge は出力スキーマを指定する判定コマンドの登録である。
type Judge struct {
	Command
	OutputSchema string `toml:"output_schema"`
}

// ExternalOperation は外部操作と任意の照会コマンドの登録である。
// occurrence_field は、同じ入力で繰り返す操作の発生単位となる入力項目を指定する。
type ExternalOperation struct {
	Command
	Query           *Command `toml:"query"`
	OccurrenceField string   `toml:"occurrence_field"`
}

// Deny は束縛のあるセッションに適用する禁止規則である。
type Deny struct {
	Rules []string `toml:"rules"`
}

// Router は分類表の既定に追記する項目である。
type Router struct {
	HumanConfirmation []string `toml:"human_confirmation"`
}

// Parse は渡された TOML を解析・検証する。ファイルの読み出しは行わない。
func Parse(data []byte) (Config, error) {
	var c Config
	metadata, err := toml.NewDecoder(bytes.NewReader(data)).Decode(&c)
	if err != nil {
		return Config{}, fmt.Errorf("設定の解析: %w", err)
	}
	if keys := metadata.Undecoded(); len(keys) != 0 {
		return Config{}, fmt.Errorf("未定義の設定項目: %s", keys[0])
	}
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

// Validate は設定値の必須項目と形式を検証する。パスの存在や kit との一致は確認しない。
func (c Config) Validate() error {
	if strings.TrimSpace(c.KitVersion) == "" {
		return fmt.Errorf("kit_version が必要です")
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
			return fmt.Errorf("judges.%s.output_schema が必要です", name)
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
			return fmt.Errorf("external.%s.occurrence_field は空白だけにできません", name)
		}
	}
	for _, rule := range c.Deny.Rules {
		if !validDenyRule(rule) {
			return fmt.Errorf("deny.rules の形式が不正です: %q", rule)
		}
	}
	for _, item := range c.Router.HumanConfirmation {
		if strings.TrimSpace(item) == "" {
			return fmt.Errorf("router.human_confirmation の項目は空にできません")
		}
	}
	return nil
}

func validateCommand(section, name string, command Command) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("%s の登録名は空にできません", section)
	}
	if strings.TrimSpace(command.Executable) == "" {
		return fmt.Errorf("%s.%s.executable が必要です", section, name)
	}
	if command.TimeoutSeconds <= 0 || command.TimeoutSeconds > math.MaxInt64/int64(time.Second) {
		return fmt.Errorf("%s.%s.timeout_seconds は time.Duration に収まる正の秒数が必要です", section, name)
	}
	return nil
}

func validDenyRule(rule string) bool {
	if strings.HasPrefix(rule, "Bash(") {
		return strings.HasSuffix(rule, " *)") && strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(rule, "Bash("), " *)")) != ""
	}
	if strings.HasPrefix(rule, "Agent(") {
		return strings.HasSuffix(rule, ")") && strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(rule, "Agent("), ")")) != ""
	}
	return rule != "" && !strings.ContainsAny(rule, "()") && strings.IndexFunc(rule, unicode.IsSpace) < 0
}

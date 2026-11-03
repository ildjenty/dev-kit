package form

import (
	"encoding/json"
	"fmt"
	"strings"
)

// RenderMarkdown は固定フォーマットの値を人向けに描画する。
// Progress、JudgmentResult、PlannerResult、Finding、Question、Instruction、Decision、Evidence の値を受け取る。
func RenderMarkdown(value any) (string, error) {
	var m markdown
	switch v := value.(type) {
	case Progress:
		m.title("作業結果")
		m.field("終了の種別", string(v.TerminationKind))
		m.field("指示 ID", v.InstructionID)
		m.list("実施内容", v.WorkPerformed)
		m.list("確認済み事項", v.Confirmed)
		m.list("未確認事項", v.Unconfirmed)
		m.list("障害", v.Obstacles)
		for _, state := range v.TemporaryState {
			m.heading("一時状態")
			m.field("内容", state.Content)
			m.field("確認方法", state.VerificationMethod)
			m.field("出典", state.Source)
			m.field("無効になる条件", state.InvalidWhen)
		}
		m.list("再開条件", v.ResumeConditions)
		m.list("対象外の作業", v.OutOfScope)
		m.field("変更理由", v.ChangeReason)
	case JudgmentResult:
		m.title("判定結果")
		m.field("判定", string(v.Judgment))
		m.field("観点", v.Aspect)
		m.field("指示 ID", v.InstructionID)
		if err := m.json("対象の識別", v.TargetVersion); err != nil {
			return "", err
		}
		if err := m.json("規範の識別", v.Norms); err != nil {
			return "", err
		}
		m.list("確認した完了条件 ID", v.CompletionConditionIDs)
		m.list("確認した範囲", v.CheckedScope)
		m.list("未確認の範囲", v.UncheckedScope)
		m.field("方法", v.Method)
		m.field("環境", v.Environment)
		for _, finding := range v.Findings {
			m.finding(finding)
		}
	case PlannerResult:
		m.title("計画者の結果")
		m.field("指示 ID", v.InstructionID)
		if err := m.json("計画", v.Plan); err != nil {
			return "", err
		}
	case Finding:
		m.finding(v)
	case Question:
		if err := m.question(v); err != nil {
			return "", err
		}
	case Instruction:
		m.title("次の指示")
		m.field("指示 ID", v.ID)
		m.field("種別", string(v.Kind))
		m.field("対象のキー", v.TargetKey)
		m.field("出し直し回数", fmt.Sprint(v.Reissue))
		m.field("指示", v.Text)
		m.field("brief", v.Brief)
		m.field("対象の展開先", v.TargetPath)
		m.field("実行主体", v.Runner)
		if v.Question != nil {
			if err := m.question(*v.Question); err != nil {
				return "", err
			}
		}
		if v.External != nil {
			if err := m.json("外部操作の確認対象", v.External); err != nil {
				return "", err
			}
		}
	case Decision:
		m.title("決定")
		m.field("内容", v.Content)
		m.field("決定 ID", v.ID)
		m.field("有効性", v.Validity)
		if v.Provisional {
			m.field("確定の区分", "暫定")
		} else {
			m.field("確定の区分", "確定")
		}
		m.field("決定者", v.Decider)
		m.list("適用条件", v.Conditions)
		m.references("置換元", v.Supersedes)
		m.references("依存する決定", v.DependsOn)
		m.references("証拠", v.Evidence)
		for _, alternative := range v.RejectedAlternatives {
			m.heading("却下した案")
			m.field("案", alternative.Proposal)
			m.field("理由", alternative.Reason)
			m.field("具体例", alternative.Example)
		}
	case Evidence:
		m.title("証拠")
		m.field("調査結果", v.Content)
		m.field("証拠 ID", v.ID)
		m.references("出典", v.Sources)
		m.field("確認方法", v.VerificationMethod)
	default:
		return "", fmt.Errorf("Markdown の描画対象ではない型: %T", value)
	}
	return m.String(), nil
}

type markdown struct {
	strings.Builder
}

func (m *markdown) title(title string) {
	m.WriteString("# " + title + "\n\n")
}

func (m *markdown) heading(title string) {
	m.WriteString("## " + title + "\n\n")
}

func (m *markdown) field(label, value string) {
	if value != "" {
		m.WriteString(label + ": " + value + "\n\n")
	}
}

func (m *markdown) list(title string, items []string) {
	if len(items) == 0 {
		return
	}
	m.heading(title)
	for _, item := range items {
		m.WriteString("- " + strings.ReplaceAll(item, "\n", "\n  ") + "\n")
	}
	m.WriteString("\n")
}

func (m *markdown) references(title string, refs []Reference) {
	items := make([]string, 0, len(refs))
	for _, ref := range refs {
		if ref.RecordID != "" {
			items = append(items, "記録: "+ref.RecordID)
		} else {
			items = append(items, "外部参照: "+ref.External)
		}
	}
	m.list(title, items)
}

func (m *markdown) json(title string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	m.heading(title)
	m.WriteString("```json\n" + string(data) + "\n```\n\n")
	return nil
}

func (m *markdown) finding(v Finding) {
	m.heading("指摘 " + v.ID)
	m.field("種別", string(v.Kind))
	m.field("blocking", fmt.Sprint(v.Blocking))
	m.field("失敗のシナリオ", v.Scenario)
	if v.Exemption != nil {
		m.field("適用除外の主張", v.Exemption.Claim)
		m.references("適用除外の根拠", v.Exemption.Evidence)
	}
	m.list("解決した指摘 ID", v.ResolvedFindingIDs)
}

func (m *markdown) question(v Question) error {
	m.title("確認事項")
	m.WriteString(v.Text + "\n\n")
	m.field("問い ID", v.ID)
	m.field("画面 ID", v.ScreenID)
	m.field("停止しているステージ", v.StoppedStage)
	m.field("停止理由の分類", v.StopCategory)
	m.field("停止理由", v.StopReason)
	m.field("回答後に進むステージ", v.NextStage)
	if err := m.json("承認対象", v.ApprovalTarget); err != nil {
		return err
	}
	m.list("選択肢", v.Options)
	m.field("推奨", v.Recommendation)
	m.field("推奨の理由", v.RecommendationReason)
	m.references("根拠", v.Evidence)
	if v.StopObservation != nil {
		m.references("停止の観測", []Reference{*v.StopObservation})
	}
	m.WriteString("自由な言葉で回答してください。選択肢にない答え、条件付きの答え、質問返しもできます。後から回答を言い直すこともできます。\n\n")
	return nil
}

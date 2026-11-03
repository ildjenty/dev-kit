package check

import (
	"encoding/json"
	"fmt"

	"github.com/ildjenty/dev-kit/internal/form"
)

type RequestKind string

const (
	RoundRequest     RequestKind = "round"
	PlanRequest      RequestKind = "plan"
	DetectionRequest RequestKind = "detection"
)

// Norm は固定済みの規範の識別と本文である。brief と計画も規範として渡す。
type Norm struct {
	Reference form.NormReference `json:"reference"`
	Content   string             `json:"content"`
}

// Authorization は判定対象に適用される承認または委任の範囲である。
type Authorization struct {
	Target   form.ApprovalTarget `json:"target"`
	Scope    string              `json:"scope"`
	Evidence []form.Reference    `json:"evidence"`
}

// RequestInput は呼び出し側が解決した判定の入力である。
// ラウンドでは Progress、差分の観点では有効な Authorizations の一覧を渡す。
type RequestInput struct {
	Kind                   RequestKind
	InstructionID          string
	Runner                 string
	Aspect                 Aspect
	TargetVersion          form.TargetVersion
	TargetPath             string
	Norms                  []Norm
	OutputSchema           json.RawMessage
	CompletionConditionIDs []string
	Progress               *form.Progress
	PreviousFindings       []Finding
	Authorizations         []Authorization
}

// Request は brief に渡す判定依頼の内容であり、依頼文自体は組み立てない。
type Request struct {
	RequestInput
	PreviousFindingIDs []string
}

// PrepareRequest は修正ラウンドの指摘 ID を付けて依頼の内容を決める。
func PrepareRequest(input RequestInput) (Request, error) {
	switch input.Kind {
	case RoundRequest:
		if input.Progress == nil {
			return Request{}, fmt.Errorf("ラウンドの判定には終了ブロックが必要です")
		}
	case PlanRequest, DetectionRequest:
	default:
		return Request{}, fmt.Errorf("不明な判定依頼の種別: %q", input.Kind)
	}
	if input.InstructionID == "" || input.Runner == "" || input.Aspect.ID == "" || input.TargetPath == "" {
		return Request{}, fmt.Errorf("指示 ID・実行主体・観点・対象の場所が必要です")
	}
	if !json.Valid(input.OutputSchema) {
		return Request{}, fmt.Errorf("出力スキーマの JSON が必要です")
	}
	request := Request{RequestInput: input, PreviousFindingIDs: []string{}}
	for _, finding := range input.PreviousFindings {
		request.PreviousFindingIDs = append(request.PreviousFindingIDs, finding.ID)
	}
	return request, nil
}

func (r Request) normReferences() []form.NormReference {
	refs := make([]form.NormReference, 0, len(r.Norms))
	for _, norm := range r.Norms {
		refs = append(refs, norm.Reference)
	}
	return refs
}

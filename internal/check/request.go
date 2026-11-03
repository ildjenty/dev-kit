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

// Norm holds a fixed norm identity and content. Provide the brief and plan as norms too.
type Norm struct {
	Reference form.NormReference `json:"reference"`
	Content   string             `json:"content"`
}

// Authorization describes an approval or delegation scope applicable to the judgment target.
type Authorization struct {
	Target   form.ApprovalTarget `json:"target"`
	Scope    string              `json:"scope"`
	Evidence []form.Reference    `json:"evidence"`
}

// RequestInput holds judgment inputs resolved by the caller.
// Provide Progress for rounds and valid Authorizations for the diff aspect.
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

// Request holds judgment request content for brief without composing the request text.
type Request struct {
	RequestInput
	PreviousFindingIDs []string
}

// PrepareRequest prepares request content with finding IDs for correction rounds.
func PrepareRequest(input RequestInput) (Request, error) {
	switch input.Kind {
	case RoundRequest:
		if input.Progress == nil {
			return Request{}, fmt.Errorf("round judgment requires a termination block")
		}
	case PlanRequest, DetectionRequest:
	default:
		return Request{}, fmt.Errorf("unknown judgment request kind: %q", input.Kind)
	}
	if input.InstructionID == "" || input.Runner == "" || input.Aspect.ID == "" || input.TargetPath == "" {
		return Request{}, fmt.Errorf("instruction ID, runner, aspect, and target path are required")
	}
	if !json.Valid(input.OutputSchema) {
		return Request{}, fmt.Errorf("output schema JSON is required")
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

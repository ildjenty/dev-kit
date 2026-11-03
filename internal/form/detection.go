package form

// DetectionObservation reports an independent assessment of one detection example.
type DetectionObservation struct {
	File               string         `json:"file"`
	Judgment           JudgmentResult `json:"judgment"`
	Reason             string         `json:"reason"`
	MatchesBlocking    bool           `json:"matches_blocking"`
	RespectsDifference bool           `json:"respects_difference"`
}

// DetectionResult is the final response to a detection test instruction.
type DetectionResult struct {
	InstructionID string                 `json:"instruction_id"`
	Observations  []DetectionObservation `json:"observations"`
}

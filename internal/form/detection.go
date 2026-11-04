package form

// DetectionObservation reports an independent assessment of one detection example.
type DetectionObservation struct {
	File     string         `json:"file"`
	Judgment JudgmentResult `json:"judgment"`
	Reason   string         `json:"reason"`
}

// DetectionResult is the final response to a detection test instruction.
type DetectionResult struct {
	InstructionID string                 `json:"instruction_id"`
	Observations  []DetectionObservation `json:"observations"`
}

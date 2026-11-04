package form

// DetectionFinding is a blocking finding reported for one detection example.
type DetectionFinding struct {
	Scenario string `json:"scenario"`
}

// DetectionObservation reports an independent assessment of one anonymous detection example:
// the judgment value, its blocking findings, and the grounds.
type DetectionObservation struct {
	ID       string             `json:"id"`
	Judgment JudgmentValue      `json:"judgment"`
	Blocking []DetectionFinding `json:"blocking"`
	Reason   string             `json:"reason"`
}

// DetectionResult is the final response to a detection test instruction. The shared identity
// (instruction, aspect, target, norms) appears once, as the instruction ID.
type DetectionResult struct {
	InstructionID string                 `json:"instruction_id"`
	Observations  []DetectionObservation `json:"observations"`
}

package check

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

type CaseKind string

const (
	Violation  CaseKind = "violation"
	Conforming CaseKind = "conforming"
	Allowed    CaseKind = "allowed"
)

type DetectionCase struct {
	Kind     CaseKind      `json:"kind"`
	File     string        `json:"file"`
	Expected JudgmentValue `json:"expected"`
}

type DetectionSpec struct {
	Aspect string          `json:"aspect"`
	Cases  []DetectionCase `json:"cases"`
}

// ParseDetectionSpec parses specifications for violation, conforming, and allowed examples.
func ParseDetectionSpec(data []byte) (DetectionSpec, error) {
	var spec DetectionSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return DetectionSpec{}, err
	}
	if spec.Aspect == "" || strings.HasPrefix(spec.Aspect, "command:") {
		return DetectionSpec{}, fmt.Errorf("detection test requires a judge aspect")
	}
	kinds := make(map[CaseKind]bool)
	files := make(map[string]bool)
	for _, example := range spec.Cases {
		expected := Pass
		switch example.Kind {
		case Violation:
			expected = Fail
		case Conforming, Allowed:
		default:
			return DetectionSpec{}, fmt.Errorf("unknown detection case kind: %q", example.Kind)
		}
		if example.Expected != expected || example.File == "" || files[example.File] {
			return DetectionSpec{}, fmt.Errorf("invalid detection case file or expected judgment: %q", example.File)
		}
		kinds[example.Kind], files[example.File] = true, true
	}
	if !kinds[Violation] || !kinds[Conforming] || !kinds[Allowed] {
		return DetectionSpec{}, fmt.Errorf("violation, conforming, and allowed examples are required")
	}
	return spec, nil
}

// DetectionIdentity identifies content for reusing a passing detection test.
type DetectionIdentity struct {
	AspectHash string `json:"aspect_hash"`
	SpecHash   string `json:"spec_hash"`
	RunnerHash string `json:"runner_hash"`
}

// NewDetectionIdentity hashes the original specification, example contents, and expected judgments.
// examples maps file names relative to the specification to their contents.
func NewDetectionIdentity(aspect, specSource, runner []byte, examples map[string][]byte) (DetectionIdentity, error) {
	spec, err := ParseDetectionSpec(specSource)
	if err != nil {
		return DetectionIdentity{}, err
	}
	contents := make([][]byte, 0, len(spec.Cases)+1)
	contents = append(contents, specSource)
	for _, example := range spec.Cases {
		content, ok := examples[example.File]
		if !ok {
			return DetectionIdentity{}, fmt.Errorf("detection example content not found: %q", example.File)
		}
		contents = append(contents, content)
	}
	data, err := json.Marshal(contents)
	if err != nil {
		return DetectionIdentity{}, err
	}
	return DetectionIdentity{contentHash(aspect), contentHash(data), contentHash(runner)}, nil
}

func contentHash(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }

// DetectionObservation holds an actual judgment and an assessment of whether it used the intended reason.
// MatchesBlocking applies to violations; RespectsDifference applies to conforming and allowed examples.
type DetectionObservation struct {
	File               string   `json:"file"`
	Judgment           Judgment `json:"judgment"`
	Reason             string   `json:"reason"`
	MatchesBlocking    bool     `json:"matches_blocking"`
	RespectsDifference bool     `json:"respects_difference"`
}

type DetectionCaseResult struct {
	DetectionCase
	Observation DetectionObservation `json:"observation"`
	Passed      bool                 `json:"passed"`
}

type DetectionTest struct {
	Aspect   string                `json:"aspect"`
	Runner   string                `json:"runner"`
	Identity DetectionIdentity     `json:"identity"`
	Cases    []DetectionCaseResult `json:"cases"`
	Passed   bool                  `json:"passed"`
}

// AssessDetection compares parsed specifications and results against expected judgments and reasons.
func AssessDetection(spec DetectionSpec, runner string, identity DetectionIdentity, observations []DetectionObservation) (DetectionTest, error) {
	if runner == "" || strings.HasPrefix(runner, "command:") {
		return DetectionTest{}, fmt.Errorf("detection test requires a judge runner")
	}
	if identity.AspectHash == "" || identity.SpecHash == "" || identity.RunnerHash == "" {
		return DetectionTest{}, fmt.Errorf("detection test content hashes are required")
	}
	byFile := make(map[string]DetectionObservation)
	for _, observation := range observations {
		if _, exists := byFile[observation.File]; exists {
			return DetectionTest{}, fmt.Errorf("duplicate detection test result: %q", observation.File)
		}
		byFile[observation.File] = observation
	}
	if len(byFile) != len(spec.Cases) {
		return DetectionTest{}, fmt.Errorf("detection test results are incomplete")
	}
	result := DetectionTest{Aspect: spec.Aspect, Runner: runner, Identity: identity, Cases: []DetectionCaseResult{}, Passed: true}
	for _, example := range spec.Cases {
		observation, ok := byFile[example.File]
		if !ok {
			return DetectionTest{}, fmt.Errorf("detection test result not found: %q", example.File)
		}
		judgment := observation.Judgment
		if judgment.Aspect != spec.Aspect || judgment.Runner != runner {
			return DetectionTest{}, fmt.Errorf("detection test aspect or runner does not match: %q", example.File)
		}
		blocking := slices.ContainsFunc(judgment.Findings, func(f Finding) bool { return f.Blocking })
		passed := judgment.Judgment == example.Expected && observation.Reason != ""
		if example.Kind == Violation {
			passed = passed && blocking && observation.MatchesBlocking
		} else {
			passed = passed && !blocking && observation.RespectsDifference
		}
		result.Cases = append(result.Cases, DetectionCaseResult{example, observation, passed})
		result.Passed = result.Passed && passed
	}
	return result, nil
}

type DetectionRequirement struct {
	Aspect   string
	Runner   string
	Identity DetectionIdentity
}

// MissingDetection looks for passing records matching the current aspect, specification, and runner contents.
// Command checks do not require detection tests.
func MissingDetection(required []DetectionRequirement, records []DetectionTest) []DetectionRequirement {
	missing := []DetectionRequirement{}
	for _, requirement := range required {
		if strings.HasPrefix(requirement.Runner, "command:") {
			continue
		}
		if requirement.Identity.AspectHash == "" || requirement.Identity.SpecHash == "" || requirement.Identity.RunnerHash == "" {
			missing = append(missing, requirement)
			continue
		}
		if !slices.ContainsFunc(records, func(record DetectionTest) bool {
			return record.Passed && record.Aspect == requirement.Aspect && record.Runner == requirement.Runner && record.Identity == requirement.Identity
		}) {
			missing = append(missing, requirement)
		}
	}
	return missing
}

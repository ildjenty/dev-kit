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
	ID       string        `json:"id"`
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
	for i := range spec.Cases {
		example := &spec.Cases[i]
		example.ID = fmt.Sprintf("e%d", i+1)
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

// DetectionExample is judge-visible input without paths, kinds, or expectations.
type DetectionExample struct {
	ID   string `json:"id"`
	Body string `json:"body"`
}

// PrepareDetectionExamples maps private specification files to opaque example IDs.
func PrepareDetectionExamples(spec DetectionSpec, examples map[string]string) ([]DetectionExample, error) {
	result := make([]DetectionExample, 0, len(spec.Cases))
	for _, example := range spec.Cases {
		body, ok := examples[example.File]
		if !ok {
			return nil, fmt.Errorf("detection example content not found: %q", example.File)
		}
		result = append(result, DetectionExample{ID: example.ID, Body: body})
	}
	return result, nil
}

// DetectionIdentity identifies content for reusing a passing detection test: the runner
// definition, the judgment request template with its rules, the criteria documents actually
// supplied, and the specification with its example documents.
type DetectionIdentity struct {
	RunnerHash   string `json:"runner_hash"`
	TemplateHash string `json:"template_hash"`
	CriteriaHash string `json:"criteria_hash"`
	ExamplesHash string `json:"examples_hash"`
}

// Complete reports whether every identity component is present.
func (i DetectionIdentity) Complete() bool {
	return i.RunnerHash != "" && i.TemplateHash != "" && i.CriteriaHash != "" && i.ExamplesHash != ""
}

// NewDetectionIdentity hashes the runner definition, request template, supplied criteria,
// the original specification, and the example contents keyed by file names relative to it.
func NewDetectionIdentity(runner, template []byte, criteria []Criterion, specSource []byte, examples map[string][]byte) (DetectionIdentity, error) {
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
	if len(criteria) == 0 {
		return DetectionIdentity{}, fmt.Errorf("detection test requires criteria documents")
	}
	documents := make([]string, 0, len(criteria))
	for _, criterion := range criteria {
		if criterion.ContentHash == "" {
			return DetectionIdentity{}, fmt.Errorf("criteria document hash is missing: %s/%s", criterion.Source, criterion.ID)
		}
		documents = append(documents, criterion.Source+"\x00"+criterion.ID+"\x00"+criterion.ContentHash)
	}
	slices.Sort(documents)
	set, err := json.Marshal(documents)
	if err != nil {
		return DetectionIdentity{}, err
	}
	return DetectionIdentity{RunnerHash: contentHash(runner), TemplateHash: contentHash(template), CriteriaHash: contentHash(set), ExamplesHash: contentHash(data)}, nil
}

func contentHash(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }

// DetectionObservation holds an actual judgment and its reason for an opaque example ID.
// File contains the example ID used by the detection result schema, never its source path.
type DetectionObservation struct {
	File     string   `json:"file"`
	Judgment Judgment `json:"judgment"`
	Reason   string   `json:"reason"`
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
	if !identity.Complete() {
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
		observation, ok := byFile[example.ID]
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
			passed = passed && blocking
		} else {
			passed = passed && !blocking
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

// MissingDetection looks for passing records matching the current runner, template, criteria, and examples.
// Command checks do not require detection tests.
func MissingDetection(required []DetectionRequirement, records []DetectionTest) []DetectionRequirement {
	missing := []DetectionRequirement{}
	for _, requirement := range required {
		if strings.HasPrefix(requirement.Runner, "command:") {
			continue
		}
		if !requirement.Identity.Complete() {
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

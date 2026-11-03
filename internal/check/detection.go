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

// ParseDetectionSpec は違反例・適合例・許容例の仕様を解析する。
func ParseDetectionSpec(data []byte) (DetectionSpec, error) {
	var spec DetectionSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return DetectionSpec{}, err
	}
	if spec.Aspect == "" || strings.HasPrefix(spec.Aspect, "command:") {
		return DetectionSpec{}, fmt.Errorf("検出試験には判定者の観点が必要です")
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
			return DetectionSpec{}, fmt.Errorf("不明な検出試験の種別: %q", example.Kind)
		}
		if example.Expected != expected || example.File == "" || files[example.File] {
			return DetectionSpec{}, fmt.Errorf("検出試験のファイルまたは期待判定が不正です: %q", example.File)
		}
		kinds[example.Kind], files[example.File] = true, true
	}
	if !kinds[Violation] || !kinds[Conforming] || !kinds[Allowed] {
		return DetectionSpec{}, fmt.Errorf("違反例・適合例・許容例が必要です")
	}
	return spec, nil
}

// DetectionIdentity は合格を再利用できる内容の同一性を表す。
type DetectionIdentity struct {
	AspectHash string `json:"aspect_hash"`
	SpecHash   string `json:"spec_hash"`
	RunnerHash string `json:"runner_hash"`
}

// NewDetectionIdentity は元の仕様と例の本文・期待判定を含めてハッシュを求める。
// examples は仕様からの相対ファイル名をキーにして本文を渡す。
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
			return DetectionIdentity{}, fmt.Errorf("検出試験の本文がない: %q", example.File)
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

// DetectionObservation は実際の判定と、狙った理由で判定したかの照合結果である。
// MatchesBlocking は違反例、RespectsDifference は適合例・許容例で使う。
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

// AssessDetection は解析済みの仕様と結果を受け取り、期待判定と根拠を照合する。
func AssessDetection(spec DetectionSpec, runner string, identity DetectionIdentity, observations []DetectionObservation) (DetectionTest, error) {
	if runner == "" || strings.HasPrefix(runner, "command:") {
		return DetectionTest{}, fmt.Errorf("検出試験には判定者の担当が必要です")
	}
	if identity.AspectHash == "" || identity.SpecHash == "" || identity.RunnerHash == "" {
		return DetectionTest{}, fmt.Errorf("検出試験の内容ハッシュが必要です")
	}
	byFile := make(map[string]DetectionObservation)
	for _, observation := range observations {
		if _, exists := byFile[observation.File]; exists {
			return DetectionTest{}, fmt.Errorf("検出試験の結果が重複: %q", observation.File)
		}
		byFile[observation.File] = observation
	}
	if len(byFile) != len(spec.Cases) {
		return DetectionTest{}, fmt.Errorf("検出試験の結果が揃っていません")
	}
	result := DetectionTest{Aspect: spec.Aspect, Runner: runner, Identity: identity, Cases: []DetectionCaseResult{}, Passed: true}
	for _, example := range spec.Cases {
		observation, ok := byFile[example.File]
		if !ok {
			return DetectionTest{}, fmt.Errorf("検出試験の結果がない: %q", example.File)
		}
		judgment := observation.Judgment
		if judgment.Aspect != spec.Aspect || judgment.Runner != runner {
			return DetectionTest{}, fmt.Errorf("検出試験の観点または担当が一致しません: %q", example.File)
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

// MissingDetection は現在の観点・仕様・担当の内容と一致する合格記録を探す。
// command の check は検出試験を要しない。
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

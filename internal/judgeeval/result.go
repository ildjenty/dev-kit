package judgeeval

import (
	"fmt"
	"io"
	"slices"
)

type Reported struct {
	ID              string `json:"id"`
	Quality         string `json:"quality"`
	File            string `json:"file"`
	Line            string `json:"line"`
	FailureScenario string `json:"failure_scenario"`
	Evidence        string `json:"evidence"`
}

type Match struct {
	Finding
	ReportedID  string `json:"reported_id"`
	RoutingMiss bool   `json:"routing_miss"`
}

type Counts struct {
	Expected            int `json:"expected"`
	Found               int `json:"found"`
	Missed              int `json:"missed"`
	RoutingMissed       int `json:"routing_missed"`
	NotExpected         int `json:"not_expected"`
	NotExpectedReported int `json:"not_expected_reported"`
	Other               int `json:"other"`
}

func (c *Counts) add(other Counts) {
	c.Expected += other.Expected
	c.Found += other.Found
	c.Missed += other.Missed
	c.RoutingMissed += other.RoutingMissed
	c.NotExpected += other.NotExpected
	c.NotExpectedReported += other.NotExpectedReported
	c.Other += other.Other
}

type QualityResult struct {
	Quality     string     `json:"quality"`
	Launched    bool       `json:"launched"`
	Expected    []Match    `json:"expected"`
	NotExpected []Match    `json:"not_expected"`
	Other       []Reported `json:"other"`
	Totals      Counts     `json:"totals"`
}

type QualityTotal struct {
	Quality string `json:"quality"`
	Totals  Counts `json:"totals"`
}

type CaseResult struct {
	Name              string          `json:"name"`
	Description       string          `json:"description"`
	Artifacts         string          `json:"artifacts"`
	Route             document        `json:"route"`
	LaunchedQualities []string        `json:"launched_qualities"`
	ExpectedQualities []string        `json:"expected_qualities"`
	Reported          []Reported      `json:"reported"`
	Qualities         []QualityResult `json:"qualities"`
	Totals            Counts          `json:"totals"`
	Error             string          `json:"error,omitempty"`
}

type Result struct {
	Limitation string         `json:"limitation"`
	Provider   string         `json:"provider"`
	Cases      []CaseResult   `json:"cases"`
	Qualities  []QualityTotal `json:"qualities"`
	Totals     Counts         `json:"totals"`
	Errored    []string       `json:"errored"`
}

func matchingSchema(c Case, reported []Reported) document {
	groups := document{}
	for name, findings := range map[string][]Finding{"expected": c.Expected, "not_expected": c.NotExpected} {
		properties := document{}
		for _, finding := range findings {
			ids := []string{""}
			for _, actual := range reported {
				if actual.Quality == finding.Quality {
					ids = append(ids, actual.ID)
				}
			}
			properties[finding.ID] = document{"type": "string", "enum": ids}
		}
		groups[name] = strictObject(properties)
	}
	return strictObject(groups)
}

func aggregate(c Case, launched []string, reported []Reported, answer document) (CaseResult, error) {
	if err := validate(matchingSchema(c, reported), answer); err != nil {
		return CaseResult{}, fmt.Errorf("matching: %w", err)
	}
	result := CaseResult{Name: c.Name, Description: c.Description, LaunchedQualities: launched, ExpectedQualities: []string{}, Reported: reported, Qualities: []QualityResult{}}
	qualities := append([]string{}, launched...)
	for _, item := range c.Expected {
		if !slices.Contains(result.ExpectedQualities, item.Quality) {
			result.ExpectedQualities = append(result.ExpectedQualities, item.Quality)
		}
	}
	for _, item := range append(append([]Finding{}, c.Expected...), c.NotExpected...) {
		if !slices.Contains(qualities, item.Quality) {
			qualities = append(qualities, item.Quality)
		}
	}
	slices.Sort(qualities)
	slices.Sort(result.ExpectedQualities)
	matched := map[string]bool{}
	matches := map[string][]Match{"expected": {}, "not_expected": {}}
	for group, findings := range map[string][]Finding{"expected": c.Expected, "not_expected": c.NotExpected} {
		ids := answer[group].(map[string]any)
		for _, item := range findings {
			id := ids[item.ID].(string)
			if id != "" {
				matched[id] = true
			}
			matches[group] = append(matches[group], Match{Finding: item, ReportedID: id, RoutingMiss: group == "expected" && id == "" && !slices.Contains(launched, item.Quality)})
		}
	}
	for _, q := range qualities {
		item := QualityResult{Quality: q, Launched: slices.Contains(launched, q), Expected: []Match{}, NotExpected: []Match{}, Other: []Reported{}}
		for _, match := range matches["expected"] {
			if match.Quality != q {
				continue
			}
			item.Expected = append(item.Expected, match)
			item.Totals.Expected++
			if match.ReportedID != "" {
				item.Totals.Found++
			} else {
				item.Totals.Missed++
			}
			if match.RoutingMiss {
				item.Totals.RoutingMissed++
			}
		}
		for _, match := range matches["not_expected"] {
			if match.Quality == q {
				item.NotExpected = append(item.NotExpected, match)
				item.Totals.NotExpected++
				if match.ReportedID != "" {
					item.Totals.NotExpectedReported++
				}
			}
		}
		for _, finding := range reported {
			if finding.Quality == q && !matched[finding.ID] {
				item.Other = append(item.Other, finding)
				item.Totals.Other++
			}
		}
		result.Qualities = append(result.Qualities, item)
		result.Totals.add(item.Totals)
	}
	return result, nil
}

func (r *Result) total() {
	qualities := map[string]Counts{}
	r.Errored = []string{}
	for _, c := range r.Cases {
		if c.Error != "" {
			r.Errored = append(r.Errored, c.Name)
			continue
		}
		r.Totals.add(c.Totals)
		for _, q := range c.Qualities {
			counts := qualities[q.Quality]
			counts.add(q.Totals)
			qualities[q.Quality] = counts
		}
	}
	names := []string{}
	for q := range qualities {
		names = append(names, q)
	}
	slices.Sort(names)
	for _, q := range names {
		r.Qualities = append(r.Qualities, QualityTotal{Quality: q, Totals: qualities[q]})
	}
}

func printTotals(out io.Writer, label string, counts Counts) error {
	_, err := fmt.Fprintf(out, "%s: expected %d, found %d, missed %d (routing %d), not-expected reported %d/%d, other %d (not scored as wrong)\n", label, counts.Expected, counts.Found, counts.Missed, counts.RoutingMissed, counts.NotExpectedReported, counts.NotExpected, counts.Other)
	return err
}

func printCase(out io.Writer, c CaseResult) error {
	if _, err := fmt.Fprintf(out, "  Routing source: %v; launched: %v; expected qualities: %v\n", c.Route["source"], c.LaunchedQualities, c.ExpectedQualities); err != nil {
		return err
	}
	for _, q := range c.Qualities {
		if err := printTotals(out, "  "+q.Quality, q.Totals); err != nil {
			return err
		}
		for _, finding := range q.Expected {
			status := "FOUND " + finding.ReportedID
			if finding.ReportedID == "" {
				status = "MISSED"
			}
			if finding.RoutingMiss {
				status += " (routing: quality not launched)"
			}
			if _, err := fmt.Fprintf(out, "    %s %s %s: %s\n", status, finding.ID, finding.File, finding.Summary); err != nil {
				return err
			}
		}
		for _, finding := range q.NotExpected {
			if finding.ReportedID != "" {
				if _, err := fmt.Fprintf(out, "    NOT-EXPECTED REPORTED %s (%s) %s: %s\n", finding.ID, finding.ReportedID, finding.File, finding.Summary); err != nil {
					return err
				}
			}
		}
		for _, finding := range q.Other {
			if _, err := fmt.Fprintf(out, "    OTHER %s %s:%s: %s; evidence: %s\n", finding.ID, finding.File, finding.Line, finding.FailureScenario, finding.Evidence); err != nil {
				return err
			}
		}
	}
	return printTotals(out, "  Case total", c.Totals)
}

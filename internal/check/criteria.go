package check

import "fmt"

// Aspect names a judgment. Its criteria are the documents selected by the check.
type Aspect struct {
	ID string `json:"id"`
}

// Criterion identifies one fixed criteria document selected for a judgment.
// Content is supplied to the request text and is not part of the recorded request.
type Criterion struct {
	Source      string `json:"source"`
	ID          string `json:"id"`
	ContentHash string `json:"content_hash"`
	Content     string `json:"-"`
}

// CriteriaSelection is the resolution of one criteria selection from a fixed snapshot.
type CriteriaSelection struct {
	Source    string
	Select    string
	Documents []Criterion
}

// ResolveCriteria combines resolved selections in order without duplicates.
// A selection that resolves to no documents stops the judgment before it starts.
func ResolveCriteria(selections []CriteriaSelection) ([]Criterion, error) {
	criteria := []Criterion{}
	seen := map[string]bool{}
	for _, selection := range selections {
		if len(selection.Documents) == 0 {
			return nil, fmt.Errorf("criteria selection %q from source %q matched no documents; fix the selection or add the documents before judging", selection.Select, selection.Source)
		}
		for _, document := range selection.Documents {
			key := document.Source + "\x00" + document.ID
			if !seen[key] {
				seen[key] = true
				criteria = append(criteria, document)
			}
		}
	}
	return criteria, nil
}

package record

import "fmt"

func ProposalRoundTarget(entry Document, path string) (Document, error) {
	version, exists := entry["proposal_version"]
	if !exists {
		return nil, fmt.Errorf("%s: proposal_version is required (null only before a proposal is committed)", path)
	}
	if version == nil {
		if entry["cleared"] == true || len(stringsOf(entry["errors"])) == 0 || entry["commit"] != nil && entry["commit"] != "" {
			return nil, fmt.Errorf("%s: null proposal_version requires a failed round without a committed proposal", path)
		}
	} else {
		value, ok := version.(float64)
		if n, isInt := version.(int); isInt {
			value, ok = float64(n), true
		}
		if !ok || value < 1 || value != float64(int(value)) {
			return nil, fmt.Errorf("%s: proposal_version must be a positive integer or explicit null before a proposal is committed", path)
		}
	}
	return Document{"proposal_version": version}, nil
}

func WriteLoopJudgments(ctx ContextData, judgments []Document, now string) error {
	for i, judgment := range judgments {
		for _, value := range array(judgment["findings"]) {
			finding := object(value)
			key := finding["resolution_key"]
			resolved := false
			for _, later := range judgments[i+1:] {
				if later["judge"] != judgment["judge"] || later["phase"] != judgment["phase"] || later["verdict"] == "not_assessed" {
					continue
				}
				resolved = true
				for _, next := range array(later["findings"]) {
					f := object(next)
					if key != nil && f["resolution_key"] == key || key == nil && f["text"] == finding["text"] && f["evidence"] == finding["evidence"] {
						resolved = false
					}
				}
			}
			finding["resolved"] = resolved
			delete(finding, "resolution_key")
		}
		if _, err := Write(ctx, "judgment", judgment, now); err != nil {
			return err
		}
	}
	return nil
}

package record

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

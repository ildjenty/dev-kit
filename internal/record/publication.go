package record

func RecordedCommitsOf(results, judgments []Document) map[string]bool {
	out := map[string]bool{}
	for _, r := range results {
		if s := str(r["commit"]); s != "" {
			out[s] = true
		}
	}
	for _, j := range judgments {
		if s := str(object(j["target"])["commit"]); s != "" {
			out[s] = true
		}
	}
	return out
}

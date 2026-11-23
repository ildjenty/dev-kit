package record

const Limit = 3

func ClassifyStart(h HistoryData, workflow string, answered []string) (string, error) {
	stage, err := StageOf(workflow)
	if err != nil {
		return "", err
	}
	if len(answered) > 0 {
		return "answer", nil
	}
	if len(h.Runs) == 0 {
		return "first", nil
	}
	last := h.Runs[len(h.Runs)-1]
	finished := false
	for _, r := range h.Results {
		if r["run_id"] == last["run_id"] {
			finished = true
		}
	}
	if last["workflow"] == workflow && !finished {
		return "retry_after_error", nil
	}
	if len(h.Results) == 0 {
		return "other", nil
	}
	latest := h.Results[len(h.Results)-1]
	if latest["outcome"] == "back" && object(latest["back"])["to"] == workflow {
		return "step_back", nil
	}
	if latest["outcome"] == "stop" && latest["workflow"] == workflow {
		return "continue_after_stop", nil
	}
	if latest["outcome"] == "pass" || latest["outcome"] == "carry" {
		previous, err := StageOf(str(latest["workflow"]))
		if err != nil {
			return "", err
		}
		if previous < stage {
			return "next", nil
		}
	}
	return "other", nil
}
func RestartsOf(h HistoryData) int {
	continued := map[string]bool{}
	for _, c := range h.Continuations {
		continued[str(c["run_id"])] = true
	}
	index := -1
	for i, run := range h.Runs {
		if continued[str(run["run_id"])] {
			index = i
		}
	}
	count := 0
	for _, run := range h.Runs[index+1:] {
		if run["counted"] == true {
			count++
		}
	}
	return count
}

type LimitStatus struct {
	Start    string `json:"start"`
	Counted  bool   `json:"counted"`
	Restarts int    `json:"restarts"`
	Limit    int    `json:"limit"`
	Reached  bool   `json:"reached"`
}

func LimitState(h HistoryData, workflow string, answered []string, limit int) (LimitStatus, error) {
	start, err := ClassifyStart(h, workflow, answered)
	if err != nil {
		return LimitStatus{}, err
	}
	counted := start == "step_back" || start == "continue_after_stop" || start == "retry_after_error"
	restarts := RestartsOf(h)
	return LimitStatus{start, counted, restarts, limit, counted && restarts >= limit}, nil
}

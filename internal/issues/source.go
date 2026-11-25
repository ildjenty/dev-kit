package issues

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func (c Client) waitingPR(body string) (*Reference, error) {
	match := sourcePRPattern.FindStringSubmatch(body)
	if match == nil {
		return nil, nil
	}
	number, err := strconv.Atoi(match[1])
	if err != nil {
		return nil, fmt.Errorf("invalid source PR: %w", err)
	}
	data, err := c.run(nil, "pr", "view", strconv.Itoa(number), "--json", "number,title,url,state,mergedAt")
	if err != nil {
		return nil, fmt.Errorf("source PR #%d could not be read: %w", number, err)
	}
	var pr struct {
		Reference
		MergedAt *string `json:"mergedAt"`
	}
	if err := json.Unmarshal(data, &pr); err != nil {
		return nil, fmt.Errorf("parse source PR #%d: %w", number, err)
	}
	if pr.Number != number || pr.State == "" {
		return nil, fmt.Errorf("invalid source PR #%d response", number)
	}
	if pr.MergedAt != nil && *pr.MergedAt != "" {
		return nil, nil
	}
	pr.State = strings.ToLower(pr.State)
	return &pr.Reference, nil
}

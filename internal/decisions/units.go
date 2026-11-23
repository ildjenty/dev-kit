package decisions

import "fmt"

func Units(data Data) []Item {
	done := map[string]bool{}
	for _, u := range data.Units {
		done[text(u["id"])] = u["state"] == "done"
	}
	result := []Item{}
	for _, u := range data.Units {
		deps, ok := list(u["depends_on"])
		if deps == nil {
			deps = []string{}
		}
		ready := ok && !done[text(u["id"])]
		for _, dep := range deps {
			ready = ready && done[dep]
		}
		state := "open"
		if done[text(u["id"])] {
			state = "done"
		}
		result = append(result, Item{"id": u["id"], "title": u["title"], "depends_on": deps, "state": state, "ready": ready})
	}
	return result
}

func UnitStartReasons(data Data, id string) []string {
	for _, u := range data.Units {
		if u["id"] != id {
			continue
		}
		if u["state"] == "done" {
			return []string{fmt.Sprintf("unit %s is already done", id)}
		}
		reasons := []string{}
		deps, _ := list(u["depends_on"])
		for _, dep := range deps {
			done := false
			for _, other := range data.Units {
				if other["id"] == dep && other["state"] == "done" {
					done = true
				}
			}
			if !done {
				reasons = append(reasons, fmt.Sprintf("unit %s waits for %s", id, dep))
			}
		}
		return reasons
	}
	return []string{fmt.Sprintf("unit %s does not exist", id)}
}

func UnitDecisions(data Data, id string) ([]Item, error) {
	for _, unit := range data.Units {
		if unit["id"] != id {
			continue
		}
		agreed := []Item{}
		refs, _ := list(unit["decisions"])
		for _, ref := range refs {
			var found Item
			for _, d := range data.Decisions {
				if d["id"] == ref && d["status"] == "agreed" {
					found = d
				}
			}
			if found == nil {
				return nil, fmt.Errorf("unit %s cites decision %s which is not agreed", id, ref)
			}
			agreed = append(agreed, found)
		}
		return agreed, nil
	}
	return nil, fmt.Errorf("unit %s does not exist", id)
}

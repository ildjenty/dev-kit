package engine

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/ildjenty/dev-kit/internal/brief"
	"github.com/ildjenty/dev-kit/internal/check"
	"github.com/ildjenty/dev-kit/internal/ledger"
)

// spill names a movable brief body.
type spill struct {
	label string
	body  *string
}

// fitBrief keeps a brief within [briefs] max_bytes by moving the largest bodies, whole, to fixed
// files listed in their place. Nothing is truncated; the moved files and the final size are
// recorded in the instruction context.
func (e *Engine) fitBrief(meta *ledger.Instruction, limit int, items []spill, build func() (brief.Brief, error)) (brief.Brief, error) {
	if meta.Context == nil {
		meta.Context = &ledger.InstructionContext{}
	}
	meta.Context.Overflow = nil
	for {
		b, err := build()
		if err != nil || len(b.Text) <= limit {
			meta.Context.BriefBytes = len(b.Text)
			return b, err
		}
		index := largestSpill(items)
		if index < 0 {
			return e.fitRequest(meta, limit, b)
		}
		name := fmt.Sprintf("brief/%s-overflow-%d.md", meta.ID, len(meta.Context.Overflow)+1)
		if err := e.store().Publish(name, []byte(*items[index].body)); err != nil {
			return brief.Brief{}, err
		}
		path := filepath.Join(e.workRoot(), name)
		*items[index].body = fmt.Sprintf("Moved to %s (%d bytes) because the brief exceeded [briefs] max_bytes. Read the whole file.", path, len(*items[index].body))
		meta.Context.Overflow = append(meta.Context.Overflow, items[index].label+": "+path)
		items = append(items[:index], items[index+1:]...)
	}
}

func largestSpill(items []spill) int {
	index, size := -1, 200
	for i, item := range items {
		if len(*item.body) > size {
			index, size = i, len(*item.body)
		}
	}
	return index
}

// fitRequest moves the complete remaining request to a file when its fixed envelope is too large.
func (e *Engine) fitRequest(meta *ledger.Instruction, limit int, b brief.Brief) (brief.Brief, error) {
	if meta.Context == nil {
		meta.Context = &ledger.InstructionContext{}
	}
	if len(b.Text) > limit {
		name := "brief/" + meta.ID + "-body-" + hash([]byte(b.Text))[:12] + ".md"
		path := filepath.Join(e.workRoot(), name)
		text := fmt.Sprintf("Instruction ID: %s\nMoved to %s. Read the whole request file.\n", meta.ID, path)
		if len(text) > limit {
			return brief.Brief{}, fmt.Errorf("configuration error: [briefs] max_bytes = %d cannot fit the minimal request (%d bytes)", limit, len(text))
		}
		if err := e.store().Publish(name, []byte(b.Text)); err != nil {
			return brief.Brief{}, err
		}
		meta.Context.Overflow = append(meta.Context.Overflow, "Remaining request: "+path)
		b.Text = text
	}
	meta.Context.BriefBytes = len(b.Text)
	return b, nil
}

// fitCommandRequest uses a JSON file reference when a command's complete input exceeds the limit.
func (e *Engine) fitCommandRequest(request check.Request, limit int, body []byte) ([]byte, error) {
	if len(body) <= limit {
		return body, nil
	}
	name := "brief/" + request.InstructionID + "-command-" + hash(body)[:12] + ".json"
	pointer, err := json.Marshal(struct {
		InstructionID string
		Kind          check.RequestKind
		RequestPath   string `json:"request_path"`
	}{request.InstructionID, request.Kind, filepath.Join(e.workRoot(), name)})
	if err != nil {
		return nil, err
	}
	if len(pointer) > limit {
		return nil, fmt.Errorf("configuration error: [briefs] max_bytes = %d cannot fit the minimal command request (%d bytes)", limit, len(pointer))
	}
	return pointer, e.store().Publish(name, body)
}

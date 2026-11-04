package engine

import (
	"fmt"
	"path/filepath"

	"github.com/ildjenty/dev-kit/internal/brief"
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
			meta.Context.BriefBytes = len(b.Text)
			return b, nil
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

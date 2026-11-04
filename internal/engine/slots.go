package engine

import (
	"github.com/ildjenty/dev-kit/internal/config"
	"github.com/ildjenty/dev-kit/internal/form"
	"github.com/ildjenty/dev-kit/internal/ledger"
)

// slots tracks free concurrency for subagent instructions. Issued but unaccepted subagent
// instructions count against both [parallel] max_concurrent and their role's max_concurrent.
type slots struct {
	global  int
	roles   map[string]int
	pending map[string]bool
}

func subagentKind(kind form.InstructionKind) bool {
	switch kind {
	case form.InstructionPlan, form.InstructionJudgment, form.InstructionPlanCheck, form.InstructionDetectionTest,
		form.InstructionUnitWork, form.InstructionJudgmentBrief:
		return true
	}
	return false
}

func newSlots(state *ledger.State, settings config.Config) *slots {
	s := &slots{global: settings.MaxConcurrent(), roles: map[string]int{}, pending: map[string]bool{}}
	roles := settings.RoleSettings()
	for id := range roles {
		s.roles[id] = settings.RoleMaxConcurrent(id)
	}
	for _, instruction := range state.UnacceptedInstructions() {
		if !subagentKind(instruction.Kind) {
			continue
		}
		s.pending[string(instruction.Kind)+"\x00"+instruction.TargetKey] = true
		s.global--
		for id, role := range roles {
			if role.Agent != "" && role.Agent == instruction.Runner {
				s.roles[id]--
			}
		}
	}
	return s
}

// take reserves a slot for a new instruction of role. An instruction already outstanding for the
// same kind and key (including a reissue to a new session) needs no further slot.
func (s *slots) take(kind form.InstructionKind, key, role string) bool {
	if s.pending[string(kind)+"\x00"+key] {
		return true
	}
	limit, known := s.roles[role]
	if s.global <= 0 || known && limit <= 0 {
		return false
	}
	s.global--
	if known {
		s.roles[role]--
	}
	s.pending[string(kind)+"\x00"+key] = true
	return true
}

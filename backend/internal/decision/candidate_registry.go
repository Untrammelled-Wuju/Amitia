package decision

import (
	"fmt"
	"sort"
	"sync"
)

type CandidateActionType string

const (
	CandidateActionChat        CandidateActionType = "chat"
	CandidateActionProactive   CandidateActionType = "proactive"
	CandidateActionToolCall    CandidateActionType = "tool_call"
	CandidateActionAskClarify  CandidateActionType = "ask_clarify"
	CandidateActionOfferHelp   CandidateActionType = "offer_help"
	CandidateActionSetBoundary CandidateActionType = "set_boundary"
	CandidateActionExpress     CandidateActionType = "express"
	CandidateActionWait        CandidateActionType = "wait"
	CandidateActionDefault     CandidateActionType = "default"
)

type CandidateActionDef struct {
	ID              string              `json:"id"`
	Type            CandidateActionType `json:"type"`
	Tag             BehaviorTag         `json:"tag"`
	Label           string              `json:"label"`
	Description     string              `json:"description,omitempty"`
	BaseScore       float64             `json:"baseScore"`
	Preconds        []string            `json:"preconds,omitempty"`
	Overrides       []string            `json:"overrides,omitempty"`
	AllowedTriggers []GoalTriggerKind   `json:"allowedTriggers,omitempty"`
}

type CandidateRegistry struct {
	mu      sync.RWMutex
	actions map[string]CandidateActionDef
}

func NewCandidateRegistry() *CandidateRegistry {
	return &CandidateRegistry{
		actions: make(map[string]CandidateActionDef),
	}
}

func DefaultCandidateRegistry() *CandidateRegistry {
	r := NewCandidateRegistry()
	mustRegisterCandidate(r, CandidateActionDef{
		ID:              "chat_reply",
		Type:            CandidateActionChat,
		Tag:             BehaviorTagReply,
		Label:           "聊天回复",
		BaseScore:       0.60,
		AllowedTriggers: []GoalTriggerKind{GoalTriggerUserMessage, GoalTriggerVoice, GoalTriggerInternal},
	})
	mustRegisterCandidate(r, CandidateActionDef{
		ID:              "proactive_greet",
		Type:            CandidateActionProactive,
		Tag:             BehaviorTagProactiveCheck,
		Label:           "主动问候",
		BaseScore:       0.30,
		AllowedTriggers: []GoalTriggerKind{GoalTriggerProactive, GoalTriggerInternal},
	})
	mustRegisterCandidate(r, CandidateActionDef{
		ID:              "ask_clarify",
		Type:            CandidateActionAskClarify,
		Tag:             BehaviorTagAskClarify,
		Label:           "请求澄清",
		BaseScore:       0.40,
		AllowedTriggers: []GoalTriggerKind{GoalTriggerUserMessage, GoalTriggerVoice, GoalTriggerInternal},
	})
	mustRegisterCandidate(r, CandidateActionDef{
		ID:              "offer_support",
		Type:            CandidateActionOfferHelp,
		Tag:             BehaviorTagOfferSupport,
		Label:           "提供支持",
		BaseScore:       0.50,
		AllowedTriggers: []GoalTriggerKind{GoalTriggerUserMessage, GoalTriggerVoice, GoalTriggerProactive, GoalTriggerInternal},
	})
	mustRegisterCandidate(r, CandidateActionDef{
		ID:              "set_boundary",
		Type:            CandidateActionSetBoundary,
		Tag:             BehaviorTagSetBoundary,
		Label:           "设立边界",
		BaseScore:       0.20,
		Preconds:        []string{"boundary_crossed"},
		AllowedTriggers: []GoalTriggerKind{GoalTriggerUserMessage, GoalTriggerVoice, GoalTriggerInternal},
	})
	mustRegisterCandidate(r, CandidateActionDef{
		ID:              "express_emotion",
		Type:            CandidateActionExpress,
		Tag:             BehaviorTagReply,
		Label:           "表达情绪",
		BaseScore:       0.35,
		AllowedTriggers: []GoalTriggerKind{GoalTriggerUserMessage, GoalTriggerVoice, GoalTriggerProactive, GoalTriggerInternal},
	})
	mustRegisterCandidate(r, CandidateActionDef{
		ID:              "wait_observe",
		Type:            CandidateActionWait,
		Tag:             BehaviorTagDelay,
		Label:           "等待观察",
		BaseScore:       0.10,
		AllowedTriggers: []GoalTriggerKind{GoalTriggerUserMessage, GoalTriggerVoice, GoalTriggerProactive, GoalTriggerInternal, GoalTriggerRecovery},
	})
	mustRegisterCandidate(r, CandidateActionDef{
		ID:              "tool_search",
		Type:            CandidateActionToolCall,
		Tag:             BehaviorTagReply,
		Label:           "工具搜索",
		BaseScore:       0.25,
		Preconds:        []string{"information_goal"},
		AllowedTriggers: []GoalTriggerKind{GoalTriggerUserMessage, GoalTriggerVoice, GoalTriggerInternal},
	})
	return r
}

func mustRegisterCandidate(registry *CandidateRegistry, def CandidateActionDef) {
	if err := registry.Register(def); err != nil {
		panic(err)
	}
}

func (r *CandidateRegistry) Register(def CandidateActionDef) error {
	if def.ID == "" {
		return fmt.Errorf("candidate action ID is required")
	}
	if def.Type == "" {
		return fmt.Errorf("candidate action Type is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.actions[def.ID]; exists {
		return fmt.Errorf("candidate action %s already registered", def.ID)
	}
	r.actions[def.ID] = def
	return nil
}

func (r *CandidateRegistry) Get(id string) (CandidateActionDef, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, ok := r.actions[id]
	if !ok {
		return CandidateActionDef{}, false
	}
	return cloneCandidateActionDef(def), true
}

func (r *CandidateRegistry) ByType(actionType CandidateActionType) []CandidateActionDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]CandidateActionDef, 0)
	for _, def := range r.actions {
		if def.Type == actionType {
			result = append(result, cloneCandidateActionDef(def))
		}
	}
	sortCandidateDefs(result)
	return result
}

func (r *CandidateRegistry) All() []CandidateActionDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]CandidateActionDef, 0, len(r.actions))
	for _, def := range r.actions {
		result = append(result, cloneCandidateActionDef(def))
	}
	sortCandidateDefs(result)
	return result
}

func (r *CandidateRegistry) AllExcept(excludes []string) []CandidateActionDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	excludeSet := make(map[string]bool, len(excludes))
	for _, id := range excludes {
		excludeSet[id] = true
	}
	result := make([]CandidateActionDef, 0)
	for _, def := range r.actions {
		if !excludeSet[def.ID] {
			result = append(result, cloneCandidateActionDef(def))
		}
	}
	sortCandidateDefs(result)
	return result
}

func (r *CandidateRegistry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.actions)
}

func sortCandidateDefs(defs []CandidateActionDef) {
	sort.SliceStable(defs, func(i, j int) bool {
		return defs[i].ID < defs[j].ID
	})
}

func cloneCandidateActionDef(def CandidateActionDef) CandidateActionDef {
	next := def
	if def.Preconds != nil {
		next.Preconds = append([]string(nil), def.Preconds...)
	}
	if def.Overrides != nil {
		next.Overrides = append([]string(nil), def.Overrides...)
	}
	if def.AllowedTriggers != nil {
		next.AllowedTriggers = append([]GoalTriggerKind(nil), def.AllowedTriggers...)
	}
	return next
}

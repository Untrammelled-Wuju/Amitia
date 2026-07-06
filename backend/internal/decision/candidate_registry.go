package decision

import "sync"

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
	ID          string              `json:"id"`
	Type        CandidateActionType `json:"type"`
	Tag         BehaviorTag         `json:"tag"`
	Label       string              `json:"label"`
	Description string              `json:"description,omitempty"`
	BaseScore   float64             `json:"baseScore"`
	Preconds    []string            `json:"preconds,omitempty"`
	Overrides   []string            `json:"overrides,omitempty"`
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
	r.Register(CandidateActionDef{
		ID:        "chat_reply",
		Type:      CandidateActionChat,
		Tag:       BehaviorTagReply,
		Label:     "聊天回复",
		BaseScore: 0.60,
	})
	r.Register(CandidateActionDef{
		ID:        "proactive_greet",
		Type:      CandidateActionProactive,
		Tag:       BehaviorTagProactiveCheck,
		Label:     "主动问候",
		BaseScore: 0.30,
	})
	r.Register(CandidateActionDef{
		ID:        "ask_clarify",
		Type:      CandidateActionAskClarify,
		Tag:       BehaviorTagAskClarify,
		Label:     "请求澄清",
		BaseScore: 0.40,
	})
	r.Register(CandidateActionDef{
		ID:        "offer_support",
		Type:      CandidateActionOfferHelp,
		Tag:       BehaviorTagOfferSupport,
		Label:     "提供支持",
		BaseScore: 0.50,
	})
	r.Register(CandidateActionDef{
		ID:        "set_boundary",
		Type:      CandidateActionSetBoundary,
		Tag:       BehaviorTagSetBoundary,
		Label:     "设立边界",
		BaseScore: 0.20,
		Preconds:  []string{"boundary_crossed"},
	})
	r.Register(CandidateActionDef{
		ID:        "express_emotion",
		Type:      CandidateActionExpress,
		Tag:       BehaviorTagReply,
		Label:     "表达情绪",
		BaseScore: 0.35,
	})
	r.Register(CandidateActionDef{
		ID:        "wait_observe",
		Type:      CandidateActionWait,
		Tag:       BehaviorTagDelay,
		Label:     "等待观察",
		BaseScore: 0.10,
	})
	r.Register(CandidateActionDef{
		ID:        "tool_search",
		Type:      CandidateActionToolCall,
		Tag:       BehaviorTagReply,
		Label:     "工具搜索",
		BaseScore: 0.25,
	})
	return r
}

func (r *CandidateRegistry) Register(def CandidateActionDef) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.actions[def.ID] = def
}

func (r *CandidateRegistry) Get(id string) (CandidateActionDef, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, ok := r.actions[id]
	return def, ok
}

func (r *CandidateRegistry) ByType(actionType CandidateActionType) []CandidateActionDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]CandidateActionDef, 0)
	for _, def := range r.actions {
		if def.Type == actionType {
			result = append(result, def)
		}
	}
	return result
}

func (r *CandidateRegistry) All() []CandidateActionDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]CandidateActionDef, 0, len(r.actions))
	for _, def := range r.actions {
		result = append(result, def)
	}
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
			result = append(result, def)
		}
	}
	return result
}

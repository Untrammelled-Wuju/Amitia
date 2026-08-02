package behavior

import (
	"time"
)

type bindingCandidate interface {
	GetID() string
	GetSemantic() string
	GetPreferredAction() string
	GetPriorityOffset() int
	GetCooldownMS() int64
	IsEnabled() bool
}

type Resolver struct {
	clock    Clock
	fallback *FallbackGraph
	bindings *BindingResolver
}

type BindingResolver struct {
	evaluate func(scope interface{}, eventType string, origin EventOrigin, payload map[string]interface{}) []interface{}
}

func NewResolver(clock Clock, fallback *FallbackGraph) *Resolver {
	if clock == nil {
		clock = NewRealClock()
	}
	if fallback == nil {
		fallback = DefaultFallbackGraph()
	}
	return &Resolver{clock: clock, fallback: fallback}
}

func (r *Resolver) SetBindingEvaluator(fn func(scope interface{}, eventType string, origin EventOrigin, payload map[string]interface{}) []interface{}) {
	r.bindings = &BindingResolver{evaluate: fn}
}

func (r *Resolver) Resolve(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope, activePet *ActivePetSnapshot) ([]CandidateAction, error) {
	if activePet == nil {
		return nil, ErrNoActiveInstallation
	}

	available := make(map[string]bool)
	for key, cap := range activePet.Actions {
		if cap.Available {
			available[key] = true
		}
	}
	if activePet.DefaultAction != "" {
		available[activePet.DefaultAction] = true
	}

	candidates := r.generateCandidates(ctx, event, available)
	return candidates, nil
}

func (r *Resolver) generateCandidates(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope, available map[string]bool) []CandidateAction {
	var candidates []CandidateAction
	now := r.clock.Now()

	if event.EventType == "manual.action.requested" {
		payload := parsePayload(event.Payload)
		actionKey, _ := payload["actionKey"].(string)
		force, _ := payload["force"].(bool)
		priority := 900
		if force {
			priority = 1000
		}
		candidates = append(candidates, CandidateAction{
			Semantic:        "manual_" + actionKey,
			PreferredKeys:   []string{actionKey},
			SourceEventID:   event.EventID,
			SourceLayer:     "manual",
			Priority:        priority,
			CreatedAt:       now,
			MinPlay:         0,
			InterruptPolicy: "force",
		})
	}

	if ctx.DesktopGesture.CurrentGesture == "fall" {
		candidates = append(candidates, makeCandidate("gesture_drop", []string{"fall", "land"}, event.EventID, "desktop", 870, now))
	} else if ctx.DesktopGesture.CurrentGesture == "drag" {
		candidates = append(candidates, makeCandidate("gesture_drag", []string{"dragged", "picked_up"}, event.EventID, "desktop", 850, now))
	} else if ctx.DesktopGesture.CurrentGesture == "dropped" {
		candidates = append(candidates, makeCandidate("gesture_drop", []string{"dropped", "land"}, event.EventID, "desktop", 850, now))
	} else if ctx.DesktopGesture.CurrentGesture == "clicked" {
		candidates = append(candidates, makeCandidate("gesture_click", []string{"clicked"}, event.EventID, "desktop", 760, now, withCooldown(2*time.Second)))
	} else if ctx.DesktopGesture.CurrentGesture == "double_clicked" {
		candidates = append(candidates, makeCandidate("gesture_double_click", []string{"double_clicked"}, event.EventID, "desktop", 770, now, withCooldown(2*time.Second)))
	} else if ctx.DesktopGesture.CurrentGesture == "hovered" {
		candidates = append(candidates, makeCandidate("gesture_hover", []string{"hovered"}, event.EventID, "desktop", 350, now, withCooldown(1*time.Second)))
	} else if ctx.DesktopGesture.CurrentGesture == "edge" {
		candidates = append(candidates, makeCandidate("physics_edge_sit", []string{"edge_sit"}, event.EventID, "desktop", 510, now))
	}

	if ctx.Voice.State == "speaking" {
		candidates = append(candidates, makeCandidate("dialogue_speaking", []string{"speaking"}, event.EventID, "voice", 800, now, withMin(500*time.Millisecond), withUninterruptible(false)))
	} else if ctx.Voice.State == "listening" {
		candidates = append(candidates, makeCandidate("dialogue_listening", []string{"listening"}, event.EventID, "voice", 720, now))
	} else if ctx.Voice.State == "processing" || ctx.Voice.State == "thinking" {
		candidates = append(candidates, makeCandidate("dialogue_thinking", []string{"thinking"}, event.EventID, "voice", 680, now))
	}

	if len(ctx.ActiveTools) > 0 {
		hasWork := false
		for _, tool := range ctx.ActiveTools {
			if tool.DisplayClass == "research" || tool.DisplayClass == "work" {
				hasWork = true
				break
			}
		}
		if hasWork {
			candidates = append(candidates, makeCandidate("working", []string{"work", "study", "thinking"}, event.EventID, "tool", 680, now, withMin(1*time.Second)))
		} else {
			candidates = append(candidates, makeCandidate("working", []string{"work", "thinking"}, event.EventID, "tool", 680, now, withMin(1*time.Second)))
		}
	}

	phase := ctx.Transient.InteractionPhase
	if ctx.Voice.State == "" && len(ctx.ActiveTools) == 0 {
		switch phase {
		case "received":
			candidates = append(candidates, makeCandidate("dialogue_listening", []string{"listening"}, event.EventID, "transient", 720, now, withExpires(5*time.Second)))
		case "context_loading":
			candidates = append(candidates, makeCandidate("dialogue_thinking", []string{"thinking"}, event.EventID, "transient", 680, now, withExpires(15*time.Second)))
		case "response_started":
			candidates = append(candidates, makeCandidate("dialogue_thinking", []string{"thinking"}, event.EventID, "transient", 680, now, withExpires(120*time.Second)))
		case "response_ready":
			candidates = append(candidates, makeCandidate("dialogue_speaking", []string{"speaking"}, event.EventID, "transient", 800, now, withExpires(30*time.Second)))
		case "completed":
			if ctx.Foreground.Semantic != "dialogue_speaking" {
				candidates = append(candidates, makeCandidate("calm_idle", []string{"idle_breathing"}, event.EventID, "transient", 250, now))
			}
		case "failed":
			candidates = append(candidates, makeCandidate("emotion_confused", []string{"confused", "thinking"}, event.EventID, "transient", 610, now, withCooldown(60*time.Second)))
		}
	}

	if ctx.Transient.ProactiveID != "" {
		intent := ctx.Transient.ProactiveIntent
		if intent == "greeting" || intent == "" {
			candidates = append(candidates, makeCandidate("dialogue_greeting", []string{"greeting", "wave", "bow"}, event.EventID, "proactive", 560, now))
		} else if intent == "reminder" {
			candidates = append(candidates, makeCandidate("attention_gesture", []string{"point", "wave", "greeting"}, event.EventID, "proactive", 570, now))
		}
	}

	if ctx.Voice.State == "" && phase == "" && len(ctx.ActiveTools) == 0 && ctx.DesktopGesture.CurrentGesture == "" {
		if ctx.Stable.AffectLabel != "" {
			if emotionCandidate := r.resolveEmotionCandidate(ctx, event, now); emotionCandidate != nil {
				candidates = append(candidates, *emotionCandidate)
			}
		}

		if ctx.Stable.ActivityKey != "" {
			if activityCandidate := r.resolveActivityCandidate(ctx, event, now); activityCandidate != nil {
				candidates = append(candidates, *activityCandidate)
			}
		}

		candidates = append(candidates, makeCandidate("fallback_idle", []string{"idle_normal"}, event.EventID, "stable", 100, now))
	}

	if r.bindings != nil && r.bindings.evaluate != nil {
		payload := parsePayload(event.Payload)
		rawMatched := r.bindings.evaluate(nil, event.EventType, event.Origin, payload)
		for _, rb := range rawMatched {
			b, ok := rb.(bindingCandidate)
			if !ok {
				continue
			}
			if !b.IsEnabled() {
				continue
			}
			keys := []string{b.GetPreferredAction()}
			priority := 300 + b.GetPriorityOffset()
			if priority > 900 {
				priority = 900
			}
			cooldown := time.Duration(b.GetCooldownMS()) * time.Millisecond
			if cooldown < 500*time.Millisecond {
				cooldown = 500 * time.Millisecond
			}
			candidates = append(candidates, CandidateAction{
				Semantic:      b.GetSemantic(),
				PreferredKeys: keys,
				SourceEventID: event.EventID,
				SourceLayer:   "binding",
				Priority:      priority,
				CreatedAt:     now,
				CooldownKey:   b.GetID(),
				Cooldown:      cooldown,
			})
		}
	}

	return candidates
}

func (r *Resolver) resolveEmotionCandidate(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope, now time.Time) *CandidateAction {
	label := ctx.Stable.AffectLabel
	switch {
	case isPositiveHighArousal(label):
		return &CandidateAction{
			Semantic: "emotion_excited", PreferredKeys: []string{"excited", "happy", "wave"},
			SourceEventID: event.EventID, SourceLayer: "stable", Priority: 640,
			CreatedAt: now, CooldownKey: "semantic:emotion", Cooldown: 10 * time.Second,
		}
	case isPositiveModerate(label):
		return &CandidateAction{
			Semantic: "emotion_happy", PreferredKeys: []string{"happy", "excited", "wave"},
			SourceEventID: event.EventID, SourceLayer: "stable", Priority: 420,
			CreatedAt: now, CooldownKey: "semantic:emotion", Cooldown: 10 * time.Second,
		}
	case isNegativeHighTension(label):
		return &CandidateAction{
			Semantic: "emotion_stressed", PreferredKeys: []string{"stressed", "worried", "thinking"},
			SourceEventID: event.EventID, SourceLayer: "stable", Priority: 610,
			CreatedAt: now, CooldownKey: "semantic:emotion", Cooldown: 10 * time.Second,
		}
	case isNeutral(label):
		return &CandidateAction{
			Semantic: "calm_idle", PreferredKeys: []string{"idle_breathing", "idle_normal"},
			SourceEventID: event.EventID, SourceLayer: "stable", Priority: 300,
			CreatedAt: now, CooldownKey: "semantic:emotion", Cooldown: 10 * time.Second,
		}
	}
	return nil
}

func (r *Resolver) resolveActivityCandidate(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope, now time.Time) *CandidateAction {
	activity := ctx.Stable.ActivityKey
	switch activity {
	case "work":
		return &CandidateAction{
			Semantic: "working", PreferredKeys: []string{"work", "study", "thinking"},
			SourceEventID: event.EventID, SourceLayer: "stable", Priority: 480,
			CreatedAt: now, CooldownKey: "semantic:activity", Cooldown: 30 * time.Second,
		}
	case "study":
		return &CandidateAction{
			Semantic: "studying", PreferredKeys: []string{"study", "work", "thinking"},
			SourceEventID: event.EventID, SourceLayer: "stable", Priority: 460,
			CreatedAt: now, CooldownKey: "semantic:activity", Cooldown: 30 * time.Second,
		}
	case "entertainment":
		return &CandidateAction{
			Semantic: "relaxing", PreferredKeys: []string{"relax", "happy", "wave"},
			SourceEventID: event.EventID, SourceLayer: "stable", Priority: 400,
			CreatedAt: now, CooldownKey: "semantic:activity", Cooldown: 30 * time.Second,
		}
	case "sleep":
		return &CandidateAction{
			Semantic: "sleeping", PreferredKeys: []string{"sleep", "idle_sleep"},
			SourceEventID: event.EventID, SourceLayer: "stable", Priority: 520,
			CreatedAt: now, CooldownKey: "semantic:activity", Cooldown: 30 * time.Second,
		}
	case "exercise":
		return &CandidateAction{
			Semantic: "exercising", PreferredKeys: []string{"exercise", "wave", "happy"},
			SourceEventID: event.EventID, SourceLayer: "stable", Priority: 440,
			CreatedAt: now, CooldownKey: "semantic:activity", Cooldown: 30 * time.Second,
		}
	}
	return nil
}

func (r *Resolver) resolveToolCandidate(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope, now time.Time) *CandidateAction {
	for _, tool := range ctx.ActiveTools {
		if tool.DisplayClass == "research" || tool.DisplayClass == "work" {
			return &CandidateAction{
				Semantic: "working", PreferredKeys: []string{"work", "study", "thinking"},
				SourceEventID: event.EventID, SourceLayer: "tool", Priority: 680,
				CreatedAt: now, CooldownKey: "semantic:tool", Cooldown: 5 * time.Second,
			}
		}
	}
	return nil
}

func makeCandidate(semantic string, keys []string, eventID string, layer string, priority int, now time.Time, opts ...func(*CandidateAction)) CandidateAction {
	c := CandidateAction{
		Semantic:      semantic,
		PreferredKeys: keys,
		SourceEventID: eventID,
		SourceLayer:   layer,
		Priority:      priority,
		CreatedAt:     now,
		CooldownKey:   "semantic:" + semantic,
		Cooldown:      5 * time.Second,
	}
	for _, opt := range opts {
		opt(&c)
	}
	return c
}

func withCooldown(d time.Duration) func(*CandidateAction) {
	return func(c *CandidateAction) { c.Cooldown = d }
}

func withMin(ms time.Duration) func(*CandidateAction) {
	return func(c *CandidateAction) { c.MinPlay = ms }
}

func withExpires(d time.Duration) func(*CandidateAction) {
	return func(c *CandidateAction) { c.ExpiresAt = timePtr(time.Now().Add(d)) }
}

func withUninterruptible(v bool) func(*CandidateAction) {
	return func(c *CandidateAction) {
		c.InterruptPolicy = map[bool]string{true: "uninterruptible", false: "queue"}[v]
	}
}

func timePtr(t time.Time) *time.Time { return &t }

func isPositiveHighArousal(label string) bool {
	switch label {
	case "excited", "pleasant_aroused", "happy_aroused", "enthusiastic":
		return true
	}
	return false
}

func isPositiveModerate(label string) bool {
	switch label {
	case "happy", "pleasant", "content", "joyful", "cheerful":
		return true
	}
	return false
}

func isNegativeHighTension(label string) bool {
	switch label {
	case "angry", "unpleasant_aroused", "frustrated", "irritated", "rage",
		"stressed", "worried", "anxious":
		return true
	}
	return false
}

func isNeutral(label string) bool {
	switch label {
	case "calm", "neutral", "idle":
		return true
	}
	return false
}

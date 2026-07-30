package behavior

import (
	"time"
)

type Resolver struct {
	clock      Clock
	fallback   *FallbackGraph
	bindings   *BindingResolver
}

type BindingResolver struct {
	evaluate func(eventType string, origin EventOrigin, payload map[string]interface{}) []BehaviorBinding
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

func (r *Resolver) SetBindingEvaluator(fn func(eventType string, origin EventOrigin, payload map[string]interface{}) []BehaviorBinding) {
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
			Semantic:      "manual_" + actionKey,
			PreferredKeys: []string{actionKey},
			SourceEventID: event.EventID,
			SourceLayer:   "manual",
			Priority:      priority,
			CreatedAt:     now,
			MinPlay:       0,
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
			if ctx.Foreground.Semantic == "dialogue_speaking" {
			} else {
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
		matchedBindings := r.bindings.evaluate(event.EventType, event.Origin, payload)
		for _, b := range matchedBindings {
			if !b.Enabled {
				continue
			}
			keys := []string{b.PreferredAction}
			priority := 300 + b.PriorityOffset
			if priority > 900 {
				priority = 900
			}
			cooldown := time.Duration(b.CooldownMS) * time.Millisecond
			if cooldown < 500*time.Millisecond {
				cooldown = 500 * time.Millisecond
			}
			candidates = append(candidates, CandidateAction{
				Semantic:      b.Semantic,
				PreferredKeys: keys,
				SourceEventID: event.EventID,
				SourceLayer:   "binding",
				Priority:      priority,
				CreatedAt:     now,
				CooldownKey:   b.ID,
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
			CreatedAt: now, CooldownKey: "semantic:emotion", Cooldown: 30 * time.Second,
		}
	case isNegativeLowArousal(label):
		return &CandidateAction{
			Semantic: "emotion_sad", PreferredKeys: []string{"sad", "tired"},
			SourceEventID: event.EventID, SourceLayer: "stable", Priority: 420,
			CreatedAt: now, CooldownKey: "semantic:emotion", Cooldown: 30 * time.Second,
		}
	case isNegativeHighArousal(label):
		return &CandidateAction{
			Semantic: "emotion_angry", PreferredKeys: []string{"angry", "disagreeing", "shake_head"},
			SourceEventID: event.EventID, SourceLayer: "stable", Priority: 650,
			CreatedAt: now, CooldownKey: "semantic:emotion", Cooldown: 30 * time.Second,
		}
	}
	return nil
}

func (r *Resolver) resolveActivityCandidate(ctx *BehaviorContextSnapshot, event BehaviorEventEnvelope, now time.Time) *CandidateAction {
	key := ctx.Stable.ActivityKey
	mapping := map[string][]string{
		"sleep": {"sleep", "sleep_on_desktop", "sit"},
		"rest":  {"sit", "sleep_on_desktop"},
		"eat":   {"eat"},
		"drink": {"drink"},
		"read":  {"read"},
		"write": {"write"},
		"phone": {"use_phone"},
		"work":  {"work"},
		"study": {"study", "read", "work"},
	}
	keys, ok := mapping[key]
	if !ok {
		return nil
	}
	semantic := "activity_" + key
	priority := 500
	if key == "work" {
		priority = 680
	}
	return &CandidateAction{
		Semantic:      semantic,
		PreferredKeys: keys,
		SourceEventID: event.EventID,
		SourceLayer:   "stable",
		Priority:      priority,
		CreatedAt:     now,
		Durable:       true,
	}
}

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

func isNegativeLowArousal(label string) bool {
	switch label {
	case "sad", "unpleasant_calm", "depressed", "tired", "bored":
		return true
	}
	return false
}

func isNegativeHighArousal(label string) bool {
	switch label {
	case "angry", "unpleasant_aroused", "frustrated", "irritated", "rage":
		return true
	}
	return false
}

type candidateOption func(*CandidateAction)

func withCooldown(d time.Duration) candidateOption {
	return func(c *CandidateAction) { c.Cooldown = d; c.CooldownKey = "semantic:" + c.Semantic }
}

func withMin(d time.Duration) candidateOption {
	return func(c *CandidateAction) { c.MinPlay = d }
}

func withExpires(d time.Duration) candidateOption {
	return func(c *CandidateAction) {
		t := time.Now().Add(d)
		c.ExpiresAt = &t
	}
}

func withUninterruptible(v bool) candidateOption {
	return func(c *CandidateAction) {
		if !v {
			c.InterruptPolicy = "interruptible"
		} else {
			c.InterruptPolicy = "uninterruptible"
		}
	}
}

func makeCandidate(semantic string, keys []string, eventID, layer string, priority int, now time.Time, opts ...candidateOption) CandidateAction {
	c := CandidateAction{
		Semantic:      semantic,
		PreferredKeys: keys,
		SourceEventID: eventID,
		SourceLayer:   layer,
		Priority:      priority,
		CreatedAt:     now,
		InterruptPolicy: "interruptible",
	}
	for _, opt := range opts {
		opt(&c)
	}
	return c
}

package behavior

import (
	"fmt"
	"strings"
	"sync"
)

type FallbackGraph struct {
	edges    map[string][]string
	compiled bool
	hasCycle bool
	maxDepth int
}

func NewFallbackGraph() *FallbackGraph {
	return &FallbackGraph{
		edges:    make(map[string][]string),
		maxDepth: MaxFallbackDepth,
	}
}

func (g *FallbackGraph) AddEdge(from string, to ...string) {
	g.edges[from] = append(g.edges[from], to...)
}

func (g *FallbackGraph) Compile() error {
	for start := range g.edges {
		if g.detectCycle(start, make(map[string]bool), 0) {
			g.hasCycle = true
			return fmt.Errorf("fallback graph has cycle starting from %s", start)
		}
	}
	g.compiled = true
	return nil
}

func (g *FallbackGraph) detectCycle(node string, visited map[string]bool, depth int) bool {
	if depth > g.maxDepth {
		return false
	}
	if visited[node] {
		return true
	}
	visited[node] = true
	defer delete(visited, node)
	for _, next := range g.edges[node] {
		if g.detectCycle(next, visited, depth+1) {
			return true
		}
	}
	return false
}

func (g *FallbackGraph) Resolve(semantic string, availableActions map[string]bool) (string, int, error) {
	if !g.compiled {
		if err := g.Compile(); err != nil {
			return "", 0, err
		}
	}
	visited := make(map[string]bool)
	return g.resolveDFS(semantic, availableActions, visited, 0)
}

func (g *FallbackGraph) resolveDFS(node string, available map[string]bool, visited map[string]bool, depth int) (string, int, error) {
	if depth > g.maxDepth {
		return "", depth, fmt.Errorf("max fallback depth exceeded")
	}
	if visited[node] {
		return "", depth, fmt.Errorf("cycle detected at %s", node)
	}
	visited[node] = true

	if available[node] {
		return node, depth, nil
	}

	nexts, ok := g.edges[node]
	if !ok || len(nexts) == 0 {
		return "", depth, fmt.Errorf("no fallback available for %s", node)
	}

	for _, next := range nexts {
		result, d, err := g.resolveDFS(next, available, visited, depth+1)
		if err == nil {
			return result, d, nil
		}
	}
	return "", depth, fmt.Errorf("no available action in fallback chain from %s", node)
}

func DefaultFallbackGraph() *FallbackGraph {
	g := NewFallbackGraph()
	g.AddEdge("speaking", "agreeing", "idle_breathing", "idle_normal")
	g.AddEdge("listening", "idle_look_around", "idle_breathing", "idle_normal")
	g.AddEdge("thinking", "work", "idle_look_around", "idle_normal")
	g.AddEdge("working", "work", "study", "thinking", "idle_normal")
	g.AddEdge("happy", "excited", "wave", "idle_normal")
	g.AddEdge("sad", "tired", "idle_normal")
	g.AddEdge("angry", "disagreeing", "shake_head", "idle_normal")
	g.AddEdge("sleep", "sleep_on_desktop", "sit", "idle_normal")
	g.AddEdge("study", "read", "write", "work", "idle_normal")
	g.AddEdge("clicked", "surprised", "idle_blink", "idle_normal")
	g.AddEdge("dragged", "picked_up", "idle_normal")
	g.AddEdge("greeting", "wave", "bow", "idle_normal")
	g.AddEdge("goodbye", "wave", "idle_normal")
	g.AddEdge("excited", "happy", "wave", "idle_normal")
	g.AddEdge("scared", "sad", "tired", "idle_normal")
	g.AddEdge("cry", "sad", "tired", "idle_normal")
	g.AddEdge("surprised", "idle_blink", "idle_normal")
	g.AddEdge("confused", "thinking", "idle_look_around", "idle_normal")
	g.AddEdge("embarrassed", "shy", "idle_normal")
	g.AddEdge("proud", "happy", "idle_normal")
	g.AddEdge("tired", "idle_breathing", "idle_normal")
	g.AddEdge("waiting", "idle_look_around", "idle_breathing", "idle_normal")
	g.AddEdge("researching", "read", "study", "thinking", "idle_normal")
	g.AddEdge("organizing", "write", "work", "idle_normal")
	g.AddEdge("generic_work", "work", "thinking", "idle_normal")
	g.AddEdge("attentive_idle", "idle_look_around", "idle_breathing", "idle_normal")
	g.AddEdge("calm_idle", "idle_breathing", "idle_normal")
	g.AddEdge("micro_idle", "idle_blink", "idle_normal")
	g.AddEdge("relaxed_idle", "idle_sway", "idle_breathing", "idle_normal")
	g.AddEdge("fallback_idle", "idle_normal")
	g.AddEdge("emotion_happy", "happy", "excited", "wave", "idle_normal")
	g.AddEdge("emotion_sad", "sad", "tired", "idle_normal")
	g.AddEdge("emotion_angry", "angry", "disagreeing", "idle_normal")
	g.AddEdge("emotion_excited", "excited", "happy", "wave", "idle_normal")
	g.AddEdge("emotion_scared", "scared", "sad", "idle_normal")
	g.AddEdge("emotion_cry", "cry", "sad", "idle_normal")
	g.AddEdge("emotion_surprised", "surprised", "idle_blink", "idle_normal")
	g.AddEdge("emotion_confused", "confused", "thinking", "idle_normal")
	g.AddEdge("emotion_shy", "shy", "idle_normal")
	g.AddEdge("emotion_embarrassed", "embarrassed", "shy", "idle_normal")
	g.AddEdge("emotion_proud", "proud", "happy", "idle_normal")
	g.AddEdge("emotion_tired", "tired", "idle_breathing", "idle_normal")
	g.AddEdge("activity_sit", "sit", "idle_normal")
	g.AddEdge("activity_sleep", "sleep", "sleep_on_desktop", "sit", "idle_normal")
	g.AddEdge("activity_wake", "wake_up", "idle_normal")
	g.AddEdge("activity_eat", "eat", "idle_normal")
	g.AddEdge("activity_drink", "drink", "idle_normal")
	g.AddEdge("activity_read", "read", "idle_normal")
	g.AddEdge("activity_write", "write", "idle_normal")
	g.AddEdge("activity_phone", "use_phone", "idle_normal")
	g.AddEdge("activity_work", "work", "idle_normal")
	g.AddEdge("activity_study", "study", "read", "idle_normal")
	g.AddEdge("activity_desktop_sleep", "sleep_on_desktop", "sit", "idle_normal")
	g.AddEdge("dialogue_listening", "listening", "idle_look_around", "idle_normal")
	g.AddEdge("dialogue_thinking", "thinking", "idle_look_around", "idle_normal")
	g.AddEdge("dialogue_speaking", "speaking", "agreeing", "idle_normal")
	g.AddEdge("dialogue_agreeing", "agreeing", "nod", "idle_normal")
	g.AddEdge("dialogue_disagreeing", "disagreeing", "shake_head", "idle_normal")
	g.AddEdge("dialogue_waiting", "waiting", "idle_look_around", "idle_normal")
	g.AddEdge("dialogue_greeting", "greeting", "wave", "bow", "idle_normal")
	g.AddEdge("dialogue_goodbye", "goodbye", "wave", "idle_normal")
	g.AddEdge("gesture_click", "clicked", "surprised", "idle_normal")
	g.AddEdge("gesture_double_click", "double_clicked", "surprised", "idle_normal")
	g.AddEdge("gesture_hover", "hovered", "idle_normal")
	g.AddEdge("gesture_drag", "dragged", "picked_up", "idle_normal")
	g.AddEdge("gesture_pickup", "picked_up", "idle_normal")
	g.AddEdge("gesture_drop", "dropped", "land", "idle_normal")
	g.AddEdge("physics_fall", "fall", "land", "idle_normal")
	g.AddEdge("physics_edge_sit", "edge_sit", "sit", "idle_normal")
	g.AddEdge("physics_edge_climb", "edge_climb", "idle_normal")
	g.AddEdge("jump_transition", "jump", "land", "idle_normal")
	g.AddEdge("land_transition", "land", "idle_normal")
	g.AddEdge("orientation_change", "turn_around", "idle_normal")
	g.AddEdge("locomotion_left", "walk_left", "idle_normal")
	g.AddEdge("locomotion_right", "walk_right", "idle_normal")
	g.AddEdge("urgent_locomotion_left", "run_left", "walk_left", "idle_normal")
	g.AddEdge("urgent_locomotion_right", "run_right", "walk_right", "idle_normal")
	g.AddEdge("break_gesture", "stretch", "idle_normal")
	g.AddEdge("agreement_gesture", "nod", "idle_normal")
	g.AddEdge("disagreement_gesture", "shake_head", "idle_normal")
	g.AddEdge("celebration_gesture", "clap", "idle_normal")
	g.AddEdge("attention_gesture", "point", "idle_normal")
	g.AddEdge("formal_greeting", "bow", "wave", "idle_normal")
	g.AddEdge("wave_gesture", "wave", "idle_normal")

	_ = g.Compile()
	return g
}

func (g *FallbackGraph) EdgesString() string {
	var sb strings.Builder
	for from, tos := range g.edges {
		sb.WriteString(fmt.Sprintf("%s -> %v\n", from, tos))
	}
	return sb.String()
}

type ActionUnavailableCache struct {
	mu       sync.RWMutex
	disabled map[string]bool
}

func NewActionUnavailableCache() *ActionUnavailableCache {
	return &ActionUnavailableCache{disabled: make(map[string]bool)}
}

func (c *ActionUnavailableCache) MarkUnavailable(actionKey string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.disabled[actionKey] = true
}

func (c *ActionUnavailableCache) IsDisabled(actionKey string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.disabled[actionKey]
}

func (c *ActionUnavailableCache) IsAvailable(actionKey string, installationActions map[string]ActionCapability) bool {
	if c.IsDisabled(actionKey) {
		return false
	}
	cap, ok := installationActions[actionKey]
	return ok && cap.Available
}

func (c *ActionUnavailableCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.disabled = make(map[string]bool)
}

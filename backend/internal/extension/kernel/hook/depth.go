package hook

import (
	"context"
	"fmt"
	"sync"
)

const DefaultMaxDepth = 4

type DepthGuard struct {
	mu        sync.Mutex
	maxDepth  int
	stacks    map[string]*callStack
}

type callStack struct {
	invocationID string
	stack        []stackEntry
}

type stackEntry struct {
	ContributionID string
	HookPointID    string
	Depth          int
}

func NewDepthGuard(maxDepth int) *DepthGuard {
	if maxDepth <= 0 {
		maxDepth = DefaultMaxDepth
	}
	return &DepthGuard{
		maxDepth: maxDepth,
		stacks:   make(map[string]*callStack),
	}
}

func (g *DepthGuard) CheckAndEnter(invocationID, contributionID, hookPointID string, parentDepth int) (int, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	stack, ok := g.stacks[invocationID]
	if !ok {
		stack = &callStack{invocationID: invocationID}
		g.stacks[invocationID] = stack
	}

	for _, entry := range stack.stack {
		if entry.ContributionID == contributionID && entry.HookPointID == hookPointID {
			return 0, fmt.Errorf("%w: contribution %s already in call stack for hook point %s", ErrRecursion, contributionID, hookPointID)
		}
	}

	newDepth := parentDepth + 1
	if newDepth > g.maxDepth {
		return 0, fmt.Errorf("%w: depth %d exceeds max %d", ErrDepthExceeded, newDepth, g.maxDepth)
	}

	stack.stack = append(stack.stack, stackEntry{
		ContributionID: contributionID,
		HookPointID:    hookPointID,
		Depth:          newDepth,
	})

	return newDepth, nil
}

func (g *DepthGuard) Exit(invocationID, contributionID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	stack, ok := g.stacks[invocationID]
	if !ok {
		return
	}
	for i := len(stack.stack) - 1; i >= 0; i-- {
		if stack.stack[i].ContributionID == contributionID {
			stack.stack = append(stack.stack[:i], stack.stack[i+1:]...)
			break
		}
	}
	if len(stack.stack) == 0 {
		delete(g.stacks, invocationID)
	}
}

func (g *DepthGuard) CurrentDepth(invocationID string) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	stack, ok := g.stacks[invocationID]
	if !ok {
		return 0
	}
	if len(stack.stack) == 0 {
		return 0
	}
	return stack.stack[len(stack.stack)-1].Depth
}

func (g *DepthGuard) Cleanup(invocationID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.stacks, invocationID)
}

type DepthContextKey struct{}

func DepthFromContext(ctx context.Context) int {
	if v, ok := ctx.Value(DepthContextKey{}).(int); ok {
		return v
	}
	return 0
}

func ContextWithDepth(ctx context.Context, depth int) context.Context {
	return context.WithValue(ctx, DepthContextKey{}, depth)
}

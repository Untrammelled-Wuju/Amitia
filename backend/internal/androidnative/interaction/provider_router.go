package interaction

import (
	"context"
	"errors"
	"sort"
	"time"
)

// providerRouteCandidate is an operation-specific provider candidate. Operation
// support and display support are decided before the candidate is added, so the
// router only compares providers that can actually perform the requested work.
type providerRouteCandidate struct {
	name      string
	strategy  string
	provider  any
	baseScore float64
	execute   func() error
}

type providerRouteStats struct {
	FailureRate float64
	LatencyMS   float64
	Attempts    uint64
	LastFailure time.Time
}

type providerHealthCacheEntry struct {
	health    ProviderCapabilityHealth
	expiresAt time.Time
}

const providerHealthRouteCacheTTL = 2 * time.Second

func (s *Service) routeCandidateScore(ctx context.Context, candidate providerRouteCandidate) (float64, bool) {
	if candidate.provider == nil || candidate.execute == nil {
		return 0, false
	}
	health := s.cachedProviderHealth(ctx, candidate.name, candidate.provider)
	var healthScore float64
	switch health.State {
	case ProviderStateReady:
		healthScore = 20
	case ProviderStateSupported:
		healthScore = 5
	case ProviderStateDegraded:
		healthScore = -15
	default:
		return 0, false
	}

	s.routeMu.Lock()
	stats := s.routeStats[candidate.name]
	s.routeMu.Unlock()
	failurePenalty := stats.FailureRate * 60
	latencyPenalty := stats.LatencyMS / 50
	if latencyPenalty > 20 {
		latencyPenalty = 20
	}
	return candidate.baseScore + healthScore - failurePenalty - latencyPenalty, true
}

func (s *Service) cachedProviderHealth(ctx context.Context, name string, provider any) ProviderCapabilityHealth {
	now := time.Now().UTC()
	s.routeMu.Lock()
	if cached, ok := s.routeHealth[name]; ok && now.Before(cached.expiresAt) {
		s.routeMu.Unlock()
		return cached.health
	}
	s.routeMu.Unlock()

	health := probeProviderHealth(ctx, name, provider)
	s.routeMu.Lock()
	s.routeHealth[name] = providerHealthCacheEntry{health: health, expiresAt: now.Add(providerHealthRouteCacheTTL)}
	s.routeMu.Unlock()
	return health
}

func (s *Service) rankProviderCandidates(ctx context.Context, candidates []providerRouteCandidate) []providerRouteCandidate {
	type scored struct {
		candidate providerRouteCandidate
		score     float64
		order     int
	}
	ranked := make([]scored, 0, len(candidates))
	for i, candidate := range candidates {
		score, usable := s.routeCandidateScore(ctx, candidate)
		if !usable {
			continue
		}
		ranked = append(ranked, scored{candidate: candidate, score: score, order: i})
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score == ranked[j].score {
			return ranked[i].order < ranked[j].order
		}
		return ranked[i].score > ranked[j].score
	})
	out := make([]providerRouteCandidate, 0, len(ranked))
	for _, item := range ranked {
		out = append(out, item.candidate)
	}
	return out
}

func (s *Service) executeRankedProvider(ctx context.Context, candidates []providerRouteCandidate) (string, error) {
	ranked := s.rankProviderCandidates(ctx, candidates)
	if len(ranked) == 0 {
		return "", &Error{Code: INTERACTION_UNAVAILABLE, Message: "no healthy provider supports this operation"}
	}
	var lastErr error
	for _, candidate := range ranked {
		started := time.Now()
		err := candidate.execute()
		s.recordProviderRoute(candidate.name, time.Since(started), err)
		if err == nil {
			return candidate.strategy, nil
		}
		lastErr = err
		if !providerErrorCanFallback(err) {
			return "", err
		}
	}
	if lastErr == nil {
		lastErr = &Error{Code: INTERACTION_UNAVAILABLE, Message: "all providers unavailable"}
	}
	return "", lastErr
}

func (s *Service) recordProviderRoute(name string, latency time.Duration, err error) {
	const alpha = 0.2
	s.routeMu.Lock()
	defer s.routeMu.Unlock()
	stats := s.routeStats[name]
	stats.Attempts++
	latencyMS := float64(latency.Microseconds()) / 1000
	if stats.LatencyMS == 0 {
		stats.LatencyMS = latencyMS
	} else {
		stats.LatencyMS = stats.LatencyMS*(1-alpha) + latencyMS*alpha
	}
	failure := 0.0
	if err != nil {
		failure = 1
		stats.LastFailure = time.Now().UTC()
	}
	stats.FailureRate = stats.FailureRate*(1-alpha) + failure*alpha
	s.routeStats[name] = stats
}

// providerErrorCanFallback is deliberately conservative. Retrying through a
// second side-effect provider is safe only when the first provider tells us the
// action was not executed. Timeout/cancellation/context-change/unknown outcomes
// stop routing so a click/input cannot be duplicated accidentally.
func providerErrorCanFallback(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var interactionErr *Error
	if !errors.As(err, &interactionErr) {
		return false
	}
	switch interactionErr.Code {
	case INTERACTION_UNSUPPORTED,
		INTERACTION_UNAVAILABLE,
		INTERACTION_ACTION_UNSUPPORTED,
		INTERACTION_ROOT_UNAVAILABLE,
		INTERACTION_ADB_UNAVAILABLE,
		INTERACTION_SHIZUKU_UNAVAILABLE,
		INTERACTION_NATIVE_HOST_UNAVAILABLE,
		INTERACTION_DISPLAY_UNAVAILABLE:
		return true
	case INTERACTION_TIMEOUT,
		INTERACTION_CANCELLED,
		INTERACTION_CONTEXT_CHANGED,
		INTERACTION_OUTCOME_UNKNOWN,
		INTERACTION_NODE_STALE:
		return false
	default:
		return false
	}
}

func (s *Service) executeTapRouted(ctx context.Context, displayID, x, y int, allowCoordinate, allowShizuku, allowRoot, allowADB bool) (string, error) {
	candidates := make([]providerRouteCandidate, 0, 4)
	if allowCoordinate && s.coordinate != nil {
		candidates = append(candidates, providerRouteCandidate{name: "accessibility_gesture", strategy: StrategyCoordinate, provider: s.coordinate, baseScore: 100, execute: func() error { return s.coordinate.Tap(ctx, displayID, x, y) }})
	}
	if displayID == 0 && allowShizuku && s.policy.AllowShizukuFallback && s.shizuku != nil {
		candidates = append(candidates, providerRouteCandidate{name: "shizuku", strategy: StrategyShizuku, provider: s.shizuku, baseScore: 95, execute: func() error { return s.shizuku.Tap(ctx, x, y) }})
	}
	if displayID == 0 && allowRoot && s.policy.AllowRootFallback && s.root != nil {
		candidates = append(candidates, providerRouteCandidate{name: "root", strategy: StrategyRoot, provider: s.root, baseScore: 90, execute: func() error { return s.root.Tap(ctx, x, y) }})
	}
	if displayID == 0 && allowADB && s.policy.AllowADBFallback && s.adb != nil {
		candidates = append(candidates, providerRouteCandidate{name: "adb", strategy: StrategyADB, provider: s.adb, baseScore: 80, execute: func() error { return s.adb.Tap(ctx, x, y) }})
	}
	return s.executeRankedProvider(ctx, candidates)
}

func (s *Service) executeSwipeRouted(ctx context.Context, req SwipeRequest, allowCoordinate, allowShizuku, allowRoot, allowADB bool) (string, error) {
	candidates := make([]providerRouteCandidate, 0, 4)
	if allowCoordinate && s.coordinate != nil {
		candidates = append(candidates, providerRouteCandidate{name: "accessibility_gesture", strategy: StrategyCoordinate, provider: s.coordinate, baseScore: 100, execute: func() error { return s.coordinate.Swipe(ctx, req) }})
	}
	if req.DisplayID == 0 && allowShizuku && s.policy.AllowShizukuFallback && s.shizuku != nil {
		candidates = append(candidates, providerRouteCandidate{name: "shizuku", strategy: StrategyShizuku, provider: s.shizuku, baseScore: 95, execute: func() error { return s.shizuku.Swipe(ctx, req.StartX, req.StartY, req.EndX, req.EndY, req.DurationMS) }})
	}
	if req.DisplayID == 0 && allowRoot && s.policy.AllowRootFallback && s.root != nil {
		candidates = append(candidates, providerRouteCandidate{name: "root", strategy: StrategyRoot, provider: s.root, baseScore: 90, execute: func() error { return s.root.Swipe(ctx, req.StartX, req.StartY, req.EndX, req.EndY, req.DurationMS) }})
	}
	if req.DisplayID == 0 && allowADB && s.policy.AllowADBFallback && s.adb != nil {
		candidates = append(candidates, providerRouteCandidate{name: "adb", strategy: StrategyADB, provider: s.adb, baseScore: 80, execute: func() error { return s.adb.Swipe(ctx, req.StartX, req.StartY, req.EndX, req.EndY, req.DurationMS) }})
	}
	return s.executeRankedProvider(ctx, candidates)
}

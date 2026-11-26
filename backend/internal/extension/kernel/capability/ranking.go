package capability

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
)

var (
	ErrRankingStrategyMissing     = errors.New("ranking strategy is missing")
	ErrRankingVersionUnknown      = errors.New("ranking version is unknown")
	ErrRankingWeightsInvalid      = errors.New("ranking weights are invalid")
	ErrRankingRollbackUnavailable = errors.New("ranking rollback is unavailable")
	ErrRankingSnapshotDrift       = errors.New("ranking snapshot digest does not match")
)

const (
	RankingVersionLexical = "lexical-v1"
	RankingVersionHybrid  = "hybrid-v2"
)

type RankingWeights struct {
	Lexical  float64 `json:"lexical"`
	Domain   float64 `json:"domain"`
	Usage    float64 `json:"usage"`
	Explicit float64 `json:"explicit"`
}

func (w RankingWeights) Valid() bool {
	total := 0.0
	for _, value := range []float64{w.Lexical, w.Domain, w.Usage, w.Explicit} {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			return false
		}
		total += value
	}
	return total > 0
}

type UsageStat struct {
	Calls     int64 `json:"calls"`
	Successes int64 `json:"successes"`
	Failures  int64 `json:"failures"`
}

func (s UsageStat) Reliability() float64 {
	calls := float64(s.Successes + s.Failures)
	return (float64(s.Successes) + 1) / (calls + 2)
}

func (s UsageStat) Volume() float64 {
	return math.Log1p(float64(s.Calls))
}

type UsageProvider interface {
	Usage(ctx context.Context, toolID string) (UsageStat, bool, error)
}

type RankCandidate struct {
	Tool    ToolDefinition
	Lexical int
	Matched []string
	Domains []TaskDomain
}

type RankedTool struct {
	Tool    ToolDefinition
	Score   float64
	Matched []string
	Domains []TaskDomain
	Signals map[string]float64
}

type RankingStrategy interface {
	Version() string
	Weights() RankingWeights
	Rank(ctx context.Context, profile TaskProfile, query string, candidates []RankCandidate) ([]RankedTool, error)
}

type LexicalStrategy struct{}

func (LexicalStrategy) Version() string {
	return RankingVersionLexical
}

func (LexicalStrategy) Weights() RankingWeights {
	return RankingWeights{Lexical: 1}
}

func (LexicalStrategy) Rank(ctx context.Context, profile TaskProfile, query string, candidates []RankCandidate) ([]RankedTool, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ranked := make([]RankedTool, 0, len(candidates))
	for _, candidate := range candidates {
		score := float64(candidate.Lexical)
		ranked = append(ranked, RankedTool{
			Tool:    candidate.Tool,
			Score:   score,
			Matched: candidate.Matched,
			Domains: candidate.Domains,
			Signals: map[string]float64{"lexical": score},
		})
	}
	sortRanked(ranked)
	return ranked, nil
}

type HybridStrategy struct {
	weights RankingWeights
	usage   UsageProvider
}

func NewHybridStrategy(weights RankingWeights, usage UsageProvider) (*HybridStrategy, error) {
	if !weights.Valid() {
		return nil, fmt.Errorf("%w: lexical=%v domain=%v usage=%v explicit=%v", ErrRankingWeightsInvalid, weights.Lexical, weights.Domain, weights.Usage, weights.Explicit)
	}
	return &HybridStrategy{weights: weights, usage: usage}, nil
}

func (h *HybridStrategy) Version() string {
	return RankingVersionHybrid
}

func (h *HybridStrategy) Weights() RankingWeights {
	return h.weights
}

func (h *HybridStrategy) Rank(ctx context.Context, profile TaskProfile, query string, candidates []RankCandidate) ([]RankedTool, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ranked := make([]RankedTool, 0, len(candidates))
	for _, candidate := range candidates {
		signals := map[string]float64{
			"lexical":  float64(candidate.Lexical),
			"domain":   domainSignal(profile, candidate.Domains),
			"explicit": explicitSignal(candidate.Tool, query),
		}
		if h.usage != nil {
			stat, found, err := h.usage.Usage(ctx, candidate.Tool.ID)
			if err != nil {
				return nil, fmt.Errorf("resolve usage for tool %s: %w", candidate.Tool.ID, err)
			}
			if found {
				signals["usage"] = stat.Reliability() * (1 + stat.Volume())
			} else {
				signals["usage"] = 0
			}
		} else {
			signals["usage"] = 0
		}
		score := h.weights.Lexical*signals["lexical"] +
			h.weights.Domain*signals["domain"]*10 +
			h.weights.Explicit*signals["explicit"]*20 +
			h.weights.Usage*signals["usage"]*10
		ranked = append(ranked, RankedTool{
			Tool:    candidate.Tool,
			Score:   score,
			Matched: candidate.Matched,
			Domains: candidate.Domains,
			Signals: signals,
		})
	}
	sortRanked(ranked)
	return ranked, nil
}

func domainSignal(profile TaskProfile, domains []TaskDomain) float64 {
	if profile.Domain == TaskDomainUnknown {
		return 0
	}
	for _, domain := range domains {
		if domain == profile.Domain {
			return 1
		}
	}
	return 0
}

func explicitSignal(tool ToolDefinition, query string) float64 {
	lowered := normalizeName(query)
	if lowered == "" {
		return 0
	}
	for _, name := range []string{tool.Name, tool.ModelName, tool.ID} {
		candidate := normalizeName(name)
		if len([]rune(candidate)) < 3 {
			continue
		}
		if strings.Contains(lowered, candidate) {
			return 1
		}
	}
	return 0
}

func normalizeName(value string) string {
	var builder strings.Builder
	for _, char := range strings.ToLower(value) {
		if char == '_' || char == '-' || char == ' ' || char == '.' {
			continue
		}
		builder.WriteRune(char)
	}
	return builder.String()
}

func sortRanked(ranked []RankedTool) {
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].Score != ranked[j].Score {
			return ranked[i].Score > ranked[j].Score
		}
		return ranked[i].Tool.ID < ranked[j].Tool.ID
	})
}

type StrategySnapshot struct {
	Version  string         `json:"version"`
	Previous string         `json:"previous,omitempty"`
	Weights  RankingWeights `json:"weights"`
	Digest   string         `json:"digest"`
}

type rankingSnapshotBody struct {
	Version  string         `json:"version"`
	Previous string         `json:"previous,omitempty"`
	Weights  RankingWeights `json:"weights"`
}

func (s StrategySnapshot) computeDigest() (string, error) {
	body, err := json.Marshal(rankingSnapshotBody{Version: s.Version, Previous: s.Previous, Weights: s.Weights})
	if err != nil {
		return "", fmt.Errorf("marshal ranking snapshot: %w", err)
	}
	sum := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

type RankingPolicy struct {
	mu         sync.RWMutex
	strategies map[string]RankingStrategy
	active     string
	previous   string
	rankings   int64
}

func NewRankingPolicy(strategies ...RankingStrategy) (*RankingPolicy, error) {
	if len(strategies) == 0 {
		return nil, fmt.Errorf("%w: no strategies registered", ErrRankingStrategyMissing)
	}
	policy := &RankingPolicy{strategies: map[string]RankingStrategy{}}
	for _, strategy := range strategies {
		if strategy == nil {
			return nil, fmt.Errorf("%w: nil strategy", ErrRankingStrategyMissing)
		}
		version := strings.TrimSpace(strategy.Version())
		if version == "" {
			return nil, fmt.Errorf("%w: strategy has no version", ErrRankingStrategyMissing)
		}
		if _, exists := policy.strategies[version]; exists {
			return nil, fmt.Errorf("%w: duplicate version %s", ErrRankingStrategyMissing, version)
		}
		policy.strategies[version] = strategy
	}
	policy.active = strings.TrimSpace(strategies[0].Version())
	return policy, nil
}

func (p *RankingPolicy) Activate(ctx context.Context, version string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	next, ok := p.strategies[strings.TrimSpace(version)]
	if !ok {
		return fmt.Errorf("%w: %s", ErrRankingVersionUnknown, version)
	}
	if next.Version() == p.active {
		return nil
	}
	p.previous = p.active
	p.active = next.Version()
	return nil
}

func (p *RankingPolicy) Version() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.active
}

func (p *RankingPolicy) Rankings() int64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.rankings
}

func (p *RankingPolicy) Snapshot() StrategySnapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.snapshotLocked()
}

func (p *RankingPolicy) snapshotLocked() StrategySnapshot {
	snapshot := StrategySnapshot{Version: p.active, Previous: p.previous}
	if strategy := p.strategies[p.active]; strategy != nil {
		snapshot.Weights = strategy.Weights()
	}
	digest, err := snapshot.computeDigest()
	if err == nil {
		snapshot.Digest = digest
	}
	return snapshot
}

func (p *RankingPolicy) Restore(ctx context.Context, snapshot StrategySnapshot) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	digest, err := snapshot.computeDigest()
	if err != nil {
		return err
	}
	if digest != snapshot.Digest {
		return fmt.Errorf("%w: want=%s got=%s", ErrRankingSnapshotDrift, digest, snapshot.Digest)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.strategies[snapshot.Version]; !ok {
		return fmt.Errorf("%w: %s", ErrRankingVersionUnknown, snapshot.Version)
	}
	if snapshot.Previous != "" {
		if _, ok := p.strategies[snapshot.Previous]; !ok {
			return fmt.Errorf("%w: %s", ErrRankingVersionUnknown, snapshot.Previous)
		}
	}
	p.active = snapshot.Version
	p.previous = snapshot.Previous
	return nil
}

func (p *RankingPolicy) Rollback(ctx context.Context) (StrategySnapshot, error) {
	if err := ctx.Err(); err != nil {
		return StrategySnapshot{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.previous == "" {
		return p.snapshotLocked(), fmt.Errorf("%w: active=%s", ErrRankingRollbackUnavailable, p.active)
	}
	p.active, p.previous = p.previous, p.active
	return p.snapshotLocked(), nil
}

func (p *RankingPolicy) Rank(ctx context.Context, profile TaskProfile, query string, candidates []RankCandidate) ([]RankedTool, StrategySnapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, StrategySnapshot{}, err
	}
	p.mu.RLock()
	strategy := p.strategies[p.active]
	snapshot := p.snapshotLocked()
	p.mu.RUnlock()
	if strategy == nil {
		return nil, snapshot, fmt.Errorf("%w: %s", ErrRankingVersionUnknown, snapshot.Version)
	}
	ranked, err := strategy.Rank(ctx, profile, query, candidates)
	if err != nil {
		return nil, snapshot, err
	}
	p.mu.Lock()
	p.rankings++
	p.mu.Unlock()
	return ranked, snapshot, nil
}

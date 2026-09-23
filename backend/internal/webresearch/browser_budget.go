package webresearch

import (
	"context"
	"sync"
	"time"
)

type browserBudgetContextKey struct{}

type browserExecutionBudget struct {
	gate      chan struct{}
	mu        sync.Mutex
	pagesLeft int
	timeLeft  time.Duration
	timeUsed  time.Duration
}

func newBrowserExecutionBudget(maxPages int, maxTime time.Duration) *browserExecutionBudget {
	if maxPages <= 0 {
		maxPages = 1
	}
	if maxTime <= 0 {
		maxTime = 15 * time.Second
	}
	return &browserExecutionBudget{
		gate:      make(chan struct{}, 1),
		pagesLeft: maxPages,
		timeLeft:  maxTime,
	}
}

func contextWithBrowserBudget(ctx context.Context, cfg Config) context.Context {
	return context.WithValue(ctx, browserBudgetContextKey{}, newBrowserExecutionBudget(cfg.MaxBrowserPages, cfg.MaxBrowserTime))
}

func browserBudgetFromContext(ctx context.Context) *browserExecutionBudget {
	if ctx == nil {
		return nil
	}
	budget, _ := ctx.Value(browserBudgetContextKey{}).(*browserExecutionBudget)
	return budget
}

func (b *browserExecutionBudget) acquire(ctx context.Context) (context.Context, func(), *Error) {
	if b == nil {
		return ctx, func() {}, nil
	}
	select {
	case b.gate <- struct{}{}:
	case <-ctx.Done():
		return nil, nil, newError(ErrCancelled, "browser budget wait cancelled", false, ctx.Err())
	}

	b.mu.Lock()
	if b.pagesLeft <= 0 || b.timeLeft <= 0 {
		b.mu.Unlock()
		<-b.gate
		return nil, nil, newError(ErrBudgetExhausted, "browser budget exhausted", false, nil)
	}
	b.pagesLeft--
	remaining := b.timeLeft
	b.mu.Unlock()

	child, cancel := context.WithTimeout(ctx, remaining)
	started := time.Now()
	release := func() {
		cancel()
		elapsed := time.Since(started)
		b.mu.Lock()
		b.timeLeft -= elapsed
		b.timeUsed += elapsed
		if b.timeLeft < 0 {
			b.timeLeft = 0
		}
		b.mu.Unlock()
		<-b.gate
	}
	return child, release, nil
}

func (b *browserExecutionBudget) used() time.Duration {
	if b == nil {
		return 0
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.timeUsed
}

func (r *Runtime) browserRead(ctx context.Context, rawURL string) (fetchedPage, *Error) {
	if r == nil || !r.config.BrowserEnabled || r.browser == nil || !r.browser.Available(ctx) {
		return fetchedPage{}, newError(ErrBrowserUnavailable, "browser runtime unavailable", true, nil)
	}
	budget := browserBudgetFromContext(ctx)
	browserCtx, release, err := budget.acquire(ctx)
	if err != nil {
		r.observe(context.WithoutCancel(ctx), Observation{Name: "web.browser", Operation: "read", Domain: domainOf(rawURL), PolicyDecision: "budget_blocked", ErrorCode: string(err.Code)})
		return fetchedPage{}, err
	}
	defer release()
	started := time.Now()
	page, readErr := r.browser.Read(browserCtx, rawURL)
	observation := Observation{Name: "web.browser", Operation: "read", Domain: domainOf(rawURL), PolicyDecision: "allowed", DurationMs: time.Since(started).Milliseconds()}
	if readErr != nil {
		observation.ErrorCode = string(readErr.Code)
	}
	r.observe(context.WithoutCancel(ctx), observation)
	return page, readErr
}

func (r *Runtime) browserScreenshot(ctx context.Context, rawURL string, page *int, fullPage bool) (ScreenshotResult, *Error) {
	if r == nil || !r.config.BrowserEnabled || r.browser == nil || !r.browser.Available(ctx) {
		return ScreenshotResult{}, newError(ErrBrowserUnavailable, "browser runtime unavailable", true, nil)
	}
	budget := browserBudgetFromContext(ctx)
	browserCtx, release, err := budget.acquire(ctx)
	if err != nil {
		r.observe(context.WithoutCancel(ctx), Observation{Name: "web.browser", Operation: "screenshot", Domain: domainOf(rawURL), PolicyDecision: "budget_blocked", ErrorCode: string(err.Code)})
		return ScreenshotResult{}, err
	}
	defer release()
	started := time.Now()
	shot, shotErr := r.browser.Screenshot(browserCtx, rawURL, page, fullPage)
	observation := Observation{Name: "web.browser", Operation: "screenshot", Domain: domainOf(rawURL), PolicyDecision: "allowed", DurationMs: time.Since(started).Milliseconds()}
	if shotErr != nil {
		observation.ErrorCode = string(shotErr.Code)
	}
	r.observe(context.WithoutCancel(ctx), observation)
	return shot, shotErr
}

package knowledge

import "context"

type resourceGate struct {
	sem chan struct{}
}

func newResourceGate(limit int) *resourceGate {
	if limit <= 0 {
		return nil
	}
	return &resourceGate{sem: make(chan struct{}, limit)}
}

func (g *resourceGate) Acquire(ctx context.Context) (func(), error) {
	if g == nil || g.sem == nil {
		return func() {}, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case g.sem <- struct{}{}:
		return func() { <-g.sem }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (g *resourceGate) Capacity() int {
	if g == nil || g.sem == nil {
		return 0
	}
	return cap(g.sem)
}

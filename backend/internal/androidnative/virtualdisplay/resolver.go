package virtualdisplay

import "context"

type DisplayTargetResolver interface {
	Resolve(ctx context.Context, displayID int) (*DisplayTarget, error)
}

type DefaultResolver struct {
	store *Store
}

func NewDefaultResolver(store *Store) *DefaultResolver {
	return &DefaultResolver{store: store}
}

func (r *DefaultResolver) Resolve(ctx context.Context, displayID int) (*DisplayTarget, error) {
	if r.store == nil {
		return nil, NewError(ErrVirtualDisplayUnavailable, "store not initialized")
	}
	rec := r.store.Get()
	if rec == nil {
		return nil, NewError(ErrVirtualDisplayNotFound, "no virtual display found")
	}
	return &DisplayTarget{
		DisplayID:  rec.DisplayID,
		Generation: rec.Generation,
		Width:      rec.Width,
		Height:     rec.Height,
		DPI:        rec.DensityDPI,
		Rotation:   rec.Rotation,
	}, nil
}

type PrimaryResolver struct{}

func (r *PrimaryResolver) Resolve(ctx context.Context, displayID int) (*DisplayTarget, error) {
	if displayID != 0 {
		return nil, NewError(ErrVirtualDisplayIdMismatch, "primary resolver only handles display 0")
	}
	return &DisplayTarget{
		DisplayID:  0,
		Generation: 0,
		Width:      0,
		Height:     0,
		DPI:        0,
		Rotation:   0,
	}, nil
}

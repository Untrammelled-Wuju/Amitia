package artifact

import (
	"context"
	"io"
)

type Resolver interface {
	Resolve(ctx context.Context, actor string, resourceURI string) (Artifact, error)
	Open(ctx context.Context, actor string, resourceURI string) (io.ReadCloser, Artifact, error)
}

type resolver struct {
	svc *Service
}

func NewResolver(svc *Service) Resolver {
	return &resolver{svc: svc}
}

func (r *resolver) Resolve(ctx context.Context, actor string, resourceURI string) (Artifact, error) {
	id, err := ParseURI(resourceURI)
	if err != nil {
		return Artifact{}, ErrInvalidReference(err.Error())
	}
	return r.svc.GetOwned(ctx, actor, id)
}

func (r *resolver) Open(ctx context.Context, actor string, resourceURI string) (io.ReadCloser, Artifact, error) {
	art, err := r.Resolve(ctx, actor, resourceURI)
	if err != nil {
		return nil, Artifact{}, err
	}
	rc, _, err := r.svc.OpenBlob(ctx, art.BlobDigest)
	if err != nil {
		return nil, Artifact{}, err
	}
	return rc, art, nil
}

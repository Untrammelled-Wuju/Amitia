package secret

import (
	"context"
	"strings"
)

type Store interface {
	Put(ctx context.Context, namespace string, value []byte) (string, error)
	Get(ctx context.Context, ref string) ([]byte, error)
	Delete(ctx context.Context, ref string) error
}

func sanitizeNamespace(value string) string {
	value = strings.TrimSpace(value)
	var result strings.Builder
	for _, item := range value {
		if item >= 'a' && item <= 'z' || item >= 'A' && item <= 'Z' || item >= '0' && item <= '9' || item == '-' || item == '_' {
			result.WriteRune(item)
		}
	}
	if result.Len() == 0 {
		return "default"
	}
	return result.String()
}

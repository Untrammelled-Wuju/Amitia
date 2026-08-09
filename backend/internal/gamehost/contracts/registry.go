package contracts

import (
	"context"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type PluginRegistry interface {
	Register(
		ctx context.Context,
		descriptor domain.PluginDescriptor,
	) error

	Unregister(
		ctx context.Context,
		pluginID domain.PluginID,
	) error

	Get(
		ctx context.Context,
		pluginID domain.PluginID,
	) (domain.PluginDescriptor, error)

	List(
		ctx context.Context,
	) ([]domain.PluginDescriptor, error)

	ListByExtension(
		ctx context.Context,
		extensionID string,
	) ([]domain.PluginDescriptor, error)
}

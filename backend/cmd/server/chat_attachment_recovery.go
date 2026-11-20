package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/artifact"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/runtimeorchestrator"
	amitialog "log"
)

const chatAttachmentRecoveryInterval = 2 * time.Minute

type chatAttachmentRecoveryComponent struct {
	services *AppServices
	mu       sync.Mutex
	cancel   context.CancelFunc
	done     chan struct{}
}

func newChatAttachmentRecoveryComponent(services *AppServices) *chatAttachmentRecoveryComponent {
	return &chatAttachmentRecoveryComponent{services: services}
}

func (c *chatAttachmentRecoveryComponent) Descriptor() runtimeorchestrator.ComponentDescriptor {
	enabled := c != nil && c.services != nil && c.services.Artifact != nil && c.services.Artifact.Service != nil && c.services.DeviceMesh != nil
	return runtimeorchestrator.ComponentDescriptor{
		ID:           runtimeorchestrator.ComponentID("component.chat-attachment-recovery"),
		Phase:        runtimeorchestrator.PhaseApplication,
		Enabled:      enabled,
		Required:     false,
		Dependencies: []runtimeorchestrator.ComponentID{runtimeorchestrator.ComponentExtensionKernel},
		Capabilities: []string{"chat.attachments.recovery"},
		Profiles:     profilesAll,
	}
}

func (c *chatAttachmentRecoveryComponent) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cancel != nil {
		return nil
	}
	workerCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	c.cancel = cancel
	c.done = done
	go func() {
		defer close(done)
		ticker := time.NewTicker(chatAttachmentRecoveryInterval)
		defer ticker.Stop()
		for {
			if err := c.reconcile(workerCtx); err != nil {
				amitialog.Printf("[original-error] chat attachment recovery failed: %+v", err)
			}
			select {
			case <-workerCtx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return nil
}

func (c *chatAttachmentRecoveryComponent) Ready(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cancel == nil {
		return errors.New("chat attachment recovery worker is not running")
	}
	return nil
}

func (c *chatAttachmentRecoveryComponent) Stop(context.Context) error {
	c.mu.Lock()
	cancel, done := c.cancel, c.done
	c.cancel, c.done = nil, nil
	c.mu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()
	<-done
	return nil
}

func (c *chatAttachmentRecoveryComponent) reconcile(ctx context.Context) error {
	if c == nil || c.services == nil || c.services.Artifact == nil || c.services.Artifact.Service == nil || c.services.DeviceMesh == nil {
		return errors.New("chat attachment recovery dependencies are unavailable")
	}
	records, err := c.services.Artifact.Service.ListChatAttachmentReferenceBindings(ctx, 2048)
	if err != nil {
		return fmt.Errorf("list chat attachment bindings: %w", err)
	}
	var failures []error
	for _, record := range records {
		if err := ctx.Err(); err != nil {
			return errors.Join(append(failures, err)...)
		}
		if err := c.reconcileRecord(ctx, record); err != nil {
			failures = append(failures, fmt.Errorf("reconcile artifact %s binding %s: %w", record.Artifact.ID, record.State, err))
		}
	}
	return errors.Join(failures...)
}

type chatAttachmentBindingMessage struct {
	ConversationID string                                 `json:"conversationId"`
	Attachments    []coordination.ChatAttachmentReference `json:"attachments"`
}

func (c *chatAttachmentRecoveryComponent) reconcileRecord(ctx context.Context, record artifact.ChatAttachmentReferenceBindingRecord) error {
	binding, err := coordination.ParseChatAttachmentReferenceBinding(record.Token)
	if err != nil {
		return fmt.Errorf("parse persisted chat attachment binding: %w", err)
	}
	if binding.Reference.ArtifactID != string(record.Artifact.ID) {
		return fmt.Errorf("chat attachment binding artifact mismatch: binding=%s record=%s: %w", binding.Reference.ArtifactID, record.Artifact.ID, coordination.ErrWrongOwner)
	}
	expected := chatAttachmentReference(record.Artifact)
	if binding.Reference != expected {
		return fmt.Errorf("chat attachment binding metadata mismatch: binding=%+v artifact=%+v: %w", binding.Reference, expected, coordination.ErrWrongOwner)
	}
	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	resource, err := c.services.DeviceMesh.Resource(callCtx, binding.Scope, "message", binding.MessageID)
	if err != nil {
		return fmt.Errorf("read message for attachment recovery: %w", err)
	}
	referenced := false
	if resource != nil && !resource.Deleted {
		var document chatAttachmentBindingMessage
		if err := json.Unmarshal(resource.Body, &document); err != nil {
			return fmt.Errorf("decode message for attachment recovery: %w", err)
		}
		for _, item := range document.Attachments {
			if item.ArtifactID != binding.Reference.ArtifactID {
				continue
			}
			if item != binding.Reference {
				return fmt.Errorf("saved message attachment metadata changed: message=%+v binding=%+v: %w", item, binding.Reference, coordination.ErrWrongOwner)
			}
			referenced = true
		}
	}
	if record.State == artifact.ChatAttachmentReferencePending {
		if referenced {
			if err := c.services.DeviceMesh.SetChatAttachmentReference(callCtx, binding.Scope, binding.Reference, binding.MessageID, coordination.ChatAttachmentReferenceCommit); err != nil {
				return fmt.Errorf("commit recovered chat attachment binding: %w", err)
			}
			return nil
		}
		if err := c.services.DeviceMesh.SetChatAttachmentReference(callCtx, binding.Scope, binding.Reference, binding.MessageID, coordination.ChatAttachmentReferenceAbort); err != nil {
			return fmt.Errorf("abort stale chat attachment binding: %w", err)
		}
		return nil
	}
	if referenced {
		return nil
	}
	if err := c.services.DeviceMesh.SetChatAttachmentReference(callCtx, binding.Scope, binding.Reference, binding.MessageID, coordination.ChatAttachmentReferenceRemove); err != nil {
		return fmt.Errorf("remove stale confirmed chat attachment binding: %w", err)
	}
	return nil
}

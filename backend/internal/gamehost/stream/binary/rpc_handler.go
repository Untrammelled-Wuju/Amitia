package binary

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/gamehost/channel"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/resource"
	"github.com/u-ai/backend/internal/gamehost/rpc"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

const (
	MethodBinaryCreate  rpc.Method = "binary.create"
	MethodBinaryWrite   rpc.Method = "binary.write"
	MethodBinarySeal    rpc.Method = "binary.seal"
	MethodBinaryRead    rpc.Method = "binary.read"
	MethodBinaryStat    rpc.Method = "binary.stat"
	MethodBinaryRelease rpc.Method = "binary.release"
	MethodBinaryAbort   rpc.Method = "binary.abort"

	DefaultBinaryChunkBytes = 512 * 1024
	MaxBinaryChunkBytes     = 2 * 1024 * 1024
	MaxBinaryRPCObjectBytes = int64(1 << 30)
)

type NegotiatedFeatureChecker interface {
	HasNegotiatedCapability(connectionID string, feature domain.Capability) bool
}

type BinaryResourceAdmission interface {
	AcquireBinaryObject(ctx context.Context, subj resource.RuntimeIdentitySubject, requestedBytes int64) (resource.AdmissionDecision, resource.BinaryRevertFunc)
}

type BinaryTransferService struct {
	resolver       *Resolver
	registry       ObjectRegistry
	channels       channel.Registry
	featureChecker NegotiatedFeatureChecker
	admission      BinaryResourceAdmission

	mu       sync.Mutex
	writes   map[BinaryObjectID]*binaryWriteSession
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	lifetime BinaryLifetimePolicy
}

type binaryWriteSession struct {
	owner        BinaryOwner
	handle       WritingHandle
	expectedSize int64
	written      int64
	lastActivity time.Time
}

func NewBinaryTransferService(resolver *Resolver, registry ObjectRegistry, channels channel.Registry, checker NegotiatedFeatureChecker, admission BinaryResourceAdmission) (*BinaryTransferService, error) {
	if resolver == nil || registry == nil || channels == nil || checker == nil || admission == nil {
		return nil, fmt.Errorf("binary rpc: resolver, object registry, channel registry, feature checker and resource admission are required")
	}
	return &BinaryTransferService{
		resolver:       resolver,
		registry:       registry,
		channels:       channels,
		featureChecker: checker,
		admission:      admission,
		writes:         make(map[BinaryObjectID]*binaryWriteSession),
		lifetime:       DefaultLifetimePolicy(),
	}, nil
}

func (s *BinaryTransferService) Start(parent context.Context) {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.cancel != nil {
		s.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel
	s.wg.Add(1)
	s.mu.Unlock()
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = s.SweepExpired(context.Background(), time.Now().UTC())
			}
		}
	}()
}

func (s *BinaryTransferService) Shutdown(ctx context.Context) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	cancel := s.cancel
	s.cancel = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
		s.wg.Wait()
	}

	// Abort all in-flight writers before provider shutdown. This closes file
	// handles on Windows and prevents partially uploaded objects from surviving
	// host shutdown merely because they never reached READY state.
	s.mu.Lock()
	activeWrites := make(map[BinaryObjectID]*binaryWriteSession, len(s.writes))
	for id, session := range s.writes {
		activeWrites[id] = session
		delete(s.writes, id)
	}
	s.mu.Unlock()
	for id, session := range activeWrites {
		s.abortSession(ctx, id, session)
	}

	records := s.registry.GetActiveObjects()
	seenRuntime := make(map[domain.RuntimeInstanceID]struct{})
	for _, record := range records {
		seenRuntime[record.Owner.RuntimeID] = struct{}{}
	}
	var errs []error
	for runtimeID := range seenRuntime {
		if err := s.CleanupRuntime(ctx, runtimeID); err != nil {
			errs = append(errs, err)
		}
	}
	if err := s.resolver.Shutdown(ctx); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (s *BinaryTransferService) SweepExpired(ctx context.Context, now time.Time) error {
	if s == nil || s.registry == nil {
		return nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	var errs []error
	var staleIDs []BinaryObjectID
	var staleSessions []*binaryWriteSession
	s.mu.Lock()
	for id, session := range s.writes {
		if session != nil && !session.lastActivity.IsZero() && !now.Before(session.lastActivity.Add(s.lifetime.MessageTTL)) {
			staleIDs = append(staleIDs, id)
			staleSessions = append(staleSessions, session)
			delete(s.writes, id)
		}
	}
	s.mu.Unlock()
	for i, session := range staleSessions {
		s.abortSession(ctx, staleIDs[i], session)
	}

	for _, record := range s.registry.GetActiveObjects() {
		expires := s.lifetime.ExpiryTime(record.Lifetime, record.CreatedAt)
		if expires.IsZero() || now.Before(expires) {
			continue
		}

		s.mu.Lock()
		session, writing := s.writes[record.ID]
		if writing {
			delete(s.writes, record.ID)
		}
		s.mu.Unlock()
		if writing {
			s.abortSession(ctx, record.ID, session)
			continue
		}
		if err := s.resolver.Release(ctx, record.Owner, record.ID); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (s *BinaryTransferService) Register(registry rpc.HandlerRegistry) error {
	if s == nil || registry == nil {
		return fmt.Errorf("binary rpc: transfer service and handler registry are required")
	}
	for _, method := range []rpc.Method{
		MethodBinaryCreate,
		MethodBinaryWrite,
		MethodBinarySeal,
		MethodBinaryRead,
		MethodBinaryStat,
		MethodBinaryRelease,
		MethodBinaryAbort,
	} {
		if err := registry.Register(method, s); err != nil {
			return fmt.Errorf("binary rpc: register %s: %w", method, err)
		}
	}
	return nil
}

func (s *BinaryTransferService) Handle(ctx context.Context, request rpc.RPCRequest) (rpc.RPCResponse, error) {
	if s == nil || s.resolver == nil || s.registry == nil || s.channels == nil || s.featureChecker == nil || s.admission == nil {
		return rpc.RPCResponse{}, fmt.Errorf("binary rpc: transfer service unavailable")
	}
	if strings.TrimSpace(request.ConnectionID) == "" || request.PluginID == "" || request.RuntimeID == "" || request.ServiceID == "" {
		return binaryRPCError(request.ID, domain.ErrInvalidArgument, "binary rpc: trusted connection, plugin, runtime and service identity are required"), nil
	}
	if !s.featureChecker.HasNegotiatedCapability(request.ConnectionID, domain.CapabilityBinaryStreaming) {
		return binaryRPCError(request.ID, domain.ErrPermissionDenied, "binary_streaming was not negotiated for this service connection"), nil
	}

	switch request.Method {
	case MethodBinaryCreate:
		return s.handleCreate(ctx, request)
	case MethodBinaryWrite:
		return s.handleWrite(ctx, request)
	case MethodBinarySeal:
		return s.handleSeal(ctx, request)
	case MethodBinaryRead:
		return s.handleRead(ctx, request)
	case MethodBinaryStat:
		return s.handleStat(ctx, request)
	case MethodBinaryRelease:
		return s.handleRelease(ctx, request)
	case MethodBinaryAbort:
		return s.handleAbort(ctx, request)
	default:
		return binaryRPCError(request.ID, domain.ErrNotFound, "binary rpc: method not found"), nil
	}
}

type binaryCreateInput struct {
	ChannelID    string                     `json:"channelId"`
	ExpectedSize *int64                     `json:"expectedSize"`
	MediaType    string                     `json:"mediaType,omitempty"`
	Lifetime     BinaryLifetime             `json:"lifetime,omitempty"`
	Metadata     map[string]json.RawMessage `json:"metadata,omitempty"`
}

func (s *BinaryTransferService) handleCreate(ctx context.Context, request rpc.RPCRequest) (rpc.RPCResponse, error) {
	var input binaryCreateInput
	if err := decodeBinaryPayload(request.Payload, &input); err != nil {
		return binaryRPCError(request.ID, domain.ErrInvalidArgument, err.Error()), nil
	}
	input.ChannelID = strings.TrimSpace(input.ChannelID)
	input.MediaType = strings.TrimSpace(input.MediaType)
	if input.ChannelID == "" {
		return binaryRPCError(request.ID, domain.ErrInvalidArgument, "binary.create: channelId is required"), nil
	}
	if input.ExpectedSize == nil {
		return binaryRPCError(request.ID, domain.ErrInvalidArgument, "binary.create: expectedSize is required"), nil
	}
	expectedSize := *input.ExpectedSize
	if expectedSize < 0 || expectedSize > MaxBinaryRPCObjectBytes {
		return binaryRPCError(request.ID, domain.ErrResourceExhausted, "binary.create: expectedSize is outside the supported range"), nil
	}
	if input.Lifetime == "" {
		input.Lifetime = BinaryLifetimeMessage
	}
	if err := input.Lifetime.Validate(); err != nil {
		return binaryRPCError(request.ID, domain.ErrInvalidArgument, "binary.create: invalid lifetime"), nil
	}

	declared, err := s.channels.Resolve(ctx, request.RuntimeID, request.ServiceID, domain.ChannelID(input.ChannelID))
	if err != nil {
		return binaryRPCError(request.ID, domain.ErrNotFound, "binary.create: declared binary channel was not found"), nil
	}
	if declared.PluginID != request.PluginID || declared.Kind != domain.ChannelKindBinary {
		return binaryRPCError(request.ID, domain.ErrPermissionDenied, "binary.create: channel is not an owned binary channel"), nil
	}
	if err := channel.ValidateDirection(declared, protocol.ChannelDirectionPluginToHost); err != nil {
		return binaryRPCError(request.ID, domain.ErrPermissionDenied, "binary.create: channel direction does not permit plugin uploads"), nil
	}

	decision, revert := s.admission.AcquireBinaryObject(ctx, resource.RuntimeIdentitySubject{
		PluginID: string(request.PluginID), RuntimeID: string(request.RuntimeID),
		ServiceID: string(request.ServiceID), Generation: request.Generation,
	}, expectedSize)
	if !decision.Allowed {
		return binaryRPCError(request.ID, domain.ErrResourceExhausted, "binary.create: resource admission denied: "+string(decision.Reason)), nil
	}
	if revert == nil {
		revert = func() {}
	}

	owner := BinaryOwner{
		PluginID:  request.PluginID,
		RuntimeID: request.RuntimeID,
		ServiceID: request.ServiceID,
		ChannelID: domain.ChannelID(input.ChannelID),
	}
	handle, err := s.resolver.Create(ctx, owner, BinaryStorageFile, CreateRequest{
		ExpectedSize: expectedSize,
		MediaType:    input.MediaType,
		Lifetime:     input.Lifetime,
		Metadata:     input.Metadata,
	})
	if err != nil {
		revert()
		return rpc.RPCResponse{}, err
	}

	s.mu.Lock()
	if _, exists := s.writes[handle.ObjectID]; exists {
		s.mu.Unlock()
		if handle.Abort != nil {
			_ = handle.Abort()
		}
		_ = s.registry.Release(ctx, handle.ObjectID)
		revert()
		return rpc.RPCResponse{}, fmt.Errorf("binary.create: duplicate object id %s", handle.ObjectID)
	}
	s.writes[handle.ObjectID] = &binaryWriteSession{
		owner:        owner,
		handle:       handle,
		expectedSize: expectedSize,
		lastActivity: time.Now().UTC(),
	}
	s.mu.Unlock()

	return binaryRPCSuccess(request.ID, map[string]any{
		"id":            handle.ObjectID,
		"kind":          BinaryStorageFile,
		"chunkSize":     DefaultBinaryChunkBytes,
		"maxChunkSize":  MaxBinaryChunkBytes,
		"maxObjectSize": MaxBinaryRPCObjectBytes,
	})
}

type binaryWriteInput struct {
	ID     BinaryObjectID `json:"id"`
	Offset int64          `json:"offset"`
	Data   string         `json:"data"`
}

func (s *BinaryTransferService) handleWrite(ctx context.Context, request rpc.RPCRequest) (rpc.RPCResponse, error) {
	var input binaryWriteInput
	var decoded []byte

	// stdio transports may carry binary.write as an actual binary frame. The
	// dispatcher maps its authenticated frame header into these trusted request
	// fields, avoiding base64 expansion for high-rate frames/audio. JSON/base64
	// remains the transport-neutral fallback for other transports. Mixed forms
	// are rejected so one request has exactly one source of truth.
	if request.BinaryObjectID != "" || len(request.BinaryPayload) > 0 {
		if len(request.Payload) != 0 {
			return binaryRPCError(request.ID, domain.ErrInvalidArgument, "binary.write: mixed binary-frame and JSON payload is forbidden"), nil
		}
		input.ID = BinaryObjectID(request.BinaryObjectID)
		input.Offset = request.BinaryOffset
		decoded = append([]byte(nil), request.BinaryPayload...)
	} else {
		if err := decodeBinaryPayload(request.Payload, &input); err != nil {
			return binaryRPCError(request.ID, domain.ErrInvalidArgument, err.Error()), nil
		}
		data, err := base64.StdEncoding.DecodeString(input.Data)
		if err != nil {
			return binaryRPCError(request.ID, domain.ErrInvalidArgument, "binary.write: data must be standard base64"), nil
		}
		decoded = data
	}
	if err := ValidateBinaryObjectID(input.ID); err != nil {
		return binaryRPCError(request.ID, domain.ErrInvalidArgument, "binary.write: invalid id"), nil
	}
	if input.Offset < 0 {
		return binaryRPCError(request.ID, domain.ErrInvalidArgument, "binary.write: offset must not be negative"), nil
	}
	if len(decoded) == 0 || len(decoded) > MaxBinaryChunkBytes {
		return binaryRPCError(request.ID, domain.ErrResourceExhausted, "binary.write: chunk size is outside the supported range"), nil
	}

	s.mu.Lock()
	session, ok := s.writes[input.ID]
	if !ok {
		s.mu.Unlock()
		return binaryRPCError(request.ID, domain.ErrNotFound, "binary.write: active write was not found"), nil
	}
	if !sameBinaryOwner(session.owner, request) {
		s.mu.Unlock()
		return binaryRPCError(request.ID, domain.ErrNotFound, "binary.write: object owner mismatch"), nil
	}
	if input.Offset != session.written {
		s.mu.Unlock()
		return binaryRPCError(request.ID, domain.ErrInvalidState, fmt.Sprintf("binary.write: offset %d does not match next offset %d", input.Offset, session.written)), nil
	}
	if session.written > MaxBinaryRPCObjectBytes-int64(len(decoded)) {
		s.mu.Unlock()
		return binaryRPCError(request.ID, domain.ErrResourceExhausted, "binary.write: object exceeds maximum size"), nil
	}
	if session.written+int64(len(decoded)) > session.expectedSize {
		s.mu.Unlock()
		return binaryRPCError(request.ID, domain.ErrInvalidArgument, "binary.write: chunk exceeds expectedSize"), nil
	}
	written, writeErr := session.handle.Writer.Write(decoded)
	if writeErr == nil && written != len(decoded) {
		writeErr = io.ErrShortWrite
	}
	if writeErr != nil {
		delete(s.writes, input.ID)
		s.mu.Unlock()
		s.abortSession(ctx, input.ID, session)
		return rpc.RPCResponse{}, fmt.Errorf("binary.write: %w", writeErr)
	}
	session.written += int64(written)
	session.lastActivity = time.Now().UTC()
	nextOffset := session.written
	s.mu.Unlock()

	return binaryRPCSuccess(request.ID, map[string]any{
		"id":         input.ID,
		"written":    written,
		"nextOffset": nextOffset,
	})
}

type binaryObjectInput struct {
	ID BinaryObjectID `json:"id"`
}

func (s *BinaryTransferService) handleSeal(ctx context.Context, request rpc.RPCRequest) (rpc.RPCResponse, error) {
	var input binaryObjectInput
	if err := decodeBinaryPayload(request.Payload, &input); err != nil {
		return binaryRPCError(request.ID, domain.ErrInvalidArgument, err.Error()), nil
	}
	if err := ValidateBinaryObjectID(input.ID); err != nil {
		return binaryRPCError(request.ID, domain.ErrInvalidArgument, "binary.seal: invalid id"), nil
	}

	s.mu.Lock()
	session, ok := s.writes[input.ID]
	if !ok {
		s.mu.Unlock()
		return binaryRPCError(request.ID, domain.ErrNotFound, "binary.seal: active write was not found"), nil
	}
	if !sameBinaryOwner(session.owner, request) {
		s.mu.Unlock()
		return binaryRPCError(request.ID, domain.ErrNotFound, "binary.seal: object owner mismatch"), nil
	}
	if session.written != session.expectedSize {
		s.mu.Unlock()
		return binaryRPCError(request.ID, domain.ErrInvalidArgument, fmt.Sprintf("binary.seal: wrote %d bytes, expected %d", session.written, session.expectedSize)), nil
	}
	delete(s.writes, input.ID)
	actualSize := session.written
	s.mu.Unlock()

	ref, err := session.handle.Seal(actualSize, nil)
	if err != nil {
		s.abortSession(ctx, input.ID, session)
		return rpc.RPCResponse{}, err
	}
	return binaryRPCSuccess(request.ID, map[string]any{"reference": ref})
}

type binaryReadInput struct {
	Reference BinaryReference `json:"reference"`
	Offset    int64           `json:"offset,omitempty"`
	MaxBytes  int             `json:"maxBytes,omitempty"`
}

func (s *BinaryTransferService) handleRead(ctx context.Context, request rpc.RPCRequest) (rpc.RPCResponse, error) {
	var input binaryReadInput
	if err := decodeBinaryPayload(request.Payload, &input); err != nil {
		return binaryRPCError(request.ID, domain.ErrInvalidArgument, err.Error()), nil
	}
	if input.Offset < 0 || input.Offset > input.Reference.Size {
		return binaryRPCError(request.ID, domain.ErrInvalidArgument, "binary.read: offset is outside the object"), nil
	}
	maxBytes := input.MaxBytes
	if maxBytes <= 0 {
		maxBytes = DefaultBinaryChunkBytes
	}
	if maxBytes > MaxBinaryChunkBytes {
		return binaryRPCError(request.ID, domain.ErrResourceExhausted, "binary.read: maxBytes exceeds the per-frame limit"), nil
	}

	owner, err := s.ownerForReference(ctx, request, input.Reference)
	if err != nil {
		return rpc.RPCResponse{}, err
	}
	resolved, err := s.resolver.Resolve(ctx, owner, input.Reference)
	if err != nil {
		return rpc.RPCResponse{}, err
	}
	if resolved.Reader == nil {
		return rpc.RPCResponse{}, ErrObjectNotReady
	}
	defer resolved.Reader.Close()

	if input.Offset > 0 {
		if seeker, ok := resolved.Reader.(io.Seeker); ok {
			if _, err := seeker.Seek(input.Offset, io.SeekStart); err != nil {
				return rpc.RPCResponse{}, fmt.Errorf("binary.read: seek: %w", err)
			}
		} else if _, err := io.CopyN(io.Discard, resolved.Reader, input.Offset); err != nil {
			return rpc.RPCResponse{}, fmt.Errorf("binary.read: skip: %w", err)
		}
	}

	remaining := input.Reference.Size - input.Offset
	readLimit := int64(maxBytes)
	if remaining < readLimit {
		readLimit = remaining
	}
	buf := make([]byte, int(readLimit))
	n, readErr := io.ReadFull(resolved.Reader, buf)
	if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
		return rpc.RPCResponse{}, fmt.Errorf("binary.read: %w", readErr)
	}
	buf = buf[:n]
	nextOffset := input.Offset + int64(n)
	return binaryRPCSuccess(request.ID, map[string]any{
		"id":         input.Reference.ID,
		"offset":     input.Offset,
		"nextOffset": nextOffset,
		"data":       base64.StdEncoding.EncodeToString(buf),
		"eof":        nextOffset >= input.Reference.Size,
		"size":       input.Reference.Size,
	})
}

func (s *BinaryTransferService) handleStat(ctx context.Context, request rpc.RPCRequest) (rpc.RPCResponse, error) {
	var input binaryObjectInput
	if err := decodeBinaryPayload(request.Payload, &input); err != nil {
		return binaryRPCError(request.ID, domain.ErrInvalidArgument, err.Error()), nil
	}
	if err := ValidateBinaryObjectID(input.ID); err != nil {
		return binaryRPCError(request.ID, domain.ErrInvalidArgument, "binary.stat: invalid id"), nil
	}
	record, err := s.registry.Get(ctx, input.ID)
	if err != nil {
		return rpc.RPCResponse{}, err
	}
	if !sameBinaryOwner(record.Owner, request) {
		return binaryRPCError(request.ID, domain.ErrNotFound, "binary.stat: object owner mismatch"), nil
	}
	return binaryRPCSuccess(request.ID, map[string]any{
		"id":        record.ID,
		"kind":      record.Kind,
		"size":      record.Size,
		"mediaType": record.MediaType,
		"lifetime":  record.Lifetime,
		"checksum":  record.Checksum,
		"metadata":  record.Metadata,
		"state":     record.State,
	})
}

func (s *BinaryTransferService) handleRelease(ctx context.Context, request rpc.RPCRequest) (rpc.RPCResponse, error) {
	var input binaryObjectInput
	if err := decodeBinaryPayload(request.Payload, &input); err != nil {
		return binaryRPCError(request.ID, domain.ErrInvalidArgument, err.Error()), nil
	}
	if err := ValidateBinaryObjectID(input.ID); err != nil {
		return binaryRPCError(request.ID, domain.ErrInvalidArgument, "binary.release: invalid id"), nil
	}

	s.mu.Lock()
	if session, ok := s.writes[input.ID]; ok {
		if !sameBinaryOwner(session.owner, request) {
			s.mu.Unlock()
			return binaryRPCError(request.ID, domain.ErrNotFound, "binary.release: object owner mismatch"), nil
		}
		delete(s.writes, input.ID)
		s.mu.Unlock()
		s.abortSession(ctx, input.ID, session)
		return binaryRPCSuccess(request.ID, map[string]any{"released": true})
	}
	s.mu.Unlock()

	record, err := s.registry.Get(ctx, input.ID)
	if err != nil {
		if err == ErrObjectNotFound || err == ErrObjectReleased {
			return binaryRPCSuccess(request.ID, map[string]any{"released": true})
		}
		return rpc.RPCResponse{}, err
	}
	if !sameBinaryOwner(record.Owner, request) {
		return binaryRPCError(request.ID, domain.ErrNotFound, "binary.release: object owner mismatch"), nil
	}
	if err := s.resolver.Release(ctx, record.Owner, input.ID); err != nil {
		return rpc.RPCResponse{}, err
	}
	return binaryRPCSuccess(request.ID, map[string]any{"released": true})
}

func (s *BinaryTransferService) handleAbort(ctx context.Context, request rpc.RPCRequest) (rpc.RPCResponse, error) {
	var input binaryObjectInput
	if err := decodeBinaryPayload(request.Payload, &input); err != nil {
		return binaryRPCError(request.ID, domain.ErrInvalidArgument, err.Error()), nil
	}
	if err := ValidateBinaryObjectID(input.ID); err != nil {
		return binaryRPCError(request.ID, domain.ErrInvalidArgument, "binary.abort: invalid id"), nil
	}
	s.mu.Lock()
	session, ok := s.writes[input.ID]
	if ok && sameBinaryOwner(session.owner, request) {
		delete(s.writes, input.ID)
	}
	s.mu.Unlock()
	if !ok {
		return binaryRPCSuccess(request.ID, map[string]any{"aborted": true})
	}
	if !sameBinaryOwner(session.owner, request) {
		return binaryRPCError(request.ID, domain.ErrNotFound, "binary.abort: object owner mismatch"), nil
	}
	s.abortSession(ctx, input.ID, session)
	return binaryRPCSuccess(request.ID, map[string]any{"aborted": true})
}

func (s *BinaryTransferService) ownerForReference(ctx context.Context, request rpc.RPCRequest, ref BinaryReference) (BinaryOwner, error) {
	if err := ref.Validate(); err != nil {
		return BinaryOwner{}, err
	}
	record, err := s.registry.Get(ctx, ref.ID)
	if err != nil {
		return BinaryOwner{}, err
	}
	if !sameBinaryOwner(record.Owner, request) {
		return BinaryOwner{}, ErrOwnerMismatch
	}
	return record.Owner, nil
}

// CleanupRuntime aborts in-flight writes first, then releases sealed provider
// objects. It is safe to call repeatedly during emergency stop, uninstall and
// startup recovery.
func (s *BinaryTransferService) CleanupRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID) error {
	if s == nil {
		return nil
	}
	var sessions []*binaryWriteSession
	var ids []BinaryObjectID
	s.mu.Lock()
	for id, session := range s.writes {
		if session.owner.RuntimeID == runtimeID {
			ids = append(ids, id)
			sessions = append(sessions, session)
			delete(s.writes, id)
		}
	}
	s.mu.Unlock()
	for i, session := range sessions {
		s.abortSession(ctx, ids[i], session)
	}
	return s.resolver.ReleaseByRuntime(ctx, runtimeID)
}

func (s *BinaryTransferService) CleanupService(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) error {
	if s == nil {
		return nil
	}
	var sessions []*binaryWriteSession
	var ids []BinaryObjectID
	s.mu.Lock()
	for id, session := range s.writes {
		if session.owner.RuntimeID == runtimeID && session.owner.ServiceID == serviceID {
			ids = append(ids, id)
			sessions = append(sessions, session)
			delete(s.writes, id)
		}
	}
	s.mu.Unlock()
	for i, session := range sessions {
		s.abortSession(ctx, ids[i], session)
	}
	return s.resolver.ReleaseByService(ctx, runtimeID, serviceID)
}

func (s *BinaryTransferService) abortSession(ctx context.Context, id BinaryObjectID, session *binaryWriteSession) {
	if session == nil {
		return
	}
	if session.handle.Abort != nil {
		_ = session.handle.Abort()
	} else if session.handle.Writer != nil {
		_ = session.handle.Writer.Close()
	}
	_ = s.registry.Release(ctx, id)
}

func sameBinaryOwner(owner BinaryOwner, request rpc.RPCRequest) bool {
	return owner.PluginID == request.PluginID && owner.RuntimeID == request.RuntimeID && owner.ServiceID == request.ServiceID
}

func decodeBinaryPayload(payload json.RawMessage, target any) error {
	if len(payload) == 0 {
		return fmt.Errorf("binary rpc: payload is required")
	}
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("binary rpc: invalid payload: %w", err)
	}
	return nil
}

func binaryRPCSuccess(requestID string, value any) (rpc.RPCResponse, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return rpc.RPCResponse{}, err
	}
	return rpc.RPCResponse{RequestID: requestID, Payload: payload}, nil
}

func binaryRPCError(requestID string, code domain.ErrorCode, message string) rpc.RPCResponse {
	return rpc.RPCResponse{
		RequestID: requestID,
		Error: &rpc.RPCRoutedError{
			Code:    string(code),
			Message: message,
		},
	}
}

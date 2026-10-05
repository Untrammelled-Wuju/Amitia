package coordination

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const ResourceSchema = `CREATE TABLE IF NOT EXISTS kernel_device_owned_resources (
	owner_id TEXT NOT NULL, kind TEXT NOT NULL, resource_id TEXT NOT NULL,
	role_id TEXT NOT NULL, source_id TEXT NOT NULL DEFAULT '', revision INTEGER NOT NULL, deleted INTEGER NOT NULL DEFAULT 0,
	body BLOB NOT NULL, updated_at TEXT NOT NULL,
	PRIMARY KEY(owner_id, kind, resource_id))`

const InboxSchema = `CREATE TABLE IF NOT EXISTS kernel_device_owned_inbox (
	owner_id TEXT NOT NULL, request_id TEXT NOT NULL, payload_hash TEXT NOT NULL,
	ack BLOB NOT NULL, committed_at TEXT NOT NULL, PRIMARY KEY(owner_id, request_id))`

const OutboxSchema = `CREATE TABLE IF NOT EXISTS kernel_device_owned_outbox (
	request_id TEXT NOT NULL, owner_id TEXT NOT NULL, scope BLOB NOT NULL,
	payload BLOB NOT NULL, payload_hash TEXT NOT NULL, created_at TEXT NOT NULL,
	attempts INTEGER NOT NULL DEFAULT 0, next_attempt_at TEXT NOT NULL,
	PRIMARY KEY(owner_id,request_id))`

var ErrResourceVersion = errors.New("数据版本已变化，请重新加载")
var ErrRequestConflict = errors.New("同一请求编号不能用于不同数据")
var ErrWrongOwner = errors.New("禁止将设备专属数据保存到其他设备")
var ErrPendingLimit = errors.New("待保存数据已达到上限，请等待设备恢复连接")

type Resource struct {
	OwnerID  string          `json:"ownerId"`
	Kind     string          `json:"kind"`
	ID       string          `json:"id"`
	RoleID   string          `json:"roleId"`
	SourceID string          `json:"sourceId,omitempty"`
	Revision int64           `json:"revision"`
	Deleted  bool            `json:"deleted"`
	Body     json.RawMessage `json:"body"`
}

type Mutation struct {
	Kind             string          `json:"kind"`
	ID               string          `json:"id"`
	RoleID           string          `json:"roleId"`
	SourceID         string          `json:"sourceId,omitempty"`
	ExpectedRevision int64           `json:"expectedRevision"`
	Deleted          bool            `json:"deleted"`
	Body             json.RawMessage `json:"body"`
}

type Commit struct {
	LeaseProof   *CommitLeaseProof `json:"leaseProof,omitempty"`
	Scope        ExecutionScope    `json:"scope"`
	Mutations    []Mutation        `json:"mutations"`
	Dependencies []ResourceVersion `json:"dependencies,omitempty"`
}

type ResourceVersion struct {
	Kind     string `json:"kind"`
	ID       string `json:"id"`
	Revision int64  `json:"revision"`
}

type Acknowledgement struct {
	RequestID string           `json:"requestId"`
	OwnerID   string           `json:"ownerId"`
	Versions  map[string]int64 `json:"versions"`
}

type OwnershipStore struct {
	db      *sql.DB
	ownerID string
}

func NewOwnershipStore(db *sql.DB, ownerID string) *OwnershipStore {
	return &OwnershipStore{db: db, ownerID: ownerID}
}

func payloadHash(payload []byte) string {
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}

func validKind(kind string) bool {
	switch kind {
	case "conversation", "message", "summary", "memory", "working", "profile", "episodic", "fact", "vector", "graph", "continuity", "checkpoint", "tool-result":
		return true
	default:
		return false
	}
}

func (s *OwnershipStore) Apply(ctx context.Context, commit Commit) (Acknowledgement, error) {
	return s.apply(ctx, commit, nil)
}

func (s *OwnershipStore) apply(ctx context.Context, commit Commit, guard func(*sql.Tx) error) (Acknowledgement, error) {
	scope := commit.Scope
	if scope.ResourceOwnerID != s.ownerID || scope.RequestID == "" || scope.RoleID == "" || len(commit.Mutations) == 0 || len(commit.Mutations) > 256 {
		return Acknowledgement{}, ErrWrongOwner
	}
	payload, err := json.Marshal(commit)
	if err != nil {
		return Acknowledgement{}, err
	}
	if len(payload) > 4<<20 {
		return Acknowledgement{}, ErrPendingLimit
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Acknowledgement{}, err
	}
	defer tx.Rollback()
	if err := s.validateSourceCommitAuthority(ctx, tx, commit); err != nil {
		return Acknowledgement{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO kernel_device_owned_inbox(owner_id,request_id,payload_hash,ack,committed_at) VALUES(?,?,?,'','') ON CONFLICT DO NOTHING`, s.ownerID, scope.RequestID, payloadHash(payload)); err != nil {
		return Acknowledgement{}, err
	}
	var previousHash string
	var previousAck []byte
	if err = tx.QueryRowContext(ctx, `SELECT payload_hash,ack FROM kernel_device_owned_inbox WHERE owner_id=? AND request_id=?`, s.ownerID, scope.RequestID).Scan(&previousHash, &previousAck); err != nil {
		return Acknowledgement{}, err
	}
	if previousHash != payloadHash(payload) {
		return Acknowledgement{}, ErrRequestConflict
	}
	if len(previousAck) != 0 {
		var ack Acknowledgement
		if err := json.Unmarshal(previousAck, &ack); err != nil {
			return ack, err
		}
		return ack, nil
	}
	if guard != nil {
		if err := guard(tx); err != nil {
			return Acknowledgement{}, err
		}
	}
	if err := s.validateCommitLease(ctx, tx, commit); err != nil {
		return Acknowledgement{}, err
	}
	if len(commit.Dependencies) > 4096 {
		return Acknowledgement{}, ErrPendingLimit
	}
	seenDependencies := make(map[string]bool)
	for _, dependency := range commit.Dependencies {
		key := dependency.Kind + "/" + dependency.ID
		if !validKind(dependency.Kind) || dependency.ID == "" || dependency.Revision < 1 || seenDependencies[key] {
			return Acknowledgement{}, ErrWrongOwner
		}
		seenDependencies[key] = true
		var revision int64
		var role string
		var deleted bool
		var dependencyBody json.RawMessage
		if err := tx.QueryRowContext(ctx, `SELECT revision,role_id,deleted,body FROM kernel_device_owned_resources WHERE owner_id=? AND kind=? AND resource_id=?`, s.ownerID, dependency.Kind, dependency.ID).Scan(&revision, &role, &deleted, &dependencyBody); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return Acknowledgement{}, ErrResourceVersion
			}
			return Acknowledgement{}, err
		}
		if deleted || revision != dependency.Revision || role != scope.RoleID {
			return Acknowledgement{}, ErrResourceVersion
		}
		if dependency.Kind == "memory" && !ResourceUsable(dependencyBody, time.Now()) {
			return Acknowledgement{}, ErrResourceVersion
		}
	}
	ack := Acknowledgement{RequestID: scope.RequestID, OwnerID: s.ownerID, Versions: make(map[string]int64)}
	changedMemories := []string{}
	type changedMessage struct{ conversation, request string }
	changedMessages := []changedMessage{}
	deletedConversations := []string{}
	for _, mutation := range commit.Mutations {
		if !validKind(mutation.Kind) || mutation.ID == "" || mutation.RoleID != scope.RoleID || !json.Valid(mutation.Body) || mutation.ExpectedRevision < 0 {
			return Acknowledgement{}, ErrWrongOwner
		}
		key := mutation.Kind + "/" + mutation.ID
		if _, exists := ack.Versions[key]; exists {
			return Acknowledgement{}, ErrRequestConflict
		}
		var revision int64
		var storedRole string
		var storedDeleted bool
		err := tx.QueryRowContext(ctx, `SELECT revision,role_id,deleted FROM kernel_device_owned_resources WHERE owner_id=? AND kind=? AND resource_id=?`, s.ownerID, mutation.Kind, mutation.ID).Scan(&revision, &storedRole, &storedDeleted)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return Acknowledgement{}, err
		}
		if err == nil && storedRole != scope.RoleID {
			return Acknowledgement{}, ErrWrongOwner
		}
		if revision != mutation.ExpectedRevision {
			return Acknowledgement{}, ErrResourceVersion
		}
		if storedDeleted && !mutation.Deleted && (mutation.Kind == "memory" || mutation.Kind == "message" || mutation.Kind == "conversation" || mutation.Kind == "checkpoint") {
			return Acknowledgement{}, ErrResourceVersion
		}
		if mutation.Kind == "conversation" && mutation.Deleted {
			deletedConversations = append(deletedConversations, mutation.ID)
		}
		if mutation.Kind == "conversation" && !mutation.Deleted && revision > 0 {
			var previousBody []byte
			if err := tx.QueryRowContext(ctx, `SELECT body FROM kernel_device_owned_resources WHERE owner_id=? AND kind='conversation' AND resource_id=?`, s.ownerID, mutation.ID).Scan(&previousBody); err != nil {
				return Acknowledgement{}, err
			}
			var previous, next struct {
				ClearRevision int64 `json:"clearRevision"`
			}
			if json.Unmarshal(previousBody, &previous) != nil || json.Unmarshal(mutation.Body, &next) != nil {
				return Acknowledgement{}, ErrWrongOwner
			}
			if next.ClearRevision != previous.ClearRevision {
				if next.ClearRevision != revision+1 {
					return Acknowledgement{}, ErrResourceVersion
				}
				deletedConversations = append(deletedConversations, mutation.ID)
			}
		}
		if mutation.Kind == "message" && revision > 0 {
			var previousBody []byte
			if err := tx.QueryRowContext(ctx, `SELECT body FROM kernel_device_owned_resources WHERE owner_id=? AND kind='message' AND resource_id=?`, s.ownerID, mutation.ID).Scan(&previousBody); err != nil {
				return Acknowledgement{}, err
			}
			var previous struct {
				ConversationID string `json:"conversationId"`
				RequestID      string `json:"requestId"`
				Content        string `json:"content"`
			}
			var next struct {
				Content string `json:"content"`
			}
			if json.Unmarshal(previousBody, &previous) != nil || json.Unmarshal(mutation.Body, &next) != nil {
				return Acknowledgement{}, ErrWrongOwner
			}
			if mutation.Deleted || previous.Content != next.Content {
				changedMessages = append(changedMessages, changedMessage{previous.ConversationID, previous.RequestID})
			}
		}
		if !mutation.Deleted && (mutation.Kind == "vector" || mutation.Kind == "graph" || mutation.SourceID != "") {
			if mutation.SourceID == "" {
				return Acknowledgement{}, ErrWrongOwner
			}
			var sourceDeleted bool
			var sourceRole string
			var sourceBody json.RawMessage
			if err := tx.QueryRowContext(ctx, `SELECT deleted,role_id,body FROM kernel_device_owned_resources WHERE owner_id=? AND kind='memory' AND resource_id=?`, s.ownerID, mutation.SourceID).Scan(&sourceDeleted, &sourceRole, &sourceBody); err != nil {
				return Acknowledgement{}, err
			}
			if sourceDeleted || sourceRole != scope.RoleID || !ResourceUsable(sourceBody, time.Now()) {
				return Acknowledgement{}, ErrWrongOwner
			}
		}
		body := []byte(mutation.Body)
		if mutation.Deleted {
			body = []byte(`null`)
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO kernel_device_owned_resources(owner_id,kind,resource_id,role_id,source_id,revision,deleted,body,updated_at) VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(owner_id,kind,resource_id) DO UPDATE SET source_id=excluded.source_id,revision=excluded.revision,deleted=excluded.deleted,body=excluded.body,updated_at=excluded.updated_at`, s.ownerID, mutation.Kind, mutation.ID, mutation.RoleID, mutation.SourceID, revision+1, mutation.Deleted, body, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return Acknowledgement{}, err
		}
		if mutation.Kind == "memory" && (revision > 0 || mutation.Deleted) {
			changedMemories = append(changedMemories, mutation.ID)
		}
		ack.Versions[key] = revision + 1
	}
	for _, conversation := range deletedConversations {
		rows, err := tx.QueryContext(ctx, `SELECT resource_id FROM kernel_device_owned_resources WHERE owner_id=? AND role_id=? AND kind='memory' AND deleted=0 AND json_extract(CAST(body AS TEXT),'$.conversationId')=?`, s.ownerID, scope.RoleID, conversation)
		if err != nil {
			return Acknowledgement{}, err
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return Acknowledgement{}, err
			}
			changedMemories = append(changedMemories, id)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return Acknowledgement{}, err
		}
		rows.Close()
		if _, err := tx.ExecContext(ctx, `UPDATE kernel_device_owned_resources SET deleted=1,body='null',revision=revision+1 WHERE owner_id=? AND role_id=? AND kind<>'conversation' AND deleted=0 AND json_extract(CAST(body AS TEXT),'$.conversationId')=?`, s.ownerID, scope.RoleID, conversation); err != nil {
			return Acknowledgement{}, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE kernel_device_owned_resources SET deleted=1,body='null',revision=revision+1 WHERE owner_id=? AND role_id=? AND kind='checkpoint' AND deleted=0 AND json_extract(CAST(body AS TEXT),'$.mutations[0].body.conversationId')=?`, s.ownerID, scope.RoleID, conversation); err != nil {
			return Acknowledgement{}, err
		}
	}
	for _, changed := range changedMessages {
		if changed.conversation == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `UPDATE kernel_device_owned_resources SET deleted=1,body='null',revision=revision+1 WHERE owner_id=? AND role_id=? AND kind IN ('summary','working') AND deleted=0 AND json_extract(CAST(body AS TEXT),'$.conversationId')=?`, s.ownerID, scope.RoleID, changed.conversation); err != nil {
			return Acknowledgement{}, err
		}
		if changed.request == "" {
			continue
		}
		rows, err := tx.QueryContext(ctx, `SELECT resource_id FROM kernel_device_owned_resources WHERE owner_id=? AND role_id=? AND kind='memory' AND deleted=0 AND json_extract(CAST(body AS TEXT),'$.executionScope.requestId')=?`, s.ownerID, scope.RoleID, changed.request)
		if err != nil {
			return Acknowledgement{}, err
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return Acknowledgement{}, err
			}
			changedMemories = append(changedMemories, id)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return Acknowledgement{}, err
		}
		rows.Close()
		if _, err := tx.ExecContext(ctx, `UPDATE kernel_device_owned_resources SET deleted=1,body='null',revision=revision+1 WHERE owner_id=? AND role_id=? AND kind='memory' AND deleted=0 AND json_extract(CAST(body AS TEXT),'$.executionScope.requestId')=?`, s.ownerID, scope.RoleID, changed.request); err != nil {
			return Acknowledgement{}, err
		}
	}
	for _, source := range changedMemories {
		statement := `UPDATE kernel_device_owned_resources SET deleted=1,body='null',revision=revision+1 WHERE owner_id=? AND source_id=? AND deleted=0`
		arguments := []any{s.ownerID, source}
		for _, mutation := range commit.Mutations {
			if mutation.SourceID == source && !mutation.Deleted {
				statement += ` AND NOT(kind=? AND resource_id=?)`
				arguments = append(arguments, mutation.Kind, mutation.ID)
			}
		}
		if _, err := tx.ExecContext(ctx, statement, arguments...); err != nil {
			return Acknowledgement{}, err
		}
	}
	encoded, err := json.Marshal(ack)
	if err != nil {
		return Acknowledgement{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE kernel_device_owned_inbox SET ack=?,committed_at=? WHERE owner_id=? AND request_id=?`, encoded, time.Now().UTC().Format(time.RFC3339Nano), s.ownerID, scope.RequestID); err != nil {
		return Acknowledgement{}, err
	}
	if err = tx.Commit(); err != nil {
		return Acknowledgement{}, err
	}
	return ack, nil
}

func (s *OwnershipStore) List(ctx context.Context, kind, role string, includeDeleted bool) ([]Resource, error) {
	if !validKind(kind) || role == "" {
		return nil, ErrWrongOwner
	}
	rows, err := s.db.QueryContext(ctx, `SELECT resource_id,source_id,revision,deleted,body FROM kernel_device_owned_resources WHERE owner_id=? AND kind=? AND role_id=? AND (? OR deleted=0) ORDER BY updated_at,resource_id LIMIT 1000`, s.ownerID, kind, role, includeDeleted)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	resources := make([]Resource, 0)
	for rows.Next() {
		resource := Resource{OwnerID: s.ownerID, Kind: kind, RoleID: role}
		if err := rows.Scan(&resource.ID, &resource.SourceID, &resource.Revision, &resource.Deleted, &resource.Body); err != nil {
			return nil, err
		}
		resources = append(resources, resource)
	}
	return resources, rows.Err()
}

func (s *OwnershipStore) ListForQuery(ctx context.Context, kind, role string, query DataQuery) ([]Resource, error) {
	resources, _, err := s.ListPage(ctx, kind, role, query)
	return resources, err
}

type resourceCursor struct {
	Owner        string `json:"owner"`
	Kind         string `json:"kind"`
	Role         string `json:"role"`
	Conversation string `json:"conversation"`
	Order        string `json:"order"`
	ID           string `json:"id"`
	SearchHash   string `json:"searchHash,omitempty"`
}

func ConversationSearchHash(search string) string {
	if search == "" {
		return ""
	}
	digest := sha256.Sum256([]byte(search))
	return hex.EncodeToString(digest[:])
}

func (s *OwnershipStore) ListPage(ctx context.Context, kind, role string, query DataQuery) ([]Resource, string, error) {
	search, err := NormalizeConversationSearch(query.SearchQuery)
	if err != nil {
		return nil, "", err
	}
	if search != "" && (kind != "conversation" || !query.ListConversations || query.ConversationID != "") {
		return nil, "", ErrWrongOwner
	}
	searchHash := ConversationSearchHash(search)
	if !validKind(kind) || role == "" {
		return nil, "", ErrWrongOwner
	}
	if query.Cursor != "" && query.ResourceKind != kind {
		return nil, "", ErrWrongOwner
	}
	var cursor resourceCursor
	if query.Cursor != "" {
		if len(query.Cursor) > 4096 {
			return nil, "", ErrWrongOwner
		}
		encoded, err := base64.RawURLEncoding.DecodeString(query.Cursor)
		if err != nil || json.Unmarshal(encoded, &cursor) != nil || cursor.Owner != s.ownerID || cursor.Kind != kind || cursor.Role != role || cursor.Conversation != query.ConversationID || cursor.SearchHash != searchHash || cursor.Order == "" || cursor.ID == "" || len(cursor.ID) > 512 || len(cursor.Order) > 128 {
			return nil, "", ErrWrongOwner
		}
	}
	limit := query.Limit
	if limit <= 0 || limit > 128 {
		limit = 128
	}
	order := `updated_at`
	if kind == "message" {
		created := `json_extract(CAST(body AS TEXT),'$.createdAt')`
		order = `CASE WHEN ` + created + ` GLOB '????-??-??T??:??:??*Z' THEN substr(` + created + `,1,19)||'.'||CASE WHEN substr(` + created + `,20,1)='.' THEN substr(replace(substr(` + created + `,21),'Z','')||'000000000',1,9) ELSE '000000000' END||'Z' ELSE updated_at END`
	}
	statement := `SELECT resource_id,source_id,revision,deleted,body,` + order + ` AS resource_order FROM kernel_device_owned_resources WHERE owner_id=? AND kind=? AND role_id=? AND deleted=0`
	arguments := []any{s.ownerID, kind, role}
	switch kind {
	case "message", "summary", "working":
		if query.ConversationID == "" {
			return []Resource{}, "", nil
		}
		statement += ` AND json_extract(CAST(body AS TEXT),'$.conversationId')=?`
		arguments = append(arguments, query.ConversationID)
	case "conversation":
		if search != "" {
			statement += OwnedConversationSearchPredicate
			arguments = append(arguments, search, search)
		}
		if !query.ListConversations {
			if query.ConversationID == "" {
				return []Resource{}, "", nil
			}
			statement += ` AND resource_id=?`
			arguments = append(arguments, query.ConversationID)
		}
	case "checkpoint":
		if query.RequestID == "" {
			return []Resource{}, "", nil
		}
		statement += ` AND resource_id IN (?,?)`
		arguments = append(arguments, "turn/"+query.RequestID, "memory/"+query.RequestID)
	}
	statement = `SELECT resource_id,source_id,revision,deleted,body,resource_order FROM (` + statement + `)`
	if query.Cursor != "" {
		statement += ` WHERE resource_order<? OR (resource_order=? AND resource_id<?)`
		arguments = append(arguments, cursor.Order, cursor.Order, cursor.ID)
	}
	statement += ` ORDER BY resource_order DESC,resource_id DESC LIMIT ?`
	arguments = append(arguments, limit+1)
	rows, err := s.db.QueryContext(ctx, statement, arguments...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	resources := []Resource{}
	orders := []string{}
	pageBytes := 2
	truncated := false
	for rows.Next() {
		resource := Resource{OwnerID: s.ownerID, Kind: kind, RoleID: role}
		var ordered string
		if err := rows.Scan(&resource.ID, &resource.SourceID, &resource.Revision, &resource.Deleted, &resource.Body, &ordered); err != nil {
			return nil, "", err
		}
		if kind == "message" {
			encoded, err := json.Marshal(resource)
			if err != nil {
				return nil, "", err
			}
			if pageBytes+len(encoded)+1 > 3<<20 {
				if len(resources) == 0 {
					return nil, "", ErrPendingLimit
				}
				truncated = true
				break
			}
			pageBytes += len(encoded) + 1
		}
		resources = append(resources, resource)
		orders = append(orders, ordered)
	}
	next := ""
	if len(resources) > limit || truncated {
		count := len(resources)
		if count > limit {
			count = limit
		}
		last := resources[count-1]
		encoded, _ := json.Marshal(resourceCursor{Owner: s.ownerID, Kind: kind, Role: role, Conversation: query.ConversationID, Order: orders[count-1], ID: last.ID, SearchHash: searchHash})
		next = base64.RawURLEncoding.EncodeToString(encoded)
		resources = resources[:count]
	}
	for i, j := 0, len(resources)-1; i < j; i, j = i+1, j-1 {
		resources[i], resources[j] = resources[j], resources[i]
	}
	return resources, next, rows.Err()
}

func (s *OwnershipStore) Get(ctx context.Context, kind, id string) (*Resource, error) {
	if !validKind(kind) || id == "" {
		return nil, ErrWrongOwner
	}
	resource := &Resource{OwnerID: s.ownerID, Kind: kind, ID: id}
	err := s.db.QueryRowContext(ctx, `SELECT role_id,source_id,revision,deleted,CAST(body AS BLOB) FROM kernel_device_owned_resources WHERE owner_id=? AND kind=? AND resource_id=?`, s.ownerID, kind, id).Scan(&resource.RoleID, &resource.SourceID, &resource.Revision, &resource.Deleted, &resource.Body)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return resource, err
}

type PendingCommit struct {
	Commit   Commit
	Hash     string
	Attempts int
}

func (s *Service) PendingRequest(ctx context.Context, owner, request string) (*PendingCommit, error) {
	var payload []byte
	var pending PendingCommit
	err := s.db.QueryRowContext(ctx, `SELECT payload,payload_hash,attempts FROM kernel_device_owned_outbox WHERE owner_id=? AND request_id=?`, owner, request).Scan(&payload, &pending.Hash, &pending.Attempts)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(payload, &pending.Commit); err != nil {
		return nil, err
	}
	return &pending, nil
}

func (s *Service) Enqueue(ctx context.Context, commit Commit) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if commit.Scope.Coordinated || commit.Scope.ResourceOwnerID == "" || commit.Scope.RequestID == "" {
		return ErrWrongOwner
	}
	if err := s.Validate(ctx, commit.Scope); err != nil {
		return err
	}
	payload, err := json.Marshal(commit)
	if err != nil {
		return err
	}
	if len(payload) > 4<<20 {
		return ErrPendingLimit
	}
	scope, err := json.Marshal(commit.Scope)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE kernel_device_coordination SET provider_epoch=provider_epoch WHERE space_id=? AND device_id=?`, commit.Scope.SpaceID, commit.Scope.InitiatorDeviceID); err != nil {
		return err
	}
	var rejectedHash, rejectedCode string
	rejectedErr := tx.QueryRowContext(ctx, `SELECT payload_hash,error_code FROM kernel_device_owned_delivery_failures WHERE owner_id=? AND request_id=?`, commit.Scope.ResourceOwnerID, commit.Scope.RequestID).Scan(&rejectedHash, &rejectedCode)
	if rejectedErr == nil {
		if rejectedHash != payloadHash(payload) {
			return ErrRequestConflict
		}
		return errors.Join(ErrDeliveryRejected, ErrorFromProtocol(rejectedCode))
	}
	if !errors.Is(rejectedErr, sql.ErrNoRows) {
		return rejectedErr
	}
	var previous string
	err = tx.QueryRowContext(ctx, `SELECT payload_hash FROM kernel_device_owned_outbox WHERE owner_id=? AND request_id=?`, commit.Scope.ResourceOwnerID, commit.Scope.RequestID).Scan(&previous)
	if err == nil {
		if previous != payloadHash(payload) {
			return ErrRequestConflict
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	var count, size int64
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(LENGTH(payload)),0) FROM kernel_device_owned_outbox WHERE owner_id=?`, commit.Scope.ResourceOwnerID).Scan(&count, &size); err != nil {
		return err
	}
	if count >= 128 || size+int64(len(payload)) > 32<<20 {
		return ErrPendingLimit
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, `INSERT INTO kernel_device_owned_outbox(request_id,owner_id,scope,payload,payload_hash,created_at,next_attempt_at) VALUES(?,?,?,?,?,?,?)`, commit.Scope.RequestID, commit.Scope.ResourceOwnerID, scope, payload, payloadHash(payload), now, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) Pending(ctx context.Context, owner string) ([]PendingCommit, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT payload,payload_hash,attempts FROM kernel_device_owned_outbox WHERE owner_id=? AND next_attempt_at<=? ORDER BY created_at,request_id LIMIT 32`, owner, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []PendingCommit
	for rows.Next() {
		var payload []byte
		var pending PendingCommit
		if err := rows.Scan(&payload, &pending.Hash, &pending.Attempts); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(payload, &pending.Commit); err != nil {
			return nil, err
		}
		result = append(result, pending)
	}
	return result, rows.Err()
}

func (s *Service) PendingOwners(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT owner_id FROM kernel_device_owned_outbox WHERE next_attempt_at<=? ORDER BY owner_id LIMIT 128`, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	owners := make([]string, 0)
	for rows.Next() {
		var owner string
		if err := rows.Scan(&owner); err != nil {
			return nil, err
		}
		owners = append(owners, owner)
	}
	return owners, rows.Err()
}

func (s *Service) RetryLater(ctx context.Context, pending PendingCommit) error {
	shift := pending.Attempts
	if shift < 0 {
		shift = 0
	}
	if shift > 6 {
		shift = 6
	}
	delay := 5 * time.Second * time.Duration(1<<shift)
	if delay > 5*time.Minute {
		delay = 5 * time.Minute
	}
	_, err := s.db.ExecContext(ctx, `UPDATE kernel_device_owned_outbox SET attempts=attempts+1,next_attempt_at=? WHERE owner_id=? AND request_id=? AND payload_hash=?`, time.Now().UTC().Add(delay).Format(time.RFC3339Nano), pending.Commit.Scope.ResourceOwnerID, pending.Commit.Scope.RequestID, pending.Hash)
	return err
}

func (s *Service) Acknowledge(ctx context.Context, ack Acknowledgement, hash string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM kernel_device_owned_outbox WHERE request_id=? AND owner_id=? AND payload_hash=?`, ack.RequestID, ack.OwnerID, hash)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		var retained int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM kernel_device_owned_outbox WHERE request_id=? AND owner_id=?`, ack.RequestID, ack.OwnerID).Scan(&retained); err != nil {
			return err
		}
		if retained == 0 {
			return nil
		}
		return fmt.Errorf("%w: 保存确认不匹配", ErrRequestConflict)
	}
	return nil
}

func (s *Service) DiscardPending(ctx context.Context, pending PendingCommit, roleSource ...DataPort) error {
	var roleErr error
	if len(roleSource) > 0 {
		roleErr = ValidateRoleRevision(ctx, roleSource[0], pending.Commit.Scope)
		if roleErr != nil && !errors.Is(roleErr, ErrRoleRequired) && !errors.Is(roleErr, ErrScopeExpired) {
			return roleErr
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.Validate(ctx, pending.Commit.Scope); err == nil && roleErr == nil {
		return ErrRequestConflict
	} else if err != nil && !errors.Is(err, ErrScopeExpired) && !errors.Is(err, ErrWrongOwner) {
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM kernel_device_owned_outbox WHERE owner_id=? AND request_id=? AND payload_hash=?`, pending.Commit.Scope.ResourceOwnerID, pending.Commit.Scope.RequestID, pending.Hash)
	return err
}

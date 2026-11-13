package chat

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/system/dataportability"
)

const (
	ComponentIDChatConversations = "chat.conversations"
	ComponentIDChatMessages      = "chat.messages"

	chatConversationsLogicalName = "chat.conversations.owned.v1"
	chatMessagesLogicalName      = "chat.messages.owned.v1"
)

type ChatBackupContributor struct {
	KernelDB *sql.DB
}

func NewChatBackupContributor(kernelDB *sql.DB) *ChatBackupContributor {
	return &ChatBackupContributor{KernelDB: kernelDB}
}

func (c *ChatBackupContributor) ID() string             { return "chat" }
func (c *ChatBackupContributor) Name() string           { return "Chat" }
func (c *ChatBackupContributor) Dependencies() []string { return []string{"character"} }
func (c *ChatBackupContributor) ComponentIDs() []string {
	return []string{ComponentIDChatConversations, ComponentIDChatMessages}
}

type ownedChatRecordV1 struct {
	OwnerID   string          `json:"ownerId"`
	RoleID    string          `json:"roleId"`
	ID        string          `json:"id"`
	Revision  int64           `json:"revision"`
	Body      json.RawMessage `json:"body"`
	UpdatedAt string          `json:"updatedAt"`
}

func (c *ChatBackupContributor) Plan(ctx context.Context, req dataportability.BackupRequest) ([]dataportability.BackupComponentPlan, error) {
	if req.EffectiveScope() != dataportability.ScopeAll && req.EffectiveScope() != dataportability.ScopeCharacter {
		return nil, nil
	}
	if c == nil || c.KernelDB == nil {
		return nil, errors.New("聊天备份数据库不可用")
	}
	conversations, err := c.countRecords(ctx, req, "conversation")
	if err != nil {
		return nil, err
	}
	messages, err := c.countRecords(ctx, req, "message")
	if err != nil {
		return nil, err
	}
	return []dataportability.BackupComponentPlan{
		{
			ID:            ComponentIDChatConversations,
			Kind:          dataportability.KindNDJSON,
			LogicalName:   chatConversationsLogicalName,
			Required:      true,
			SourceOfTruth: true,
			ItemCount:     conversations,
			EstimatedSize: conversations * 1024,
		},
		{
			ID:            ComponentIDChatMessages,
			Kind:          dataportability.KindNDJSON,
			LogicalName:   chatMessagesLogicalName,
			Required:      true,
			SourceOfTruth: true,
			ItemCount:     messages,
			EstimatedSize: messages * 2048,
		},
	}, nil
}

func (c *ChatBackupContributor) countRecords(ctx context.Context, req dataportability.BackupRequest, kind string) (int64, error) {
	query := `SELECT COUNT(*) FROM kernel_device_owned_resources WHERE kind=? AND deleted=0`
	args := []any{kind}
	if req.EffectiveScope() == dataportability.ScopeCharacter && req.CharacterID != "" {
		query += ` AND role_id=?`
		args = append(args, req.CharacterID)
	}
	var count int64
	if err := c.KernelDB.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("统计聊天备份记录失败(%s): %w", kind, err)
	}
	return count, nil
}

func (c *ChatBackupContributor) Export(ctx context.Context, req dataportability.BackupRequest, out dataportability.BackupWriter) error {
	if req.EffectiveScope() != dataportability.ScopeAll && req.EffectiveScope() != dataportability.ScopeCharacter {
		return nil
	}
	if c == nil || c.KernelDB == nil {
		return errors.New("聊天备份数据库不可用")
	}
	if err := c.exportKind(ctx, req, out, "conversation", ComponentIDChatConversations, chatConversationsLogicalName); err != nil {
		return err
	}
	return c.exportKind(ctx, req, out, "message", ComponentIDChatMessages, chatMessagesLogicalName)
}

func (c *ChatBackupContributor) exportKind(ctx context.Context, req dataportability.BackupRequest, out dataportability.BackupWriter, kind string, componentID string, logicalName string) error {
	component, err := out.CreateComponent(componentID, logicalName, dataportability.KindNDJSON)
	if err != nil {
		return fmt.Errorf("导出聊天备份组件失败(%s): %w", kind, err)
	}
	defer component.Close()

	query := `SELECT owner_id,role_id,resource_id,revision,body,updated_at FROM kernel_device_owned_resources WHERE kind=? AND deleted=0`
	args := []any{kind}
	if req.EffectiveScope() == dataportability.ScopeCharacter && req.CharacterID != "" {
		query += ` AND role_id=?`
		args = append(args, req.CharacterID)
	}
	query += ` ORDER BY owner_id,role_id,resource_id`
	rows, err := c.KernelDB.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("查询聊天备份记录失败(%s): %w", kind, err)
	}
	defer rows.Close()
	writer := bufio.NewWriter(component)
	for rows.Next() {
		var record ownedChatRecordV1
		var body []byte
		if err := rows.Scan(&record.OwnerID, &record.RoleID, &record.ID, &record.Revision, &body, &record.UpdatedAt); err != nil {
			return fmt.Errorf("读取聊天备份记录失败(%s): %w", kind, err)
		}
		record.Body = body
		encoded, err := json.Marshal(record)
		if err != nil {
			return fmt.Errorf("编码聊天备份记录失败(%s): %w", kind, err)
		}
		if _, err := writer.Write(append(encoded, '\n')); err != nil {
			return fmt.Errorf("写入聊天备份记录失败(%s): %w", kind, err)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("遍历聊天备份记录失败(%s): %w", kind, err)
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("刷新聊天备份记录失败(%s): %w", kind, err)
	}
	return nil
}

func readChatRecords(in dataportability.BackupReader, componentID string, invalid error) ([]ownedChatRecordV1, error) {
	reader, err := in.ReadComponent(componentID)
	if err != nil {
		return nil, fmt.Errorf("读取聊天备份组件失败(%s): %w", componentID, err)
	}
	defer reader.Close()
	records := make([]ownedChatRecordV1, 0)
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64<<10), 8<<20)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var record ownedChatRecordV1
		if err := json.Unmarshal(line, &record); err != nil {
			return nil, fmt.Errorf("%w: %v", invalid, err)
		}
		if record.OwnerID == "" || record.RoleID == "" || record.ID == "" || record.Revision < 1 || len(record.Body) == 0 || !json.Valid(record.Body) {
			return nil, fmt.Errorf("%w: 记录字段缺失或无效", invalid)
		}
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("%w: %v", invalid, err)
	}
	return records, nil
}

func (c *ChatBackupContributor) PreviewImport(ctx context.Context, req dataportability.ImportPreviewRequest, in dataportability.BackupReader) ([]dataportability.ImportComponentPreview, error) {
	conversations, err := readChatRecords(in, ComponentIDChatConversations, dataportability.ErrImportPackageInvalid)
	if err != nil {
		return nil, err
	}
	messages, err := readChatRecords(in, ComponentIDChatMessages, dataportability.ErrImportPackageInvalid)
	if err != nil {
		return nil, err
	}
	if c == nil || c.KernelDB == nil {
		return nil, errors.New("聊天备份数据库不可用")
	}
	convPreview := dataportability.ImportComponentPreview{
		ComponentID: ComponentIDChatConversations,
		Kind:        dataportability.KindNDJSON,
		LogicalName: chatConversationsLogicalName,
		ItemCount:   int64(len(conversations)),
		Collisions:  make([]dataportability.ComponentCollision, 0),
		Warnings:    make([]string, 0),
	}
	for _, record := range conversations {
		exists, err := chatResourceExists(ctx, c.KernelDB, record.OwnerID, "conversation", record.ID)
		if err != nil {
			return nil, err
		}
		if exists {
			convPreview.Collisions = append(convPreview.Collisions, dataportability.ComponentCollision{
				SourceID:   record.OwnerID + "/" + record.ID,
				TargetID:   record.ID,
				EntityType: "conversation",
				Policy:     dataportability.CollisionDuplicate,
			})
		}
	}
	msgPreview := dataportability.ImportComponentPreview{
		ComponentID: ComponentIDChatMessages,
		Kind:        dataportability.KindNDJSON,
		LogicalName: chatMessagesLogicalName,
		ItemCount:   int64(len(messages)),
		Collisions:  make([]dataportability.ComponentCollision, 0),
		Warnings:    make([]string, 0),
	}
	for _, record := range messages {
		exists, err := chatResourceExists(ctx, c.KernelDB, record.OwnerID, "message", record.ID)
		if err != nil {
			return nil, err
		}
		if exists {
			msgPreview.Collisions = append(msgPreview.Collisions, dataportability.ComponentCollision{
				SourceID:   record.OwnerID + "/" + record.ID,
				TargetID:   record.ID,
				EntityType: "message",
				Policy:     dataportability.CollisionDuplicate,
			})
		}
	}
	return []dataportability.ImportComponentPreview{convPreview, msgPreview}, nil
}

type chatQueryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func chatResourceExists(ctx context.Context, db chatQueryRower, ownerID string, kind string, resourceID string) (bool, error) {
	var exists bool
	if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM kernel_device_owned_resources WHERE owner_id=? AND kind=? AND resource_id=?)`, ownerID, kind, resourceID).Scan(&exists); err != nil {
		return false, fmt.Errorf("检查聊天记录冲突失败(%s): %w", kind, err)
	}
	return exists, nil
}

func (c *ChatBackupContributor) Import(ctx context.Context, req dataportability.ImportRequest, in dataportability.BackupReader) error {
	opts := dataportability.RestoreOptions{
		OperationID:        req.OperationID,
		Purpose:            dataportability.RestorePurposeOrdinary,
		CharacterPolicy:    req.CharacterPolicy,
		DefaultCharacterID: req.DefaultCharacterID,
		ActivateImported:   req.ActivateImported,
		IdentityMap:        req.IdentityMap,
		SecretProvider:     req.SecretProvider,
	}
	return c.RestoreChats(ctx, in, opts)
}

func (c *ChatBackupContributor) RestoreChats(ctx context.Context, in dataportability.BackupReader, opts dataportability.RestoreOptions) error {
	if opts.IdentityMap == nil {
		return dataportability.ErrImportIdentityMapMissing
	}
	if c == nil || c.KernelDB == nil {
		return errors.New("聊天备份数据库不可用")
	}
	conversations, err := readChatRecords(in, ComponentIDChatConversations, dataportability.ErrImportPackageInvalid)
	if err != nil {
		return err
	}
	messages, err := readChatRecords(in, ComponentIDChatMessages, dataportability.ErrImportPackageInvalid)
	if err != nil {
		return err
	}
	tx, err := c.KernelDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("开启聊天恢复事务失败: %w", err)
	}
	defer tx.Rollback()

	conversationTargets := make(map[string]string, len(conversations))
	for _, record := range conversations {
		roleID := remapChatRole(opts, record.RoleID)
		body := record.Body
		targetID := record.ID
		exists, err := chatResourceExists(ctx, tx, record.OwnerID, "conversation", record.ID)
		if err != nil {
			return err
		}
		if exists {
			switch opts.CharacterPolicy {
			case dataportability.CollisionSkip:
				conversationTargets[record.ID] = record.ID
				opts.IdentityMap.AddConversation(record.ID, record.ID)
				continue
			case dataportability.CollisionDuplicate:
				targetID = uuid.NewString()
				body, err = patchChatBody(body, map[string]string{"id": targetID, "conversationId": targetID})
				if err != nil {
					return fmt.Errorf("%w: %v", dataportability.ErrImportPackageInvalid, err)
				}
			case dataportability.CollisionReplace:
			default:
				return dataportability.ErrImportCollisionPolicyInvalid
			}
		} else {
			body, err = patchChatBody(body, map[string]string{"id": targetID, "conversationId": targetID})
			if err != nil {
				return fmt.Errorf("%w: %v", dataportability.ErrImportPackageInvalid, err)
			}
		}
		if err := writeChatRecord(ctx, tx, record.OwnerID, "conversation", targetID, roleID, record.Revision, body, record.UpdatedAt, opts.CharacterPolicy == dataportability.CollisionReplace); err != nil {
			return err
		}
		conversationTargets[record.ID] = targetID
		opts.IdentityMap.AddConversation(record.ID, targetID)
	}

	for _, record := range messages {
		conversationID := chatRecordConversationID(record.Body)
		if conversationID == "" {
			return fmt.Errorf("%w: 消息缺少会话标识", dataportability.ErrImportPackageInvalid)
		}
		targetConversationID, ok := conversationTargets[conversationID]
		if !ok {
			targetConversationID, ok = opts.IdentityMap.GetConversation(conversationID)
		}
		if !ok || targetConversationID == "" {
			return dataportability.ErrImportMessageRefInvalid
		}
		roleID := remapChatRole(opts, record.RoleID)
		body, err := patchChatBody(record.Body, map[string]string{"conversationId": targetConversationID})
		if err != nil {
			return fmt.Errorf("%w: %v", dataportability.ErrImportPackageInvalid, err)
		}
		if roleID != record.RoleID {
			body, err = patchChatBody(body, map[string]string{"characterId": roleID})
			if err != nil {
				return fmt.Errorf("%w: %v", dataportability.ErrImportPackageInvalid, err)
			}
		}
		targetID := record.ID
		exists, err := chatResourceExists(ctx, tx, record.OwnerID, "message", record.ID)
		if err != nil {
			return err
		}
		if exists {
			switch opts.CharacterPolicy {
			case dataportability.CollisionSkip:
				opts.IdentityMap.AddMessage(record.ID, record.ID)
				continue
			case dataportability.CollisionDuplicate:
				targetID = uuid.NewString() + "/" + chatRecordRoleSuffix(body)
				body, err = patchChatBody(body, map[string]string{"id": targetID})
				if err != nil {
					return fmt.Errorf("%w: %v", dataportability.ErrImportPackageInvalid, err)
				}
			case dataportability.CollisionReplace:
			default:
				return dataportability.ErrImportCollisionPolicyInvalid
			}
		} else {
			if err := writeChatRecord(ctx, tx, record.OwnerID, "message", targetID, roleID, record.Revision, body, record.UpdatedAt, false); err != nil {
				return err
			}
			opts.IdentityMap.AddMessage(record.ID, targetID)
			continue
		}
		if err := writeChatRecord(ctx, tx, record.OwnerID, "message", targetID, roleID, record.Revision, body, record.UpdatedAt, opts.CharacterPolicy == dataportability.CollisionReplace); err != nil {
			return err
		}
		opts.IdentityMap.AddMessage(record.ID, targetID)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交聊天恢复事务失败: %w", err)
	}
	return nil
}

func writeChatRecord(ctx context.Context, tx *sql.Tx, ownerID string, kind string, resourceID string, roleID string, revision int64, body []byte, updatedAt string, replace bool) error {
	statement := `INSERT INTO kernel_device_owned_resources(owner_id,kind,resource_id,role_id,source_id,revision,deleted,body,updated_at) VALUES(?,?,?,?,'',?,0,?,?)`
	if replace {
		statement += ` ON CONFLICT(owner_id,kind,resource_id) DO UPDATE SET role_id=excluded.role_id,revision=excluded.revision,deleted=0,body=excluded.body,updated_at=excluded.updated_at`
	}
	if _, err := tx.ExecContext(ctx, statement, ownerID, kind, resourceID, roleID, revision, body, updatedAt); err != nil {
		return fmt.Errorf("写入聊天恢复记录失败(%s): %w", kind, err)
	}
	return nil
}

func remapChatRole(opts dataportability.RestoreOptions, roleID string) string {
	if roleID == "" || opts.IdentityMap == nil {
		return roleID
	}
	if mapped, ok := opts.IdentityMap.GetCharacter(roleID); ok && mapped != "" {
		return mapped
	}
	if opts.DefaultCharacterID != "" {
		return opts.DefaultCharacterID
	}
	return roleID
}

func chatRecordConversationID(body []byte) string {
	document := map[string]any{}
	if err := json.Unmarshal(body, &document); err != nil {
		return ""
	}
	value, _ := document["conversationId"].(string)
	return value
}

func chatRecordRoleSuffix(body []byte) string {
	document := map[string]any{}
	if err := json.Unmarshal(body, &document); err != nil {
		return "message"
	}
	if value, ok := document["role"].(string); ok && value != "" {
		return value
	}
	return "message"
}

func patchChatBody(body []byte, patches map[string]string) ([]byte, error) {
	document := map[string]json.RawMessage{}
	if err := json.Unmarshal(body, &document); err != nil {
		return nil, err
	}
	for key, value := range patches {
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		document[key] = encoded
	}
	return json.Marshal(document)
}

package permission

type ScopeType string

const (
	ScopeGlobal       ScopeType = "global"
	ScopeCharacter    ScopeType = "character"
	ScopeConversation ScopeType = "conversation"
	ScopeExtension    ScopeType = "extension"
	ScopeModule       ScopeType = "module"
	ScopeTool         ScopeType = "tool"
	ScopeResource     ScopeType = "resource"
	ScopeTarget       ScopeType = "target"
	ScopeInvocation   ScopeType = "invocation"
	ScopeSession      ScopeType = "session"
)

type PermissionScope struct {
	Type           ScopeType `json:"type"`
	ID             string    `json:"id,omitempty"`
	CharacterID    string    `json:"characterId,omitempty"`
	ConversationID string    `json:"conversationId,omitempty"`
	ExtensionID    string    `json:"extensionId,omitempty"`
	ToolID         string    `json:"toolId,omitempty"`
	ResourceID     string    `json:"resourceId,omitempty"`
}

func ScopeForCharacter(charID string) PermissionScope {
	return PermissionScope{Type: ScopeCharacter, CharacterID: charID}
}

func ScopeForConversation(convID string) PermissionScope {
	return PermissionScope{Type: ScopeConversation, ConversationID: convID}
}

func ScopeForExtension(extID string) PermissionScope {
	return PermissionScope{Type: ScopeExtension, ExtensionID: extID}
}

func ScopeForInvocation(invID string) PermissionScope {
	return PermissionScope{Type: ScopeInvocation, ID: invID}
}

func ScopeGlobalOnly() PermissionScope {
	return PermissionScope{Type: ScopeGlobal}
}

func (s PermissionScope) Contains(other PermissionScope) bool {
	if s.Type == ScopeGlobal {
		return true
	}
	if s.Type != other.Type {
		return false
	}
	switch s.Type {
	case ScopeCharacter:
		return s.CharacterID == other.CharacterID
	case ScopeConversation:
		return s.ConversationID == other.ConversationID
	case ScopeExtension:
		return s.ExtensionID == other.ExtensionID
	case ScopeTool:
		return s.ToolID == other.ToolID
	default:
		return s.ID == other.ID
	}
}

func (s PermissionScope) IsGlobal() bool {
	return s.Type == ScopeGlobal
}

func (s PermissionScope) IsValid() bool {
	return s.Type != ""
}

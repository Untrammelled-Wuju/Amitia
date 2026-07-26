package scope

import "fmt"

func NewGlobalScope() ScopeRef {
	return ScopeRef{Type: ScopeGlobal}
}

func NewCharacterScope(charID string) ScopeRef {
	return ScopeRef{Type: ScopeCharacter, CharacterID: charID}
}

func NewConversationScope(convID string) ScopeRef {
	return ScopeRef{Type: ScopeConversation, ConversationID: convID}
}

func NewExtensionScope(extID string) ScopeRef {
	return ScopeRef{Type: ScopeExtension, ExtensionID: extID}
}

func NewModuleScope(extID, moduleID string) ScopeRef {
	return ScopeRef{Type: ScopeModule, ExtensionID: extID, ModuleID: moduleID}
}

func NewResourceScope(resourceType, resourceID string) ScopeRef {
	return ScopeRef{Type: ScopeResource, ResourceType: resourceType, ResourceID: resourceID}
}

func NewInvocationScope(invID string) ScopeRef {
	return ScopeRef{Type: ScopeInvocation, InvocationID: invID}
}

func NewSessionScope(sessionID string) ScopeRef {
	return ScopeRef{Type: ScopeSession, SessionID: sessionID}
}

func (s ScopeRef) Validate() error {
	if s.Type == "" {
		return fmt.Errorf("scope type is required")
	}
	switch s.Type {
	case ScopeGlobal:
		return nil
	case ScopeCharacter:
		if s.CharacterID == "" {
			return fmt.Errorf("character scope requires CharacterID")
		}
		if s.ConversationID != "" || s.ExtensionID != "" || s.ModuleID != "" || s.ResourceID != "" || s.InvocationID != "" || s.SessionID != "" {
			return fmt.Errorf("character scope must not contain other scope fields")
		}
	case ScopeConversation:
		if s.ConversationID == "" {
			return fmt.Errorf("conversation scope requires ConversationID")
		}
		if s.CharacterID != "" || s.ExtensionID != "" || s.ModuleID != "" || s.ResourceID != "" || s.InvocationID != "" || s.SessionID != "" {
			return fmt.Errorf("conversation scope must not contain other scope fields")
		}
	case ScopeExtension:
		if s.ExtensionID == "" {
			return fmt.Errorf("extension scope requires ExtensionID")
		}
		if s.ModuleID != "" {
			return fmt.Errorf("extension scope must not contain ModuleID, use module scope")
		}
	case ScopeModule:
		if s.ExtensionID == "" || s.ModuleID == "" {
			return fmt.Errorf("module scope requires ExtensionID and ModuleID")
		}
	case ScopeResource:
		if s.ResourceType == "" || s.ResourceID == "" {
			return fmt.Errorf("resource scope requires ResourceType and ResourceID")
		}
	case ScopeInvocation:
		if s.InvocationID == "" {
			return fmt.Errorf("invocation scope requires InvocationID")
		}
	case ScopeSession:
		if s.SessionID == "" {
			return fmt.Errorf("session scope requires SessionID")
		}
	default:
		return fmt.Errorf("unknown scope type: %s", s.Type)
	}
	return nil
}

func (s ScopeRef) IsGlobal() bool {
	return s.Type == ScopeGlobal
}

func (s ScopeRef) Contains(other ScopeRef) bool {
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
	case ScopeModule:
		return s.ExtensionID == other.ExtensionID && s.ModuleID == other.ModuleID
	case ScopeResource:
		return s.ResourceType == other.ResourceType && s.ResourceID == other.ResourceID
	case ScopeInvocation:
		return s.InvocationID == other.InvocationID
	case ScopeSession:
		return s.SessionID == other.SessionID
	default:
		return false
	}
}

func (s ScopeRef) String() string {
	switch s.Type {
	case ScopeGlobal:
		return "global"
	case ScopeCharacter:
		return fmt.Sprintf("character/%s", s.CharacterID)
	case ScopeConversation:
		return fmt.Sprintf("conversation/%s", s.ConversationID)
	case ScopeExtension:
		return fmt.Sprintf("extension/%s", s.ExtensionID)
	case ScopeModule:
		return fmt.Sprintf("extension/%s/module/%s", s.ExtensionID, s.ModuleID)
	case ScopeResource:
		return fmt.Sprintf("resource/%s/%s", s.ResourceType, s.ResourceID)
	case ScopeInvocation:
		return fmt.Sprintf("invocation/%s", s.InvocationID)
	case ScopeSession:
		return fmt.Sprintf("session/%s", s.SessionID)
	default:
		return "unknown"
	}
}

func (s ScopeRef) Valid() bool {
	return s.Validate() == nil
}

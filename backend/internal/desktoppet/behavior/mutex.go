package behavior

type MutexManager struct {
	groups map[string]string
}

func NewMutexManager() *MutexManager {
	return &MutexManager{groups: make(map[string]string)}
}

func DefaultMutexGroups() map[string][]string {
	return map[string][]string{
		"conversation_frontstage": {"listening", "thinking", "speaking", "waiting", "greeting", "goodbye"},
		"physical_interaction":    {"clicked", "double_clicked", "dragged", "picked_up", "dropped", "fall"},
		"locomotion":              {"walk_left", "walk_right", "run_left", "run_right", "jump", "land", "edge_climb", "turn_around"},
		"life_activity":           {"sleep", "eat", "drink", "read", "write", "use_phone", "work", "study", "sit", "sleep_on_desktop"},
		"emotion_expression":      {"happy", "excited", "shy", "sad", "cry", "angry", "surprised", "confused", "embarrassed", "scared", "proud", "tired"},
		"natural_idle":            {"idle_normal", "idle_breathing", "idle_blink", "idle_look_around", "idle_sway", "stretch"},
	}
}

func (m *MutexManager) FindConflict(semantic string, currentForeground *ForegroundActionState) (bool, string) {
	if currentForeground == nil || currentForeground.Semantic == "" {
		return false, ""
	}
	groups := DefaultMutexGroups()
	for groupName, members := range groups {
		currentInGroup := contains(members, currentForeground.Semantic)
		newInGroup := contains(members, semantic)
		if currentInGroup && newInGroup && currentForeground.Semantic != semantic {
			return true, groupName
		}
	}
	return false, ""
}

func (m *MutexManager) IsPhysicalInteraction(semantic string) bool {
	groups := DefaultMutexGroups()
	physical := groups["physical_interaction"]
	return contains(physical, semantic)
}

func (m *MutexManager) IsSystemSafety(semantic string) bool {
	return semantic == "system_force_hide" || semantic == "system_destroy" || semantic == "safety_recovery"
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

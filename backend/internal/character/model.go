package character



type Character struct {
	ID                  string    `gorm:"column:id;primaryKey" json:"id"`
	Name                string    `gorm:"column:name;not null" json:"name"`
	Avatar              string    `gorm:"column:avatar" json:"avatar"`
	Identity            string    `gorm:"column:identity" json:"identity"`
	Personality         string    `gorm:"column:personality" json:"personality"`
	SpeakingStyle       string    `gorm:"column:speaking_style" json:"speakingStyle"`
	RelationshipStyle   string    `gorm:"column:relationship_style" json:"relationshipStyle"`
	SystemPrompt        string    `gorm:"column:system_prompt" json:"systemPrompt"`
	BoundaryRules       string    `gorm:"column:boundary_rules" json:"boundaryRules"`
	PersonalitySliders  string    `gorm:"column:personality_sliders" json:"personalitySliders"`
	Description         string    `gorm:"column:description" json:"description"`
	BasePrompt          string    `gorm:"column:base_prompt" json:"basePrompt"`
	GeneratedPrompt     string    `gorm:"column:generated_prompt" json:"generatedPrompt"`
	IsDefault           int       `gorm:"column:is_default;default:0" json:"isDefault"`
	Status              string    `gorm:"column:status;default:enabled" json:"status"`
	PersonalityConfig   string    `gorm:"column:personality_config;default:{}" json:"personalityConfig"`
	ChatStyleConfig     string    `gorm:"column:chat_style_config;default:{}" json:"chatStyleConfig"`
	SceneRules          string    `gorm:"column:scene_rules;default:{}" json:"sceneRules"`
	IsActive            int       `gorm:"column:is_active;default:0" json:"isActive"`
	SortOrder           int       `gorm:"column:sort_order;default:0" json:"sortOrder"`
	ConversationID      string    `gorm:"column:conversation_id" json:"conversationId"`
	CreatedAt           string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt           string `gorm:"column:updated_at" json:"updatedAt"`
	Gender              string    `gorm:"column:gender;default:UNSPECIFIED" json:"gender"`
	GenderLabel         *string   `gorm:"column:gender_label" json:"genderLabel"`
	Pronoun             string    `gorm:"column:pronoun;default:TA" json:"pronoun"`
	SelfReference       string    `gorm:"column:self_reference;default:我" json:"selfReference"`
	UserAddressingStyle *string   `gorm:"column:user_addressing_style" json:"userAddressingStyle"`
	GenderExpression    int       `gorm:"column:gender_expression;default:30" json:"genderExpression"`
	LifeIdentity        string    `gorm:"column:life_identity;default:CUSTOM" json:"lifeIdentity"`
}

func (Character) TableName() string { return "characters" }


type CharacterTemplate struct {
	ID           string    `gorm:"column:id;primaryKey" json:"id"`
	Name         string    `gorm:"column:name;not null" json:"name"`
	Category     string    `gorm:"column:category" json:"category"`
	Description  string    `gorm:"column:description" json:"description"`
	Builtin      int       `gorm:"column:builtin" json:"builtin"`
	TemplateJSON string    `gorm:"column:template_json" json:"templateJson"`
	CreatedAt    string `gorm:"column:created_at" json:"createdAt"`
}

func (CharacterTemplate) TableName() string { return "character_templates" }



type CreateCharacterRequest struct {
	IsDefault          bool    `json:"isDefault"`
	Name              string `json:"name"`
	Identity          string `json:"identity"`
	Personality       string `json:"personality"`
	SpeakingStyle     string `json:"speakingStyle"`
	RelationshipStyle string `json:"relationshipStyle"`
	SystemPrompt      string `json:"systemPrompt"`
	BoundaryRules     string `json:"boundaryRules"`
	Description       string `json:"description"`
	Gender            string `json:"gender"`
	Pronoun           string `json:"pronoun"`
	SelfReference     string `json:"selfReference"`
	GenderExpression  int    `json:"genderExpression"`
	LifeIdentity      string `json:"lifeIdentity"`
}

type UpdateCharacterRequest struct {
	IsDefault          *bool   `json:"isDefault"`
	Name              *string `json:"name"`
	Identity          *string `json:"identity"`
	Personality       *string `json:"personality"`
	SpeakingStyle     *string `json:"speakingStyle"`
	RelationshipStyle *string `json:"relationshipStyle"`
	SystemPrompt      *string `json:"systemPrompt"`
	BoundaryRules     *string `json:"boundaryRules"`
	Description       *string `json:"description"`
	Status            *string `json:"status"`
	IsActive          *int    `json:"isActive"`
	SortOrder         *int    `json:"sortOrder"`
	Gender            *string `json:"gender"`
	Pronoun           *string `json:"pronoun"`
	SelfReference     *string `json:"selfReference"`
	GenderExpression  *int    `json:"genderExpression"`
	LifeIdentity      *string `json:"lifeIdentity"`
	PersonalityConfig *string `json:"personalityConfig"`
	ChatStyleConfig   *string `json:"chatStyleConfig"`
	SceneRules        *string `json:"sceneRules"`
	Avatar            *string `json:"avatar"`
}



type RoleProfileResponse struct {
	ID                  string  `json:"id"`
	CharacterID         string  `json:"characterId"`
	RoleName            string  `json:"roleName"`
	Gender              string  `json:"gender"`
	GenderLabel         *string `json:"genderLabel"`
	Pronoun             string  `json:"pronoun"`
	SelfReference       string  `json:"selfReference"`
	GenderExpression    int     `json:"genderExpression"`
	LifeIdentity        string  `json:"lifeIdentity"`
	UserAddressingStyle *string `json:"userAddressingStyle"`
}

package business

type Memory struct {
	ID                    string  `json:"id"`
	SpaceID               string  `json:"spaceId"`
	CharacterID           string  `json:"characterId"`
	MemoryType            string  `json:"memoryType"`
	MemorySubtype         string  `json:"memorySubtype"`
	Source                string  `json:"source"`
	Scope                 string  `json:"scope"`
	Key                   string  `json:"key"`
	Value                 string  `json:"value"`
	Importance            int     `json:"importance"`
	Confidence            int     `json:"confidence"`
	ExpiresAt             *string `json:"expiresAt"`
	EntityID              string  `json:"entityId"`
	EntityType            string  `json:"entityType"`
	SourceMsgID           string  `json:"sourceMsgId"`
	SourceConvID          string  `json:"sourceConvId"`
	VerifiedStatus        string  `json:"verifiedStatus"`
	LastVerifiedAt        *string `json:"lastVerifiedAt"`
	UseCount              int     `json:"useCount"`
	LastUsedAt            *string `json:"lastUsedAt"`
	RetentionLevel        int     `json:"retentionLevel"`
	MemoryStrength        float64 `json:"memoryStrength"`
	StrengthUpdatedAt     *string `json:"strengthUpdatedAt"`
	LastReinforcedAt      *string `json:"lastReinforcedAt"`
	ReinforceCount        int     `json:"reinforceCount"`
	RetrievedCount        int     `json:"retrievedCount"`
	InjectedCount         int     `json:"injectedCount"`
	DecayState            string  `json:"decayState"`
	Pinned                bool    `json:"pinned"`
	ArchivedAt            *string `json:"archivedAt"`
	SupersededBy          string  `json:"supersededBy"`
	SensitivityLevel      string  `json:"sensitivityLevel"`
	AllowContextUse       *bool   `json:"allowContextUse"`
	AllowProactiveMention bool    `json:"allowProactiveMention"`
	RequiresConfirmation  bool    `json:"requiresConfirmation"`
	Version               int     `json:"version"`
	DerivationKey         string  `json:"derivationKey"`
	CreatedAt             string  `json:"createdAt"`
	UpdatedAt             string  `json:"updatedAt"`
}

type HybridSearchResult struct {
	Memory            Memory  `json:"memory"`
	Score             float64 `json:"score"`
	VectorScore       float64 `json:"vectorScore"`
	KeywordScore      float64 `json:"keywordScore"`
	MatchType         string  `json:"matchType"`
	CollectionName    string  `json:"collectionName"`
	MemoryLayer       string  `json:"memoryLayer"`
	SourceType        string  `json:"sourceType,omitempty"`
	RRFScore          float64 `json:"rrfScore,omitempty"`
	RetentionStrength float64 `json:"retentionStrength,omitempty"`
	TemporalBoost     float64 `json:"temporalBoost"`
	ValidityPenalty   float64 `json:"validityPenalty"`
	TemporalReference string  `json:"temporalReference"`
}

const MaxMemoryInjectTotal = 15
const (
	RetentionL1 = 1
	RetentionL2 = 2
	RetentionL3 = 3
	RetentionL4 = 4
	RetentionL5 = 5
)

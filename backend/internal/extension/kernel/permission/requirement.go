package permission

import "encoding/json"

type PermissionRequirement struct {
	PermissionID string               `json:"permissionId"`
	Scope        PermissionScope      `json:"scope,omitempty"`
	Conditions   json.RawMessage      `json:"conditions,omitempty"`
	Optional     bool                 `json:"optional"`
}

type ExpectedSideEffect struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Reversible  bool   `json:"reversible"`
}

type PermissionTarget struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Path string `json:"path,omitempty"`
	URL  string `json:"url,omitempty"`
}

type PermissionEvaluationRequest struct {
	Subject      PermissionSubject      `json:"subject"`
	Requirements []PermissionRequirement `json:"requirements"`
	InvocationID string                 `json:"invocationId,omitempty"`
	Input        json.RawMessage        `json:"input,omitempty"`
	Target       PermissionTarget       `json:"target,omitempty"`
	RiskLevel    string                 `json:"riskLevel,omitempty"`
	SideEffects  []ExpectedSideEffect   `json:"sideEffects,omitempty"`
	IsBackground bool                   `json:"isBackground"`
	ParentGrants []PermissionGrant      `json:"parentGrants,omitempty"`
}

type ApprovalRequest struct {
	ToolName     string               `json:"toolName"`
	ExtensionName string              `json:"extensionName"`
	Source       string               `json:"source"`
	InputSummary string               `json:"inputSummary"`
	RiskLevel    string               `json:"riskLevel"`
	SideEffects  []ExpectedSideEffect `json:"sideEffects"`
	Target       PermissionTarget     `json:"target"`
	Reversible   bool                 `json:"reversible"`
	Repeatable   bool                 `json:"repeatable"`
	LongTerm     bool                 `json:"longTerm"`
	TimeoutMS    int64                `json:"timeoutMs"`
}

type PermissionReason struct {
	Code       string `json:"code"`
	Permission string `json:"permission,omitempty"`
	Detail     string `json:"detail,omitempty"`
}

type PermissionEvaluationResult struct {
	Decision        PermissionDecision   `json:"decision"`
	Missing         []PermissionRequirement `json:"missing,omitempty"`
	MatchedGrants   []PermissionGrant    `json:"matchedGrants,omitempty"`
	ApprovalRequest *ApprovalRequest     `json:"approvalRequest,omitempty"`
	Reasons         []PermissionReason   `json:"reasons,omitempty"`
}

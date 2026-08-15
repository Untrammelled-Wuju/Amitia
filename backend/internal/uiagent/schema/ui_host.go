package schema

type SchemaUIHostMode string

const (
	SchemaAttached      SchemaUIHostMode = "attached"
	SchemaBackgroundRun SchemaUIHostMode = "background_run"
	SchemaDetached      SchemaUIHostMode = "detached"
	SchemaHeadless      SchemaUIHostMode = "headless"
)

type SchemaUIHostState string

const (
	SchemaHostInit       SchemaUIHostState = "init"
	SchemaHostCompiling  SchemaUIHostState = "compiling"
	SchemaHostRendering  SchemaUIHostState = "rendering"
	SchemaHostRunning    SchemaUIHostState = "running"
	SchemaHostBackground SchemaUIHostState = "background"
	SchemaHostUnmounted  SchemaUIHostState = "unmounted"
	SchemaHostError      SchemaUIHostState = "error"
)

type UIHostState struct {
	Mode         SchemaUIHostMode  `json:"mode"`
	State        SchemaUIHostState `json:"state"`
	RuntimeID    string            `json:"runtimeId,omitempty"`
	WorkspaceID  string            `json:"workspaceId,omitempty"`
	PreviewToken string            `json:"previewToken,omitempty"`
	ErrorMessage string            `json:"errorMessage,omitempty"`
}

type StateMachineEdge struct {
	From SchemaUIHostState `json:"from"`
	To   SchemaUIHostState `json:"to"`
	Rule string            `json:"rule"`
}

var SchemaHostTransitions = []StateMachineEdge{
	{From: SchemaHostInit, To: SchemaHostCompiling, Rule: "compile"},
	{From: SchemaHostCompiling, To: SchemaHostRendering, Rule: "render"},
	{From: SchemaHostRendering, To: SchemaHostRunning, Rule: "ready"},
	{From: SchemaHostRunning, To: SchemaHostBackground, Rule: "backgroundize"},
	{From: SchemaHostRunning, To: SchemaHostUnmounted, Rule: "unmount"},
	{From: SchemaHostRunning, To: SchemaHostError, Rule: "error"},
	{From: SchemaHostBackground, To: SchemaHostRunning, Rule: "foregroundize"},
	{From: SchemaHostBackground, To: SchemaHostUnmounted, Rule: "unmount"},
	{From: SchemaHostError, To: SchemaHostRunning, Rule: "recover"},
	{From: SchemaHostUnmounted, To: SchemaHostRunning, Rule: "remount"},
}

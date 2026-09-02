package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/workflow"
)

// AgentAdminToolController bridges kernel-owned model tools to application
// services without making the extension kernel depend on the chat/server layer.
// Implementations must enforce the invocation user scope and must never return
// stored secrets in tool results.
type AgentAdminToolController interface {
	CanExecuteAgentAdminTool(toolName string) bool
	ExecuteAgentAdminTool(ctx context.Context, toolName string, input json.RawMessage, invocation capability.ToolInvocationContext) (json.RawMessage, error)
}

type agentAdminToolService struct {
	controller       AgentAdminToolController
	workflowRegistry *workflow.WorkflowRegistry
	workflowExecutor *workflow.WorkflowExecutor
	toolRegistry     *capability.ToolRegistry
}

func newAgentAdminToolService(controller AgentAdminToolController, workflowRegistry *workflow.WorkflowRegistry, workflowExecutor *workflow.WorkflowExecutor, toolRegistry *capability.ToolRegistry) *agentAdminToolService {
	return &agentAdminToolService{controller: controller, workflowRegistry: workflowRegistry, workflowExecutor: workflowExecutor, toolRegistry: toolRegistry}
}

var workflowAdminHandlers = map[string]struct{}{
	"get_all_workflows": {}, "create_workflow": {}, "get_workflow": {}, "update_workflow": {},
	"patch_workflow": {}, "delete_workflow": {}, "trigger_workflow": {},
}

func (s *agentAdminToolService) CanHandle(name string) bool {
	if name == "calculate" || name == "sleep" {
		return s != nil
	}
	if _, ok := workflowAdminHandlers[name]; ok {
		return s != nil && s.workflowRegistry != nil
	}
	return s != nil && s.controller != nil && s.controller.CanExecuteAgentAdminTool(name)
}

func (s *agentAdminToolService) Dispatch(ctx context.Context, name string, input json.RawMessage, invocation capability.ToolInvocationContext) (json.RawMessage, error) {
	if name == "calculate" {
		var req struct {
			Expression string `json:"expression"`
		}
		if err := decodeToolInput(input, &req); err != nil {
			return nil, err
		}
		value, err := evaluateCalculatorExpression(req.Expression)
		if err != nil {
			return nil, err
		}
		return marshalToolResult(map[string]any{"expression": req.Expression, "result": value})
	}
	if name == "sleep" {
		var req struct {
			DurationMS   int64 `json:"duration_ms"`
			Milliseconds int64 `json:"milliseconds"`
		}
		if err := decodeToolInput(input, &req); err != nil {
			return nil, err
		}
		duration := req.DurationMS
		if duration <= 0 {
			duration = req.Milliseconds
		}
		if duration < 0 || duration > 300000 {
			return nil, fmt.Errorf("sleep duration must be between 0 and 300000 ms")
		}
		timer := time.NewTimer(time.Duration(duration) * time.Millisecond)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
			return marshalToolResult(map[string]any{"sleptMs": duration})
		}
	}
	if _, ok := workflowAdminHandlers[name]; ok {
		return s.dispatchWorkflow(ctx, name, input, invocation)
	}
	if s.controller == nil || !s.controller.CanExecuteAgentAdminTool(name) {
		return nil, fmt.Errorf("agent admin handler %s not found", name)
	}
	return s.controller.ExecuteAgentAdminTool(ctx, name, input, invocation)
}

func (s *agentAdminToolService) dispatchWorkflow(ctx context.Context, name string, input json.RawMessage, invocation capability.ToolInvocationContext) (json.RawMessage, error) {
	if s.workflowRegistry == nil {
		return nil, fmt.Errorf("workflow registry unavailable")
	}
	switch name {
	case "get_all_workflows":
		defs := s.workflowRegistry.List(workflow.WorkflowFilter{})
		out := make([]workflow.WorkflowDefinition, 0, len(defs))
		for _, def := range defs {
			if workflowVisibleToInvocation(def, invocation.UserID) {
				out = append(out, def)
			}
		}
		return marshalToolResult(map[string]any{"workflows": out, "count": len(out)})
	case "get_workflow":
		var req struct {
			ID string `json:"id"`
		}
		if err := decodeToolInput(input, &req); err != nil {
			return nil, err
		}
		def, err := s.getWorkflowForInvocation(req.ID, invocation.UserID, false)
		if err != nil {
			return nil, err
		}
		return marshalToolResult(def)
	case "create_workflow":
		var req struct {
			Definition json.RawMessage `json:"definition"`
		}
		if err := decodeToolInput(input, &req); err != nil {
			return nil, err
		}
		if len(req.Definition) == 0 {
			req.Definition = input
		}
		var raw map[string]any
		if err := json.Unmarshal(req.Definition, &raw); err != nil {
			return nil, fmt.Errorf("invalid workflow definition: %w", err)
		}
		if strings.TrimSpace(fmt.Sprint(raw["id"])) == "" {
			raw["id"] = uuid.NewString()
		}
		if _, ok := raw["enabled"]; !ok {
			raw["enabled"] = true
		}
		raw["source"] = "user"
		metadata, _ := raw["metadata"].(map[string]any)
		if metadata == nil {
			metadata = map[string]any{}
		}
		if strings.TrimSpace(invocation.UserID) != "" {
			metadata["ownerUserId"] = strings.TrimSpace(invocation.UserID)
		}
		raw["metadata"] = metadata
		encoded, _ := json.Marshal(raw)
		var def workflow.WorkflowDefinition
		if err := json.Unmarshal(encoded, &def); err != nil {
			return nil, err
		}
		normalized, err := workflow.NormalizeDefinition(def)
		if err != nil {
			return nil, err
		}
		if _, exists := s.workflowRegistry.Get(normalized.ID); exists {
			return nil, fmt.Errorf("workflow %s already exists", normalized.ID)
		}
		if err := s.workflowRegistry.UpsertContext(ctx, normalized); err != nil {
			return nil, err
		}
		if err := SyncUserWorkflowAgentTool(ctx, s.toolRegistry, normalized); err != nil {
			_ = s.workflowRegistry.UnregisterContext(ctx, normalized.ID)
			return nil, err
		}
		return marshalToolResult(normalized)
	case "update_workflow":
		var req struct {
			ID         string          `json:"id"`
			Definition json.RawMessage `json:"definition"`
		}
		if err := decodeToolInput(input, &req); err != nil {
			return nil, err
		}
		current, err := s.getWorkflowForInvocation(req.ID, invocation.UserID, true)
		if err != nil {
			return nil, err
		}
		if len(req.Definition) == 0 {
			return nil, fmt.Errorf("definition is required")
		}
		var def workflow.WorkflowDefinition
		if err := json.Unmarshal(req.Definition, &def); err != nil {
			return nil, err
		}
		def.ID = current.ID
		def.Source = "user"
		if def.Metadata == nil {
			def.Metadata = map[string]any{}
		}
		def.Metadata["ownerUserId"] = workflowOwner(current)
		normalized, err := workflow.NormalizeDefinition(def)
		if err != nil {
			return nil, err
		}
		if err := s.workflowRegistry.UpsertContext(ctx, normalized); err != nil {
			return nil, err
		}
		if err := SyncUserWorkflowAgentTool(ctx, s.toolRegistry, normalized); err != nil {
			return nil, err
		}
		return marshalToolResult(normalized)
	case "patch_workflow":
		var req struct {
			ID    string          `json:"id"`
			Patch json.RawMessage `json:"patch"`
		}
		if err := decodeToolInput(input, &req); err != nil {
			return nil, err
		}
		current, err := s.getWorkflowForInvocation(req.ID, invocation.UserID, true)
		if err != nil {
			return nil, err
		}
		if len(req.Patch) == 0 {
			return nil, fmt.Errorf("patch is required")
		}
		baseJSON, _ := json.Marshal(current)
		var base, patch map[string]any
		if json.Unmarshal(baseJSON, &base) != nil || json.Unmarshal(req.Patch, &patch) != nil {
			return nil, fmt.Errorf("workflow patch must be a JSON object")
		}
		mergeJSONObjects(base, patch)
		base["id"] = current.ID
		base["source"] = "user"
		metadata, _ := base["metadata"].(map[string]any)
		if metadata == nil {
			metadata = map[string]any{}
		}
		metadata["ownerUserId"] = workflowOwner(current)
		base["metadata"] = metadata
		merged, _ := json.Marshal(base)
		var def workflow.WorkflowDefinition
		if err := json.Unmarshal(merged, &def); err != nil {
			return nil, err
		}
		normalized, err := workflow.NormalizeDefinition(def)
		if err != nil {
			return nil, err
		}
		if err := s.workflowRegistry.UpsertContext(ctx, normalized); err != nil {
			return nil, err
		}
		if err := SyncUserWorkflowAgentTool(ctx, s.toolRegistry, normalized); err != nil {
			return nil, err
		}
		return marshalToolResult(normalized)
	case "delete_workflow":
		var req struct {
			ID string `json:"id"`
		}
		if err := decodeToolInput(input, &req); err != nil {
			return nil, err
		}
		def, err := s.getWorkflowForInvocation(req.ID, invocation.UserID, true)
		if err != nil {
			return nil, err
		}
		if err := s.workflowRegistry.UnregisterContext(ctx, def.ID); err != nil {
			return nil, err
		}
		if err := RemoveUserWorkflowAgentTool(ctx, s.toolRegistry, def.ID); err != nil {
			return nil, err
		}
		return marshalToolResult(map[string]any{"deleted": true, "id": def.ID})
	case "trigger_workflow":
		if s.workflowExecutor == nil {
			return nil, fmt.Errorf("workflow executor unavailable")
		}
		var req struct {
			ID    string          `json:"id"`
			Input json.RawMessage `json:"input"`
		}
		if err := decodeToolInput(input, &req); err != nil {
			return nil, err
		}
		def, err := s.getWorkflowForInvocation(req.ID, invocation.UserID, false)
		if err != nil {
			return nil, err
		}
		if !def.Enabled {
			return nil, fmt.Errorf("workflow %s is disabled", def.ID)
		}
		if len(req.Input) == 0 {
			req.Input = json.RawMessage(`{}`)
		}
		result, err := s.workflowExecutor.Execute(ctx, workflow.ExecuteRequest{
			WorkflowID: def.ID,
			Input:      req.Input,
			Context:    workflow.ExecutionContext{UserID: invocation.UserID, CharacterID: invocation.CharacterID, ConversationID: invocation.ConversationID, InvocationID: invocation.InvocationID},
		})
		if err != nil {
			return nil, err
		}
		return marshalToolResult(result)
	default:
		return nil, fmt.Errorf("workflow admin handler %s not found", name)
	}
}

func (s *agentAdminToolService) getWorkflowForInvocation(id, userID string, requireOwner bool) (workflow.WorkflowDefinition, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return workflow.WorkflowDefinition{}, fmt.Errorf("workflow id is required")
	}
	def, ok := s.workflowRegistry.Get(id)
	if !ok {
		return workflow.WorkflowDefinition{}, fmt.Errorf("workflow %s not found", id)
	}
	if requireOwner {
		if def.Source != "user" || !workflowOwnedBy(def, userID) {
			return workflow.WorkflowDefinition{}, fmt.Errorf("workflow %s is not editable by this user", id)
		}
	} else if !workflowVisibleToInvocation(def, userID) {
		return workflow.WorkflowDefinition{}, fmt.Errorf("workflow %s is not visible to this user", id)
	}
	return def, nil
}

func workflowOwner(def workflow.WorkflowDefinition) string {
	if def.Metadata == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(def.Metadata["ownerUserId"]))
}

func workflowOwnedBy(def workflow.WorkflowDefinition, userID string) bool {
	owner := workflowOwner(def)
	userID = strings.TrimSpace(userID)
	if owner == "" {
		return userID == ""
	}
	return owner == userID
}

func workflowVisibleToInvocation(def workflow.WorkflowDefinition, userID string) bool {
	if def.Source != "user" {
		return true
	}
	return workflowOwnedBy(def, userID)
}

func mergeJSONObjects(dst, patch map[string]any) {
	for key, value := range patch {
		if value == nil {
			delete(dst, key)
			continue
		}
		childPatch, patchOK := value.(map[string]any)
		childDst, dstOK := dst[key].(map[string]any)
		if patchOK && dstOK {
			mergeJSONObjects(childDst, childPatch)
			dst[key] = childDst
			continue
		}
		dst[key] = value
	}
}

func evaluateCalculatorExpression(expression string) (float64, error) {
	expression = strings.TrimSpace(expression)
	if expression == "" || len(expression) > 4096 {
		return 0, fmt.Errorf("calculator expression is required and must be at most 4096 bytes")
	}
	node, err := parser.ParseExpr(expression)
	if err != nil {
		return 0, fmt.Errorf("invalid calculator expression: %w", err)
	}
	return evalCalculatorNode(node, 0)
}

func evalCalculatorNode(node ast.Expr, depth int) (float64, error) {
	if depth > 64 {
		return 0, fmt.Errorf("calculator expression is too deeply nested")
	}
	switch n := node.(type) {
	case *ast.BasicLit:
		if n.Kind != token.INT && n.Kind != token.FLOAT {
			return 0, fmt.Errorf("unsupported calculator literal")
		}
		v, err := strconv.ParseFloat(n.Value, 64)
		if err != nil {
			return 0, err
		}
		return v, nil
	case *ast.ParenExpr:
		return evalCalculatorNode(n.X, depth+1)
	case *ast.UnaryExpr:
		v, err := evalCalculatorNode(n.X, depth+1)
		if err != nil {
			return 0, err
		}
		switch n.Op {
		case token.ADD:
			return v, nil
		case token.SUB:
			return -v, nil
		}
		return 0, fmt.Errorf("unsupported unary operator %s", n.Op)
	case *ast.BinaryExpr:
		left, err := evalCalculatorNode(n.X, depth+1)
		if err != nil {
			return 0, err
		}
		right, err := evalCalculatorNode(n.Y, depth+1)
		if err != nil {
			return 0, err
		}
		var result float64
		switch n.Op {
		case token.ADD:
			result = left + right
		case token.SUB:
			result = left - right
		case token.MUL:
			result = left * right
		case token.QUO:
			if right == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			result = left / right
		case token.REM:
			if right == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			result = math.Mod(left, right)
		case token.XOR:
			result = math.Pow(left, right)
		default:
			return 0, fmt.Errorf("unsupported binary operator %s", n.Op)
		}
		if math.IsNaN(result) || math.IsInf(result, 0) {
			return 0, fmt.Errorf("calculator result is not finite")
		}
		return result, nil
	case *ast.Ident:
		switch strings.ToLower(n.Name) {
		case "pi":
			return math.Pi, nil
		case "e":
			return math.E, nil
		}
		return 0, fmt.Errorf("unknown calculator identifier %q", n.Name)
	case *ast.CallExpr:
		ident, ok := n.Fun.(*ast.Ident)
		if !ok {
			return 0, fmt.Errorf("unsupported calculator function")
		}
		args := make([]float64, len(n.Args))
		for i := range n.Args {
			v, err := evalCalculatorNode(n.Args[i], depth+1)
			if err != nil {
				return 0, err
			}
			args[i] = v
		}
		return callCalculatorFunction(strings.ToLower(ident.Name), args)
	default:
		return 0, fmt.Errorf("unsupported calculator expression")
	}
}

func callCalculatorFunction(name string, args []float64) (float64, error) {
	one := func(fn func(float64) float64) (float64, error) {
		if len(args) != 1 {
			return 0, fmt.Errorf("%s expects one argument", name)
		}
		v := fn(args[0])
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, fmt.Errorf("%s produced a non-finite result", name)
		}
		return v, nil
	}
	two := func(fn func(float64, float64) float64) (float64, error) {
		if len(args) != 2 {
			return 0, fmt.Errorf("%s expects two arguments", name)
		}
		v := fn(args[0], args[1])
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, fmt.Errorf("%s produced a non-finite result", name)
		}
		return v, nil
	}
	switch name {
	case "abs":
		return one(math.Abs)
	case "sqrt":
		return one(math.Sqrt)
	case "sin":
		return one(math.Sin)
	case "cos":
		return one(math.Cos)
	case "tan":
		return one(math.Tan)
	case "asin":
		return one(math.Asin)
	case "acos":
		return one(math.Acos)
	case "atan":
		return one(math.Atan)
	case "log", "ln":
		return one(math.Log)
	case "log10":
		return one(math.Log10)
	case "exp":
		return one(math.Exp)
	case "floor":
		return one(math.Floor)
	case "ceil":
		return one(math.Ceil)
	case "round":
		return one(math.Round)
	case "pow":
		return two(math.Pow)
	case "min":
		return two(math.Min)
	case "max":
		return two(math.Max)
	case "atan2":
		return two(math.Atan2)
	default:
		return 0, fmt.Errorf("unknown calculator function %q", name)
	}
}

func decodeToolInput(raw json.RawMessage, out any) error {
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("invalid tool input: %w", err)
	}
	return nil
}

func marshalToolResult(value any) (json.RawMessage, error) {
	out, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return out, nil
}

type agentAdminToolSpec struct {
	name, description, input string
	risk                     capability.RiskLevel
	side                     capability.SideEffectLevel
	approval                 bool
	timeout                  time.Duration
}

func registerAgentAdminTools(ctx context.Context, registry *capability.ToolRegistry, service *agentAdminToolService) error {
	if registry == nil || service == nil {
		return nil
	}
	obj := `{"type":"object","additionalProperties":true}`
	specs := []agentAdminToolSpec{
		{name: "calculate", description: "Evaluate a bounded arithmetic expression with common math functions without invoking a shell.", input: `{"type":"object","required":["expression"],"additionalProperties":false,"properties":{"expression":{"type":"string","minLength":1,"maxLength":4096}}}`, risk: capability.RiskLow, side: capability.SideEffectReadOnly, timeout: 3 * time.Second},
		{name: "sleep", description: "Wait for a bounded duration while remaining cancellation-aware.", input: `{"type":"object","additionalProperties":false,"properties":{"duration_ms":{"type":"integer","minimum":0,"maximum":300000},"milliseconds":{"type":"integer","minimum":0,"maximum":300000}}}`, risk: capability.RiskLow, side: capability.SideEffectReadOnly, timeout: 5 * time.Minute},
		{name: "get_all_workflows", description: "List workflows visible to the current user, including system workflows and only that user's private workflows.", input: `{"type":"object","additionalProperties":false,"properties":{}}`, risk: capability.RiskLow, side: capability.SideEffectReadOnly, timeout: 5 * time.Second},
		{name: "get_workflow", description: "Get one visible workflow definition by id.", input: `{"type":"object","required":["id"],"additionalProperties":false,"properties":{"id":{"type":"string"}}}`, risk: capability.RiskLow, side: capability.SideEffectReadOnly, timeout: 5 * time.Second},
		{name: "create_workflow", description: "Create and persist a user-owned workflow definition. Agent-callable workflows are immediately synchronized into the model tool registry.", input: `{"type":"object","required":["definition"],"additionalProperties":false,"properties":{"definition":{"type":"object"}}}`, risk: capability.RiskHigh, side: capability.SideEffectSystem, approval: true, timeout: 10 * time.Second},
		{name: "update_workflow", description: "Replace a user-owned workflow definition while preserving its id and owner.", input: `{"type":"object","required":["id","definition"],"additionalProperties":false,"properties":{"id":{"type":"string"},"definition":{"type":"object"}}}`, risk: capability.RiskHigh, side: capability.SideEffectSystem, approval: true, timeout: 10 * time.Second},
		{name: "patch_workflow", description: "Apply a JSON merge-style patch to a user-owned workflow definition and resynchronize its agent tool exposure.", input: `{"type":"object","required":["id","patch"],"additionalProperties":false,"properties":{"id":{"type":"string"},"patch":{"type":"object"}}}`, risk: capability.RiskHigh, side: capability.SideEffectSystem, approval: true, timeout: 10 * time.Second},
		{name: "delete_workflow", description: "Delete a user-owned workflow and remove its model tool registration.", input: `{"type":"object","required":["id"],"additionalProperties":false,"properties":{"id":{"type":"string"}}}`, risk: capability.RiskHigh, side: capability.SideEffectSystem, approval: true, timeout: 10 * time.Second},
		{name: "trigger_workflow", description: "Run a visible enabled workflow with explicit JSON input under the current invocation scope.", input: `{"type":"object","required":["id"],"additionalProperties":false,"properties":{"id":{"type":"string"},"input":{}}}`, risk: capability.RiskHigh, side: capability.SideEffectExternal, approval: true, timeout: 30 * time.Minute},
	}
	// Application-backed handlers are registered only when the server
	// supplied an implementation, so headless/kernel tests do not advertise dead tools.
	for _, name := range []string{
		"start_chat_service", "create_new_chat", "list_chats", "find_chat", "agent_status", "switch_chat", "update_chat_title", "delete_chat", "send_message_to_ai", "send_message_to_ai_streaming", "list_character_cards", "get_chat_messages", "get_chat_messages_range",
		"list_model_configs", "create_model_config", "update_model_config", "delete_model_config", "activate_model_config", "test_model_config_connection", "get_model_routes", "update_model_routes",
		"list_function_model_configs", "get_function_model_config", "set_function_model_config",
		"list_tts_configs", "create_tts_config", "update_tts_config", "delete_tts_config", "activate_tts_config", "list_asr_configs", "create_asr_config", "update_asr_config", "delete_asr_config", "activate_asr_config",
		"get_speech_services_config", "set_speech_services_config", "list_sandbox_packages", "set_sandbox_package_enabled", "restart_mcp_with_logs",
		"link_memories", "query_memory_links",
	} {
		if service.controller != nil && service.controller.CanExecuteAgentAdminTool(name) {
			specs = append(specs, applicationAdminToolSpec(name))
		}
	}
	for _, spec := range specs {
		if spec.input == "" {
			spec.input = obj
		}
		if spec.timeout <= 0 {
			spec.timeout = 30 * time.Second
		}
		def := capability.ToolDefinition{
			ID: "builtin.agent_admin." + spec.name, ModelName: spec.name, Source: capability.ToolSourceBuiltin,
			Name: spec.name, Description: spec.description, InputSchema: json.RawMessage(spec.input), OutputSchema: json.RawMessage(obj),
			RiskLevel: spec.risk, SideEffect: spec.side, HasSideEffects: spec.side != capability.SideEffectReadOnly && spec.side != capability.SideEffectNone,
			Idempotent: spec.side == capability.SideEffectReadOnly, Retryable: spec.side == capability.SideEffectReadOnly, Enabled: true, Compatible: true, TimeoutMS: spec.timeout.Milliseconds(),
			ToolVersion: capability.ToolVersion{SchemaVersion: 1, Revision: "agent-admin-v1"}, ModelExposure: capability.ModelExposureRule{ExposedByDefault: true, Categories: []string{"system", "management"}, Priority: 45},
			ExecutionPolicy: capability.ToolExecutionPolicy{Timeout: spec.timeout, MaxConcurrency: 2, Idempotent: spec.side == capability.SideEffectReadOnly, ApprovalRequired: spec.approval, AllowBackground: false},
			ResultPolicy:    capability.ToolResultPolicy{SanitizeError: true, MaxOutputBytes: 512 * 1024}, Runtime: capability.RuntimeBinding{RuntimeType: capability.RuntimeTypeBuiltin, RuntimeID: "agent_admin", HandlerName: spec.name},
		}
		if err := registry.Register(ctx, def); err != nil {
			return fmt.Errorf("register agent admin tool %s: %w", spec.name, err)
		}
	}
	return nil
}

func applicationAdminToolSpec(name string) agentAdminToolSpec {
	readOnly := map[string]bool{"start_chat_service": true, "list_chats": true, "find_chat": true, "agent_status": true, "list_character_cards": true, "get_chat_messages": true, "get_chat_messages_range": true, "list_model_configs": true, "test_model_config_connection": true, "get_model_routes": true, "list_function_model_configs": true, "get_function_model_config": true, "list_tts_configs": true, "list_asr_configs": true, "get_speech_services_config": true, "list_sandbox_packages": true, "query_memory_links": true}
	desc := map[string]string{
		"start_chat_service": "Inspect chat service readiness.", "create_new_chat": "Create a conversation for a character.", "list_chats": "List conversations with bounded filters.", "find_chat": "Find conversations by keyword.", "agent_status": "Read chat/agent service status and statistics.", "switch_chat": "Select a conversation as the default for subsequent chat-manager tool calls by this user.", "update_chat_title": "Update a conversation title.", "delete_chat": "Delete a conversation.", "send_message_to_ai": "Send a message through Amitia's normal chat pipeline and return the completed AI response.", "send_message_to_ai_streaming": "Send a message through the normal chat pipeline; returns the completed response plus lines because model tools themselves use bounded non-SSE transport.", "list_character_cards": "List available character cards.", "get_chat_messages": "Read a bounded page of conversation messages.", "get_chat_messages_range": "Read messages in a bounded sequence range.",
		"list_model_configs": "List model configurations with credentials redacted and include function bindings.", "create_model_config": "Create a model configuration.", "update_model_config": "Update a model configuration.", "delete_model_config": "Delete a model configuration.", "activate_model_config": "Activate a model configuration.", "test_model_config_connection": "Test a model endpoint using its saved configuration without exposing the API key.", "get_model_routes": "Read function/model route bindings.", "update_model_routes": "Replace function/model route bindings.",
		"list_function_model_configs": "List function-to-model configuration bindings.", "get_function_model_config": "Read the model configuration bound to one function type.", "set_function_model_config": "Bind a function type to a model configuration.",
		"list_tts_configs": "List TTS configurations with credentials redacted.", "create_tts_config": "Create a TTS configuration.", "update_tts_config": "Update a TTS configuration.", "delete_tts_config": "Delete a TTS configuration.", "activate_tts_config": "Activate a TTS configuration.", "list_asr_configs": "List ASR configurations with credentials redacted.", "create_asr_config": "Create an ASR configuration.", "update_asr_config": "Update an ASR configuration.", "delete_asr_config": "Delete an ASR configuration.", "activate_asr_config": "Activate an ASR configuration.",
		"get_speech_services_config": "Read active TTS/STT service configuration with credentials redacted.", "set_speech_services_config": "Update active TTS/STT service configuration through the existing repositories.",
		"list_sandbox_packages": "List installed Amitia extension packages and their enablement state.", "set_sandbox_package_enabled": "Enable or disable an installed Amitia extension package through the kernel lifecycle.", "restart_mcp_with_logs": "Reconnect all enabled MCP servers and return bounded per-server status/error logs.",
		"link_memories": "Create a typed graph relation between two memory nodes.", "query_memory_links": "Query graph neighbors for a memory node within the current user scope.",
	}[name]
	if readOnly[name] {
		return agentAdminToolSpec{name: name, description: desc, risk: capability.RiskLow, side: capability.SideEffectReadOnly, timeout: 60 * time.Second}
	}
	return agentAdminToolSpec{name: name, description: desc, risk: capability.RiskHigh, side: capability.SideEffectSystem, approval: true, timeout: 60 * time.Second}
}

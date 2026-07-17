package extension

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

const diagnosticPluginID = "dev.amitia.plugin.diagnostic"
const diagnosticSkillID = "dev.amitia.skill.diagnostic.runtime"

type diagnosticPlugin struct {
	mu        sync.RWMutex
	host      PluginHost
	loadedAt  time.Time
	enabledAt time.Time
	events    int64
	schedules int64
	replies   int64
}

func newDiagnosticPlugin() Plugin { return &diagnosticPlugin{} }

func (p *diagnosticPlugin) Manifest() PluginManifest {
	configSchema := json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"diagnosticLabel":{"type":"string","maxLength":80},"accessToken":{"type":"string","maxLength":256}},"required":["diagnosticLabel","accessToken"]}`)
	stateSchema := json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"events":{"type":"integer","minimum":0},"schedules":{"type":"integer","minimum":0},"replies":{"type":"integer","minimum":0}},"required":["events","schedules","replies"]}`)
	surface := json.RawMessage(`{"$schema":"https://schemas.amitia.dev/extensions/v1/surface.schema.json","version":"1.0","title":"运行时诊断","sections":[{"id":"settings","type":"form","title":"诊断设置","fields":[{"key":"diagnosticLabel","label":"诊断标签","component":"text","required":true},{"key":"accessToken","label":"访问凭证","component":"secret"}]},{"id":"health","type":"status","title":"健康状态","source":"plugin-health"},{"id":"runtime-check","type":"action","label":"检查运行时","skill":"dev.amitia.skill.diagnostic.runtime"},{"id":"state","type":"table","title":"状态计数","source":"plugin-state","columns":[{"key":"events","label":"事件","component":"number"},{"key":"schedules","label":"调度","component":"number"},{"key":"replies","label":"回复","component":"number"}]}]}`)
	return PluginManifest{Schema: "https://schemas.amitia.dev/extensions/v1/manifest.schema.json", APIVersion: "extensions.amitia.dev/v1alpha1", Kind: "Plugin", Metadata: ManifestMetadata{ID: diagnosticPluginID, Name: "运行时诊断", Version: "1.0.0", Description: "验证官方 Plugin Runtime 的只读诊断插件", Author: "Amitia", License: "AGPL-3.0-only", Tags: []string{"official", "diagnostic"}}, Compatibility: ManifestCompatibility{EngineMin: "1.0.0", EngineMaxExclusive: "2.0.0"}, Entry: SkillEntry{Kind: "builtin", Name: "diagnostic"}, Capabilities: []string{"runtime.character.read", "storage.own.read", "storage.own.write", "event.own.emit", "scheduler.own.manage", "surface.render"}, Hooks: []PluginHook{HookOnLoad, HookOnEnable, HookBeforePrompt, HookAfterReply, HookOnEvent, HookOnSchedule, HookOnDisable, HookOnUnload}, Subscriptions: []string{"dev.amitia.reply.completed.v1"}, RegisteredSkills: []string{diagnosticSkillID}, Execution: PluginExecution{HookTimeoutMS: 300, MaxConcurrency: 2, FailureThreshold: 3, CircuitOpenMS: 60000, HalfOpenMaxRequest: 1}, ConfigSchema: configSchema, DefaultConfig: json.RawMessage(`{"diagnosticLabel":"Amitia","accessToken":""}`), State: PluginStateManifest{SchemaVersion: "1.0.0", Schema: stateSchema, Default: json.RawMessage(`{"events":0,"schedules":0,"replies":0}`)}, Surface: surface, Enabled: false}
}

func (p *diagnosticPlugin) OnLoad(ctx context.Context, host PluginHost) error {
	p.mu.Lock()
	p.host, p.loadedAt = host, time.Now().UTC()
	p.mu.Unlock()
	definition, handler, err := p.diagnosticSkill()
	if err != nil {
		return err
	}
	return host.RegisterSkill(ctx, definition, handler)
}

func (p *diagnosticPlugin) OnEnable(context.Context) error {
	p.mu.Lock()
	p.enabledAt = time.Now().UTC()
	p.mu.Unlock()
	return nil
}
func (p *diagnosticPlugin) BeforePrompt(context.Context, ExtensionSnapshot) ([]ContextContribution, error) {
	return nil, nil
}
func (p *diagnosticPlugin) AfterReply(context.Context, ExtensionSnapshot, ReplyView) error {
	p.mu.Lock()
	p.replies++
	p.mu.Unlock()
	return nil
}
func (p *diagnosticPlugin) OnEvent(context.Context, ExtensionEvent) error {
	p.mu.Lock()
	p.events++
	p.mu.Unlock()
	return nil
}
func (p *diagnosticPlugin) OnSchedule(context.Context, PluginScheduleInvocation) error {
	p.mu.Lock()
	p.schedules++
	p.mu.Unlock()
	return nil
}
func (p *diagnosticPlugin) OnDisable(context.Context) error { return nil }
func (p *diagnosticPlugin) OnUnload(context.Context) error {
	p.mu.Lock()
	p.host = nil
	p.mu.Unlock()
	return nil
}
func (p *diagnosticPlugin) CurrentVersion() string { return "1.0.0" }
func (p *diagnosticPlugin) Migrate(_ context.Context, fromVersion string, state json.RawMessage) (string, json.RawMessage, error) {
	return "1.0.0", state, nil
}

func (p *diagnosticPlugin) diagnosticSkill() (SkillDefinition, SkillHandler, error) {
	inputSchema := json.RawMessage(`{"type":"object","additionalProperties":false}`)
	outputSchema := json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"pluginId":{"type":"string"},"version":{"type":"string"},"loadedAt":{"type":"string"},"enabledAt":{"type":"string"},"events":{"type":"integer"},"schedules":{"type":"integer"},"replies":{"type":"integer"}},"required":["pluginId","version","loadedAt","enabledAt","events","schedules","replies"]}`)
	manifest := Manifest{Schema: "https://schemas.amitia.dev/extensions/v1/manifest.schema.json", APIVersion: "extensions.amitia.dev/v1alpha1", Kind: "Skill", Metadata: ManifestMetadata{ID: diagnosticSkillID, Name: "Plugin Runtime 诊断", Version: "1.0.0", Description: "返回只读 Plugin Runtime 诊断信息", Author: "Amitia", License: "AGPL-3.0-only"}, Compatibility: ManifestCompatibility{EngineMin: "1.0.0", EngineMaxExclusive: "2.0.0"}, Entry: SkillEntry{Kind: "builtin", Name: "plugin-runtime-diagnostic"}, Capabilities: []string{}, Triggers: []SkillTrigger{TriggerLLM, TriggerManual, TriggerSystemEvent}, Execution: ManifestExecution{TimeoutMS: 500, Idempotent: true}, InputSchema: inputSchema, OutputSchema: outputSchema, Enabled: false, AllowLLM: true, AllowManual: true}
	raw, err := json.Marshal(manifest)
	if err != nil {
		return SkillDefinition{}, nil, err
	}
	definition := SkillDefinition{ID: diagnosticSkillID, ModelName: "plugin_runtime_diagnostic", Name: manifest.Metadata.Name, Description: manifest.Metadata.Description, Version: "1.0.0", Source: SkillSourceBuiltin, Entry: manifest.Entry, InputSchema: inputSchema, OutputSchema: outputSchema, Capabilities: []string{}, Triggers: manifest.Triggers, Timeout: 500 * time.Millisecond, TimeoutMS: 500, Idempotent: true, Enabled: false, Author: "Amitia", License: "AGPL-3.0-only", Manifest: raw}
	handler := func(context.Context, ExecuteSkillRequest) (SkillResult, error) {
		p.mu.RLock()
		payload := map[string]any{"pluginId": diagnosticPluginID, "version": "1.0.0", "loadedAt": p.loadedAt.Format(time.RFC3339Nano), "enabledAt": p.enabledAt.Format(time.RFC3339Nano), "events": p.events, "schedules": p.schedules, "replies": p.replies}
		p.mu.RUnlock()
		output, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			return SkillResult{}, marshalErr
		}
		return SkillResult{Status: RunSucceeded, Output: output, VisibleText: "Plugin Runtime 运行正常"}, nil
	}
	return definition, handler, nil
}

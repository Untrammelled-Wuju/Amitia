//go:build legacy_migration

package extension

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"
)

func (s *PackageService) runPackageWorkflowTests(ctx context.Context, request PreviewPackageImportRequest, parsed parsedExtensionPackage, compiled CompiledWorkflow) PackageDryRunReport {
	started := time.Now()
	report := PackageDryRunReport{Status: "passed", Cases: []PackageDryRunCaseReport{}, Capabilities: append([]string(nil), compiled.Capabilities...), SideEffects: []SideEffectRecord{}}
	if s.workflowInstaller == nil || s.workflowInstaller.executor == nil {
		report.Status = "failed"
		report.FailedCount = 1
		report.Cases = append(report.Cases, PackageDryRunCaseReport{ID: "runtime", Name: "运行时检查", Status: "failed", Error: NewExtensionError(ErrPackageTestFailed, "Workflow Dry Run 运行时不可用", "", false, nil)})
		report.CaseCount = 1
		report.DurationMS = time.Since(started).Milliseconds()
		return report
	}
	testCases := []WorkshopTestCase{}
	if len(parsed.Tests) > 0 {
		if err := json.Unmarshal(parsed.Tests, &testCases); err != nil {
			report.Status = "failed"
			report.FailedCount = 1
			report.Cases = append(report.Cases, PackageDryRunCaseReport{ID: "tests", Name: "测试定义", Status: "failed", Error: NewExtensionError(ErrPackageTestFailed, "测试用例格式无效", fmt.Sprint(err), false, err)})
			report.CaseCount = 1
			report.DurationMS = time.Since(started).Milliseconds()
			return report
		}
	}
	if len(testCases) == 0 {
		testCases = []WorkshopTestCase{{ID: "default-dry-run", Name: "默认 Dry Run", Mode: string(WorkflowDryRun), Input: packageSchemaSample(parsed.Manifest.InputSchema), Config: normalizeJSON(parsed.Manifest.DefaultConfig)}}
	}
	scope := ExecutionScope{UserID: request.UserID, CharacterID: request.ScopeID, Channel: "package-preview", Trigger: TriggerManual, TraceID: "package-preview", RequestID: "package-preview"}
	if request.ScopeType != string(ScopeCharacter) {
		scope.CharacterID = ""
	}
	for index, testCase := range testCases {
		caseStarted := time.Now()
		caseReport := PackageDryRunCaseReport{ID: strings.TrimSpace(testCase.ID), Name: strings.TrimSpace(testCase.Name), Mode: testCase.Mode, Status: "passed", Assertions: []AssertionResult{}, Steps: []WorkflowStepResult{}}
		if caseReport.ID == "" {
			caseReport.ID = fmt.Sprintf("case-%d", index+1)
		}
		if caseReport.Name == "" {
			caseReport.Name = caseReport.ID
		}
		mode := WorkflowExecutionMode(testCase.Mode)
		if mode != WorkflowDryRun && mode != WorkflowMocked {
			mode = WorkflowMocked
		}
		caseReport.Mode = string(mode)
		input := normalizeJSON(testCase.Input)
		config := normalizeJSON(testCase.Config)
		if len(testCase.Input) == 0 {
			input = packageSchemaSample(parsed.Manifest.InputSchema)
		}
		if len(testCase.Config) == 0 {
			config = normalizeJSON(parsed.Manifest.DefaultConfig)
		}
		if err := s.validator.Validate("package-test-input", parsed.Manifest.InputSchema, input); err != nil {
			caseReport.Status = "failed"
			caseReport.Error = NewExtensionError(ErrPackageTestFailed, "测试输入不符合 Schema", err.Error(), false, err)
		} else if len(parsed.Manifest.ConfigSchema) > 0 {
			if err := s.validator.Validate("package-test-config", parsed.Manifest.ConfigSchema, config); err != nil {
				caseReport.Status = "failed"
				caseReport.Error = NewExtensionError(ErrPackageTestFailed, "测试配置不符合 Schema", err.Error(), false, err)
			}
		}
		if caseReport.Error == nil {
			execution, err := s.workflowInstaller.executor.Execute(ctx, WorkflowExecutionRequest{Workflow: compiled, Input: input, Config: config, Scope: scope, Mode: mode, HTTPMocks: testCase.HTTPMocks, SkillMocks: testCase.SkillMocks}, parsed.Manifest.OutputSchema)
			caseReport.Steps = execution.Steps
			caseReport.Output = execution.Output
			report.SideEffects = append(report.SideEffects, execution.SideEffects...)
			if err != nil {
				caseReport.Status = "failed"
				caseReport.Error = asExtensionError(err)
			} else {
				caseReport.Assertions = evaluateAssertions(testCase.Assertions, execution, s.validator)
				if len(testCase.ExpectedOutput) > 0 {
					var expected interface{}
					var actual interface{}
					_ = json.Unmarshal(testCase.ExpectedOutput, &expected)
					_ = json.Unmarshal(execution.Output, &actual)
					passed := reflect.DeepEqual(expected, actual)
					assertion := AssertionResult{Type: "expected_output", Passed: passed}
					if !passed {
						assertion.Message = "输出与 expectedOutput 不一致"
					}
					caseReport.Assertions = append(caseReport.Assertions, assertion)
				}
				for _, assertion := range caseReport.Assertions {
					if !assertion.Passed {
						caseReport.Status = "failed"
					}
				}
			}
		}
		caseReport.DurationMS = time.Since(caseStarted).Milliseconds()
		if caseReport.Status == "passed" {
			report.PassedCount++
		} else {
			report.FailedCount++
			report.Status = "failed"
		}
		report.Cases = append(report.Cases, caseReport)
	}
	report.CaseCount = len(report.Cases)
	report.DurationMS = time.Since(started).Milliseconds()
	return report
}

func packageSchemaSample(schema json.RawMessage) json.RawMessage {
	var root map[string]interface{}
	if json.Unmarshal(normalizeJSON(schema), &root) != nil {
		return json.RawMessage(`{}`)
	}
	value := packageSchemaSampleValue(root)
	raw, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return raw
}

func packageSchemaSampleValue(schema map[string]interface{}) interface{} {
	if value, ok := schema["default"]; ok {
		return value
	}
	if values, ok := schema["enum"].([]interface{}); ok && len(values) > 0 {
		return values[0]
	}
	switch schema["type"] {
	case "object":
		result := map[string]interface{}{}
		properties, _ := schema["properties"].(map[string]interface{})
		required, _ := schema["required"].([]interface{})
		for _, item := range required {
			name := fmt.Sprint(item)
			property, _ := properties[name].(map[string]interface{})
			result[name] = packageSchemaSampleValue(property)
		}
		return result
	case "array":
		return []interface{}{}
	case "string":
		return "sample"
	case "integer":
		return 0
	case "number":
		return float64(0)
	case "boolean":
		return false
	default:
		return map[string]interface{}{}
	}
}

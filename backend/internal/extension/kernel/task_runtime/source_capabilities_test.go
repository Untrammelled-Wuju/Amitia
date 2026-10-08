package task_runtime

import (
	"context"
	"encoding/json"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestSourceTaskCapabilitiesDoNotAdvertiseNativePortsAndPreflightRejectsUnsupportedHost(t *testing.T) {
	for _, platform := range []string{"linux", "android", "darwin"} {
		unsupported := sourceTaskCapabilitiesForPlatform(platform)
		if unsupported.IsolatedExecution || !strings.Contains(unsupported.UnavailableReason, platform) || !IsTaskErrorCode(validateSourceTaskCapabilities(unsupported), ErrTaskDependencyUnavailable) {
			t.Fatal("unsupported platform accepted task submission", platform, unsupported)
		}
	}
	capabilities := CurrentSourceTaskCapabilities()
	if capabilities.ExecuteTool || capabilities.EmitEvent {
		t.Fatal("unimplemented task host ports advertised")
	}
	if runtime.GOOS != "windows" {
		if capabilities.IsolatedExecution || !strings.Contains(capabilities.UnavailableReason, "强隔离") {
			t.Fatal("unsupported Linux/Android isolation advertised")
		}
		service, _ := deviceTaskCatalogFixture(t)
		if _, err := service.DescribeInstalledTaskCatalog(t.Context(), "device", DeviceTaskCatalogRequest{}); !IsTaskErrorCode(err, ErrTaskDependencyUnavailable) {
			t.Fatal("unsupported task catalog exposed executable tasks", err)
		}
		if _, err := service.DescribeInstalledTask(t.Context(), "task-00", "device"); !IsTaskErrorCode(err, ErrTaskDependencyUnavailable) {
			t.Fatal("target enqueue preparation accepted unsupported sandbox", err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := validateSourceTaskExecutionAvailable(ctx); err != context.Canceled {
		t.Fatal("cancelled preflight succeeded", err)
	}
}

func TestSourceTaskDeclaredNativeToolRequirementRejectedBeforeEntryAndQueue(t *testing.T) {
	service, store := deviceTaskCatalogFixture(t)
	store.definitions[0].PermissionRequirementStrings = []string{"service.tool.execute"}
	if err := validateSourceTaskDeclaredCapabilities(store.definitions[0]); !IsTaskErrorCode(err, ErrTaskDependencyUnavailable) || !strings.Contains(err.Error(), "工具执行") {
		t.Fatal("unsupported declared tool capability accepted", err)
	}
	if CurrentSourceTaskCapabilities().IsolatedExecution {
		if _, err := service.DescribeInstalledTask(t.Context(), store.definitions[0].TaskID, "device"); !IsTaskErrorCode(err, ErrTaskDependencyUnavailable) {
			t.Fatal("enqueue target preparation accepted native tool requirement", err)
		}
		page, err := service.DescribeInstalledTaskCatalog(t.Context(), "device", DeviceTaskCatalogRequest{})
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range page.Entries {
			if entry.Definition.TaskID == store.definitions[0].TaskID {
				t.Fatal("catalog advertised task requiring unsupported native tool")
			}
		}
	}
}

func TestTaskProcessActualSourceHostNativeCallsAwaitExplicitRejection(t *testing.T) {
	for _, method := range []string{"executeTool", "emitEvent"} {
		t.Run(method, func(t *testing.T) {
			handler := `module.exports=async(input,ctx)=>{try{await ctx.host.` + method + `('fixture',{});return {success:true,output:{falseSuccess:true}};}catch(error){return {success:true,output:{message:error.message,capabilities:ctx.host.capabilities}};}};`
			host := actualSourceTaskHost(t, handler)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			var result json.RawMessage
			if err := host.Start(ctx, json.RawMessage(`{}`), nil, nil, 1, 1, ProcessCallbacks{OnFinished: func(_ string, value json.RawMessage, _, _, _ string) { result = append(json.RawMessage(nil), value...) }}); err != nil {
				t.Fatal(err)
			}
			code, err := host.Wait()
			if err != nil || code != 0 || !strings.Contains(string(result), "未提供已授权") || strings.Contains(string(result), "falseSuccess") {
				t.Fatalf("native %s falsely succeeded or killed host without acknowledged error: %s %d %v", method, result, code, err)
			}
		})
	}
}

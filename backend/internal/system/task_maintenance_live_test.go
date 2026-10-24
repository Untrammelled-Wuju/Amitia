package system

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	kernelsqlite "github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/extension/kernel/script_host"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
	"github.com/u-ai/backend/internal/scriptruntime/nodeenv"
	"github.com/u-ai/backend/internal/timeoutpolicy"
)

type maintenanceNodeResolver struct{ binary string }

func (r maintenanceNodeResolver) Resolve(context.Context) (nodeenv.Environment, error) {
	return nodeenv.Environment{NodeBinary: r.binary}, nil
}

type maintenanceHostResolver struct{ entry string }

func (r maintenanceHostResolver) Resolve(context.Context, script_host.Kind) (script_host.Artifact, error) {
	return script_host.Artifact{Kind: script_host.KindTaskHost, EntryPath: r.entry}, nil
}

func TestTaskMaintenanceRealHTTPQueueSQLiteAndNode(t *testing.T) {
	node := os.Getenv("AMITIA_TEST_NODE")
	if node == "" {
		t.Skip("需要指定项目 Node 运行时")
	}
	restore := timeoutpolicy.Configure(timeoutpolicy.Default())
	defer restore()
	root, err := filepath.Abs("../../../runtime/task-host")
	if err != nil {
		t.Fatal(err)
	}
	hostDir := filepath.Join(t.TempDir(), "host")
	cmd := exec.Command(node, filepath.Join(root, "node_modules/typescript/bin/tsc"), "--project", filepath.Join(root, "tsconfig.json"), "--outDir", hostDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("真实 TaskHost 构建失败: %v %s", err, output)
	}
	if err := os.WriteFile(filepath.Join(hostDir, "package.json"), []byte(`{"type":"commonjs"}`), 0600); err != nil {
		t.Fatal(err)
	}
	entry := filepath.Join(t.TempDir(), "task.cjs")
	if err := os.WriteFile(entry, []byte(`module.exports=async(input,ctx)=>{await ctx.progress.report({current:1,total:2,stage:'running'});await new Promise(resolve=>setTimeout(resolve,800));return {success:true,output:{value:input.value}};};`), 0600); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "kernel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err := kernelsqlite.Migrate(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	repository := kernelsqlite.NewTaskRepository(db)
	cfg := task_runtime.DefaultTaskRuntimeConfig()
	cfg.NodeEnvironmentResolver = maintenanceNodeResolver{node}
	cfg.HostArtifactResolver = maintenanceHostResolver{filepath.Join(hostDir, "index.js")}
	cfg.WorkspaceRoot = t.TempDir()
	cfg.ProcessDiagnostics = os.Stderr
	cfg.EntryResolver = func(ctx context.Context, definition *task_runtime.TaskDefinition) (string, error) {
		return task_runtime.ResolveTaskEntry(ctx, filepath.Dir(entry), definition)
	}
	runtime := task_runtime.NewTaskRuntimeService(repository, cfg)
	svc := newSetupServiceTest(t)
	if err := svc.AttachTaskRuntime(runtime); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(svc, svc.db, nil, nil, nil, nil, nil)
	router := gin.New()
	router.GET("/config", handler.TaskMaintenanceConfig)
	router.PUT("/config", handler.UpdateTaskMaintenanceConfig)
	router.GET("/status", handler.TaskMaintenanceStatus)
	server := httptest.NewServer(router)
	defer server.Close()
	read := func(path string, target any) {
		t.Helper()
		response, err := server.Client().Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if err := json.NewDecoder(response.Body).Decode(target); err != nil {
			t.Fatal(err)
		}
	}
	var initial struct {
		Code int
		Data TaskMaintenanceConfig
	}
	read("/config", &initial)
	request, _ := json.Marshal(TaskMaintenanceUpdate{MaxTasks: 1, Timeout: timeoutpolicy.Settings{Seconds: 90}, ExpectedRevision: initial.Data.Revision, ExpectedTimeoutRevision: initial.Data.TimeoutRevision})
	response, err := server.Client().Do(mustMaintenanceRequest(t, server.URL+"/config", request))
	if err != nil {
		t.Fatal(err)
	}
	var saved struct {
		Code int
		Data TaskMaintenanceConfig
	}
	err = json.NewDecoder(response.Body).Decode(&saved)
	response.Body.Close()
	if err != nil || saved.Code != 200 || saved.Data.MaxTasks != 1 || !saved.Data.Applied {
		t.Fatalf("真实接口未应用策略: %+v %v", saved, err)
	}
	var ids []string
	for index := 0; index < 2; index++ {
		definition := &task_runtime.TaskDefinition{TaskID: fmt.Sprintf("maintenance-%d", index), ExtensionID: "maintenance", ModuleID: "task", ContributionID: fmt.Sprintf("maintenance-%d", index), DefinitionHash: "maintenance-test", Entry: "task.cjs", RuntimeType: string(task_runtime.RuntimeTaskJavaScript)}
		if err := task_runtime.PinTaskEntry(t.Context(), filepath.Dir(entry), definition); err != nil {
			t.Fatal(err)
		}
		if err := runtime.PutTaskDefinition(t.Context(), definition); err != nil {
			t.Fatal(err)
		}
		result, err := runtime.Enqueue(t.Context(), task_runtime.EnqueueTaskRequest{TaskDefinitionID: definition.TaskID, ExtensionID: definition.ExtensionID, ModuleID: definition.ModuleID, Input: json.RawMessage(`{"value":"verified"}`)}, definition)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, result.TaskRunID)
		run, err := repository.GetTaskRun(t.Context(), result.TaskRunID)
		if err != nil || run.DeadlineAt == nil || run.DeadlineAt.Sub(run.CreatedAt) < 89*time.Second || run.DeadlineAt.Sub(run.CreatedAt) > 91*time.Second {
			t.Fatalf("新任务未采用统一超时: %+v %v", run, err)
		}
	}
	runtime.Start(t.Context())
	defer runtime.Shutdown(context.Background())
	sawRunning, sawQueued := false, false
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		var status struct {
			Code int
			Data task_runtime.MaintenanceStatus
		}
		read("/status", &status)
		if status.Code != 200 || status.Data.ActiveCount > 1 || status.Data.LocalProcessCount > 1 {
			t.Fatalf("真实并发限制无效: %+v", status)
		}
		if status.Data.Running {
			sawRunning = true
		}
		completed := 0
		for _, run := range status.Data.Tasks {
			if run.Status == task_runtime.RunStatusQueued {
				sawQueued = true
			}
			if run.Status == task_runtime.RunStatusSucceeded {
				completed++
			}
			if run.Status.IsTerminal() && run.Status != task_runtime.RunStatusSucceeded {
				full, _ := repository.GetTaskRun(t.Context(), run.ID)
				message := ""
				if full.ErrorMessage != nil {
					message = *full.ErrorMessage
				}
				t.Fatalf("真实任务执行失败: %s %s", run.ID, message)
			}
		}
		if completed == 2 {
			if !sawRunning || !sawQueued || status.Data.Running {
				t.Fatalf("未观察到真实排队和完成状态: %+v", status)
			}
			for _, id := range ids {
				result, err := runtime.GetResult(t.Context(), id)
				if err != nil || result == nil || !bytes.Contains(result.ResultJSON, []byte("verified")) {
					t.Fatalf("真实结果缺失: %s %v", id, err)
				}
			}
			t.Log("真实 HTTP 保存、SQLite 队列、项目 Node TaskHost、并发 1、90 秒任务期限、排队和最终结果验证通过")
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	for _, id := range ids {
		run, err := repository.GetTaskRun(t.Context(), id)
		queue, queueErr := repository.GetQueueEntry(t.Context(), id)
		t.Logf("run=%+v err=%v queue=%+v queueErr=%v", run, err, queue, queueErr)
	}
	t.Fatal("真实任务未在时限内完成")
}

func mustMaintenanceRequest(t *testing.T, url string, body []byte) *http.Request {
	t.Helper()
	request, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	return request
}

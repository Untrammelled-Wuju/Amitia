package trusted_service

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/platform/process"
)

func TestRound29ContainedStartActivatesAfterProcessContainment(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX helper fixture")
	}
	cmd := exec.Command("/bin/sh", "-c", "sleep 30")
	process.ConfigureProcess(cmd)
	instance := &ServiceInstance{}
	def := &ServiceRuntimeDefinition{}
	activated := false
	plan := &sandboxLaunchPlan{AfterStart: func(_ context.Context, running *exec.Cmd) error {
		if running.Process == nil || running.Process.Pid <= 0 || instance.PID != running.Process.Pid || instance.managedProc != running {
			return errors.New("sandbox activated before contained startup identity was recorded")
		}
		activated = true
		return nil
	}}
	s := &ProcessSupervisor{}
	if err := s.startContainedService(context.Background(), instance, def, cmd, plan); err != nil {
		t.Fatal(err)
	}
	if !activated {
		t.Fatal("missing sandbox activation")
	}
	if err := process.TerminateProcessTree(instance.PID, process.ProcessTreeHandle(instance.procHandle)); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err == nil {
		t.Fatal("test child unexpectedly exited successfully")
	}
	process.CloseProcessTree(process.ProcessTreeHandle(instance.procHandle))
}

func TestRound29ContainedStartRejectsInvalidInputsBeforeLaunching(t *testing.T) {
	s := &ProcessSupervisor{}
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	if err := s.startContainedService(context.Background(), nil, &ServiceRuntimeDefinition{}, cmd, nil); err == nil || cmd.Process != nil {
		t.Fatalf("invalid instance started process: err=%v process=%v", err, cmd.Process)
	}
	if err := s.startContainedService(context.Background(), &ServiceInstance{}, nil, cmd, nil); err == nil || cmd.Process != nil {
		t.Fatalf("missing definition started process: err=%v process=%v", err, cmd.Process)
	}
}

func TestRound29ContainedStartPreservesActivationFailureForRollback(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX helper fixture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", "sleep 30")
	process.ConfigureProcess(cmd)
	inst := &ServiceInstance{}
	failure := errors.New("network activation rejected")
	plan := &sandboxLaunchPlan{AfterStart: func(context.Context, *exec.Cmd) error { return failure }}
	s := &ProcessSupervisor{}
	err := s.startContainedService(ctx, inst, &ServiceRuntimeDefinition{}, cmd, plan)
	if !errors.Is(err, failure) {
		t.Fatalf("activation failure = %v", err)
	}
	if cmd.Process == nil || inst.PID != cmd.Process.Pid {
		t.Fatal("lost running worker identity needed for rollback")
	}
	_ = process.TerminateProcessTree(inst.PID, process.ProcessTreeHandle(inst.procHandle))
	_ = cmd.Wait()
	process.CloseProcessTree(process.ProcessTreeHandle(inst.procHandle))
}

func TestRound29AllTrustedServiceProtocolsUseSingleContainedStart(t *testing.T) {
	source, err := os.ReadFile("supervisor.go")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parser.ParseFile(token.NewFileSet(), "supervisor.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	methods := map[string]bool{
		"startRPCService":                  false,
		"startPlainService":                false,
		"startManagedStdioProtocolService": false,
	}
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		_, required := methods[fn.Name.Name]
		if !required {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if sel.Sel.Name == "startContainedService" {
				methods[fn.Name.Name] = true
			}
			if sel.Sel.Name == "Start" || sel.Sel.Name == "AttachProcessTreeWithLimits" {

				if id, ok := sel.X.(*ast.Ident); ok && (id.Name == "cmd" || sel.Sel.Name == "AttachProcessTreeWithLimits") {
					t.Errorf("%s invokes old startup API %s", fn.Name.Name, sel.Sel.Name)
				}
			}
			return true
		})
	}
	for name, migrated := range methods {
		if !migrated {
			t.Errorf("%s does not use startContainedService", name)
		}
	}
}

func TestRound29UnsupportedHardLimitsNeverLaunchPlugin(t *testing.T) {
	if runtime.GOOS != "linux" || process.ResourceLimitsSupported().Processes {
		t.Skip("requires Linux host without delegated cgroup pids controller")
	}
	cmd := exec.Command("/bin/sh", "-c", "touch should-not-run")
	process.ConfigureProcess(cmd)
	inst := &ServiceInstance{}
	def := &ServiceRuntimeDefinition{Limits: ServiceResourceLimits{MaxSubprocesses: 31}}
	s := &ProcessSupervisor{}
	err := s.startContainedService(context.Background(), inst, def, cmd, nil)
	if err == nil || cmd.Process != nil || inst.PID != 0 {
		t.Fatalf("unsupported resource limits started worker: err=%v pid=%d", err, inst.PID)
	}
}

package executionjournal_test

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/executionjournal"
	"github.com/u-ai/backend/internal/execution"
	"github.com/u-ai/backend/internal/extension/kernel/plugincontract"
)

const (
	sinkModeEnv   = "AMITIA_RSI_SINK_MODE"
	sinkSocketEnv = "AMITIA_RSI_SINK_SOCKET"
	sinkLogEnv    = "AMITIA_RSI_SINK_LOG"

	sinkModeAck  = "ack"
	sinkModeDrop = "drop"

	sinkReadyMarker = "m04-s08 sink ready"
)

var errSinkAckLost = errors.New("sink 已落盘但未回执")

func TestE2ESinkHelperProcess(t *testing.T) {
	if os.Getenv(sinkModeEnv) != "1" {
		t.Skip("sink helper only runs as a child process")
	}
	socket := os.Getenv(sinkSocketEnv)
	logPath := os.Getenv(sinkLogEnv)
	if socket == "" || logPath == "" {
		fmt.Fprintf(os.Stderr, "sink helper: missing socket or log path\n")
		os.Exit(3)
	}
	_ = os.Remove(socket)
	listener, err := net.Listen("unix", socket)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sink helper: listen %v\n", err)
		os.Exit(3)
	}
	defer listener.Close()
	fmt.Fprintf(os.Stderr, "%s socket=%s\n", sinkReadyMarker, socket)
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		go sinkServe(conn, logPath)
	}
}

func sinkServe(conn net.Conn, logPath string) {
	defer conn.Close()
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return
	}
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) != 2 {
		return
	}
	operation := strings.TrimPrefix(fields[0], "op=")
	mode := strings.TrimPrefix(fields[1], "mode=")
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	if _, err := file.WriteString(operation + "\n"); err != nil {
		file.Close()
		return
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return
	}
	if err := file.Close(); err != nil {
		return
	}
	if mode == sinkModeAck {
		_, _ = conn.Write([]byte("ok\n"))
	}
}

type sinkProcess struct {
	socket string
	log    string
}

func startSink(t *testing.T) sinkProcess {
	t.Helper()
	dir, err := os.MkdirTemp("", "amitia-s08-")
	if err != nil {
		t.Fatalf("创建 sink 短路径目录失败: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	sink := sinkProcess{socket: filepath.Join(dir, "s.sock"), log: filepath.Join(dir, "sink.log")}
	stderrPath := filepath.Join(dir, "sink.stderr")
	stderrFile, err := os.Create(stderrPath)
	if err != nil {
		t.Fatalf("创建 sink stderr 文件失败: %v", err)
	}
	command := exec.Command(os.Args[0], "-test.run=TestE2ESinkHelperProcess")
	command.Env = append(os.Environ(), sinkModeEnv+"=1", sinkSocketEnv+"="+sink.socket, sinkLogEnv+"="+sink.log)
	command.Stderr = stderrFile
	if err := command.Start(); err != nil {
		stderrFile.Close()
		t.Fatalf("启动 sink 子进程失败: %v", err)
	}
	t.Cleanup(func() {
		_ = command.Process.Kill()
		_, _ = command.Process.Wait()
		stderrFile.Close()
	})
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(sink.socket); err == nil {
			return sink
		}
		time.Sleep(10 * time.Millisecond)
	}
	raw, _ := os.ReadFile(stderrPath)
	t.Fatalf("sink 子进程未在超时内监听: %v", strings.TrimSpace(string(raw)))
	return sink
}

func (s sinkProcess) deliver(operation, mode string) error {
	conn, err := net.DialTimeout("unix", s.socket, 10*time.Second)
	if err != nil {
		return fmt.Errorf("连接 sink 失败: %w", err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return fmt.Errorf("设置 sink 超时失败: %w", err)
	}
	if _, err := fmt.Fprintf(conn, "op=%s mode=%s\n", operation, mode); err != nil {
		return fmt.Errorf("写入 sink 失败: %w", err)
	}
	switch mode {
	case sinkModeAck:
		line, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil {
			return fmt.Errorf("读取 sink 回执失败: %w", err)
		}
		if strings.TrimSpace(line) != "ok" {
			return fmt.Errorf("sink 回执非法: %q", strings.TrimSpace(line))
		}
		return nil
	case sinkModeDrop:
		if _, err := io.Copy(io.Discard, conn); err != nil {
			return fmt.Errorf("等待 sink 断开时出错: %w", err)
		}
		return errSinkAckLost
	default:
		return fmt.Errorf("未知 sink 模式: %s", mode)
	}
}

func (s sinkProcess) writes(t *testing.T) (int, int) {
	t.Helper()
	raw, err := os.ReadFile(s.log)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, 0
		}
		t.Fatalf("读取 sink 落盘日志失败: %v", err)
	}
	seen := map[string]struct{}{}
	total := 0
	for _, line := range strings.Split(string(raw), "\n") {
		operation := strings.TrimSpace(line)
		if operation == "" {
			continue
		}
		total++
		seen[operation] = struct{}{}
	}
	return total, len(seen)
}

func e2eLedger(t *testing.T) (string, *executionjournal.InvocationLedger) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "e2e.db")
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("打开 e2e 库失败: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	ledger, err := executionjournal.NewLedger(db)
	if err != nil {
		t.Fatalf("创建 e2e 账本失败: %v", err)
	}
	return path, ledger
}

func e2eScope() executionjournal.LedgerScope {
	return executionjournal.LedgerScope{SpaceID: "space-e2e", RoleID: "role-e2e", DeviceID: "device-e2e", Surface: "desktop"}
}

func e2eTrace(suffix string) execution.InvocationTrace {
	return execution.InvocationTrace{RootExecutionID: "root-e2e", TraceID: "trace-e2e", InvocationID: "inv-e2e-" + suffix}
}

func e2eWriteRequest(t *testing.T, suffix, key string) executionjournal.LedgerRequest {
	t.Helper()
	request := executionjournal.LedgerRequest{
		Scope:           e2eScope(),
		IdempotencyKey:  key,
		Trace:           e2eTrace(suffix),
		CapabilityID:    "host.tool.execute",
		BindingRevision: "rev-e2e",
		PayloadHash:     "payload-e2e",
		SideEffect:      execution.SideEffectClassWrite,
	}
	if err := request.Trace.Validate(); err != nil {
		t.Fatalf("trace 非法: %v", err)
	}
	return request
}

func TestE2ESinkNoDuplicateWrites(t *testing.T) {
	sink := startSink(t)
	_, ledger := e2eLedger(t)
	const operations = 100
	executed := 0
	blocked := 0
	for index := 0; index < operations; index++ {
		operation := fmt.Sprintf("e2e-op-%03d", index)
		request := e2eWriteRequest(t, operation, operation)
		run := func() (json.RawMessage, error) {
			if err := sink.deliver(operation, sinkModeDrop); err != nil {
				return nil, fmt.Errorf("网关在回执前断开: %w", errors.Join(err, context.DeadlineExceeded))
			}
			return json.RawMessage(`{"sent":true}`), nil
		}
		entry, ran, err := ledger.Commit(t.Context(), request, run)
		if err == nil || !ran {
			t.Fatalf("%s 首次提交必须执行并报歧义错误: ran=%v err=%v", operation, ran, err)
		}
		if entry.State != executionjournal.EffectUnknown {
			t.Fatalf("%s 首次提交状态=%s want unknown", operation, entry.State)
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("%s 原始超时链丢失: %v", operation, err)
		}
		executed++
		resend := false
		_, _, replayErr := ledger.Commit(t.Context(), request, func() (json.RawMessage, error) {
			resend = true
			return nil, fmt.Errorf("重放不应执行副作用")
		})
		if replayErr == nil || !errors.Is(replayErr, executionjournal.ErrLedgerEffectUnknown) {
			t.Fatalf("%s 重发必须被 unknown 挡住: err=%v", operation, replayErr)
		}
		if resend {
			t.Fatalf("%s 重发实际执行了副作用", operation)
		}
		blocked++
	}
	writes, unique := sink.writes(t)
	duplicates := writes - unique
	if writes != operations || unique != operations || duplicates != 0 {
		t.Fatalf("sink 落盘不唯一: writes=%d unique=%d", writes, unique)
	}
	fmt.Printf("m04-s08 e2e no-duplicate ops=%d executed=%d blocked=%d sink_writes=%d unique=%d duplicates=%d\n",
		operations, executed, blocked, writes, unique, duplicates)
}

func TestE2ESinkConcurrentSameKey(t *testing.T) {
	sink := startSink(t)
	_, ledger := e2eLedger(t)
	request := e2eWriteRequest(t, "concurrent", "e2e-concurrent-key")
	var executed int32
	workers := 32
	var group sync.WaitGroup
	errs := make([]error, workers)
	for index := 0; index < workers; index++ {
		group.Add(1)
		go func(slot int) {
			defer group.Done()
			_, _, err := ledger.Commit(context.Background(), request, func() (json.RawMessage, error) {
				atomic.AddInt32(&executed, 1)
				if err := sink.deliver("e2e-concurrent", sinkModeAck); err != nil {
					return nil, fmt.Errorf("并发助手写入 sink 失败: %w", err)
				}
				return json.RawMessage(`{"sent":true}`), nil
			})
			errs[slot] = err
		}(index)
	}
	group.Wait()
	for slot, err := range errs {
		if err != nil {
			t.Fatalf("并发调用 %d 失败: %v", slot, err)
		}
	}
	writes, unique := sink.writes(t)
	if executed != 1 || writes != 1 || unique != 1 {
		t.Fatalf("并发同键出现重复副作用: executed=%d writes=%d unique=%d", executed, writes, unique)
	}
	fmt.Printf("m04-s08 e2e concurrent workers=%d executed=%d sink_writes=%d unique=%d\n",
		workers, executed, writes, unique)
}

func TestE2EIrreversibleNoResend(t *testing.T) {
	sink := startSink(t)
	_, ledger := e2eLedger(t)
	request := executionjournal.IrreversibleRequest{
		Scope:        e2eScope(),
		OperationID:  "op-e2e-ack",
		Trace:        e2eTrace("op-e2e-ack"),
		CapabilityID: "im.send",
		ProviderID:   "plugins-im/wechat/main",
		SideEffect:   execution.SideEffectClassExternal,
		PayloadHash:  "payload-e2e-ack",
	}
	dispatch := func() error {
		if err := sink.deliver("op-e2e-ack", sinkModeDrop); err != nil {
			return fmt.Errorf("消息已发出但回执丢失: %w", errors.Join(err, context.DeadlineExceeded))
		}
		return nil
	}
	record, err := ledger.DispatchIrreversible(t.Context(), request, dispatch)
	if err == nil || !errors.Is(err, executionjournal.ErrReconciliationPending) {
		t.Fatalf("丢回执必须进人工核对: err=%v", err)
	}
	if !record.NeedsHumanCheck() {
		t.Fatalf("丢回执状态=%s want needs_reconciliation", record.State)
	}
	_, secondErr := ledger.DispatchIrreversible(t.Context(), request, func() error {
		t.Fatalf("人工核对未结论前不得再次发送")
		return nil
	})
	if secondErr == nil || !errors.Is(secondErr, executionjournal.ErrReconciliationPending) {
		t.Fatalf("二次发送必须被挡住: err=%v", secondErr)
	}
	writes, unique := sink.writes(t)
	if writes != 1 || unique != 1 {
		t.Fatalf("不可逆操作重复外发: writes=%d unique=%d", writes, unique)
	}
	fmt.Printf("m04-s08 e2e irreversible state=%s sink_writes=%d unique=%d blocked=true\n", record.State, writes, unique)
}

func TestE2EDefaultCoreAvailable(t *testing.T) {
	catalog, err := plugincontract.DefaultCoreDescriptorCatalog()
	if err != nil {
		t.Fatalf("默认 Core 目录构建失败: %v", err)
	}
	entry, found := catalog.Find(plugincontract.CoreMemorySearchServiceID)
	if !found {
		t.Fatalf("默认构建缺少 %s", plugincontract.CoreMemorySearchServiceID)
	}
	if entry.ProviderID != "builtin.core/core/memory.search" {
		t.Fatalf("默认 Core ProviderID=%s", entry.ProviderID)
	}
	index := plugincontract.NewCoreImplementationIndex(entry.ProviderID)
	if !index.Has(entry.ProviderID) {
		t.Fatalf("Core 实现索引未认出默认 Core Provider: %s", entry.ProviderID)
	}
	permissions := make([]string, 0, len(entry.Descriptor.Permissions))
	for _, permission := range entry.Descriptor.Permissions {
		permissions = append(permissions, permission.Capability+"/"+permission.Risk)
	}
	if len(permissions) != 1 || permissions[0] != "memory.read/low" {
		t.Fatalf("默认 Core 权限=%v want [memory.read/low]", permissions)
	}
	fmt.Printf("m04-s08 e2e default-core provider=%s catalog=%d permissions=%s available=true\n",
		entry.ProviderID, len(catalog.Entries), strings.Join(permissions, ","))
}

func TestE2ERestartKeepsUnknownBlocked(t *testing.T) {
	sink := startSink(t)
	path := filepath.Join(t.TempDir(), "restart.db")
	open := func() (*sql.DB, *executionjournal.InvocationLedger) {
		db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
		if err != nil {
			t.Fatalf("打开重启库失败: %v", err)
		}
		db.SetMaxOpenConns(1)
		ledger, err := executionjournal.NewLedger(db)
		if err != nil {
			t.Fatalf("创建重启账本失败: %v", err)
		}
		return db, ledger
	}
	db, ledger := open()
	const operations = 20
	requests := make([]executionjournal.LedgerRequest, 0, operations)
	for index := 0; index < operations; index++ {
		operation := fmt.Sprintf("e2e-restart-%02d", index)
		request := e2eWriteRequest(t, operation, operation)
		requests = append(requests, request)
		run := func() (json.RawMessage, error) {
			if err := sink.deliver(operation, sinkModeDrop); err != nil {
				return nil, fmt.Errorf("重启前写入 sink 丢失回执: %w", errors.Join(err, context.DeadlineExceeded))
			}
			return json.RawMessage(`{"sent":true}`), nil
		}
		if _, _, err := ledger.Commit(t.Context(), request, run); err == nil {
			t.Fatalf("%s 首次提交必须报歧义错误", operation)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("关闭重启库失败: %v", err)
	}
	restartedDB, restarted := open()
	t.Cleanup(func() { restartedDB.Close() })
	blocked := 0
	for _, request := range requests {
		resend := false
		_, _, err := restarted.Commit(t.Context(), request, func() (json.RawMessage, error) {
			resend = true
			return nil, fmt.Errorf("重启后重放不应执行副作用")
		})
		if err == nil || !errors.Is(err, executionjournal.ErrLedgerEffectUnknown) || resend {
			t.Fatalf("%s 重启后重发未被挡住: resend=%v err=%v", request.IdempotencyKey, resend, err)
		}
		blocked++
	}
	writes, unique := sink.writes(t)
	if writes != operations || unique != operations {
		t.Fatalf("重启期间出现重复外发: writes=%d unique=%d", writes, unique)
	}
	fmt.Printf("m04-s08 e2e restart ops=%d blocked=%d sink_writes=%d unique=%d\n", operations, blocked, writes, unique)
}

func TestE2ECancelledWriteNoEffect(t *testing.T) {
	sink := startSink(t)
	_, ledger := e2eLedger(t)
	request := e2eWriteRequest(t, "cancelled", "e2e-cancelled")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	executed := false
	_, ran, err := ledger.Commit(ctx, request, func() (json.RawMessage, error) {
		executed = true
		if deliverErr := sink.deliver("e2e-cancelled", sinkModeAck); deliverErr != nil {
			return nil, fmt.Errorf("取消用例写入 sink 失败: %w", deliverErr)
		}
		return json.RawMessage(`{"sent":true}`), nil
	})
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("取消必须报 context.Canceled: err=%v", err)
	}
	if ran && executed {
		t.Fatalf("取消后仍执行了副作用")
	}
	entry, found, lookupErr := ledger.Lookup(t.Context(), e2eScope(), "e2e-cancelled")
	if lookupErr != nil {
		t.Fatalf("取消后查账本失败: %v", lookupErr)
	}
	if found && entry.State == executionjournal.EffectCommitted {
		t.Fatalf("取消后账本不应出现 committed: %+v", entry)
	}
	writes, unique := sink.writes(t)
	if executed || writes != 0 || unique != 0 {
		t.Fatalf("取消用例产生了外部写入: executed=%v writes=%d unique=%d", executed, writes, unique)
	}
	fmt.Printf("m04-s08 e2e cancelled ran=%v rows=%d sink_writes=%d\n", ran, map[bool]int{true: 1, false: 0}[found], writes)
}

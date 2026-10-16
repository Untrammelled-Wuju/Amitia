package workbenchterminal

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func TestTerminalProcessTreeWorker(t *testing.T) {
	switch os.Getenv("AMITIA_TERMINAL_TREE_WORKER") {
	case "child":
		time.Sleep(time.Minute)
	case "parent":
		child := exec.Command(os.Args[0], "-test.run=^TestTerminalProcessTreeWorker$")
		child.Env = append(os.Environ(), "AMITIA_TERMINAL_TREE_WORKER=child")
		if err := child.Start(); err != nil {
			t.Fatal(err)
		}
		fmt.Println(child.Process.Pid)
		_ = child.Wait()
	}
}

func TestWindowsTerminalCloseTerminatesDescendants(t *testing.T) {
	command := exec.Command(os.Args[0], "-test.run=^TestTerminalProcessTreeWorker$")
	command.Env = append(os.Environ(), "AMITIA_TERMINAL_TREE_WORKER=parent")
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = command.Start(); err != nil {
		t.Fatal(err)
	}
	p := &windowsProcess{command: command, stdin: stdin, stdout: stdout}
	defer p.Close()
	pidChannel := make(chan int, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		if scanner.Scan() {
			pid, _ := strconv.Atoi(scanner.Text())
			pidChannel <- pid
		}
	}()
	var pid int
	select {
	case pid = <-pidChannel:
	case <-time.After(10 * time.Second):
		t.Fatal("terminal child did not start")
	}
	if pid <= 0 {
		t.Fatal("invalid child pid")
	}
	handle, err := syscall.OpenProcess(syscall.SYNCHRONIZE|syscall.PROCESS_QUERY_INFORMATION|syscall.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.CloseHandle(handle)
	defer syscall.TerminateProcess(handle, 1)
	if err = p.Close(); err != nil {
		t.Fatal(err)
	}
	if result, err := syscall.WaitForSingleObject(handle, 5000); err != nil || result != syscall.WAIT_OBJECT_0 {
		t.Fatalf("terminal descendant survived close: %d %v", result, err)
	}
	_ = command.Wait()
}

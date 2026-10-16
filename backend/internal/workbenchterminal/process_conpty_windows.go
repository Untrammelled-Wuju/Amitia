//go:build windows

package workbenchterminal

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

const conptyAttribute = uintptr(0x00020016)
const extendedStartupFlag = uint32(0x00080000)

var ErrConPTYUnavailable = errors.New("Windows ConPTY is not available")

var conptyKernel = syscall.NewLazyDLL("kernel32.dll")
var procCreatePseudoConsole = conptyKernel.NewProc("CreatePseudoConsole")
var procResizePseudoConsole = conptyKernel.NewProc("ResizePseudoConsole")
var procClosePseudoConsole = conptyKernel.NewProc("ClosePseudoConsole")
var procInitializeAttribute = conptyKernel.NewProc("InitializeProcThreadAttributeList")
var procUpdateAttribute = conptyKernel.NewProc("UpdateProcThreadAttribute")
var procDeleteAttribute = conptyKernel.NewProc("DeleteProcThreadAttributeList")

type conptyStartup struct {
	syscall.StartupInfo
	Attributes uintptr
}

type conptyProcess struct {
	stdin     *os.File
	stdout    *os.File
	handle    syscall.Handle
	pid       int
	pseudo    uintptr
	closeOnce sync.Once
}

type conptyExit struct{ code uint32 }

func (e conptyExit) Error() string { return fmt.Sprintf("terminal exited with status %d", e.code) }
func (e conptyExit) ExitCode() int { return int(e.code) }

func conptyCoord(rows, cols uint16) uintptr { return uintptr(uint32(cols) | (uint32(rows) << 16)) }

func launchConPTY(cwd string, rows, cols uint16) (process, error) {
	if err := procCreatePseudoConsole.Find(); err != nil {
		return nil, ErrConPTYUnavailable
	}
	if err := procResizePseudoConsole.Find(); err != nil {
		return nil, ErrConPTYUnavailable
	}
	var inputRead, inputWrite, outputRead, outputWrite syscall.Handle
	if err := syscall.CreatePipe(&inputRead, &inputWrite, nil, 0); err != nil {
		return nil, err
	}
	defer syscall.CloseHandle(inputRead)
	if err := syscall.CreatePipe(&outputRead, &outputWrite, nil, 0); err != nil {
		syscall.CloseHandle(inputWrite)
		return nil, err
	}
	defer syscall.CloseHandle(outputWrite)
	var pseudo uintptr
	result, _, _ := procCreatePseudoConsole.Call(conptyCoord(rows, cols), uintptr(inputRead), uintptr(outputWrite), 0, uintptr(unsafe.Pointer(&pseudo)))
	if int32(result) != 0 {
		syscall.CloseHandle(inputWrite)
		syscall.CloseHandle(outputRead)
		return nil, fmt.Errorf("CreatePseudoConsole failed: HRESULT 0x%x", result)
	}
	cleanupPseudo := true
	defer func() {
		if cleanupPseudo {
			procClosePseudoConsole.Call(pseudo)
		}
	}()
	var size uintptr
	procInitializeAttribute.Call(0, 1, 0, uintptr(unsafe.Pointer(&size)))
	if size == 0 {
		syscall.CloseHandle(inputWrite)
		syscall.CloseHandle(outputRead)
		return nil, errors.New("ConPTY attribute list size unavailable")
	}
	attributes := make([]uintptr, (size+unsafe.Sizeof(uintptr(0))-1)/unsafe.Sizeof(uintptr(0)))
	listPtr := uintptr(unsafe.Pointer(&attributes[0]))
	result, _, initErr := procInitializeAttribute.Call(listPtr, 1, 0, uintptr(unsafe.Pointer(&size)))
	if result == 0 {
		syscall.CloseHandle(inputWrite)
		syscall.CloseHandle(outputRead)
		return nil, fmt.Errorf("ConPTY attribute init: %v", initErr)
	}
	defer procDeleteAttribute.Call(listPtr)
	result, _, updateErr := procUpdateAttribute.Call(listPtr, 0, conptyAttribute, uintptr(unsafe.Pointer(&pseudo)), unsafe.Sizeof(pseudo), 0, 0)
	if result == 0 {
		syscall.CloseHandle(inputWrite)
		syscall.CloseHandle(outputRead)
		return nil, fmt.Errorf("ConPTY attribute update: %v", updateErr)
	}
	exe := shellExecutable()
	commandLine, err := syscall.UTF16PtrFromString("\"" + exe + "\"")
	if err != nil {
		syscall.CloseHandle(inputWrite)
		syscall.CloseHandle(outputRead)
		return nil, err
	}
	directory, err := syscall.UTF16PtrFromString(cwd)
	if err != nil {
		syscall.CloseHandle(inputWrite)
		syscall.CloseHandle(outputRead)
		return nil, err
	}
	startup := conptyStartup{StartupInfo: syscall.StartupInfo{Cb: uint32(unsafe.Sizeof(conptyStartup{}))}, Attributes: listPtr}
	var proc syscall.ProcessInformation
	err = syscall.CreateProcess(nil, commandLine, nil, nil, false, extendedStartupFlag, nil, directory, &startup.StartupInfo, &proc)
	runtime.KeepAlive(attributes)
	if err != nil {
		syscall.CloseHandle(inputWrite)
		syscall.CloseHandle(outputRead)
		return nil, err
	}
	syscall.CloseHandle(proc.Thread)
	cleanupPseudo = false
	return &conptyProcess{stdin: os.NewFile(uintptr(inputWrite), "conpty-input"), stdout: os.NewFile(uintptr(outputRead), "conpty-output"), handle: proc.Process, pid: int(proc.ProcessId), pseudo: pseudo}, nil
}
func (p *conptyProcess) Read(b []byte) (int, error)  { return p.stdout.Read(b) }
func (p *conptyProcess) Write(b []byte) (int, error) { return p.stdin.Write(b) }
func (p *conptyProcess) Mode() string                { return "conpty" }
func (p *conptyProcess) Resize(rows, cols uint16) error {
	result, _, _ := procResizePseudoConsole.Call(p.pseudo, conptyCoord(rows, cols))
	if int32(result) != 0 {
		return fmt.Errorf("ResizePseudoConsole failed: HRESULT 0x%x", result)
	}
	return nil
}
func (p *conptyProcess) Wait() error {
	defer syscall.CloseHandle(p.handle)
	result, err := syscall.WaitForSingleObject(p.handle, syscall.INFINITE)
	if err != nil {
		return err
	}
	if result != syscall.WAIT_OBJECT_0 {
		return fmt.Errorf("ConPTY wait: %d", result)
	}
	var code uint32
	if err := syscall.GetExitCodeProcess(p.handle, &code); err != nil {
		return err
	}
	if code != 0 {
		return conptyExit{code: code}
	}
	return nil
}
func (p *conptyProcess) Close() error {
	p.closeOnce.Do(func() {
		terminateTerminalProcessTree(p.pid)
		_ = syscall.TerminateProcess(p.handle, 1)
		_ = p.stdin.Close()
		_ = p.stdout.Close()
		procClosePseudoConsole.Call(p.pseudo)
	})
	return nil
}

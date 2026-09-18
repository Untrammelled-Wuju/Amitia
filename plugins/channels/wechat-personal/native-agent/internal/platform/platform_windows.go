//go:build windows

package platform

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32                   = syscall.NewLazyDLL("user32.dll")
	gdi32                    = syscall.NewLazyDLL("gdi32.dll")
	enumWindows              = user32.NewProc("EnumWindows")
	getWindowThreadProcessID = user32.NewProc("GetWindowThreadProcessId")
	showWindow               = user32.NewProc("ShowWindow")
	getWindowRect            = user32.NewProc("GetWindowRect")
	getWindowDC              = user32.NewProc("GetWindowDC")
	releaseDC                = user32.NewProc("ReleaseDC")
	printWindow              = user32.NewProc("PrintWindow")
	createCompatibleDC       = gdi32.NewProc("CreateCompatibleDC")
	deleteDC                 = gdi32.NewProc("DeleteDC")
	createCompatibleBitmap   = gdi32.NewProc("CreateCompatibleBitmap")
	selectObject             = gdi32.NewProc("SelectObject")
	deleteObject             = gdi32.NewProc("DeleteObject")
	getDIBits                = gdi32.NewProc("GetDIBits")
	bitBlt                   = gdi32.NewProc("BitBlt")
	versionDLL               = syscall.NewLazyDLL("version.dll")
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	openProcess              = kernel32.NewProc("OpenProcess")
	virtualAllocEx           = kernel32.NewProc("VirtualAllocEx")
	virtualFreeEx            = kernel32.NewProc("VirtualFreeEx")
	writeProcessMemory       = kernel32.NewProc("WriteProcessMemory")
	createRemoteThread       = kernel32.NewProc("CreateRemoteThread")
	waitForSingleObject      = kernel32.NewProc("WaitForSingleObject")
	closeHandle              = kernel32.NewProc("CloseHandle")
	getModuleHandleW         = kernel32.NewProc("GetModuleHandleW")
	getProcAddress           = kernel32.NewProc("GetProcAddress")
	getFileVersionInfoSizeW  = versionDLL.NewProc("GetFileVersionInfoSizeW")
	getFileVersionInfoW      = versionDLL.NewProc("GetFileVersionInfoW")
	verQueryValueW           = versionDLL.NewProc("VerQueryValueW")
)

const (
	swHide              = 0
	processCreateThread = 0x0002
	processVMOperation  = 0x0008
	processVMWrite      = 0x0020
	processQueryInfo    = 0x0400
	memCommit           = 0x1000
	memReserve          = 0x2000
	memRelease          = 0x8000
	pageReadWrite       = 0x04
	infiniteWait        = 0xffffffff
	pwRenderFullContent = 2
	dibRGBColors        = 0
	srcCopy             = 0x00CC0020
)

type rect struct {
	Left, Top, Right, Bottom int32
}

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

type rgbQuad struct {
	Blue, Green, Red, Reserved byte
}

type bitmapInfo struct {
	Header bitmapInfoHeader
	Colors [1]rgbQuad
}

type vsFixedFileInfo struct {
	Signature        uint32
	StrucVersion     uint32
	FileVersionMS    uint32
	FileVersionLS    uint32
	ProductVersionMS uint32
	ProductVersionLS uint32
	FileFlagsMask    uint32
	FileFlags        uint32
	FileOS           uint32
	FileType         uint32
	FileSubtype      uint32
	FileDateMS       uint32
	FileDateLS       uint32
}

func candidates() []string {
	out := []string{}
	if v := strings.TrimSpace(os.Getenv("AMITIA_WECHAT_CLIENT_PATH")); v != "" {
		out = append(out, v)
	}
	for _, base := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)"), os.Getenv("LOCALAPPDATA")} {
		if base == "" {
			continue
		}
		out = append(out,
			filepath.Join(base, "Tencent", "Weixin", "Weixin.exe"),
			filepath.Join(base, "Tencent", "WeChat", "WeChat.exe"),
		)
	}
	return out
}

func findPath() string {
	for _, p := range candidates() {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

func findPID() int {
	for _, imageName := range []string{"Weixin.exe", "WeChat.exe"} {
		out, err := exec.Command("tasklist", "/FI", "IMAGENAME eq "+imageName, "/FO", "CSV", "/NH").Output()
		if err != nil {
			continue
		}
		rows, err := csv.NewReader(strings.NewReader(string(out))).ReadAll()
		if err != nil {
			continue
		}
		for _, row := range rows {
			if len(row) > 1 && strings.EqualFold(row[0], imageName) {
				pid, _ := strconv.Atoi(row[1])
				if pid > 0 {
					return pid
				}
			}
		}
	}
	return 0
}

func fileVersion(path string) string {
	if path == "" {
		return ""
	}
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return ""
	}
	size, _, _ := getFileVersionInfoSizeW.Call(uintptr(unsafe.Pointer(p)), 0)
	if size == 0 {
		return ""
	}
	buf := make([]byte, size)
	ok, _, _ := getFileVersionInfoW.Call(uintptr(unsafe.Pointer(p)), 0, size, uintptr(unsafe.Pointer(&buf[0])))
	if ok == 0 {
		return ""
	}
	root, _ := syscall.UTF16PtrFromString("\\")
	var block uintptr
	var blockLen uint32
	ok, _, _ = verQueryValueW.Call(
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(root)),
		uintptr(unsafe.Pointer(&block)),
		uintptr(unsafe.Pointer(&blockLen)),
	)
	if ok == 0 || block == 0 || blockLen < uint32(unsafe.Sizeof(vsFixedFileInfo{})) {
		return ""
	}
	info := (*vsFixedFileInfo)(unsafe.Pointer(block))
	major := info.FileVersionMS >> 16
	minor := info.FileVersionMS & 0xffff
	patch := info.FileVersionLS >> 16
	build := info.FileVersionLS & 0xffff
	return fmt.Sprintf("%d.%d.%d.%d", major, minor, patch, build)
}

func ProbeClient() ClientStatus {
	p := findPath()
	if p == "" {
		return ClientStatus{Message: "official Windows Weixin not found"}
	}
	pid := findPID()
	return ClientStatus{
		Found:          true,
		Running:        pid > 0,
		PID:            pid,
		Path:           p,
		Version:        fileVersion(p),
		Strategy:       "native",
		WindowManaged:  true,
		PreloadCapable: false,
		Message:        "official Windows Weixin detected",
	}
}

func StartClient(opts StartOptions) (ClientStatus, error) {
	status := ProbeClient()
	if !status.Found {
		return status, fmt.Errorf("official Windows Weixin not found")
	}
	if !status.Running {
		cmd := exec.Command(status.Path)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
		if err := cmd.Start(); err != nil {
			return status, err
		}
		_ = cmd.Process.Release()
		for i := 0; i < 30; i++ {
			time.Sleep(150 * time.Millisecond)
			status = ProbeClient()
			if status.Running {
				break
			}
		}
		if !status.Running {
			return status, fmt.Errorf("Windows Weixin process did not become ready")
		}
	}
	if strings.TrimSpace(opts.DriverLibraryPath) != "" {
		if err := attachDriverLibrary(status.PID, opts.DriverLibraryPath); err != nil {
			// Keep official-client UI login usable even when a version driver cannot
			// attach. Message capabilities remain fail-closed in ProbeDriver.
			status.Message = "official Windows Weixin detected; driver attach failed: " + err.Error()
		}
	}
	return status, nil
}

func attachDriverLibrary(pid int, dllPath string) error {
	if pid <= 0 {
		return fmt.Errorf("invalid Weixin pid")
	}
	abs, err := filepath.Abs(strings.TrimSpace(dllPath))
	if err != nil {
		return err
	}
	if !strings.EqualFold(filepath.Ext(abs), ".dll") {
		return fmt.Errorf("driver library is not a DLL")
	}
	if st, err := os.Stat(abs); err != nil || st.IsDir() {
		if err == nil {
			err = fmt.Errorf("driver path is a directory")
		}
		return fmt.Errorf("driver library unavailable: %w", err)
	}
	// If the declared driver is already attached, do not inject it twice.
	if _, attached := hookDriver(ProbeClient()); attached {
		return nil
	}
	rights := uintptr(processCreateThread | processVMOperation | processVMWrite | processQueryInfo)
	process, _, callErr := openProcess.Call(rights, 0, uintptr(pid))
	if process == 0 {
		return fmt.Errorf("OpenProcess failed: %v", callErr)
	}
	defer closeHandle.Call(process)

	wide, err := syscall.UTF16FromString(abs)
	if err != nil {
		return err
	}
	size := uintptr(len(wide) * 2)
	remote, _, callErr := virtualAllocEx.Call(process, 0, size, memCommit|memReserve, pageReadWrite)
	if remote == 0 {
		return fmt.Errorf("VirtualAllocEx failed: %v", callErr)
	}
	defer virtualFreeEx.Call(process, remote, 0, memRelease)
	var written uintptr
	ok, _, callErr := writeProcessMemory.Call(process, remote, uintptr(unsafe.Pointer(&wide[0])), size, uintptr(unsafe.Pointer(&written)))
	if ok == 0 || written != size {
		return fmt.Errorf("WriteProcessMemory failed: %v", callErr)
	}
	kernelName, _ := syscall.UTF16PtrFromString("kernel32.dll")
	kernelModule, _, callErr := getModuleHandleW.Call(uintptr(unsafe.Pointer(kernelName)))
	if kernelModule == 0 {
		return fmt.Errorf("GetModuleHandleW failed: %v", callErr)
	}
	loadLibraryName, _ := syscall.BytePtrFromString("LoadLibraryW")
	loadLibrary, _, callErr := getProcAddress.Call(kernelModule, uintptr(unsafe.Pointer(loadLibraryName)))
	if loadLibrary == 0 {
		return fmt.Errorf("GetProcAddress(LoadLibraryW) failed: %v", callErr)
	}
	thread, _, callErr := createRemoteThread.Call(process, 0, 0, loadLibrary, remote, 0, 0)
	if thread == 0 {
		return fmt.Errorf("CreateRemoteThread failed: %v", callErr)
	}
	defer closeHandle.Call(thread)
	waitForSingleObject.Call(thread, infiniteWait)
	return nil
}

func windowForPID(pid int) uintptr {
	if pid <= 0 {
		return 0
	}
	var found uintptr
	callback := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		var processID uint32
		getWindowThreadProcessID.Call(hwnd, uintptr(unsafe.Pointer(&processID)))
		if int(processID) != pid {
			return 1
		}
		var r rect
		ok, _, _ := getWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
		if ok != 0 && r.Right-r.Left > 120 && r.Bottom-r.Top > 120 {
			found = hwnd
			return 0
		}
		return 1
	})
	_, _, _ = enumWindows.Call(callback, 0)
	return found
}

func windowGeometry(hwnd uintptr) (int, int, error) {
	if hwnd == 0 {
		return 0, 0, fmt.Errorf("Weixin window not found")
	}
	var r rect
	ok, _, err := getWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	if ok == 0 {
		return 0, 0, err
	}
	width := int(r.Right - r.Left)
	height := int(r.Bottom - r.Top)
	if width <= 0 || height <= 0 {
		return width, height, fmt.Errorf("invalid Weixin window geometry")
	}
	return width, height, nil
}

func captureWindowPNG(hwnd uintptr) ([]byte, error) {
	width, height, err := windowGeometry(hwnd)
	if err != nil {
		return nil, err
	}
	windowDC, _, callErr := getWindowDC.Call(hwnd)
	if windowDC == 0 {
		return nil, callErr
	}
	defer releaseDC.Call(hwnd, windowDC)

	memoryDC, _, callErr := createCompatibleDC.Call(windowDC)
	if memoryDC == 0 {
		return nil, callErr
	}
	defer deleteDC.Call(memoryDC)

	bitmap, _, callErr := createCompatibleBitmap.Call(windowDC, uintptr(width), uintptr(height))
	if bitmap == 0 {
		return nil, callErr
	}
	defer deleteObject.Call(bitmap)
	old, _, _ := selectObject.Call(memoryDC, bitmap)
	defer selectObject.Call(memoryDC, old)

	printed, _, _ := printWindow.Call(hwnd, memoryDC, pwRenderFullContent)
	if printed == 0 {
		copied, _, _ := bitBlt.Call(memoryDC, 0, 0, uintptr(width), uintptr(height), windowDC, 0, 0, srcCopy)
		if copied == 0 {
			return nil, fmt.Errorf("PrintWindow/BitBlt failed")
		}
	}

	pixels := make([]byte, width*height*4)
	info := bitmapInfo{Header: bitmapInfoHeader{
		Size:     uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		Width:    int32(width),
		Height:   -int32(height),
		Planes:   1,
		BitCount: 32,
	}}
	rows, _, callErr := getDIBits.Call(memoryDC, bitmap, 0, uintptr(height), uintptr(unsafe.Pointer(&pixels[0])), uintptr(unsafe.Pointer(&info)), dibRGBColors)
	if rows == 0 {
		return nil, callErr
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			src := (y*width + x) * 4
			dst := y*img.Stride + x*4
			img.Pix[dst+0] = pixels[src+2]
			img.Pix[dst+1] = pixels[src+1]
			img.Pix[dst+2] = pixels[src+0]
			img.Pix[dst+3] = 0xff
		}
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func HideClient(pid int) error {
	hwnd := windowForPID(pid)
	if hwnd == 0 {
		return fmt.Errorf("Weixin window not found")
	}
	showWindow.Call(hwnd, swHide)
	return nil
}

func windowsHookPipe(pid int) string {
	return fmt.Sprintf(`\\.\pipe\amitia-wechat-%d`, pid)
}

func hookDriver(client ClientStatus) (DriverStatus, bool) {
	if !client.Running || client.PID <= 0 {
		return DriverStatus{Message: "Weixin client is not running"}, false
	}
	endpoint := windowsHookPipe(client.PID)
	payload, err := callHookDriver(client, DriverRequest{Op: "driver.capabilities"})
	if err != nil {
		return DriverStatus{Available: false, Attached: false, Kind: "amitia-windows-hook", Version: client.Version, Endpoint: endpoint, Message: "verified Windows Hook companion is not attached"}, false
	}
	var response struct {
		OK            bool            `json:"ok"`
		Kind          string          `json:"kind"`
		Version       string          `json:"version"`
		ClientVersion string          `json:"clientVersion"`
		Capabilities  map[string]bool `json:"capabilities"`
		Message       string          `json:"message"`
	}
	if err := json.Unmarshal(payload, &response); err != nil || !response.OK {
		return DriverStatus{Endpoint: endpoint, Message: "Windows Hook companion returned invalid response"}, false
	}
	attached := response.Capabilities["attached"]
	verified := attached && client.Version != "" && response.ClientVersion == client.Version
	if !verified {
		for _, key := range []string{"qr", "loginStatus", "selfProfile", "receiveText", "sendText"} {
			response.Capabilities[key] = false
		}
		if attached {
			response.Message = fmt.Sprintf("driver/client version mismatch: driver=%q client=%q", response.ClientVersion, client.Version)
		}
	}
	return DriverStatus{Available: attached, Attached: attached, Kind: response.Kind, Version: response.Version, ClientVersion: response.ClientVersion, VersionVerified: verified, Endpoint: endpoint, Capabilities: response.Capabilities, Message: response.Message}, attached
}

func probeWindowsUIDriver(client ClientStatus) DriverStatus {
	caps := map[string]bool{
		"attached": true, "qr": false, "loginStatus": false, "selfProfile": false, "receiveText": false, "sendText": false,
	}
	if !client.Running || client.PID <= 0 {
		caps["attached"] = false
		return DriverStatus{Kind: "amitia-windows-ui", Capabilities: caps, Message: "Weixin client is not running"}
	}
	hwnd := windowForPID(client.PID)
	if hwnd == 0 {
		caps["attached"] = false
		return DriverStatus{Kind: "amitia-windows-ui", Capabilities: caps, Message: "Weixin window not found"}
	}
	caps["qr"] = true
	caps["loginStatus"] = true
	return DriverStatus{Available: true, Attached: true, Kind: "amitia-windows-ui", Version: "ui-v1", ClientVersion: client.Version, VersionVerified: true, Endpoint: fmt.Sprintf("hwnd:%x", hwnd), Capabilities: caps, Message: "official Windows Weixin UI driver attached; QR/login are available, message transport requires a verified version adapter"}
}

func ProbeDriver(client ClientStatus) DriverStatus {
	if hook, attached := hookDriver(client); attached {
		useful := hook.Capabilities["qr"] || hook.Capabilities["loginStatus"] || hook.Capabilities["receiveText"] || hook.Capabilities["sendText"]
		if useful {
			return hook
		}
	}
	return probeWindowsUIDriver(client)
}

func windowsUIDriverCall(client ClientStatus, req DriverRequest) (json.RawMessage, error) {
	status := probeWindowsUIDriver(client)
	if !status.Available {
		return nil, fmt.Errorf("Windows UI driver unavailable: %s", status.Message)
	}
	hwnd := windowForPID(client.PID)
	if hwnd == 0 {
		return nil, fmt.Errorf("Weixin window not found")
	}
	switch req.Op {
	case "driver.capabilities":
		return mustJSONWindows(map[string]any{"ok": true, "kind": status.Kind, "version": status.Version, "capabilities": status.Capabilities, "message": status.Message}), nil
	case "login.qr":
		pngBytes, err := captureWindowPNG(hwnd)
		if err != nil {
			return nil, err
		}
		return mustJSONWindows(map[string]any{"imageDataUrl": "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes), "mode": "official-client-window"}), nil
	case "login.status":
		width, height, err := windowGeometry(hwnd)
		if err != nil {
			return nil, err
		}
		logged := width >= 620 && height >= 460
		statusName := "login_ui"
		if logged {
			statusName = "online_ui_heuristic"
		}
		return mustJSONWindows(map[string]any{"logged": logged, "status": statusName, "windowWidth": width, "windowHeight": height, "derived": true}), nil
	case "account.self":
		return mustJSONWindows(map[string]any{"derived": true}), nil
	default:
		return nil, fmt.Errorf("Windows UI driver does not support %s", req.Op)
	}
}

func mustJSONWindows(value any) json.RawMessage {
	data, _ := json.Marshal(value)
	return json.RawMessage(data)
}

func callHookDriver(client ClientStatus, req DriverRequest) (json.RawMessage, error) {
	if !client.Running || client.PID <= 0 {
		return nil, fmt.Errorf("Windows Weixin is not running")
	}
	pipePath := windowsHookPipe(client.PID)
	deadline := time.Now().Add(1200 * time.Millisecond)
	var f *os.File
	var err error
	for time.Now().Before(deadline) {
		f, err = os.OpenFile(pipePath, os.O_RDWR, 0)
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if err := json.NewEncoder(f).Encode(req); err != nil {
		return nil, err
	}
	line, err := bufio.NewReader(f).ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	return json.RawMessage(line), nil
}

func callDriver(client ClientStatus, req DriverRequest) (json.RawMessage, error) {
	if hook, attached := hookDriver(client); attached {
		capability := map[string]string{
			"login.qr":           "qr",
			"login.status":       "loginStatus",
			"account.self":       "selfProfile",
			"events.poll":        "receiveText",
			"messages.send_text": "sendText",
		}[req.Op]
		if req.Op == "driver.capabilities" || (capability != "" && hook.Capabilities[capability]) {
			if data, err := callHookDriver(client, req); err == nil {
				return data, nil
			}
		}
	}
	return windowsUIDriverCall(client, req)
}

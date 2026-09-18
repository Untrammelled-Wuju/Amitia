package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"

	platformpkg "amitia.local/wechat-personal-agent/internal/platform"
)

type request struct {
	ID                string          `json:"id"`
	Op                string          `json:"op"`
	DriverLibraryPath string          `json:"driverLibraryPath,omitempty"`
	DriverOp          string          `json:"driverOp,omitempty"`
	Payload           json.RawMessage `json:"payload,omitempty"`
}

type response struct {
	ID           string                    `json:"id"`
	OK           bool                      `json:"ok"`
	Error        string                    `json:"error,omitempty"`
	Platform     string                    `json:"platform"`
	Architecture string                    `json:"architecture"`
	Client       platformpkg.ClientStatus  `json:"client"`
	Driver       *platformpkg.DriverStatus `json:"driver,omitempty"`
	Data         json.RawMessage           `json:"data,omitempty"`
}

func main() {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 4096), 1<<20)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	for in.Scan() {
		line := strings.TrimSpace(in.Text())
		if line == "" {
			continue
		}
		var req request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			write(out, response{OK: false, Error: "invalid_json", Platform: runtime.GOOS, Architecture: runtime.GOARCH})
			continue
		}
		res := response{ID: req.ID, OK: true, Platform: runtime.GOOS, Architecture: runtime.GOARCH}
		switch req.Op {
		case "probe":
			res.Client = platformpkg.ProbeClient()
			driver := platformpkg.ProbeDriverForClient(res.Client)
			res.Driver = &driver
		case "start":
			client, err := platformpkg.StartClient(platformpkg.StartOptions{DriverLibraryPath: req.DriverLibraryPath})
			res.Client = client
			if err != nil {
				res.OK = false
				res.Error = err.Error()
				break
			}
			driver := platformpkg.ProbeDriverForClient(client)
			res.Driver = &driver
		case "hide":
			client := platformpkg.ProbeClient()
			res.Client = client
			if err := platformpkg.HideClient(client.PID); err != nil {
				res.OK = false
				res.Error = err.Error()
			}
		case "driver.probe":
			client := platformpkg.ProbeClient()
			res.Client = client
			driver := platformpkg.ProbeDriverForClient(client)
			res.Driver = &driver
		case "driver.call":
			if strings.TrimSpace(req.DriverOp) == "" {
				res.OK = false
				res.Error = "driverOp_required"
				break
			}
			client := platformpkg.ProbeClient()
			res.Client = client
			data, err := platformpkg.CallDriver(client, platformpkg.DriverRequest{Op: req.DriverOp, Payload: req.Payload})
			if err != nil {
				res.OK = false
				res.Error = err.Error()
				break
			}
			res.Data = data
		default:
			res.OK = false
			res.Error = fmt.Sprintf("unsupported_op:%s", req.Op)
		}
		write(out, res)
	}
}

func write(w *bufio.Writer, v response) {
	b, _ := json.Marshal(v)
	_, _ = w.Write(append(b, '\n'))
	_ = w.Flush()
}

package ioshostbridge

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const maxMessageBytes = 2 << 20

type Client struct {
	mu         sync.Mutex
	reader     *bufio.Reader
	read       io.ReadCloser
	write      io.WriteCloser
	generation string
	sequence   uint64
	closed     bool
}

type response struct {
	SchemaVersion int             `json:"schemaVersion"`
	ID            string          `json:"id"`
	Generation    string          `json:"generation"`
	Result        json.RawMessage `json:"result"`
	Error         json.RawMessage `json:"error"`
}

func NewClient(read io.ReadCloser, write io.WriteCloser, generation string) (*Client, error) {
	if read == nil || write == nil || len(generation) < 16 || len(generation) > 128 || strings.ContainsAny(generation, "\r\n\x00") {
		return nil, errors.New("iOS 宿主安全通道参数无效")
	}
	return &Client{reader: bufio.NewReaderSize(read, maxMessageBytes+1), read: read, write: write, generation: generation}, nil
}

func (c *Client) closeLocked() {
	if !c.closed {
		c.closed = true
		c.read.Close()
		c.write.Close()
	}
}

func (c *Client) Call(method string, params any, target any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return errors.New("iOS 宿主安全通道已失效")
	}
	switch method {
	case "identity.get", "identity.sign", "secret.get", "secret.set", "secret.delete", "network.privateAddresses":
	default:
		return errors.New("iOS 宿主安全通道方法未授权")
	}
	c.sequence++
	id := strconv.FormatUint(c.sequence, 10)
	payload, err := json.Marshal(struct {
		SchemaVersion int    `json:"schemaVersion"`
		ID            string `json:"id"`
		Generation    string `json:"generation"`
		Method        string `json:"method"`
		Params        any    `json:"params"`
	}{1, id, c.generation, method, params})
	if err != nil || len(payload) > maxMessageBytes {
		return errors.New("iOS 宿主安全通道请求超出限制")
	}
	type outcome struct {
		data []byte
		err  error
	}
	completed := make(chan outcome, 1)
	go func() {
		packet := append(payload, '\n')
		for len(packet) > 0 {
			n, err := c.write.Write(packet)
			if err != nil {
				completed <- outcome{err: err}
				return
			}
			if n == 0 {
				completed <- outcome{err: io.ErrShortWrite}
				return
			}
			packet = packet[n:]
		}
		line, err := c.reader.ReadSlice('\n')
		completed <- outcome{data: append([]byte(nil), line...), err: err}
	}()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	var output outcome
	select {
	case output = <-completed:
	case <-timer.C:
		c.closeLocked()
		return errors.New("iOS 宿主安全通道超时，已拒绝本次操作")
	}
	if output.err != nil || len(output.data) > maxMessageBytes {
		c.closeLocked()
		return errors.New("iOS 宿主安全通道中断")
	}
	var reply response
	if json.Unmarshal(output.data, &reply) != nil || reply.SchemaVersion != 1 || reply.ID != id || reply.Generation != c.generation || len(reply.Result) == 0 && len(reply.Error) == 0 {
		c.closeLocked()
		return errors.New("iOS 宿主安全通道响应无效")
	}
	if len(reply.Error) > 0 && string(reply.Error) != "null" {
		if len(reply.Result) > 0 && string(reply.Result) != "null" {
			c.closeLocked()
			return errors.New("iOS 宿主安全通道响应冲突")
		}
		return errors.New("iOS 宿主拒绝安全存储操作")
	}
	if len(reply.Result) == 0 || string(reply.Result) == "null" || json.Unmarshal(reply.Result, target) != nil {
		return errors.New("iOS 宿主安全通道结果无效")
	}
	return nil
}

var environmentState struct {
	sync.Mutex
	key    string
	client *Client
	err    error
}

func Required() bool {
	return os.Getenv("AMITIA_IOS_HOST_BRIDGE_REQUIRED") == "true" || os.Getenv("AMITIA_RUNTIME_PLATFORM") == "ios" || os.Getenv("AMITIA_PLATFORM") == "ios" || os.Getenv("AMITIA_RUNTIME_MODE") == "ios-ish"
}

func FromEnvironment() (*Client, error) {
	read := os.Getenv("AMITIA_IOS_HOST_BRIDGE_READ_FD")
	write := os.Getenv("AMITIA_IOS_HOST_BRIDGE_WRITE_FD")
	generation := os.Getenv("AMITIA_IOS_HOST_BRIDGE_GENERATION")
	if read == "" && write == "" && generation == "" && !Required() {
		return nil, nil
	}
	key := read + ":" + write + ":" + generation
	environmentState.Lock()
	defer environmentState.Unlock()
	if environmentState.key == key {
		return environmentState.client, environmentState.err
	}
	if environmentState.client != nil {
		return nil, errors.New("iOS 宿主安全通道不能在进程内更换")
	}
	environmentState.key = key
	r, er := strconv.Atoi(read)
	w, ew := strconv.Atoi(write)
	if er != nil || ew != nil || r < 3 || w < 3 || r == w || r > 1048576 || w > 1048576 {
		environmentState.err = errors.New("缺少有效的 iOS 宿主安全通道，禁止使用本机凭证副本")
		return nil, environmentState.err
	}
	rf, wf, err := openDescriptors(r, w)
	if err == nil {
		environmentState.client, err = NewClient(rf, wf, generation)
	}
	environmentState.err = err
	return environmentState.client, err
}

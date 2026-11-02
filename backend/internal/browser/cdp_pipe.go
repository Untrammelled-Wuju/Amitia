package browser

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"sync"
)

type cdpPipeTransport struct {
	reader  io.ReadCloser
	writer  io.WriteCloser
	input   *bufio.Reader
	writeMu sync.Mutex
}

func newCDPPipeTransport(reader io.ReadCloser, writer io.WriteCloser) *cdpPipeTransport {
	transport := &cdpPipeTransport{
		reader: reader,
		writer: writer,
	}
	if reader != nil {
		transport.input = bufio.NewReader(reader)
	}
	return transport
}

func (t *cdpPipeTransport) Close() error {
	var rErr, wErr error
	if t.reader != nil {
		rErr = t.reader.Close()
	}
	if t.writer != nil {
		wErr = t.writer.Close()
	}
	if rErr != nil {
		return rErr
	}
	return wErr
}

func (t *cdpPipeTransport) ReadMessage() ([]byte, error) {
	if t.input == nil {
		return nil, fmt.Errorf("cdp pipe transport: reader closed")
	}
	for {
		message, err := t.input.ReadBytes(0)
		if len(message) > 0 && message[len(message)-1] == 0 {
			message = message[:len(message)-1]
		}
		message = bytes.TrimSpace(message)
		if len(message) > 0 {
			return message, nil
		}
		if err != nil {
			return nil, err
		}
	}
}

func (t *cdpPipeTransport) WriteMessage(data []byte) error {
	if t.writer == nil {
		return fmt.Errorf("cdp pipe transport: writer closed")
	}
	t.writeMu.Lock()
	defer t.writeMu.Unlock()
	if _, err := t.writer.Write(bytes.TrimSpace(data)); err != nil {
		return err
	}
	if _, err := t.writer.Write([]byte{0}); err != nil {
		return err
	}
	if flusher, ok := t.writer.(interface{ Flush() error }); ok {
		return flusher.Flush()
	}
	return nil
}

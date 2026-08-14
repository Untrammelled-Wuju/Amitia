package browser

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"sync"
)

type cdpPipeTransport struct {
	reader   io.ReadCloser
	writer   io.WriteCloser
	writeMu  sync.Mutex
	scanBuf  []byte
}

func newCDPPipeTransport(reader io.ReadCloser, writer io.WriteCloser) *cdpPipeTransport {
	return &cdpPipeTransport{
		reader: reader,
		writer: writer,
	}
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
	if t.reader == nil {
		return nil, fmt.Errorf("cdp pipe transport: reader closed")
	}
	line, err := t.readLine()
	if err != nil {
		return nil, err
	}
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return t.ReadMessage()
	}
	return line, nil
}

func (t *cdpPipeTransport) WriteMessage(data []byte) error {
	if t.writer == nil {
		return fmt.Errorf("cdp pipe transport: writer closed")
	}
	t.writeMu.Lock()
	defer t.writeMu.Unlock()
	if _, err := t.writer.Write(data); err != nil {
		return err
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		if _, err := t.writer.Write([]byte{'\n'}); err != nil {
			return err
		}
	}
	if flusher, ok := t.writer.(interface{ Flush() error }); ok {
		return flusher.Flush()
	}
	return nil
}

func (t *cdpPipeTransport) readLine() ([]byte, error) {
	if t.scanBuf != nil {
		idx := bytes.IndexByte(t.scanBuf, '\n')
		if idx >= 0 {
			line := make([]byte, idx)
			copy(line, t.scanBuf[:idx])
			t.scanBuf = append(t.scanBuf[:0], t.scanBuf[idx+1:]...)
			return line, nil
		}
	}
	scanner := bufio.NewReader(t.reader)
	line, err := scanner.ReadBytes('\n')
	if err != nil {
		if len(t.scanBuf) > 0 {
			result := make([]byte, len(t.scanBuf))
			copy(result, t.scanBuf)
			t.scanBuf = nil
			return result, nil
		}
		return nil, err
	}
	if t.scanBuf != nil {
		line = append(t.scanBuf, line...)
		t.scanBuf = nil
	}
	return line, nil
}

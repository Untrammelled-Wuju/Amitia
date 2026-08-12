// migration-only: temporary compatibility adapter
// remove at step 65 cutover
package mcp

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/mcp/protocol"
)

type SSEParser struct {
	maxMessageBytes int64
	receive         chan<- protocol.Message
}

func NewSSEParser(maxMessageBytes int64, receive chan<- protocol.Message) *SSEParser {
	return &SSEParser{
		maxMessageBytes: maxMessageBytes,
		receive:         receive,
	}
}

func (p *SSEParser) Parse(ctx context.Context, reader io.Reader, onEventID func(string), onRetryDelay func(time.Duration)) error {
	scanner := bufio.NewScanner(reader)
	bufferSize := int(p.maxMessageBytes)
	if bufferSize > 16<<20 {
		bufferSize = 16 << 20
	}
	scanner.Buffer(make([]byte, 64<<10), bufferSize)

	var data strings.Builder
	currentEventID := ""

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			if data.Len() > 0 {
				payload := strings.TrimSuffix(data.String(), "\n")
				message, err := protocol.Decode([]byte(payload), p.maxMessageBytes)
				if err != nil {
					return fmt.Errorf("MCP_REMOTE_PROTOCOL_INVALID: %w", err)
				}
				select {
				case p.receive <- message:
					onEventID(currentEventID)
				case <-ctx.Done():
					return ctx.Err()
				}
				data.Reset()
				currentEventID = ""
			}
			continue
		}

		if strings.HasPrefix(line, "data:") {
			value := strings.TrimPrefix(line, "data:")
			value = strings.TrimPrefix(value, " ")
			data.WriteString(value)
			data.WriteByte('\n')
		} else if strings.HasPrefix(line, "id:") {
			value := strings.TrimPrefix(strings.TrimPrefix(line, "id:"), " ")
			if !strings.ContainsRune(value, '\x00') {
				currentEventID = value
			}
		} else if strings.HasPrefix(line, "retry:") {
			var milliseconds int
			if _, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "retry:"))); err == nil {
				if _, scanErr := fmt.Sscanf(strings.TrimSpace(strings.TrimPrefix(line, "retry:")), "%d", &milliseconds); scanErr == nil && milliseconds >= 100 && milliseconds <= 60000 {
					onRetryDelay(time.Duration(milliseconds) * time.Millisecond)
				}
			}
		} else if strings.HasPrefix(line, ":") {
			continue
		}
	}

	if err := scanner.Err(); err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("MCP_REMOTE_STREAM_FAILED: %w", err)
	}

	if data.Len() > 0 {
		payload := strings.TrimSuffix(data.String(), "\n")
		message, err := protocol.Decode([]byte(payload), p.maxMessageBytes)
		if err != nil {
			return fmt.Errorf("MCP_REMOTE_PROTOCOL_INVALID: %w", err)
		}
		select {
		case p.receive <- message:
			onEventID(currentEventID)
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

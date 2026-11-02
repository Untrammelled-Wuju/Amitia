package browser

import (
	"bytes"
	"io"
	"testing"
)

func TestCDPPipeTransportUsesNULDelimiter(t *testing.T) {
	input := io.NopCloser(bytes.NewReader([]byte("{\"id\":1}\x00{\"id\":2}\x00")))
	output := &closedBuffer{}
	transport := newCDPPipeTransport(input, output)

	first, err := transport.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	second, err := transport.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != `{"id":1}` || string(second) != `{"id":2}` {
		t.Fatalf("unexpected messages: %q %q", first, second)
	}
	if err := transport.WriteMessage([]byte(`{"id":3}`)); err != nil {
		t.Fatal(err)
	}
	if got := output.String(); got != "{\"id\":3}\x00" {
		t.Fatalf("unexpected wire payload: %q", got)
	}
}

type closedBuffer struct {
	bytes.Buffer
}

func (*closedBuffer) Close() error {
	return nil
}

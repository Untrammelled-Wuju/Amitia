package business

import (
	"errors"
	"strings"
	"testing"
)

func TestInboundRequestValidationReportsEachInvalidCause(t *testing.T) {
	cases := []struct {
		name    string
		request Request
		want    string
	}{
		{name: "blank-request-id", request: Request{Message: "hello"}, want: "请求编号为空"},
		{name: "long-request-id", request: Request{RequestID: strings.Repeat("a", 129), Message: "hello"}, want: "请求编号长度 129 超过上限 128"},
		{name: "blank-message", request: Request{RequestID: "request"}, want: "消息内容为空"},
		{name: "long-message", request: Request{RequestID: "request", Message: strings.Repeat("a", 128<<10+1)}, want: "消息内容长度 131073 超过上限 131072"},
	}
	for _, item := range cases {
		request := item.request
		err := validateInboundRequest(&request)
		if !errors.Is(err, ErrInvalidRequest) || !strings.Contains(err.Error(), item.want) {
			t.Fatalf("%s: %v", item.name, err)
		}
	}
}

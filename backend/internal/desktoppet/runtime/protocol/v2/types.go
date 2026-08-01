package v2

import (
	"encoding/json"
)

// RevisionPayload: Durable命令携带revision信息的接口
type RevisionPayload interface {
	GetRevision() int64
}

// SyncDesiredStatePayload 实现 RevisionPayload
func (p SyncDesiredStatePayload) GetRevision() int64 {
	return p.DesiredRevision
}

// marshalJSON: 安全JSON序列化
func marshalJSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

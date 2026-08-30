package protocol

type MessageType string

const (
	MessageTypeHello          MessageType = "hello"
	MessageTypeHelloAck       MessageType = "hello_ack"
	MessageTypeCommand        MessageType = "command"
	MessageTypeCommandAck     MessageType = "command_ack"
	MessageTypeEventAck       MessageType = "event_ack"
	MessageTypeRuntimeInvoke  MessageType = "runtime.invoke"
	MessageTypeRuntimeResult  MessageType = "runtime.result"
	MessageTypeRuntimeError   MessageType = "runtime.error"
	MessageTypeRuntimeEvent   MessageType = "runtime_event"
	MessageTypeRuntimeCancel  MessageType = "runtime.cancel"
	MessageTypeStateSnapshot  MessageType = "state_snapshot"
	MessageTypeError          MessageType = "error"
	MessageTypePing           MessageType = "ping"
	MessageTypePong           MessageType = "pong"
	MessageTypeTaskDispatch   MessageType = "task_dispatch"
	MessageTypeTaskCancel     MessageType = "task_cancel"
	MessageTypeTaskClaim      MessageType = "task_claim"
	MessageTypeTaskComplete   MessageType = "task_complete"
	MessageTypeTaskProgress   MessageType = "task_progress"
	MessageTypeTaskCheckpoint MessageType = "task_checkpoint"
	MessageTypeTaskHeartbeat  MessageType = "task_heartbeat"
	MessageTypeTaskPause      MessageType = "task_pause"
	MessageTypeTaskResume     MessageType = "task_resume"
)

func (t MessageType) String() string {
	return string(t)
}

func (t MessageType) IsValid() bool {
	switch t {
	case MessageTypeHello, MessageTypeHelloAck, MessageTypeCommand,
		MessageTypeCommandAck, MessageTypeEventAck, MessageTypeRuntimeInvoke, MessageTypeRuntimeResult, MessageTypeRuntimeError,
		MessageTypeRuntimeCancel,
		MessageTypeRuntimeEvent, MessageTypeStateSnapshot,
		MessageTypeError, MessageTypePing, MessageTypePong,
		MessageTypeTaskDispatch, MessageTypeTaskCancel,
		MessageTypeTaskClaim, MessageTypeTaskComplete, MessageTypeTaskProgress, MessageTypeTaskCheckpoint,
		MessageTypeTaskHeartbeat, MessageTypeTaskPause, MessageTypeTaskResume:
		return true
	}
	return false
}

func ParseMessageType(raw string) MessageType {
	return MessageType(raw)
}

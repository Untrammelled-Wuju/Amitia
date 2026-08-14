// Package v2 是桌宠 Runtime 现有实现，作为后续通用 Device Runtime Protocol 的 Adapter / migration-source。
// 其 hello/session/heartbeat/command/ack/sequence/resume/reconcile/dedup 等协议语义是未来通用 Device Runtime Protocol 的抽取来源，
// 但本包本身不能继续作为全局 Device 权威。
package v2

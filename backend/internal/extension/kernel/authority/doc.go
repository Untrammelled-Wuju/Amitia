// Package authority 负责 Extension Kernel / Cloud / Device Mesh 的架构权威边界声明。
// 本包不保存业务状态，不替代任何 Registry，只描述各领域的唯一归属。
//
// 一个领域只能有一个 canonical authority。后续 Cloud / Device / Hybrid 能力只能扩展现有
// canonical subsystem，禁止以 CloudDevice*、RemoteTask*、CloudEvent*、CloudPermission*
// 等形式再创建第二套权威系统。
package authority

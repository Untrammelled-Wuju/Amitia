# U-Ai Workbench 第 48 轮：Terminal 跨设备归属与恢复协议收口

## 实际修改

本轮基于第 47 轮累计源码继续开发，不覆盖历史功能，不新增第二套 Terminal 运行时。

- 终端 owner identity 改为固定字段顺序的长度前缀编码，避免以冒号拼接导致的身份字段边界歧义，身份同时包含 Space、Device、Runtime、UI Session、Conversation。
- 可信本机终端接口拒绝缺失 Device ID 的 Actor；现有 Terminal Manager 与生命周期 API 不更改路径与公开响应结构。
- WebSocket 恢复游标改为严格的非负十进制整数，并在一次性连接 Ticket 被消费、Socket 升级之前进行校验；非法输入返回 HTTP 400，不浪费凭据。
- 终端 HTTP 输出与连接凭据响应声明 `Cache-Control: no-store, private`，并拒绝 MIME 嗅探。
- 新增 owner 隔离的实际 Go Manager race 测试、游标边界 Go race 测试，以及源代码接线路由结构检查，并纳入统一集成门禁。

## 验证范围

Go 1.23 标准库隔离执行的 Manager、Cursor、Ticket race 用例验证真实修改的源码逻辑；Node WebSocket 客户端测试校验 Web 端既有兼容协议。工作台相关 Go 源码语法解析不等于完整工程的编译/真实认证接口测试。

## 尚需生产环境验证

- Go 1.26.1 服务完整编译、真实认证及终端 HTTP/WebSocket 集成；
- Windows ConPTY、Electron、Android/iOS Release；
- Device Mesh 多设备终端资源授权和网络分区联调；
- 长任务恢复与浏览器/CDP 真机联动。

完整构建环境缺失不作为研发范围百分比的硬上限，但未运行的生产测试不能计为已验收。

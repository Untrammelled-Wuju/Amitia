# 任务系统单轨接入与旧入口清理

日期：2026-10-21。

## 结论

任务系统此前存在三套用户提交路径：旧 Core 公共入队、Owner 范围设备提交和按部署模式切换的客户端分支。手机 Agent、手机开发者任务页与电脑任务中心均存在不同程度的旧入口、旧定义回退或不一致展示。

现已统一为：

- 用户任务目录以目标设备 Owner 范围为准。
- 用户任务只通过 `POST /api/device-mesh/v1/business/tasks` 提交。
- 设备任务必须先读取目标设备目录并携带 `taskCatalogReference`。
- Core 非设备任务通过 Owner 定义接口读取，并携带完整 `expectedExecutionScope` 提交。
- 列表、详情、进度、结果、检查点与控制接口继续使用 `/api/extensions/tasks/:id`。
- 旧 `POST /api/extensions/tasks` 已删除。

## 行业标准原则

- 单一写入口：同一业务事实只保留一个用户写入口。
- 单一事实源：任务定义由目标设备目录维护，Core 本地定义由当前 Owner 范围接口读取。
- 读取后提交：设备任务先读取目录，再提交带来源与版本引用的请求。
- 乐观并发：提交时携带 Core、模式、权限、Provider、角色及 revision，服务端校验后落库。
- 幂等重试：同一请求内容复用同一 `requestId`，网络重试不会重复执行。
- 历史隔离：缺少正式 Owner 范围的历史任务只读，不再进入执行或控制链。
- 能力自描述：运行记录直接返回 `checkpointSupported` 与 `retrySupported`，客户端不再依赖全局旧定义猜测控制能力。

## 后端修改

- 新增 `POST /api/device-mesh/v1/business/tasks/definitions`，只返回当前角色可用的 Core 非设备定义。
- Owner 提交接口支持 Core 非设备定义，并拒绝未携带目录引用的设备定义。
- 设备目录定义导入、目标解析、权限确认和目录 pin 校验保持在唯一提交入口内。
- 任务列表与详情补充真实检查点和重试能力标记。
- 删除旧 `POST /api/extensions/tasks`、公共执行目标解析和 OpenAPI 声明。

## 客户端修改

- 电脑任务中心只保留一个“执行任务”入口。
- 执行弹窗固定先选择设备和角色，再加载 Owner 定义与目标设备目录。
- 手机 Agent 改为“设备、角色、定义”顺序选择，不再调用全局定义或旧入队。
- 手机开发者任务页删除非 Owner 直接入队回退。
- 删除无引用的 `KernelTaskService` 和客户端旧 `enqueue` 包装。

## 定向验证

- 后端 `internal/extension` 定向测试通过。
- 新 Owner 定义授权测试通过。
- 任务调用者鉴权测试通过。
- `go build ./cmd/server` 通过。
- 前端类型检查通过。
- 前端任务提交与暂停能力测试通过，共 31 项。
- 前端生产构建通过。
- 手机端相关文件静态分析无错误。
- 核心独立构建与桌面运行核心 SHA-256 一致：`da0bc2533dbc54a0ad55b47eed4e398d1807bb7cc300188793bf5e1d7d228da9`。

## 真实运行验证

- 从当前源码重新构建 `desktop/resources/core/AmitiaCore`。
- 完整重启 Electron、前端、Core、Qdrant、SurrealDB、插件宿主与桌面宠物运行时。
- `/readyz` 返回 `ready`，阻断、失败和降级项目均为 0。
- 前端生产资源返回 HTTP 200。
- 旧 `POST /api/extensions/tasks` 返回 HTTP 404。
- 新任务定义接口缺少 Owner 范围时返回 HTTP 400。
- 当前真实库没有可用角色和历史任务，因此有效目录与执行链由后端隔离测试覆盖。

## 验证边界

- 当前主机为 macOS，项目安全门禁禁止在 darwin 执行已验证的设备强隔离任务，真实 TLS 设备执行测试在进入本次任务逻辑前被平台能力检查拦截。
- Flutter 定向测试被 `syntax_highlighter_plus` 的 Rust 原生资产构建阻塞，环境缺少 `rustup`；手机端仅完成静态分析与源码契约测试。
- 本轮未构建 Android Release APK，未执行真机覆盖安装。
- 本轮未提交或推送 Git。

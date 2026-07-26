# Amitia 扩展系统重构第 40 步实施文档

## 第 40 步：实现 WASM Runtime

---

## 一、步骤目标

实现面向纯计算、可移植算法、格式转换和受限数据处理的 WASM Runtime，作为 JavaScript Main Runtime 和 Trusted Service Runtime 之间的强隔离补充。

目标：

```text
WASM Module
→ Integrity/Interface Validation
→ WASM Engine
→ Fuel/Memory Limits
→ Explicit Host Imports
→ Invocation
→ Result Validation
```

---

## 二、适用范围

-文本转换；
-数据解析；
-压缩/解压受控格式；
-图像轻量处理；
-排序、过滤、评分；
-确定性计算；
-安全规则；
-可移植算法；
-第三方小型计算模块。

不适用：

-复杂长期网络；
-桌面控制；
-任意文件；
-大型原生模型；
-长期后台服务；
-依赖 Node 生态的插件。

---

## 三、Engine 选择原则

要求：

-Go 可稳定集成；
-WASI 支持可控；
-Fuel/Instruction Limit；
-Memory Limit；
-跨平台；
-无 JIT 或可配置；
-安全维护；
-可中断；
-Host Function 支持；
-实例生命周期明确。

具体 Engine 需在实现前 ADR 对比，但领域接口不绑定某单一实现。

---

## 四、Runtime Definition

```go
type WASMRuntimeDefinition struct {
    ModulePath      string
    InterfacePath   string
    ABI             string
    WASIVersion     string
    MemoryLimit     int64
    FuelLimit       uint64
    InstancePolicy  string
    AllowedImports  []string
}
```

---

## 五、接口描述

推荐使用 WIT 或 Amitia 固定 ABI。

入口示例：

```text
invoke(input_ptr, input_len) -> result_handle
```

更推荐生成类型化 Binding，避免手写内存协议。

---

## 六、Host Imports

默认无 Host Import。

按需开放：

```text
amitia.log
amitia.time
amitia.random
amitia.resource.read
amitia.storage.get
amitia.storage.cas
amitia.tool.invoke
```

每个 Import 经过 Permission、Scope 和限制。

---

## 七、WASI

默认不开放完整 WASI。

可提供受限：

-无预打开目录；
-无网络；
-无进程；
-受控时钟；
-受控随机；
-标准输出重定向日志。

如需目录，只挂载 Extension 私有临时目录或只读资源。

---

## 八、资源限制

必须支持：

-Linear Memory；
-Fuel；
-调用超时；
-Host Call 数量；
-输出大小；
-递归深度；
-实例数量；
-并发；
-日志。

---

## 九、实例策略

支持：

```text
per_invocation
pooled
singleton
```

默认：

```text
per_invocation
```

纯函数模块可使用 Pool，但需清理内存状态。

---

## 十、确定性

可选 Deterministic Mode：

-固定时钟输入；
-受控随机种子；
-无外部 I/O；
-相同输入相同输出。

适合 Workflow Transform 或安全规则。

---

## 十一、输入输出

使用 JSON、MessagePack 或生成类型 Binding。

第一版可用 JSON，但：

-大小限制；
-Schema；
-编码；
-错误；
-避免超大复制。

---

## 十二、错误

分类：

```text
module_invalid
abi_mismatch
import_denied
memory_limit
fuel_exhausted
timeout
trap
output_invalid
host_call_failed
cancelled
```

Trap 不影响宿主。

---

## 十三、取消

通过：

-Context；
-Fuel；
-Engine Interrupt；
-Deadline。

必须验证实际 Engine 可中断能力。

---

## 十四、Tool Contribution

WASM 可绑定 Tool Entry：

```text
ToolExecutor
→ WASMRuntimeAdapter
→ WASM Instance
```

仍经过 ExecutionSecurityKernel。

---

## 十五、Workflow Transform

WASM 可作为显式受限 Transform Runtime，但不直接嵌入任意 Workflow 表达式。

---

## 十六、Agent Skill

Agent Skill 可依赖 WASM Tool，但 Agent Skill 本体不执行 WASM。

---

## 十七、包结构

```text
modules/wasm/<id>/
├── module.wasm
├── interface.wit
├── schemas/
└── resources/
```

全部完整性绑定。

---

## 十八、验证

安装时静态检查：

-魔数；
-Module 格式；
-Import；
-Export；
-Memory；
-Table；
-Interface；
-禁止不允许 Feature；
-大小；
-编译成本预估。

---

## 十九、缓存

可缓存已编译 Module：

```text
module_hash + engine_version + platform
```

缓存是派生资源，可重建。

---

## 二十、Host API

WASM 不直接使用内部 JSON-RPC；嵌入 Go 时通过 Host Function 调用 Gateway Adapter。

若 Engine 放独立进程，则可复用 RPC。

第一版推荐嵌入专用 Runtime Host 组件，但不允许不可信原生扩展进入 Go；WASM 字节码由 Engine 隔离。

---

## 二十一、存储

WASM Host Import 使用 Storage Broker。

不得直接访问数据库或真实路径。

---

## 二十二、安全

需要关注：

-Engine 漏洞；
-Fuel 绕过；
-Memory Bomb；
-编译 DoS；
-Host Function 重入；
-大输出；
-无限 Host Call；
-缓存污染；
-不安全 WASI。

---

## 二十三、Health 与 Circuit

连续 Trap、超时、Fuel Exhaustion 影响 Contribution Health/Circuit。

单次 Trap 不导致宿主崩溃。

---

## 二十四、调试

开发模式支持：

-模块验证报告；
-Import/Export；
-Fuel；
-Memory；
-Trap；
-Source Map/Debug Info 可选。

生产不保留敏感调试信息。

---

## 二十五、测试要求

覆盖：

-合法模块；
-损坏模块；
-ABI；
-Import 拒绝；
-Memory；
-Fuel；
-超时；
-Cancel；
-Trap；
-大输出；
-Host Call；
-Storage；
-Tool；
-并发；
-Pool 状态泄漏；
-确定性；
-缓存；
-Engine 升级；
-Fuzz。

---

## 二十六、实施任务

1. 评估并选择 WASM Engine。
2. 定义 Runtime 接口。
3. 定义 ABI/WIT。
4. 实现 Module Validator。
5. 实现 Instance 创建。
6. 实现 Memory/Fuel/Timeout。
7. 实现 Host Imports。
8. 接入 Permission/Scope。
9. 接入 Storage Broker。
10. 实现 Tool Adapter。
11. 实现编译缓存。
12. 实现 Health/Circuit。
13. 接入 Lifecycle/Registry/Supervisor。
14. 建立示例模块。
15. 完成安全测试。

---

## 二十七、验收标准

1. WASM Runtime 独立于 JS/Service。
2. 默认无 WASI 权限。
3. Host Import 显式。
4. Memory/Fuel/Timeout 可控。
5. Trap 不影响宿主。
6. Tool 调用经过统一安全内核。
7. Storage 经过 Broker。
8. 编译缓存可重建。
9. Engine 风险有测试。
10. 第 29—40 步包与 Runtime 阶段完成，可进入 UI Contribution 阶段。

---

## 二十八、执行约束

> WASM 的安全性来自受维护 Engine、最小 Host Import 和资源限制，不得因“是 WASM”就自动授予文件、网络、WASI 或宿主对象。

禁止：

-完整 WASI 默认开启；
-无限 Fuel；
-无限 Memory；
-任意 Host Function；
-直接数据库；
-直接文件；
-未验证 Module；
-Trap 逃逸；
-把 WASM 当原生 Service 替代品。

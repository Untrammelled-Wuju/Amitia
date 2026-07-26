# Amitia 扩展系统重构第 31 步实施文档

## 第 31 步：重写 `.amitiax` 解析器与校验器

---

## 一、步骤目标

彻底重写旧 `.amitiax` Parser，使其只接受 Package Security 产出的 Sealed Staging，并按 Manifest v2、包布局、领域不变量、兼容性和安全规则生成不可变 ExtensionDefinition。

目标链路：

```text
Sealed Staging
→ Layout Inspector
→ Manifest Parser
→ JSON Schema Validator
→ Semantic Validator
→ Resource/Entry Inspector
→ Domain Builder
→ Compatibility Validator
→ Integrity Binder
→ Parse Report
```

---

## 二、解析器职责

负责：

-定位 Manifest；
-读取 JSON；
-Schema；
-路径；
-Module；
-Contribution；
-Runtime；
-Dependency；
-Permission；
-Scope；
-Resource；
-Lifecycle；
-Integrity；
-平台；
-语义校验；
-领域构建；
-报告。

不负责：

-安装提交；
-执行；
-授权；
-连接 MCP；
-启动 Runtime；
-写最终 Repository。

---

## 三、核心接口

```go
type AmitiaxParser interface {
    Parse(
        ctx context.Context,
        staging SealedStaging,
    ) AmitiaxParseResult
}
```

结果：

```go
type AmitiaxParseResult struct {
    ManifestDTO       ManifestV2DTO
    Definition        *ExtensionDefinition
    ValidationReport  ValidationReport
    Compatibility     CompatibilityReport
    ResourceInventory ResourceInventory
    EntryInventory    RuntimeEntryInventory
    ParseHash         string
}
```

---

## 四、输入约束

只接受：

```text
SealedStaging ID
```

不接受任意文件路径或 ZIP Stream。

解析前验证：

-Stage 未变；
-Archive Hash；
-Content Tree；
-Security Report；
-过期时间；
-Owner Operation。

---

## 五、解析阶段

1. 校验根目录。
2. 读取 Manifest。
3. 限制 JSON 深度和大小。
4. JSON Schema 校验。
5. 字段规范化。
6. 校验 ID/版本。
7. 校验 Module。
8. 校验 Runtime Entry。
9. 校验 Contribution Spec。
10. 校验 Dependency。
11. 校验 Permission/Scope。
12. 校验 Resource。
13. 校验 Lifecycle Migration。
14. 校验 Integrity 绑定。
15. 校验平台兼容。
16. 构建领域对象。
17. 运行领域不变量。
18. 生成报告。

---

## 六、Schema Validator

必须：

-固定 Schema Version；
-禁止远程 `$ref`；
-禁止无限递归；
-限制节点数量；
-限制字符串长度；
-限制数组；
-限制 Map；
-严格 unknown field；
-错误含 JSON Pointer。

---

## 七、Semantic Validator

检查 Schema 无法表达的规则：

-Extension ID 与 Module ID；
-全局 Contribution ID 唯一；
-Runtime Binding 存在；
-Entry 文件存在；
-类型匹配；
-Tool 模型名称冲突；
-Dependency Target 合法；
-Permission 与 Contribution 一致；
-UI Slot 支持；
-Hook Point 支持；
-MCP Command 安全；
-Migration 版本范围；
-平台入口；
-资源 Owner；
-文件声明完整。

---

## 八、路径解析

所有 Manifest 路径必须通过 SafePathResolver。

要求：

-相对路径；
-规范分隔符；
-无 `..`；
-无绝对路径；
-无链接；
-大小写唯一；
-存在；
-位于对应 Module；
-文件类型匹配。

---

## 九、Runtime Entry 校验

按类型：

### javascript/task

-`.js` 或宿主支持格式；
-位于 dist；
-完整性绑定；
-不含在线依赖要求。

### mcp/service

-平台二进制存在；
-声明可执行；
-命令参数数组；
-许可证存在。

### wasm

-`.wasm`；
-接口声明；
-导入检查。

### static/workflow

-不得声明可执行 Entry。

---

## 十、Contribution 校验

使用类型专用 Validator，不允许巨大 switch 中混入运行逻辑。

每种 Contribution 有：

-JSON Schema；
-Semantic Validator；
-Domain Builder Adapter。

---

## 十一、Permission 校验

检查：

-权限 ID 存在；
-约束字段合法；
-理由存在；
-需求与 Runtime/Contribution 匹配；
-禁止声明内部系统权限；
-禁止 wildcard 全域；
-高风险权限需 Publisher/Runtime 策略支持。

---

## 十二、Dependency 校验

检查：

-目标格式；
-版本范围；
-作用阶段；
-Required；
-本包内部引用；
-循环；
-不存在内部目标；
-平台依赖。

完整外部求解由 Dependency Resolver。

---

## 十三、完整性绑定

比较：

-Manifest 声明；
-Integrity 文件；
-Sealed Staging 内容树；
-Entry Hash；
-Resource Hash；
-Package Hash。

不一致拒绝。

---

## 十四、兼容性

输出：

```text
compatible
compatible_with_warnings
incompatible
unsupported_runtime
unsupported_platform
host_version_mismatch
feature_missing
```

不兼容包可允许只读导入预览，但不得启用。

---

## 十五、Parse Report

包含：

-错误；
-警告；
-未知字段；
-不兼容；
-资源；
-入口；
-权限；
-依赖；
-风险；
-包大小；
-平台；
-签名；
-领域 Hash。

---

## 十六、错误隔离

单一可选 Module 错误是否允许部分安装，由包策略决定。

默认：

-Required Module 错误阻止；
-Optional Module 可隔离，但必须显式声明；
-不得静默丢弃。

---

## 十七、缓存

解析结果可按：

```text
package_hash + parser_version + host_version
```

缓存。

安全报告或 Schema 变化必须失效。

---

## 十八、旧 v1 兼容

旧 v1 通过独立 Legacy Parser：

```text
v1
→ Legacy DTO
→ Migration Adapter
→ v2 Domain Input
```

禁止在 v2 Parser 中塞大量旧字段分支。

---

## 十九、Parser 版本

记录：

```text
parser_version
schema_version
domain_schema_version
```

同一包由不同 Parser 得出不同 Hash 时必须可诊断。

---

## 二十、Fuzz 与恶意输入

必须进行：

-JSON Fuzz；
-路径 Fuzz；
-深度；
-大数字；
-Unicode；
-重复 Key；
-压缩炸弹由包安全层处理；
-Manifest Bomb；
-Schema Bomb；
-引用循环；
-平台文件冲突。

---

## 二十一、实施任务

1. 建立 Parser 接口。
2. 实现 Layout Inspector。
3. 实现严格 JSON Reader。
4. 接入 Manifest v2 Schema。
5. 实现 Semantic Validator。
6. 实现类型专用 Validator。
7. 接入 SafePathResolver。
8. 实现 Runtime Entry Inspector。
9. 实现 Permission/Dependency Validator。
10. 实现 Integrity Binder。
11. 接入 Domain Builder/Validator。
12. 实现 Compatibility Report。
13. 实现 Parse Cache。
14. 保留独立 Legacy Parser。
15. 建立 Fuzz/跨平台测试。
16. 冻结旧 Parser 写入主链。

---

## 二十二、验收标准

1. v2 Parser 只接受 Sealed Staging。
2. 原始 ZIP 不直接解析。
3. Schema 和语义分层。
4. 路径全部安全解析。
5. Runtime Entry 类型匹配。
6. Contribution 使用专用 Validator。
7. Permission/Dependency 可解释。
8. 内容树完整绑定。
9. 输出 ExtensionDefinition。
10. v1 兼容独立。
11. Fuzz 不崩溃。
12. 可进入第 32 步安装事务。

---

## 二十三、执行约束

> Parser 负责把不可信包转换为已验证定义，不负责安装和运行。

禁止：

-解析时执行；
-解析时连接网络；
-解析时写最终目录；
-解析时 Grant；
-解析时注册；
-解析时启动；
-v2 Parser 反向调用旧 Manager。

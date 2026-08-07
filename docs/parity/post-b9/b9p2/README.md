# B9P2 - Parity分母净化与Baseline修订

## 状态：BLOCKED

**Blocker**: `BLOCKED_B9P1_NOT_PASS`

## 原因

B9P2的前置条件第六条明确要求：
```
B9P1 = PASS
```

当前B9P1实际状态为：
```
B9P1 = FAIL_LINUX_ARM64_BUILD
```

B9P1的三平台CGO=0构建全部失败（pre-existing desktoppet/包编译Bug），无法产出合法binary进行启动验证。

## 阻断详情

B9P1中发现的编译Bug（非B9P1引入，而是工作区已存在）：

1. **import_staging_handler.go:157:88** - `undefined: path` （Linux ARM64失败原因）
2. **package_importer.go:455:72** - `undefined: packageformat.VerdictInvalid` （Linux AMD64, Windows AMD64失败原因）
3. **pending.go:179:8** - `w.runtimeID undefined (type *waiter has no field or method runtimeID)` （Linux AMD64失败原因）

## B9P1已完成的验证项

| 验证项 | 状态 |
|--------|------|
| CGO审计（无必需CGO） | PASS |
| Post-B9源码锚点 | FROZEN (AMT-POST-B9-a3a84ec86812) |
| B1-B9历史产物完整性 | 未变化 |
| 源码完整性（4111文件） | 验证前后一致 |
| 业务源码修改 | 0 |
| 三平台CGO=0构建 | 失败 |
| Backend基础启动 | 未测试 |

## 后续步骤

1. **修复3个desktoppet编译bug**
2. **重试B9P1** (重新构建三平台binary并通过启动验证)
3. **B9P1通过后，重试B9P2**

B9P2的Scope净化工作必须在B9P1成功完成后才能进行，因为：
- 需要确认源码在构建验证过程中不会变化
- 需要Post-B9源码锚点作为正确性基准
- B9P2依赖B9P1确认的生产构建能力

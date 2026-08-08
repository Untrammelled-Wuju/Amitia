# B9P5: 状态机与错误模型收口

## 概述

B9P5 确认Amitia现有各领域状态机和错误模型的唯一事实源，将B9状态/错误协议降为跨端映射层，从架构上阻断第二套状态系统和第二套错误中心。

## 执行结果

**状态: PASS**

## 核心发现

### 状态模型盘点
- **24个状态模型**已被扫描和分类
- **14个** CANONICAL_DOMAIN_STATE
- **1个** CANONICAL_DOMAIN_OUTCOME
- **4个** OBSERVABILITY_PROJECTION
- 所有B9状态已完成Protocol Projection映射

### 错误模型盘点
- **33个Domain Error Family**
- **240+个错误码**
- 33个领域存在独立的错误体系
- 所有B9 Protocol Error已完成映射

### 关键决策

1. **Tool Execution与Observability**: observability.ExecutionStatus是权威层，execution.ExecutionStatus是其入口参数子集
2. **Task Runtime**: 有完整的15状态状态机 + 24错误码 + IsRetryableErrorCode/HTTPStatusForErrorCode
3. **Runtime领域**: Extension Runtime、Android Runtime、DesktopPet Runtime属于不同领域，各自维护状态
4. **Permission**: PermissionDecision(策略决定)与B9 PermissionStatus(授权状态)语义不同
5. **Provider/Extension/ModelRequest/Voice**: 当前无统一状态机，B9状态仅为Protocol候选

### 状态重复风险评估
- 7个疑似重复场景全部评估为合法(DISTINCT_LAYER_VALID)
- 0个真正的重复系统
- 所有State Owner唯一

### 映射缺口
- 4个Protocol Error Gap(安全完整性、图像生成、宠物运行时、安全扫描)
- 3个Task状态需要Domain Split(PLANNING/WAITING_TOOL/WAITING_USER)
- 1个ToolExecution状态需要协议扩展(PAUSED)

## 文件清单

- `B9P5_状态机与错误模型收口报告.md` - 完整审计报告
- `canonical_state_inventory.json` - 24个状态模型清单
- `canonical_error_inventory.json` - 33个错误Family清单
- `protocol_state_mapping.json` - 91条B9状态映射
- `protocol_error_mapping.json` - 21条B9错误映射
- `state_error_guard.json` - 14条架构护栏
- `state_machine_ownership.json` - 10个State Owner定义
- `duplicate_state_risk.json` - 7个重复风险评估
- `b9p4_state_error_input.json` - B9P4输入
- `verification.log` - 验证日志

## B9P4输入

本步骤输出已准备好供B9P4整合：
- `b9p4_state_error_input.json`: 包含state/error domain、mappings、conflicts、corrections
- `B9P4_input_manifest.json`: 输入清单

## 结论

B9P5成功完成。Amitia现有领域状态机和错误模型已全面审计和映射，B9协议状态已被降为跨端投影层。可以进入B9P4进行Post-B9 Resolving Layer统一冻结。

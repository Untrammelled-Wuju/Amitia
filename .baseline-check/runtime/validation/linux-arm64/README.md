# Amitia Runtime Linux ARM64 Validation

本目录包含 Amitia Runtime 在标准 Linux ARM64 环境中的完整验收验证工具。

## 目录结构

```
runtime/validation/linux-arm64/
├── validate.py              # 主入口脚本
├── environment.py           # 环境预检
├── errors.py                # 错误码定义
├── package_validator.py     # Package 验证
├── installer.py             # 安装验证
├── runtime_layout.py        # Runtime Layout 验证
├── process_runner.py        # 进程管理
├── process_inspector.py     # 进程检查
├── http_probe.py            # HTTP 探测
├── node_probe.py            # Node 验证
├── qdrant_probe.py          # Qdrant 验证
├── backend_probe.py         # Backend 验证
├── business_probe.py        # 业务探测
├── restart_probe.py         # 重启验证
├── data_probe.py            # 数据验证
├── ownership_probe.py        # Ownership 验证
├── cleanup.py               # 清理工具
├── report.py                # 报告生成
├── test_environment.py      # 环境测试
├── test_package_validator.py# Package 测试
├── test_runtime_layout.py   # Layout 测试
├── test_report.py           # 报告测试
├── README.md                # 本文档
└── .gitignore               # Git 忽略规则
```

## 验证流程

标准验证流程：

1. 环境预检 (`uname -s`, `uname -m`)
2. Package SHA 验证
3. Package Index/Metadata 验证
4. 安装 Runtime Root 到临时目录
5. 验证 Guest Layout
6. 验证 ELF (Backend/Node/Qdrant)
7. 验证版本 (Backend/Node/npm/npx)
8. Node Runtime 准备和探测
9. Plugin Host 和 Task Host 探测
10. Qdrant Profile 矩阵验证 (desktop/mobile-compact/mobile-balanced/mobile-performance)
11. Backend 启动验证
12. Backend Live/Ready 验证
13. Local Token 验证
14. 业务 Probe 验证
15. 正常停止验证
16. 重启验证
17. 数据保持验证
18. Cache 清理验证
19. Run 清理验证
20. 孤儿 Qdrant 恢复验证
21. 外部 Qdrant 保护验证
22. 最终清理验证

## 使用方法

### 完整验收

```bash
python runtime/validation/linux-arm64/validate.py \
  --runtime-package runtime/out/runtime-package/android-arm64/amitia-runtime-<version>-android-arm64.zip \
  --runtime-version <version> \
  --commit <full-commit> \
  --report runtime/out/validation/linux-arm64/linux-arm64-validation-report.json
```

### 运行单元测试

```bash
python -m unittest discover \
  -s runtime/validation/linux-arm64 \
  -p "test_*.py"
```

### 跳过特定测试（仅限开发阶段）

```bash
python runtime/validation/linux-arm64/validate.py \
  --runtime-package ... \
  --runtime-version ... \
  --commit ... \
  --skip-business-probe \
  --skip-restart-probe \
  --skip-profile-matrix
```

## 输出

- JSON 报告: `linux-arm64-validation-report.json`
- 摘要报告: `linux-arm64-validation-summary.txt`

## 环境要求

- OS: Linux
- Architecture: aarch64
- Distribution: Ubuntu 24.04.4
- Python: 3.8+
- 无系统 Go/Toolchain 依赖
- 无系统 Node 依赖
- 无系统 Qdrant 依赖
- 无互联网连接要求

## 禁止事项

- 禁止修改正式 Runtime Package
- 禁止依赖系统 Node/Qdrant/Go
- 禁止按端口/名称杀进程
- 禁止关闭 Local Token 认证
- 禁止使用 `curl`/`wget` 作为 HTTP 探测
- 禁止依赖 Git 源码目录
- 禁止修改生产业务代码绕过错误
- 禁止进入第 41 步 PRoot 验收（除非第 40 步通过）

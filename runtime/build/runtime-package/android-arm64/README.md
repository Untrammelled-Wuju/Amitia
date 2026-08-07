# Runtime Package Builder - android-arm64

组装 Android PRoot Runtime 安装包。

## 依赖组件

| 组件ID | 来源 | 必需 |
|--------|------|------|
| runtime.rootfs | 第18步 | 是 |
| runtime.guest-layout | 第19步 | 是 |
| runtime.backend | 第14步 | 是 |
| runtime.node | 第15步 | 是 |
| runtime.node-scripts | 第16步 | 是 |
| runtime.qdrant | 第17步 | 是 |
| runtime.plugin-host | runtime/plugin-host | 是 |
| runtime.task-host | runtime/task-host | 是 |

## 命令

更新 Lock：
```
python update_lock.py --runtime-version 1.0.0 --commit <full-commit-sha>
```

正式构建：
```
python build.py --runtime-version 1.0.0 --commit <full-commit-sha> --clean --offline
```

验证：
```
python verify.py --mode inputs
python verify.py --mode rootfs --artifact <rootfs.tar.xz>
python verify.py --mode runtime --artifact <runtime.tar.xz>
python verify.py --mode package --artifact <runtime-version>.zip --runtime-version <version> --commit <commit>
```

运行测试：
```
python -m unittest discover -s . -p "test_*.py"
```

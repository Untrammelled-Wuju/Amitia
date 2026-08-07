# Linux ARM64 Node Runtime Scripts

## 固定版本

| 项目     | 值          |
| -------- | ----------- |
| Node.js  | 24.19.0     |
| npm      | 11.17.0     |
| 平台     | linux       |
| 架构     | arm64       |
| 依赖     | Node Runtime 第 15 步产物 |

## 产物目录

```
scripts/node/
  lib/amitia-node-common.sh
  amitia-node-prepare.sh
  amitia-node-probe.sh
  amitia-node-exec.sh
  amitia-npm-exec.sh
  amitia-npx-exec.sh
  amitia-plugin-host.sh
  amitia-task-host.sh
  probe-node-runtime.mjs
node-runtime-scripts.json
file-manifest.json
SHA256SUMS
amitia-node-runtime-scripts-v1-linux-arm64.tar.xz
```

## Runtime Root 推导

```
SCRIPT_DIR = 当前脚本所在目录
RUNTIME_ROOT = SCRIPT_DIR/../..
```

固定入口：

- `<runtime-root>/node/bin/node`
- `<runtime-root>/node/lib/node_modules/npm/bin/npm-cli.js`
- `<runtime-root>/node/lib/node_modules/npm/bin/npx-cli.js`
- `<runtime-root>/plugin-host/dist/index.js`
- `<runtime-root>/task-host/dist/index.js`

## 环境合同

基础：`AMITIA_DATA_ROOT`、`AMITIA_CACHE_ROOT`、`AMITIA_TEMP_ROOT`

派生：

- `AMITIA_NODE_HOME=<DataRoot>/node/home`
- `AMITIA_NODE_PREFIX=<DataRoot>/node/prefix`
- `AMITIA_NPM_CACHE=<CacheRoot>/node/npm`
- `AMITIA_NODE_TMP=<TempRoot>/node`

## 环境变量与工作目录

Plugin Host 额外需要 `AMITIA_PLUGIN_WORKSPACE`
Task Host 额外需要 `AMITIA_TASK_WORKSPACE`

Task Host `cd` 到 `AMITIA_TASK_WORKSPACE`。
Plugin Host `cd` 到 `<runtime-root>/plugin-host`。

## 脚本职责

| 脚本                       | 用途                             |
| -------------------------- | -------------------------------- |
| amitia-node-prepare.sh     | 创建隔离目录、输出环境变量       |
| amitia-node-probe.sh       | Node/npm/平台/架构 JSON 探测     |
| amitia-node-exec.sh        | 普通 JS 入口前台执行             |
| amitia-npm-exec.sh         | 隔离执行 npm-cli.js              |
| amitia-npx-exec.sh         | 隔离执行 npx-cli.js              |
| amitia-plugin-host.sh      | 前台启动 Plugin Host             |
| amitia-task-host.sh        | 前台启动 Task Host               |

## 退出码

0=成功, 2=参数错误, 10=布局错误, 11=Node不存在, 12=npm-cli不存在, 13=npx-cli不存在, 14=PluginHost不存在, 15=TaskHost不存在, 20=环境变量缺失, 21=路径非法, 22=Root缺失, 23=目录创建失败, 30=Node版本不匹配, 31=npm版本不匹配, 32=平台不匹配, 33=架构不匹配, 40=PluginHost启动失败, 41=TaskHost启动失败, 50=内部脚本错误。

## 静态验证

```
python verify.py --mode static
```

ARM64 Runtime 验证：

```
python verify.py --mode runtime --runtime-root <临时组装RuntimeRoot>
```

## 不提交 Git 的文件

- `runtime/build/node-runtime-scripts/linux-arm64/.cache/`
- `runtime/build/node-runtime-scripts/linux-arm64/.work/`
- `runtime/build/node-runtime-scripts/linux-arm64/__pycache__/`
- `runtime/out/node-runtime-scripts/`

## 下游消费

- 第 20 步: Runtime 组装
- 第 27 步: PRoot 启动
- 第 28 步: Bind Mount
- 第 35 步: Plugin/Task 托管
- 第 40 步: Android 集成测试
- 第 41 步: Android PRoot 验证

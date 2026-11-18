# 回归：DSH 客户端模块加载失败（2026-11-18 晚）

## 现象

重启后原生聊天页落到错误边界：「DSH 客户端模块装载失败 2 个 … TypeError: Failed to fetch dynamically imported module: http://127.0.0.1:<port>/api/public/extensions/ui/runtime-module/com.amitia%2Fdsh-compatible-host/dsh-desktop-shell-host/react-dom/root」，DOM `textareas=0 / slots=0`。

## 证据

- Core 日志同一时刻 `POST /api/extension/action/dsh-desktop-shell-host/command` 返回 200，说明壳进程正常，问题在模块路由。
- `dsh-tree/node_modules/react-dom/package.json`：版本 18.3.1，`exports` 只有 `.`、`./client`、`./server`、`./server.browser`、`./server.node`、`./profiling`、`./test-utils`、`./package.json` —— **没有 `./root`**。
- 而模块路由把 entry id `root` 直接拼成子路径 `react-dom/root`，因此 404，动态 import 失败。

## 归因

`restore-tree`（按 npm 锁执行 `npm ci`）之后，树里的 `react-dom` 变成需要通过模块路由单独加载的形态；该路由对「非 DSH 包、entry id 为 root」的解析没有把 `root` 映射到 `exports["."]`。

## 下一步（未完成）

1. 在模块路由里把 entry id `root` 映射到 `exports["."]`（DSH 包也走同一规则），或对非 DSH 包直接按包名解析主入口。
2. 修好后重新验证：页面 `messages>0 / composer=1`、`__dshHostCommand` 为函数、头部一组「归档会话 / 置顶会话」。

## 当前状态

代码与插件均为最新（插件 0.2.30），插件测试 300/300、Core `/api/health` 200；但**原生页因上述模块路由缺陷暂不可用**，属必须修复项。

## 后续排查（同日）

1. 修掉 `react-dom/root` 解析：为非 DSH 依赖增加主入口回退后，错误从 `Failed to fetch` 变为 `ReferenceError: process is not defined`——说明文件已被加载，但服务出去的是 **CJS 构建**。
2. 再补 `process.env.NODE_ENV` 替换为 `"production"`，错误继续变为 `ReferenceError: module is not defined`。

结论：模块路由把 npm 树里 `react-dom` 的 **CommonJS 入口**（`index.js`）原样当 ESM 送给浏览器，缺 `module` / `require` 运行时，必然失败。

## 正确的修复方向

- 对纯浏览器依赖走真正的 ESM 构建：优先 `exports["."].import`/`browser` 条件，找不到时用打包后的 ESM（例如由构建期用 esbuild 预打包 `react`/`react-dom` 为单文件 ESM 放进 `vendor/`，与现有 `vendor/cordis.mjs` 同一种做法）。
- 或回退到 `restore-tree` 之前的树布局：安装前先备份 `dsh-tree/package.json` 与 `package-lock.json`，还原时优先恢复原锁而不是重新解析。

## 影响与优先级

原生页当前仍不可用（错误边界 + `slots=0`）。插件 0.2.32、Core `health=200`、插件测试 300/300 均正常，失败点集中在「模块路由 + CJS 依赖」这一层。此项优先级高于剩余功能项。

## 2026-11-18 收尾状态

- 树里不存在可回退的旧布局备份（`plugin-data/com.amitia__dsh-compatible-host` 下只有 `dsh-data` 与当前的 `dsh-tree`，无 `dsh-tree.prev`）。
- `react-dom@18.3.1` 只有 CJS 构建，`exports` 无 `import`/`browser` 条件；模块路由按文件原样返回，浏览器侧缺 `module`/`require`，因此 **在不引入构建期 ESM 打包的前提下无法修复**。
- 修复所需的最小改动：构建插件包时用 esbuild 把 `react`、`react-dom`、`react-dom/client` 预打包成单文件 ESM 放入 `src/runtime/vendor/`，并让 `readInstalledClientSource` 对这两个包命中 vendor 产物（与现有 `vendor/cordis.mjs` 同一机制）。
- 该缺陷已连续两轮阻塞原生页（错误边界、`slots=0`），其余功能（Core health 200、插件测试 300/300、插件 0.2.32 生效）正常。

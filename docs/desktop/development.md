# Amitia Electron 桌面端开发说明

## 目录

- 前端：`D:\桌面\跟进项目\U-Ai\front`
- 桌面端：`D:\桌面\跟进项目\U-Ai\desktop`

## 命令

前端源码构建：

```bash
npm run build
```

桌面端类型检查：

```bash
npm run typecheck
```

桌面端测试：

```bash
npm run test
```

桌面端开发：

```bash
npm run dev
```

## 当前依赖要求

- 前端沿用现有 Vite 服务端口 `5178`
- 后端联调仍使用项目既有链路
- 桌面端首次安装需要正确拉取 `electron` 依赖

## 开发约束

- 桌面端不直接改写 `front/dist`、`release`、`backend/WorkDone`
- 后端联调启动必须使用 `backend/WorkDone/server.exe`
- 前端新接入桌面能力时，优先走 `front/src/runtime` 适配层

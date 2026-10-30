# Proroot 隔离验证

日期：2026-10-30。用户批准独立验证，明确不修改 SnowLuma 代码。

## 结论

当前组合未通过 Linux QQ 启动验证，不能替换项目现有 PRoot。没有进入扫码登录和真实消息收发测试，原收消息故障仍未解决。

## 环境与边界

模拟器为 emulator-5554。实验由 ADB shell 身份启动，不代表 Amitia App UID、SELinux 与 seccomp 上下文已验证。实验根目录为 /data/local/tmp/amitia-proroot-trial，使用项目现有 Runtime Package 的 Ubuntu rootfs 和 Linux ARM64 Node 26.12.1。补充图形依赖来自现有 SnowLuma Debian 12 容器，保留 Ubuntu 核心 glibc，未将混合依赖环境的结果推广为所有 Proroot 环境的结论。

候选为官方 Proroot v1.2.8。五个运行文件的 SHA-256 均与 GitHub Release digest 一致。没有替换手机 APK、项目运行层、正式数据或端口，没有创建第二套长期运行服务。Xvfb 仅使用独立测试显示号并禁用 TCP 监听。

## 结果

| 检查 | 结果 |
| --- | --- |
| Ubuntu shell、id、true | 通过 |
| 项目 Node 版本、JavaScript 与文件读取 | 通过 |
| 自建最小 N-API 模块 | 通过 |
| 未修改的官方 SnowLuma 原生模块加载 | 通过 |
| PTRACE_SEIZE、PTRACE_INTERRUPT 子进程操作 | 通过，仅限 shell 身份 |
| QQ 常规无 GPU 参数启动 | GPU 子进程故障，主进程退出 |
| QQ 进程内 GPU 对照 | 进程存活，但未见登录窗口，wrapper.node 未映射 |
| QQ 单进程对照 | SIGILL，未通过 |
| App 权限上下文、扫码、实际收消息 | 未执行 |

首次原生模块加载测试曾使用错误的推送目录，随后修正路径，并确认文件存在后重新执行。该次路径错误导致的崩溃不作为 SnowLuma 模块兼容性失败证据。

依赖准备曾因设备存储不足中断；删除本实验上传的临时压缩包后，改为主机流式解包。结束时已停止隔离进程、删除设备实验目录并恢复约 3.1 GiB 可用空间。复核现有核心 ready=true、storage.ioErrors=0。

SnowLuma 官方文件在隔离测试前后哈希一致：

- snowluma-linux-arm64.node：b1473272d00fc96aab106e8f170022b3f88c1d7cd0a4e7307c0932ef23f6f746。
- snowluma-linux-arm64.so：8f8838b909bb55f9bd146e6ecd8114e4b6e94284627e5295f2003651fc320a51。

证据保存在 artifacts/proroot-isolation-20261030，包含 verification.json、官方 Release 元数据、最小原生测试源码及黑屏截图。没有将黑屏或进程存在标为 QQ 启动成功。未提交、推送或发布。

## 上游参考

- 使用方式与适用范围：https://github.com/coderredlab/proroot
- Linux QQ 启动问题与维护者的历史测试回复：https://github.com/coderredlab/proroot/issues/16

上游历史回复称其他 QQ 版本与 Debian 组合能够显示登录窗口；本轮的项目版本与 Ubuntu 组合未复现该结果，不能据此认定更换运行层一定能恢复 SnowLuma 收消息。

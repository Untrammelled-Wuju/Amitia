import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/log_folder_button.dart';
import '../../../../core/runtime/runtime_bridge_provider.dart';
import '../../../../core/runtime/runtime_bridge.dart';
import '../../../../core/runtime/runtime_bridge_error.dart';
import '../../../../core/errors/original_error.dart';
import '../../../../core/runtime/runtime_bridge_state.dart';
import '../../../../core/runtime/status/runtime_status_provider.dart';
import '../../../../core/runtime/status/runtime_status_snapshot.dart';
import '../../../../core/runtime/backend/mobile_backend_providers.dart';
import '../../../../core/backend_connection/providers/backend_connection_providers.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';

String _runtimeStateLabel(RuntimeStatusSnapshot status) {
  if (status.runtimeReady) return '运行中';
  if (!status.runtimeInstalled) return '未安装';
  switch (status.runtimeState) {
    case RuntimeBridgeState.unavailable:
      return '不可用';
    case RuntimeBridgeState.notInstalled:
      return '未安装';
    case RuntimeBridgeState.stopped:
      return '已停止';
    case RuntimeBridgeState.installing:
      return '安装中';
    case RuntimeBridgeState.starting:
      return '正在启动';
    case RuntimeBridgeState.ready:
      return '运行中';
    case RuntimeBridgeState.stopping:
      return '正在停止';
    case RuntimeBridgeState.failed:
      return '运行失败';
  }
}

String _backendStateLabel(RuntimeStatusSnapshot status) {
  if (status.businessAvailable) return '后端可用';
  if (status.runtimeReady) return '业务连接建立中';
  if (status.backendConfigured) return '后端已配置';
  return '后端未就绪';
}

bool _canStart(RuntimeStatusSnapshot status) {
  return status.runtimeInstalled &&
      !status.runtimeReady &&
      status.runtimeState != RuntimeBridgeState.starting &&
      status.runtimeState != RuntimeBridgeState.stopping &&
      status.runtimeState != RuntimeBridgeState.installing &&
      status.runtimeState != RuntimeBridgeState.failed;
}

bool _canStop(RuntimeStatusSnapshot status) {
  return status.runtimeReady ||
      status.runtimeState == RuntimeBridgeState.starting;
}

bool _canInstall(RuntimeStatusSnapshot status) {
  return !status.runtimeInstalled &&
      status.runtimeState != RuntimeBridgeState.installing &&
      status.runtimeState != RuntimeBridgeState.starting &&
      status.runtimeState != RuntimeBridgeState.stopping;
}

bool _canRepair(RuntimeStatusSnapshot status) {
  return status.runtimeInstalled &&
      status.runtimeState != RuntimeBridgeState.starting &&
      status.runtimeState != RuntimeBridgeState.stopping &&
      status.runtimeState != RuntimeBridgeState.installing;
}

class RuntimePage extends ConsumerStatefulWidget {
  const RuntimePage({super.key});

  @override
  ConsumerState<RuntimePage> createState() => _RuntimePageState();
}

class _RuntimePageState extends ConsumerState<RuntimePage> {
  bool _commandInFlight = false;

  Future<void> _runCommand(
    Future<RuntimeBridgeCommandResult> Function() action,
  ) async {
    if (_commandInFlight) return;

    setState(() => _commandInFlight = true);

    try {
      final result = await action();
      if (!mounted) return;
      _showOperationResult(result);
    } catch (error, stack) {
      if (mounted) _showError('$error\n${originalErrorText(error)}\n$stack');
      Error.throwWithStackTrace(error, stack);
    } finally {
      if (mounted) {
        setState(() => _commandInFlight = false);
      }
    }
  }

  void _showOperationResult(RuntimeBridgeCommandResult result) {
    if (result.error != null) {
      _showError(_bridgeErrorText(result.error!));
    } else if (!result.accepted) {
      _showError('命令未被接受');
    }
  }

  void _showError(String message) {
    showDialog<void>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('运行环境操作未完成'),
        content: SingleChildScrollView(child: SelectableText(message)),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('知道了'),
          ),
        ],
      ),
    );
  }

  String _bridgeErrorText(RuntimeBridgeError error) =>
      '错误码：${error.code}\n消息：${error.message}\n可重试：${error.retryable}\n详情：${error.details}';

  void _showUnavailable(RuntimeStatusSnapshot status) => _showError(
    _commandInFlight
        ? '正在处理运行环境，请稍后重试。'
        : '当前运行环境状态为 ${status.runtimeState.name}，暂不能执行此操作。',
  );

  void _resumeRuntimeCoordination() {
    ref.invalidate(backendConnectionProvider);
    ref.invalidate(backendTransportProvider);
    final lifecycle = ref.read(mobileBackendLifecycleProvider);
    lifecycle.resumeAutomaticReconciliation();
    unawaited(lifecycle.reconcile(ref.read(mobileDeploymentConfigProvider)));
  }

  Future<RuntimeBridgeCommandResult> _startRuntime() async {
    final result = await ref.read(runtimeBridgeProvider).start();
    if (result.accepted && result.error == null) {
      _resumeRuntimeCoordination();
    }
    return result;
  }

  Future<RuntimeBridgeCommandResult> _stopRuntime() async {
    final lifecycle = ref.read(mobileBackendLifecycleProvider);
    lifecycle.pauseAutomaticReconciliation();
    final result = await ref.read(runtimeBridgeProvider).stop();
    if (!result.accepted || result.error != null) {
      lifecycle.resumeAutomaticReconciliation();
    }
    return result;
  }

  Future<void> _repairRuntime() async {
    if (_commandInFlight) return;
    setState(() => _commandInFlight = true);
    final lifecycle = ref.read(mobileBackendLifecycleProvider);
    lifecycle.pauseAutomaticReconciliation();
    var coordinationResumed = false;
    try {
      final bridge = ref.read(runtimeBridgeProvider);
      if (ref.read(runtimeStatusCurrentProvider).runtimeReady) {
        final stopResult = await bridge.stop();
        if (stopResult.error != null) {
          _showError(_bridgeErrorText(stopResult.error!));
          return;
        }
        var stopSettled = false;
        for (var attempt = 0; attempt < 160; attempt++) {
          await Future<void>.delayed(const Duration(milliseconds: 250));
          if (!mounted) return;
          final state = ref.read(runtimeStatusCurrentProvider).runtimeState;
          if (state == RuntimeBridgeState.stopped ||
              state == RuntimeBridgeState.failed) {
            stopSettled = true;
            break;
          }
        }
        if (!stopSettled) {
          _showError('运行环境停止超时，请稍后重试');
          return;
        }
      }

      final repairResult = await bridge.repair();
      if (!mounted) return;
      _showOperationResult(repairResult);
      if (repairResult.accepted && repairResult.error == null) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(const SnackBar(content: Text('运行环境已重新安装，正在启动')));
        final startResult = await bridge.start();
        if (startResult.accepted && startResult.error == null) {
          _resumeRuntimeCoordination();
          coordinationResumed = true;
        } else {
          _showOperationResult(startResult);
        }
      }
    } catch (error, stack) {
      if (mounted) _showError('$error\n${originalErrorText(error)}\n$stack');
      Error.throwWithStackTrace(error, stack);
    } finally {
      if (!coordinationResumed) {
        lifecycle.resumeAutomaticReconciliation();
      }
      if (mounted) setState(() => _commandInFlight = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final statusAsync = ref.watch(runtimeStatusSnapshotProvider);

    return statusAsync.when(
      data: (status) => _buildContent(context, status),
      loading: () => const _RuntimeLoadingWidget(),
      error: (error, stack) => _RuntimeErrorWidget(
        error: '$error\n${originalErrorText(error)}\n$stack',
      ),
    );
  }

  Widget _buildContent(BuildContext context, RuntimeStatusSnapshot status) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: 'Ubuntu Runtime',
        showBackButton: true,
        fallbackRoute: AppRoutes.settings,
      ),
      body: ListView(
        padding: EdgeInsets.all(AppSpacing.pagePadding),
        children: [
          _StatusCard(status: status),
          SizedBox(height: AppSpacing.sectionGap),
          Text('运行组件', style: AppTypography.sectionTitle(context)),
          SizedBox(height: AppSpacing.md),
          Container(
            decoration: BoxDecoration(
              color: context.surfacePrimary,
              borderRadius: AppRadius.brMedium,
              border: Border.all(color: context.borderPrimary, width: 0.5),
            ),
            child: Padding(
              padding: EdgeInsets.all(AppSpacing.cardPadding),
              child: Text(
                'Runtime 版本: ${status.runtimeVersion.isEmpty ? '未知' : status.runtimeVersion}',
                style: AppTypography.caption(context),
              ),
            ),
          ),
          SizedBox(height: AppSpacing.sectionGap),
          Text('操作', style: AppTypography.sectionTitle(context)),
          SizedBox(height: AppSpacing.md),
          Wrap(
            spacing: AppSpacing.md,
            runSpacing: AppSpacing.md,
            children: [
              if (!status.runtimeInstalled)
                AmitiaButton(
                  label: '安装环境',
                  icon: Icons.download_outlined,
                  isSecondary: true,
                  onPressed: _canInstall(status) && !_commandInFlight
                      ? () => _runCommand(
                          () => ref
                              .read(runtimeBridgeProvider)
                              .reconcileEmbedded(),
                        )
                      : () => _showUnavailable(status),
                ),
              if (status.runtimeInstalled)
                AmitiaButton(
                  label: '启动',
                  icon: Icons.play_arrow,
                  onPressed: _canStart(status) && !_commandInFlight
                      ? () => _runCommand(_startRuntime)
                      : () => _showUnavailable(status),
                ),
              if (status.runtimeReady ||
                  status.runtimeState == RuntimeBridgeState.starting)
                AmitiaButton(
                  label: '停止',
                  icon: Icons.stop,
                  isSecondary: true,
                  onPressed: _canStop(status) && !_commandInFlight
                      ? () => _runCommand(_stopRuntime)
                      : () => _showUnavailable(status),
                ),
              AmitiaButton(
                label: '查看日志',
                icon: Icons.description_outlined,
                isSecondary: true,
                onPressed: () => context.push(AppRoutes.toolboxLog),
              ),
              AmitiaButton(
                label: '打开文件夹',
                icon: Icons.folder_open_outlined,
                isSecondary: true,
                onPressed: () => openLogFolder(context),
              ),
              if (status.runtimeInstalled)
                AmitiaButton(
                  label: '重新安装环境',
                  icon: Icons.build_outlined,
                  isSecondary: true,
                  onPressed: _canRepair(status) && !_commandInFlight
                      ? _repairRuntime
                      : () => _showUnavailable(status),
                ),
            ],
          ),
        ],
      ),
    );
  }
}

class _StatusCard extends StatelessWidget {
  final RuntimeStatusSnapshot status;

  const _StatusCard({required this.status});

  @override
  Widget build(BuildContext context) {
    final stateLabel = _runtimeStateLabel(status);
    final backendLabel = _backendStateLabel(status);

    return Container(
      padding: EdgeInsets.all(AppSpacing.cardPadding),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Column(
        children: [
          _InfoLine(
            label: '运行状态',
            value: stateLabel,
            type: status.runtimeReady ? BadgeType.success : BadgeType.neutral,
          ),
          _InfoLine(
            label: 'Runtime 版本',
            value: status.runtimeVersion.isEmpty ? '未知' : status.runtimeVersion,
          ),
          _InfoLine(
            label: '后端状态',
            value: backendLabel,
            type: status.businessAvailable
                ? BadgeType.success
                : BadgeType.neutral,
          ),
          _InfoLine(label: '已安装', value: status.runtimeInstalled ? '是' : '否'),
          if (status.primaryError case final error?) ...[
            Divider(height: AppSpacing.xl, color: context.borderPrimary),
            Align(
              alignment: Alignment.centerLeft,
              child: SelectableText(
                error.toString(),
                style: AppTypography.caption(context),
              ),
            ),
          ],
        ],
      ),
    );
  }
}

class _InfoLine extends StatelessWidget {
  final String label;
  final String value;
  final BadgeType? type;

  const _InfoLine({required this.label, required this.value, this.type});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 8),
      child: Row(
        children: [
          Text(label, style: AppTypography.caption(context)),
          const Spacer(),
          if (type != null)
            AmitiaStatusBadge(label: value, type: type!)
          else
            Text(value, style: AppTypography.bodySmall(context)),
        ],
      ),
    );
  }
}

class _RuntimeLoadingWidget extends StatelessWidget {
  const _RuntimeLoadingWidget();

  @override
  Widget build(BuildContext context) {
    return const Center(child: CircularProgressIndicator());
  }
}

class _RuntimeErrorWidget extends StatelessWidget {
  const _RuntimeErrorWidget({required this.error});
  final String error;

  @override
  Widget build(BuildContext context) {
    return SingleChildScrollView(
      child: SelectableText(
        'Runtime 状态加载失败\n\n原始错误：\n$error',
        style: AppTypography.body(context),
      ),
    );
  }
}

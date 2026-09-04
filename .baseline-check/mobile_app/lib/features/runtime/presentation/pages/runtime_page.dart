import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/runtime/runtime_bridge_provider.dart';
import '../../../../core/runtime/runtime_bridge.dart';
import '../../../../core/runtime/runtime_bridge_state.dart';
import '../../../../core/runtime/status/runtime_status_phase.dart';
import '../../../../core/runtime/status/runtime_status_provider.dart';
import '../../../../core/runtime/status/runtime_status_snapshot.dart';
import '../../../../shared/models/models.dart';

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
  return status.runtimeReady || status.runtimeState == RuntimeBridgeState.starting;
}

bool _canInstall(RuntimeStatusSnapshot status) {
  return !status.runtimeInstalled &&
      status.runtimeState != RuntimeBridgeState.installing &&
      status.runtimeState != RuntimeBridgeState.starting &&
      status.runtimeState != RuntimeBridgeState.stopping;
}

bool _canRepair(RuntimeStatusSnapshot status) {
  return status.runtimeInstalled &&
      !status.runtimeReady &&
      status.runtimeState != RuntimeBridgeState.starting &&
      status.runtimeState != RuntimeBridgeState.stopping &&
      status.runtimeState != RuntimeBridgeState.installing &&
      (status.runtimeState == RuntimeBridgeState.failed ||
          status.runtimeState == RuntimeBridgeState.stopped ||
          status.phase == RuntimeStatusPhase.degraded);
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
    } finally {
      if (mounted) {
        setState(() => _commandInFlight = false);
      }
    }
  }

  void _showOperationResult(RuntimeBridgeCommandResult result) {
    if (result.error != null) {
      _showError(result.error!.message);
    } else if (!result.accepted) {
      _showError('命令未被接受');
    }
  }

  void _showError(String message) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(message), duration: const Duration(seconds: 2)),
    );
  }


  @override
  Widget build(BuildContext context) {
    final statusAsync = ref.watch(runtimeStatusSnapshotProvider);

    return statusAsync.when(
      data: (status) => _buildContent(context, status),
      loading: () => const _RuntimeLoadingWidget(),
      error: (_, __) => const _RuntimeErrorWidget(),
    );
  }

  Widget _buildContent(BuildContext context, RuntimeStatusSnapshot status) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: 'Ubuntu Runtime', showBackButton: true, fallbackRoute: AppRoutes.settings),
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
                            () => ref.read(runtimeBridgeProvider).install(),
                          )
                      : null,
                ),
              if (status.runtimeInstalled)
                AmitiaButton(
                  label: '启动',
                  icon: Icons.play_arrow,
                  onPressed: _canStart(status) && !_commandInFlight
                      ? () => _runCommand(
                            () => ref.read(runtimeBridgeProvider).start(),
                          )
                      : null,
                ),
              if (status.runtimeReady || status.runtimeState == RuntimeBridgeState.starting)
                AmitiaButton(
                  label: '停止',
                  icon: Icons.stop,
                  isSecondary: true,
                  onPressed: _canStop(status) && !_commandInFlight
                      ? () => _runCommand(
                            () => ref.read(runtimeBridgeProvider).stop(),
                          )
                      : null,
                ),
              AmitiaButton(
                label: '查看日志',
                icon: Icons.description_outlined,
                isSecondary: true,
                onPressed: () => context.push(AppRoutes.toolboxLog),
              ),
              if (status.runtimeInstalled)
                AmitiaButton(
                  label: '修复环境',
                  icon: Icons.build_outlined,
                  isSecondary: true,
                  onPressed: _canRepair(status) && !_commandInFlight
                      ? () => _runCommand(
                            () => ref.read(runtimeBridgeProvider).repair(),
                          )
                      : null,
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
          _InfoLine(label: 'Runtime 版本', value: status.runtimeVersion.isEmpty ? '未知' : status.runtimeVersion),
          _InfoLine(
            label: '后端状态',
            value: backendLabel,
            type: status.businessAvailable
                ? BadgeType.success
                : BadgeType.neutral,
          ),
          _InfoLine(label: '已安装', value: status.runtimeInstalled ? '是' : '否'),
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
  const _RuntimeErrorWidget();

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Text('Runtime 状态加载失败', style: AppTypography.body(context)),
    );
  }
}

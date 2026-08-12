import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../domain/game_center_dto.dart';
import '../controllers/game_center_providers.dart';

class RuntimeDetailPage extends ConsumerWidget {
  final String runtimeId;
  final String? pluginId;

  const RuntimeDetailPage({
    super.key,
    required this.runtimeId,
    this.pluginId,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final controller = ref.watch(gameCenterControllerProvider.notifier);
    final state = ref.watch(gameCenterControllerProvider);

    final detail = state.runtimeDetail;
    final isLoading = state.runtimeLoading;
    final error = state.runtimeError;
    final hasOp = controller.hasRuntimeOp(runtimeId);

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '运行实例',
        navigation: AmitiaAppBarNavigation.back,
        actions: [
          IconButton(
            icon: const Icon(Icons.refresh),
            onPressed: hasOp ? null : () => controller.selectRuntime(runtimeId, pluginId: pluginId),
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: _buildBody(context, detail, isLoading, error, hasOp, controller, state),
      ),
    );
  }

  Widget _buildBody(
    BuildContext context,
    GameRuntimeDetail? detail,
    bool isLoading,
    String? error,
    bool hasOp,
    GameCenterController controller,
    GameCenterState state,
  ) {
    if (isLoading) {
      return const AmitiaLoadingState(message: '加载中...');
    }
    if (error != null) {
      return AmitiaErrorState(
        message: '加载失败: $error',
        onRetry: () => controller.selectRuntime(runtimeId, pluginId: pluginId),
      );
    }
    if (detail == null) {
      return const AmitiaEmptyState(
        icon: Icons.memory,
        title: '暂无运行实例信息',
      );
    }

    return ListView(
      padding: const EdgeInsets.all(AppSpacing.pagePadding),
      children: [
        _buildStatusSection(context, detail),
        const SizedBox(height: AppSpacing.lg),
        _buildControlSection(context, detail, hasOp, controller, state),
        const SizedBox(height: AppSpacing.lg),
        _buildConnectionSection(context, detail),
        const SizedBox(height: AppSpacing.lg),
        _buildServicesSection(context, detail),
        const SizedBox(height: AppSpacing.lg),
        _buildOperationsSection(context, detail, hasOp, controller),
      ],
    );
  }

  Widget _buildStatusSection(BuildContext context, GameRuntimeDetail detail) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(AppSpacing.md),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('运行状态', style: AppTypography.sectionTitle(context)),
            const SizedBox(height: AppSpacing.sm),
            _buildInfoRow(context, '状态', _stateLabel(detail.runtimeState)),
            if (detail.healthSummary != null)
              _buildInfoRow(context, '健康', _healthLabel(detail.healthSummary!.status)),
            if (detail.controlAuthority != null)
              _buildInfoRow(context, '控制模式', _modeLabel(detail.controlAuthority!.mode)),
            if (detail.controlAuthority != null)
              _buildInfoRow(context, 'Epoch', '${detail.controlAuthority!.epoch}'),
            if (detail.process != null)
              _buildInfoRow(context, '进程代数', '${detail.process!.processGeneration}'),
          ],
        ),
      ),
    );
  }

  Widget _buildControlSection(
    BuildContext context,
    GameRuntimeDetail detail,
    bool hasOp,
    GameCenterController controller,
    GameCenterState state,
  ) {
    final mode = detail.controlAuthority?.mode ?? '';
    final isUserMode = mode == 'user';

    return Card(
      child: Padding(
        padding: const EdgeInsets.all(AppSpacing.md),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('控制权', style: AppTypography.sectionTitle(context)),
            const SizedBox(height: AppSpacing.sm),
            Text('当前模式: ${_modeLabel(mode)}', style: AppTypography.body(context)),
            const SizedBox(height: AppSpacing.sm),
            if (hasOp)
              const SizedBox(
                width: 16,
                height: 16,
                child: CircularProgressIndicator(strokeWidth: 2),
              )
            else
              Wrap(
                spacing: AppSpacing.sm,
                runSpacing: AppSpacing.sm,
                children: [
                  if (!isUserMode)
                    AmitiaButton(
                      label: '用户接管',
                      isSecondary: true,
                      height: 36,
                      onPressed: () => controller.takeover(runtimeId),
                    ),
                  if (isUserMode)
                    AmitiaButton(
                      label: '释放控制权',
                      isSecondary: true,
                      height: 36,
                      onPressed: () => controller.release(runtimeId),
                    ),
                  AmitiaButton(
                    label: '紧急停止',
                    isDestructive: true,
                    height: 36,
                    onPressed: () => _confirmEmergencyStop(context, controller),
                  ),
                ],
              ),
          ],
        ),
      ),
    );
  }

  Widget _buildConnectionSection(BuildContext context, GameRuntimeDetail detail) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(AppSpacing.md),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('连接与会话', style: AppTypography.sectionTitle(context)),
            const SizedBox(height: AppSpacing.sm),
            if (detail.connection != null) ...[
              _buildInfoRow(context, '连接状态', detail.connection!.connected ? '已连接' : '未连接'),
              if (detail.connection!.protocolVersion != null)
                _buildInfoRow(context, '协议版本', detail.connection!.protocolVersion!),
              if (detail.connection!.lastHeartbeatAt != null)
                _buildInfoRow(context, '最后心跳', detail.connection!.lastHeartbeatAt!.toLocal().toString()),
            ],
            if (detail.handshake != null) ...[
              _buildInfoRow(context, '握手状态', detail.handshake!.handshakeState),
              _buildInfoRow(context, 'Ready', detail.handshake!.ready ? '是' : '否'),
              if (detail.handshake!.protocol != null)
                _buildInfoRow(context, '协议', detail.handshake!.protocol!),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildServicesSection(BuildContext context, GameRuntimeDetail detail) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(AppSpacing.md),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('服务 (${detail.services.length})', style: AppTypography.sectionTitle(context)),
            const SizedBox(height: AppSpacing.sm),
            if (detail.services.isEmpty)
              Text('暂无服务', style: AppTypography.caption(context))
            else
              ...detail.services.map((svc) => Padding(
                    padding: const EdgeInsets.only(bottom: AppSpacing.sm),
                    child: Row(
                      children: [
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(svc.serviceId, style: AppTypography.bodySmall(context)),
                              Text('状态: ${_stateLabel(svc.state)} | 健康: ${_healthLabel(svc.health)}',
                                  style: AppTypography.caption(context)),
                            ],
                          ),
                        ),
                        AmitiaStatusBadge(
                          label: svc.ready ? 'Ready' : 'Not Ready',
                          type: svc.ready ? BadgeType.success : BadgeType.neutral,
                        ),
                      ],
                    ),
                  )),
          ],
        ),
      ),
    );
  }

  Widget _buildOperationsSection(
    BuildContext context,
    GameRuntimeDetail detail,
    bool hasOp,
    GameCenterController controller,
  ) {
    final state = detail.runtimeState.toLowerCase();
    final isRunning = state == 'running' || state == 'starting';
    final isStopped = state == 'stopped' || state == 'failed';

    return Card(
      child: Padding(
        padding: const EdgeInsets.all(AppSpacing.md),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('运行操作', style: AppTypography.sectionTitle(context)),
            const SizedBox(height: AppSpacing.sm),
            if (hasOp)
              const SizedBox(
                width: 16,
                height: 16,
                child: CircularProgressIndicator(strokeWidth: 2),
              )
            else
              Wrap(
                spacing: AppSpacing.sm,
                runSpacing: AppSpacing.sm,
                children: [
                  if (isStopped)
                    AmitiaButton(
                      label: '启动',
                      isSecondary: true,
                      height: 36,
                      onPressed: () => controller.startRuntime(runtimeId),
                    ),
                  if (isRunning)
                    AmitiaButton(
                      label: '停止',
                      isSecondary: true,
                      height: 36,
                      onPressed: () => controller.stopRuntime(runtimeId),
                    ),
                  AmitiaButton(
                    label: '重启',
                    isSecondary: true,
                    height: 36,
                    onPressed: () => controller.restartRuntime(runtimeId),
                  ),
                ],
              ),
          ],
        ),
      ),
    );
  }

  Widget _buildInfoRow(BuildContext context, String label, String value) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 2),
      child: Row(
        children: [
          SizedBox(
            width: 80,
            child: Text(label, style: AppTypography.caption(context)),
          ),
          Expanded(child: Text(value, style: AppTypography.bodySmall(context))),
        ],
      ),
    );
  }

  String _stateLabel(String state) {
    switch (state.toLowerCase()) {
      case 'running':
        return '运行中';
      case 'stopped':
        return '已停止';
      case 'starting':
        return '启动中';
      case 'stopping':
        return '停止中';
      case 'failed':
        return '故障';
      default:
        return state;
    }
  }

  String _healthLabel(String health) {
    switch (health.toLowerCase()) {
      case 'healthy':
        return '正常';
      case 'degraded':
        return '异常';
      case 'unhealthy':
        return '故障';
      default:
        return health;
    }
  }

  String _modeLabel(String mode) {
    switch (mode.toLowerCase()) {
      case 'user':
        return '用户控制';
      case 'plugin':
        return '插件控制';
      case 'observe':
        return '观察模式';
      case 'assist':
        return '辅助模式';
      case 'shared':
        return '共享模式';
      case 'suspended':
        return '已暂停';
      default:
        return mode;
    }
  }

  void _confirmEmergencyStop(BuildContext context, GameCenterController controller) {
    showAmitiaConfirmDialog(
      context,
      title: '紧急停止',
      message: '紧急停止会立即阻止插件继续输出控制，终止当前运行实例并清理相关执行资源。',
      confirmLabel: '紧急停止',
      isDestructive: true,
    ).then((confirmed) {
      if (confirmed == true) {
        controller.emergencyStop(runtimeId);
      }
    });
  }
}

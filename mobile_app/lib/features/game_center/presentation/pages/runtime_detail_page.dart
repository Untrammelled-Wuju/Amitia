import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../domain/game_center_dto.dart';
import '../controllers/game_center_controller.dart';
import '../controllers/game_center_providers.dart';

class RuntimeDetailPage extends ConsumerStatefulWidget {
  final String runtimeId;
  final String? pluginId;

  const RuntimeDetailPage({
    super.key,
    required this.runtimeId,
    this.pluginId,
  });

  @override
  ConsumerState<RuntimeDetailPage> createState() => _RuntimeDetailPageState();
}

class _RuntimeDetailPageState extends ConsumerState<RuntimeDetailPage> {
  bool _developerAccess = false;

  @override
  void initState() {
    super.initState();
    Future.microtask(_loadAll);
  }

  Future<void> _loadAll() async {
    final api = ref.read(gameCenterApiProvider);
    final access = await api.developerAccess();
    if (mounted) setState(() => _developerAccess = access);
    final controller = ref.read(gameCenterControllerProvider.notifier);
    await controller.selectRuntime(widget.runtimeId, pluginId: widget.pluginId);
    await controller.loadRuntimeServices(widget.runtimeId);
    await controller.loadRuntimeHealth(widget.runtimeId);
    await controller.performHandshake(widget.runtimeId);
  }

  @override
  Widget build(BuildContext context) {
    final controller = ref.watch(gameCenterControllerProvider.notifier);
    final state = ref.watch(gameCenterControllerProvider);
    final detail = state.runtimeDetail;
    final hasOp = controller.hasRuntimeOp(widget.runtimeId);

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '运行实例',
        navigation: AmitiaAppBarNavigation.back,
        actions: [
          IconButton(
            icon: const Icon(Icons.refresh),
            onPressed: hasOp ? null : _loadAll,
            tooltip: '刷新运行时详情',
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: _buildBody(context, detail, state, hasOp, controller),
      ),
    );
  }

  Widget _buildBody(
    BuildContext context,
    GameRuntimeDetail? detail,
    GameCenterState state,
    bool hasOp,
    GameCenterController controller,
  ) {
    if (state.runtimeLoading && detail == null) {
      return const AmitiaLoadingState(message: '加载中...');
    }
    if (state.runtimeError != null && detail == null) {
      return AmitiaErrorState(
        message: '加载失败: ${state.runtimeError}',
        onRetry: _loadAll,
      );
    }
    if (detail == null || detail.runtimeId != widget.runtimeId) {
      return const AmitiaLoadingState(message: '读取运行实例...');
    }

    return RefreshIndicator(
      onRefresh: _loadAll,
      child: ListView(
        padding: EdgeInsets.all(AppSpacing.pagePadding),
        children: [
          _buildStatusSection(context, detail, state),
          SizedBox(height: AppSpacing.lg),
          _buildControlSection(context, detail, hasOp, controller),
          SizedBox(height: AppSpacing.lg),
          _buildConnectionSection(context, detail, state),
          SizedBox(height: AppSpacing.lg),
          _buildServicesSection(context, detail, state),
          SizedBox(height: AppSpacing.lg),
          _buildOperationsSection(context, detail, hasOp, controller),
        ],
      ),
    );
  }

  Widget _buildStatusSection(BuildContext context, GameRuntimeDetail detail, GameCenterState state) {
    final health = state.runtimeHealth;
    return Card(
      child: Padding(
        padding: EdgeInsets.all(AppSpacing.md),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('运行状态', style: AppTypography.sectionTitle(context)),
            SizedBox(height: AppSpacing.sm),
            _buildInfoRow(context, 'Runtime ID', detail.runtimeId),
            _buildInfoRow(context, '状态', _stateLabel(detail.runtimeState)),
            if (detail.desiredState != null) _buildInfoRow(context, '期望状态', detail.desiredState!),
            _buildInfoRow(
              context,
              '健康',
              _healthLabel(health?.status ?? detail.healthSummary?.status ?? ''),
            ),
            if (health?.lastHeartbeat != null)
              _buildInfoRow(context, '健康更新时间', health!.lastHeartbeat!.toLocal().toString()),
            if (detail.controlAuthority != null) ...[
              _buildInfoRow(context, '控制模式', _modeLabel(detail.controlAuthority!.mode)),
              _buildInfoRow(context, 'Epoch', '${detail.controlAuthority!.epoch}'),
            ],
            if (detail.process != null) ...[
              _buildInfoRow(context, '进程', detail.process!.running ? '运行中' : '未运行'),
              _buildInfoRow(context, '进程代数', '${detail.process!.processGeneration}'),
              _buildInfoRow(context, '重启次数', '${detail.process!.restartCount}'),
            ],
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
  ) {
    final mode = detail.controlAuthority?.mode ?? '';
    final isUserMode = mode == 'user';

    return Card(
      child: Padding(
        padding: EdgeInsets.all(AppSpacing.md),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('控制权', style: AppTypography.sectionTitle(context)),
            SizedBox(height: AppSpacing.sm),
            Text('当前模式: ${_modeLabel(mode)}', style: AppTypography.body(context)),
            if (detail.controlAuthority?.updatedAt != null)
              Text('更新时间: ${detail.controlAuthority!.updatedAt!.toLocal()}', style: AppTypography.caption(context)),
            SizedBox(height: AppSpacing.sm),
            if (hasOp)
              const SizedBox(width: 20, height: 20, child: CircularProgressIndicator(strokeWidth: 2))
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
                      onPressed: () async {
                        await controller.takeover(widget.runtimeId);
                        await _loadAll();
                      },
                    ),
                  if (isUserMode)
                    AmitiaButton(
                      label: '释放控制权',
                      isSecondary: true,
                      height: 36,
                      onPressed: () async {
                        await controller.release(widget.runtimeId);
                        await _loadAll();
                      },
                    ),
                  AmitiaButton(
                    label: '紧急停止',
                    isDestructive: true,
                    height: 36,
                    onPressed: () => _confirmEmergencyStop(context, controller),
                  ),
                  AmitiaButton(
                    label: 'Rearm',
                    isSecondary: true,
                    height: 36,
                    onPressed: () async {
                      await controller.rearm(widget.runtimeId);
                      await _loadAll();
                    },
                  ),
                ],
              ),
          ],
        ),
      ),
    );
  }

  Widget _buildConnectionSection(BuildContext context, GameRuntimeDetail detail, GameCenterState state) {
    final handshake = state.handshake;
    return Card(
      child: Padding(
        padding: EdgeInsets.all(AppSpacing.md),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('连接与握手', style: AppTypography.sectionTitle(context)),
            SizedBox(height: AppSpacing.sm),
            if (detail.connection != null) ...[
              _buildInfoRow(context, '连接状态', detail.connection!.connected ? '已连接' : '未连接'),
              if (detail.connection!.protocolVersion != null)
                _buildInfoRow(context, '协议版本', detail.connection!.protocolVersion!),
              if (detail.connection!.peerGeneration != null)
                _buildInfoRow(context, 'Peer 代数', '${detail.connection!.peerGeneration}'),
              if (detail.connection!.lastHeartbeatAt != null)
                _buildInfoRow(context, '最后心跳', detail.connection!.lastHeartbeatAt!.toLocal().toString()),
            ],
            if (detail.handshake != null) ...[
              _buildInfoRow(context, '握手状态', detail.handshake!.handshakeState),
              _buildInfoRow(context, 'Ready', detail.handshake!.ready ? '是' : '否'),
              if (detail.handshake!.protocol != null) _buildInfoRow(context, '协议', detail.handshake!.protocol!),
              if (detail.handshake!.sdkName != null)
                _buildInfoRow(context, 'SDK', '${detail.handshake!.sdkName} ${detail.handshake!.sdkVersion ?? ''}'.trim()),
            ],
            if (handshake != null)
              _buildInfoRow(context, '独立握手检查', handshake.accepted ? '通过' : (handshake.error ?? '未通过')),
          ],
        ),
      ),
    );
  }

  Widget _buildServicesSection(BuildContext context, GameRuntimeDetail detail, GameCenterState state) {
    final live = state.runtimeServices?.services ?? const <GameServiceSummary>[];
    final services = detail.services;
    return Card(
      child: Padding(
        padding: EdgeInsets.all(AppSpacing.md),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(child: Text('服务 (${services.length})', style: AppTypography.sectionTitle(context))),
                if (state.runtimeServicesLoading)
                  const SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2)),
              ],
            ),
            SizedBox(height: AppSpacing.sm),
            if (services.isEmpty && live.isEmpty)
              Text('暂无服务', style: AppTypography.caption(context))
            else if (services.isNotEmpty)
              ...services.map((svc) => _serviceRow(context, svc))
            else
              ...live.map((svc) => ListTile(
                    contentPadding: EdgeInsets.zero,
                    title: Text(svc.serviceId, style: AppTypography.bodySmall(context)),
                    subtitle: Text('状态: ${svc.state} | 健康: ${_healthLabel(svc.health)}', style: AppTypography.caption(context)),
                    trailing: _developerAccess
                        ? TextButton(
                            onPressed: () => _showRpcDialog(context, svc.serviceId),
                            child: const Text('RPC'),
                          )
                        : null,
                  )),
          ],
        ),
      ),
    );
  }

  Widget _serviceRow(BuildContext context, GameService svc) {
    return Padding(
      padding: EdgeInsets.only(bottom: AppSpacing.sm),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(svc.serviceId, style: AppTypography.bodySmall(context)),
                Text(
                  '状态: ${_stateLabel(svc.state)} | 健康: ${_healthLabel(svc.health)}',
                  style: AppTypography.caption(context),
                ),
                if (svc.definitionId.isNotEmpty)
                  Text('定义: ${svc.definitionId}', style: AppTypography.caption(context)),
              ],
            ),
          ),
          AmitiaStatusBadge(
            label: svc.ready ? 'Ready' : 'Not Ready',
            type: svc.ready ? BadgeType.success : BadgeType.neutral,
          ),
          if (_developerAccess) ...[
            const SizedBox(width: 8),
            TextButton(onPressed: () => _showRpcDialog(context, svc.serviceId), child: const Text('RPC')),
          ],
        ],
      ),
    );
  }

  Widget _buildOperationsSection(
    BuildContext context,
    GameRuntimeDetail detail,
    bool hasOp,
    GameCenterController controller,
  ) {
    final runtimeState = detail.runtimeState.toLowerCase();
    final isRunning = runtimeState == 'running' || runtimeState == 'starting';
    final isStopped = runtimeState == 'stopped' || runtimeState == 'failed';

    return Card(
      child: Padding(
        padding: EdgeInsets.all(AppSpacing.md),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('运行操作', style: AppTypography.sectionTitle(context)),
            SizedBox(height: AppSpacing.sm),
            if (hasOp)
              const SizedBox(width: 20, height: 20, child: CircularProgressIndicator(strokeWidth: 2))
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
                      onPressed: () async {
                        await controller.startRuntime(widget.runtimeId);
                        await _loadAll();
                      },
                    ),
                  if (isRunning)
                    AmitiaButton(
                      label: '停止',
                      isSecondary: true,
                      height: 36,
                      onPressed: () async {
                        await controller.stopRuntime(widget.runtimeId);
                        await _loadAll();
                      },
                    ),
                  AmitiaButton(
                    label: '重启',
                    isSecondary: true,
                    height: 36,
                    onPressed: () async {
                      await controller.restartRuntime(widget.runtimeId);
                      await _loadAll();
                    },
                  ),
                ],
              ),
          ],
        ),
      ),
    );
  }

  Future<void> _showRpcDialog(BuildContext context, String serviceId) async {
    if (!_developerAccess) return;
    final methodController = TextEditingController();
    final payloadController = TextEditingController(text: '{}');
    final timeoutController = TextEditingController(text: '30000');
    String result = '';
    bool loading = false;

    await showDialog<void>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (ctx, setLocal) => AlertDialog(
          title: Text('Service RPC · $serviceId'),
          content: SizedBox(
            width: 560,
            child: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  AmitiaTextField(controller: methodController, hintText: 'RPC method'),
                  const SizedBox(height: 12),
                  AmitiaTextField(controller: payloadController, maxLines: 6, hintText: 'JSON payload'),
                  const SizedBox(height: 12),
                  AmitiaTextField(controller: timeoutController, hintText: 'Timeout ms'),
                  if (result.isNotEmpty) ...[
                    const SizedBox(height: 16),
                    Text('返回结果', style: AppTypography.label(context)),
                    const SizedBox(height: 6),
                    SelectableText(result, style: AppTypography.caption(context)),
                  ],
                ],
              ),
            ),
          ),
          actions: [
            TextButton(onPressed: loading ? null : () => Navigator.pop(dialogContext), child: const Text('关闭')),
            TextButton(
              onPressed: loading
                  ? null
                  : () async {
                      final method = methodController.text.trim();
                      if (method.isEmpty) return;
                      Object? payload;
                      try {
                        final raw = payloadController.text.trim();
                        payload = raw.isEmpty ? null : jsonDecode(raw);
                      } catch (_) {
                        setLocal(() => result = 'Payload 不是合法 JSON');
                        return;
                      }
                      setLocal(() => loading = true);
                      try {
                        final response = await ref.read(gameCenterApiProvider).invokeServiceRpc(
                              widget.runtimeId,
                              serviceId,
                              method: method,
                              payload: payload,
                              timeoutMs: int.tryParse(timeoutController.text.trim()) ?? 30000,
                            );
                        setLocal(() => result = const JsonEncoder.withIndent('  ').convert(response));
                      } catch (e) {
                        setLocal(() => result = '调用失败: $e');
                      } finally {
                        setLocal(() => loading = false);
                      }
                    },
              child: loading
                  ? const SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2))
                  : const Text('调用'),
            ),
          ],
        ),
      ),
    );
    methodController.dispose();
    payloadController.dispose();
    timeoutController.dispose();
  }

  Widget _buildInfoRow(BuildContext context, String label, String value) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 3),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(width: 92, child: Text(label, style: AppTypography.caption(context))),
          Expanded(child: Text(value.isEmpty ? '-' : value, style: AppTypography.bodySmall(context))),
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
        return state.isEmpty ? '-' : state;
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
        return health.isEmpty ? '-' : health;
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
        return mode.isEmpty ? '-' : mode;
    }
  }

  void _confirmEmergencyStop(BuildContext context, GameCenterController controller) {
    showAmitiaConfirmDialog(
      context,
      title: '紧急停止',
      message: '紧急停止会立即阻止插件继续输出控制，终止当前运行实例并清理相关执行资源。',
      confirmLabel: '紧急停止',
      isDestructive: true,
    ).then((confirmed) async {
      if (confirmed == true) {
        await controller.emergencyStop(widget.runtimeId);
        await _loadAll();
      }
    });
  }
}

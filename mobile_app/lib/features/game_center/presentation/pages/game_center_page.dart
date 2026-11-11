import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../domain/game_center_dto.dart';
import '../../data/game_package_picker.dart';
import '../controllers/game_center_providers.dart';
import '../controllers/game_center_controller.dart';
import '../widgets/game_package_confirmation.dart';

class GameCenterPage extends ConsumerStatefulWidget {
  const GameCenterPage({super.key});

  @override
  ConsumerState<GameCenterPage> createState() => _GameCenterPageState();
}

class _GameCenterPageState extends ConsumerState<GameCenterPage> {
  @override
  void initState() {
    super.initState();
    Future.microtask(() async {
      final controller = ref.read(gameCenterControllerProvider.notifier);
      await controller.loadGameMode();
      if (ref.read(gameCenterControllerProvider).gameModeStatus?.owned ==
          true) {
        await controller.loadPlugins();
        await controller.loadCenterHealth();
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(gameCenterControllerProvider);
    final active = state.gameModeStatus?.owned == true;

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '游戏中心 v1',
        navigation: AmitiaAppBarNavigation.back,
        actions: [
          if (active)
            GestureDetector(
              behavior: HitTestBehavior.opaque,
              onTap: state.gameModeOperating
                  ? () => ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('正在处理游戏模式，请稍后重试。')))
                  : null,
              child: IconButton(
                icon: const Icon(Icons.logout),
                tooltip: '退出游戏模式',
                onPressed: state.gameModeOperating
                    ? null
                    : () => ref
                          .read(gameCenterControllerProvider.notifier)
                          .releaseGameMode(),
              ),
            ),
          IconButton(
            icon: const Icon(Icons.add_box_outlined),
            tooltip: '安装插件',
            onPressed: active
                ? () => _showInstallDialog(context)
                : () => showDialog<void>(
                    context: context,
                    builder: (context) => AlertDialog(
                      title: const Text('无法安装插件'),
                      content: SingleChildScrollView(
                        child: SelectableText(
                          state.gameModeLoading
                              ? '正在检查游戏模式状态，请稍后重试。'
                              : state.gameModeError ?? '请先启用游戏模式。',
                        ),
                      ),
                      actions: [
                        TextButton(
                          onPressed: () => Navigator.pop(context),
                          child: const Text('知道了'),
                        ),
                      ],
                    ),
                  ),
          ),
          IconButton(
            icon: const Icon(Icons.refresh),
            tooltip: '刷新',
            onPressed: !active || state.pluginsRefreshing
                ? () => showDialog<void>(
                    context: context,
                    builder: (context) => AlertDialog(
                      title: const Text('无法刷新游戏中心'),
                      content: SingleChildScrollView(
                        child: SelectableText(
                          state.pluginsRefreshing || state.gameModeLoading
                              ? '正在刷新游戏中心，请稍后重试。'
                              : state.gameModeError ?? '请先启用游戏模式。',
                        ),
                      ),
                      actions: [
                        TextButton(
                          onPressed: () => Navigator.pop(context),
                          child: const Text('知道了'),
                        ),
                      ],
                    ),
                  )
                : () async {
                    final controller = ref.read(
                      gameCenterControllerProvider.notifier,
                    );
                    await controller.refreshPlugins();
                    await controller.loadCenterHealth();
                  },
          ),
        ],
      ),
      body: SafeArea(top: false, child: _buildBody(context, state)),
    );
  }

  Widget _buildBody(BuildContext context, GameCenterState state) {
    if (state.gameModeLoading && state.gameModeStatus == null) {
      return const AmitiaLoadingState(message: '正在检查游戏模式状态...');
    }
    if (state.gameModeStatus == null) {
      return AmitiaErrorState(
        message: state.gameModeError ?? '无法获取游戏模式状态',
        onRetry: () =>
            ref.read(gameCenterControllerProvider.notifier).loadGameMode(),
      );
    }
    final status = state.gameModeStatus!;
    if (!status.session.active) {
      return _buildGameModeEntry(context, state);
    }
    if (!status.owned) {
      return _buildOccupiedState(context, state);
    }
    if (state.pluginsLoading) {
      return const AmitiaLoadingState(message: '加载中...');
    }
    if (state.pluginsError != null) {
      return AmitiaErrorState(
        message: '加载失败: ${state.pluginsError}',
        onRetry: () =>
            ref.read(gameCenterControllerProvider.notifier).loadPlugins(),
      );
    }
    if (state.plugins.isEmpty) {
      return AmitiaEmptyState(
        icon: Icons.sports_esports_outlined,
        title: '尚未安装游戏插件',
        subtitle: '安装游戏插件后即可在此管理',
        actionText: '安装插件',
        onAction: () => _showInstallDialog(context),
      );
    }
    return _buildPluginList(context, state);
  }

  Widget _buildGameModeEntry(BuildContext context, GameCenterState state) {
    return Center(
      child: Padding(
        padding: EdgeInsets.all(AppSpacing.pagePadding),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              Icons.sports_esports_outlined,
              size: 64,
              color: Theme.of(context).colorScheme.primary,
            ),
            const SizedBox(height: 20),
            Text('进入游戏模式', style: AppTypography.pageTitle(context)),
            const SizedBox(height: 8),
            Text(
              '同一时间只允许一台设备使用游戏模式。进入后其他设备只能查看状态并通过对话控制。',
              style: AppTypography.caption(context),
              textAlign: TextAlign.center,
            ),
            if (state.gameModeError != null) ...[
              const SizedBox(height: 12),
              Text(
                state.gameModeError!,
                style: AppTypography.caption(
                  context,
                ).copyWith(color: Theme.of(context).colorScheme.error),
                textAlign: TextAlign.center,
              ),
            ],
            const SizedBox(height: 20),
            GestureDetector(
              behavior: HitTestBehavior.opaque,
              onTap: state.gameModeOperating
                  ? () => ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('正在处理游戏模式，请稍后重试。')))
                  : null,
              child: AmitiaButton(
                label: state.gameModeOperating ? '正在进入...' : '进入游戏模式',
                isFullWidth: true,
                onPressed: state.gameModeOperating
                    ? null
                    : () => ref
                          .read(gameCenterControllerProvider.notifier)
                          .acquireGameMode(),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildOccupiedState(BuildContext context, GameCenterState state) {
    final session = state.gameModeStatus!.session;
    return Center(
      child: Padding(
        padding: EdgeInsets.all(AppSpacing.pagePadding),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              Icons.devices_other,
              size: 64,
              color: Theme.of(context).colorScheme.primary,
            ),
            const SizedBox(height: 20),
            Text('其他设备正在游戏模式中', style: AppTypography.pageTitle(context)),
            const SizedBox(height: 12),
            Card(
              child: Padding(
                padding: EdgeInsets.all(AppSpacing.md),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text('设备', style: AppTypography.label(context)),
                    Text(session.deviceId, style: AppTypography.body(context)),
                    const SizedBox(height: 8),
                    Text('状态', style: AppTypography.label(context)),
                    Text(session.state, style: AppTypography.body(context)),
                    if (session.gameTitle.isNotEmpty) ...[
                      const SizedBox(height: 8),
                      Text('游戏', style: AppTypography.label(context)),
                      Text(
                        session.gameTitle,
                        style: AppTypography.body(context),
                      ),
                    ],
                  ],
                ),
              ),
            ),
            const SizedBox(height: 12),
            Text(
              '当前 Space 同一时间只允许一台设备进入游戏模式。你可以通过对话查询或控制该设备。',
              style: AppTypography.caption(context),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 16),
            AmitiaButton(
              label: '刷新状态',
              isSecondary: true,
              isFullWidth: true,
              onPressed: () => ref
                  .read(gameCenterControllerProvider.notifier)
                  .loadGameMode(),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildPluginList(BuildContext context, GameCenterState state) {
    return RefreshIndicator(
      onRefresh: () async {
        final controller = ref.read(gameCenterControllerProvider.notifier);
        await controller.refreshPlugins();
        await controller.loadCenterHealth();
      },
      child: ListView.builder(
        padding: EdgeInsets.all(AppSpacing.pagePadding),
        itemCount: state.plugins.length + 1,
        itemBuilder: (context, index) {
          if (index == 0) {
            final center = state.centerHealth;
            return Card(
              margin: EdgeInsets.only(bottom: AppSpacing.md),
              child: Padding(
                padding: EdgeInsets.all(AppSpacing.md),
                child: Row(
                  children: [
                    Icon(
                      Icons.monitor_heart_outlined,
                      color: Theme.of(context).colorScheme.primary,
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            'Game Center Runtime',
                            style: AppTypography.cardTitle(context),
                          ),
                          Text(
                            state.centerHealthLoading
                                ? '正在检查运行状态...'
                                : '状态: ${center?.status.isNotEmpty == true ? center!.status : 'unknown'}',
                            style: AppTypography.caption(context),
                          ),
                        ],
                      ),
                    ),
                    AmitiaStatusBadge(
                      label: center?.status == 'healthy'
                          ? '正常'
                          : (center?.status ?? '未知'),
                      type: center?.status == 'healthy'
                          ? BadgeType.success
                          : BadgeType.neutral,
                    ),
                  ],
                ),
              ),
            );
          }
          final plugin = state.plugins[index - 1];
          return _PluginCard(
            plugin: plugin,
            onTap: () async {
              final updated = await ref
                  .read(gameCenterControllerProvider.notifier)
                  .updateGameMode(nextState: 'active', gameTitle: plugin.name);
              if (!context.mounted) return;
              if (!updated) {
                final error =
                    ref.read(gameCenterControllerProvider).gameModeError ??
                    '更新游戏模式失败';
                ScaffoldMessenger.of(
                  context,
                ).showSnackBar(SnackBar(content: Text(error)));
                return;
              }
              context.push(
                Uri(
                  path: AppRoutes.gamePlugin,
                  queryParameters: <String, String>{
                    'extensionId': plugin.extensionId,
                    'pluginId': plugin.pluginId,
                  },
                ).toString(),
              );
            },
            onEnable: plugin.enabled
                ? null
                : () => ref
                      .read(gameCenterControllerProvider.notifier)
                      .enable(plugin.extensionId),
            onDisable: plugin.enabled
                ? () => ref
                      .read(gameCenterControllerProvider.notifier)
                      .disable(plugin.extensionId)
                : null,
            onUninstall: () => _confirmUninstall(context, plugin),
            isOperating: ref
                .read(gameCenterControllerProvider.notifier)
                .hasPackageOp(plugin.extensionId),
          );
        },
      ),
    );
  }

  void _showInstallDialog(BuildContext context) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('安装游戏插件'),
        content: const Text('请选择 .gamex 游戏扩展包。安装将使用统一的扩展包预览、确认与事务生命周期。'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: const Text('取消'),
          ),
          TextButton(
            onPressed: () async {
              Navigator.pop(ctx);
              try {
                final path = await pickGamePackagePath();
                if (path == null) return;
                final lifecycle = ref.read(gameCenterPackageLifecycleProvider);
                final preview = await lifecycle.previewPackage(path);
                if (!context.mounted) return;
                final accepted = await showGamePackagePreviewConfirmation(
                  context,
                  preview,
                  actionLabel:
                      (preview['currentVersion'] ?? '').toString().isEmpty
                      ? '安装'
                      : '更新',
                );
                if (!accepted) return;
                final operationId = await lifecycle.commitPackage(preview);
                await ref
                    .read(gameCenterControllerProvider.notifier)
                    .refreshPlugins();
                if (!context.mounted) return;
                ScaffoldMessenger.of(context).showSnackBar(
                  SnackBar(
                    content: Text(
                      operationId.isEmpty
                          ? '游戏插件操作已完成'
                          : '游戏插件操作已完成 · $operationId',
                    ),
                  ),
                );
              } catch (e) {
                if (!context.mounted) return;
                ScaffoldMessenger.of(
                  context,
                ).showSnackBar(SnackBar(content: Text('游戏插件安装失败: $e')));
              }
            },
            child: const Text('选择文件'),
          ),
        ],
      ),
    );
  }

  Future<void> _confirmUninstall(
    BuildContext context,
    GamePluginSummary plugin,
  ) async {
    final lifecycle = ref.read(gameCenterPackageLifecycleProvider);
    try {
      final preview = await lifecycle.previewUninstall(plugin.extensionId);
      if (!context.mounted) return;
      final confirmed = await showGamePackageUninstallConfirmation(
        context,
        preview,
        displayName: plugin.name,
      );
      if (!confirmed) return;
      final controller = ref.read(gameCenterControllerProvider.notifier);
      final ok = await controller.runPackageOperation(
        plugin.extensionId,
        () => lifecycle.commitUninstall(plugin.extensionId, preview),
        clearSelectionAfterSuccess: true,
      );
      if (!context.mounted) return;
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(ok ? '游戏插件卸载完成' : '游戏插件卸载失败')));
    } catch (e) {
      if (!context.mounted) return;
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text('游戏插件卸载失败: $e')));
    }
  }
}

class _PluginCard extends StatelessWidget {
  final GamePluginSummary plugin;
  final VoidCallback onTap;
  final VoidCallback? onEnable;
  final VoidCallback? onDisable;
  final VoidCallback onUninstall;
  final bool isOperating;

  const _PluginCard({
    required this.plugin,
    required this.onTap,
    this.onEnable,
    this.onDisable,
    required this.onUninstall,
    this.isOperating = false,
  });

  @override
  Widget build(BuildContext context) {
    return Card(
      margin: EdgeInsets.only(bottom: AppSpacing.md),
      child: InkWell(
        onTap: isOperating
            ? () => ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('正在处理该游戏插件，请稍后重试。')))
            : onTap,
        borderRadius: BorderRadius.circular(12),
        child: Padding(
          padding: EdgeInsets.all(AppSpacing.md),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          plugin.name,
                          style: AppTypography.cardTitle(context),
                        ),
                        const SizedBox(height: 2),
                        Text(
                          'v${plugin.version}',
                          style: AppTypography.caption(context),
                        ),
                      ],
                    ),
                  ),
                  _buildStatusBadges(context),
                ],
              ),
              if (plugin.description.isNotEmpty) ...[
                SizedBox(height: AppSpacing.sm),
                Text(
                  plugin.description,
                  style: AppTypography.bodySmall(context),
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                ),
              ],
              SizedBox(height: AppSpacing.sm),
              Row(
                children: [
                  if (plugin.runtimeCount > 0)
                    AmitiaStatusBadge(
                      label: '${plugin.runtimeCount} 运行中',
                      type: BadgeType.info,
                    ),
                  const Spacer(),
                  if (isOperating)
                    const SizedBox(
                      width: 16,
                      height: 16,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  else ...[
                    if (onEnable != null)
                      AmitiaButtonOutline(label: '启用', onPressed: onEnable),
                    if (onDisable != null) ...[
                      SizedBox(width: AppSpacing.sm),
                      AmitiaButtonOutline(label: '禁用', onPressed: onDisable),
                    ],
                    SizedBox(width: AppSpacing.sm),
                    AmitiaButtonOutline(label: '卸载', onPressed: onUninstall),
                  ],
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildStatusBadges(BuildContext context) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        AmitiaStatusBadge(
          label: plugin.enabled ? '已启用' : '已禁用',
          type: plugin.enabled ? BadgeType.success : BadgeType.neutral,
        ),
        SizedBox(width: AppSpacing.xs),
        if (plugin.health.isNotEmpty)
          AmitiaStatusBadge(
            label: _healthLabel(plugin.health),
            type: _healthBadgeType(plugin.health),
          ),
      ],
    );
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

  BadgeType _healthBadgeType(String health) {
    switch (health.toLowerCase()) {
      case 'healthy':
        return BadgeType.success;
      case 'degraded':
        return BadgeType.warning;
      case 'unhealthy':
        return BadgeType.error;
      default:
        return BadgeType.neutral;
    }
  }
}

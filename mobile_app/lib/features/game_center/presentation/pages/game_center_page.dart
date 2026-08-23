import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:file_picker/file_picker.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../domain/game_center_dto.dart';
import '../controllers/game_center_providers.dart';
import '../controllers/game_center_controller.dart';
import 'plugin_detail_page.dart';
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
      await controller.loadPlugins();
      await controller.loadCenterHealth();
    });
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(gameCenterControllerProvider);

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '游戏中心',
        navigation: AmitiaAppBarNavigation.back,
        actions: [
          IconButton(
            icon: const Icon(Icons.add_box_outlined),
            tooltip: '安装插件',
            onPressed: () => _showInstallDialog(context),
          ),
          IconButton(
            icon: const Icon(Icons.refresh),
            onPressed: state.pluginsRefreshing
                ? null
                : () async {
                    final controller = ref.read(gameCenterControllerProvider.notifier);
                    await controller.refreshPlugins();
                    await controller.loadCenterHealth();
                  },
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: _buildBody(context, state),
      ),
    );
  }

  Widget _buildBody(BuildContext context, GameCenterState state) {
    if (state.pluginsLoading) {
      return const AmitiaLoadingState(message: '加载中...');
    }
    if (state.pluginsError != null) {
      return AmitiaErrorState(
        message: '加载失败: ${state.pluginsError}',
        onRetry: () => ref.read(gameCenterControllerProvider.notifier).loadPlugins(),
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
                    Icon(Icons.monitor_heart_outlined, color: Theme.of(context).colorScheme.primary),
                    const SizedBox(width: 12),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text('Game Center Runtime', style: AppTypography.cardTitle(context)),
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
                      label: center?.status == 'healthy' ? '正常' : (center?.status ?? '未知'),
                      type: center?.status == 'healthy' ? BadgeType.success : BadgeType.neutral,
                    ),
                  ],
                ),
              ),
            );
          }
          final plugin = state.plugins[index - 1];
          return _PluginCard(
            plugin: plugin,
            onTap: () {
              ref.read(gameCenterControllerProvider.notifier).selectPlugin(
                plugin.pluginId,
                extensionId: plugin.extensionId,
              );
              Navigator.push(
                context,
                MaterialPageRoute(
                  builder: (_) => PluginDetailPage(
                    pluginId: plugin.pluginId,
                    extensionId: plugin.extensionId,
                  ),
                ),
              );
            },
            onEnable: plugin.enabled
                ? null
                : () => ref.read(gameCenterControllerProvider.notifier).enable(plugin.extensionId),
            onDisable: plugin.enabled
                ? () => ref.read(gameCenterControllerProvider.notifier).disable(plugin.extensionId)
                : null,
            onUninstall: () => _confirmUninstall(context, plugin),
            isOperating: ref.read(gameCenterControllerProvider.notifier).hasPackageOp(plugin.extensionId),
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
        content: const Text('请选择 .amitiax 游戏扩展包。安装将使用统一的扩展包预览、确认与事务生命周期。'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: const Text('取消'),
          ),
          TextButton(
            onPressed: () async {
              Navigator.pop(ctx);
              final result = await FilePicker.platform.pickFiles(
                allowMultiple: false,
                type: FileType.custom,
                allowedExtensions: const ['amitiax'],
              );
              final path = result?.files.single.path;
              if (path == null || path.isEmpty) return;
              try {
                final lifecycle = ref.read(gameCenterPackageLifecycleProvider);
                final preview = await lifecycle.previewPackage(path);
                if (!context.mounted) return;
                final accepted = await showGamePackagePreviewConfirmation(
                  context,
                  preview,
                  actionLabel: (preview['currentVersion'] ?? '').toString().isEmpty ? '安装' : '更新',
                );
                if (!accepted) return;
                final operationId = await lifecycle.commitPackage(preview);
                await ref.read(gameCenterControllerProvider.notifier).refreshPlugins();
                if (!context.mounted) return;
                ScaffoldMessenger.of(context).showSnackBar(
                  SnackBar(content: Text(operationId.isEmpty ? '游戏插件操作已完成' : '游戏插件操作已完成 · $operationId')),
                );
              } catch (e) {
                if (!context.mounted) return;
                ScaffoldMessenger.of(context).showSnackBar(
                  SnackBar(content: Text('游戏插件安装失败: $e')),
                );
              }
            },
            child: const Text('选择文件'),
          ),
        ],
      ),
    );
  }

  Future<void> _confirmUninstall(BuildContext context, GamePluginSummary plugin) async {
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
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(ok ? '游戏插件卸载完成' : '游戏插件卸载失败')),
      );
    } catch (e) {
      if (!context.mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('游戏插件卸载失败: $e')),
      );
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
        onTap: isOperating ? null : onTap,
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
                        Text(plugin.name, style: AppTypography.cardTitle(context)),
                        const SizedBox(height: 2),
                        Text('v${plugin.version}', style: AppTypography.caption(context)),
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
                      AmitiaButtonOutline(
                        label: '启用',
                        onPressed: onEnable,
                      ),
                    if (onDisable != null) ...[
                      SizedBox(width: AppSpacing.sm),
                      AmitiaButtonOutline(
                        label: '禁用',
                        onPressed: onDisable,
                      ),
                    ],
                    SizedBox(width: AppSpacing.sm),
                    AmitiaButtonOutline(
                      label: '卸载',
                      onPressed: onUninstall,
                    ),
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

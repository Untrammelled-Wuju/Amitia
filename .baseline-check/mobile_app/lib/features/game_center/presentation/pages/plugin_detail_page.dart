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
import 'runtime_detail_page.dart';
import '../widgets/game_package_confirmation.dart';

class PluginDetailPage extends ConsumerWidget {
  final String pluginId;
  final String extensionId;

  const PluginDetailPage({
    super.key,
    required this.pluginId,
    required this.extensionId,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final controller = ref.watch(gameCenterControllerProvider.notifier);
    final state = ref.watch(gameCenterControllerProvider);

    final detail = state.pluginDetail;
    final isLoading = state.pluginDetailLoading;
    final error = state.pluginDetailError;
    final hasOp = controller.hasPackageOp(extensionId);

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: detail?.name ?? '插件详情',
        navigation: AmitiaAppBarNavigation.back,
        actions: [
          IconButton(
            icon: const Icon(Icons.refresh),
            onPressed: hasOp ? null : () => controller.selectPlugin(pluginId, extensionId: extensionId),
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: _buildBody(context, ref, detail, isLoading, error, hasOp, controller, state),
      ),
    );
  }

  Widget _buildBody(
    BuildContext context,
    WidgetRef ref,
    GamePluginDetail? detail,
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
        onRetry: () => controller.selectPlugin(pluginId, extensionId: extensionId),
      );
    }
    if (detail == null) {
      return const AmitiaEmptyState(
        icon: Icons.extension_outlined,
        title: '暂无插件信息',
      );
    }

    return ListView(
      padding: EdgeInsets.all(AppSpacing.pagePadding),
      children: [
        _buildInfoSection(context, detail),
        SizedBox(height: AppSpacing.lg),
        _buildActionsSection(context, ref, detail, hasOp, controller),
        SizedBox(height: AppSpacing.lg),
        _buildRuntimesSection(context, detail, state),
      ],
    );
  }

  Widget _buildInfoSection(BuildContext context, GamePluginDetail detail) {
    return Card(
      child: Padding(
        padding: EdgeInsets.all(AppSpacing.md),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('插件信息', style: AppTypography.sectionTitle(context)),
            SizedBox(height: AppSpacing.sm),
            _buildInfoRow(context, '名称', detail.name),
            _buildInfoRow(context, '版本', detail.version),
            _buildInfoRow(context, '状态', detail.enabled ? '已启用' : '已禁用'),
            _buildInfoRow(context, '安装状态', detail.installState),
            if (detail.description.isNotEmpty)
              _buildInfoRow(context, '描述', detail.description),
            if (detail.healthSummary != null)
              _buildInfoRow(context, '健康', detail.healthSummary!.status),
            _buildInfoRow(context, 'ExtensionID', detail.extensionId),
            _buildInfoRow(context, 'PluginID', detail.pluginId),
            if (detail.provider != null)
              _buildInfoRow(context, 'Provider', detail.provider!),
            if (detail.packageRevision != null)
              _buildInfoRow(context, 'PackageRevision', detail.packageRevision!),
            if (detail.permissions.isNotEmpty) ...[
              SizedBox(height: AppSpacing.sm),
              Text('权限', style: AppTypography.caption(context)),
              const SizedBox(height: 4),
              Wrap(
                spacing: 4,
                runSpacing: 4,
                children: detail.permissions
                    .map((p) => AmitiaStatusBadge(label: p, type: BadgeType.info))
                    .toList(),
              ),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildActionsSection(
    BuildContext context,
    WidgetRef ref,
    GamePluginDetail detail,
    bool hasOp,
    GameCenterController controller,
  ) {
    return Card(
      child: Padding(
        padding: EdgeInsets.all(AppSpacing.md),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('插件操作', style: AppTypography.sectionTitle(context)),
            SizedBox(height: AppSpacing.sm),
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
                  if (!detail.enabled)
                    AmitiaButton(
                      label: '启用',
                      isSecondary: true,
                      height: 36,
                      onPressed: () => controller.enable(detail.extensionId),
                    ),
                  if (detail.enabled)
                    AmitiaButton(
                      label: '禁用',
                      isSecondary: true,
                      height: 36,
                      onPressed: () => controller.disable(detail.extensionId),
                    ),
                  AmitiaButton(
                    label: '更新',
                    isSecondary: true,
                    height: 36,
                    onPressed: () => _pickUpdate(context, ref, controller, detail),
                  ),
                  AmitiaButton(
                    label: '卸载',
                    isDestructive: true,
                    height: 36,
                    onPressed: () => _confirmUninstall(context, ref, controller, detail),
                  ),
                ],
              ),
          ],
        ),
      ),
    );
  }

  Widget _buildRuntimesSection(BuildContext context, GamePluginDetail detail, GameCenterState state) {
    return Card(
      child: Padding(
        padding: EdgeInsets.all(AppSpacing.md),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('运行实例 (${detail.runtimes.length})', style: AppTypography.sectionTitle(context)),
            SizedBox(height: AppSpacing.sm),
            if (detail.runtimes.isEmpty)
              Text('尚无运行实例', style: AppTypography.caption(context))
            else
              ...detail.runtimes.map((rt) => InkWell(
                    onTap: () {
                      Navigator.push(
                        context,
                        MaterialPageRoute(
                          builder: (_) => RuntimeDetailPage(
                            runtimeId: rt.runtimeId,
                            pluginId: rt.pluginId,
                          ),
                        ),
                      );
                    },
                    child: Padding(
                      padding: EdgeInsets.symmetric(vertical: AppSpacing.sm),
                      child: Row(
                        children: [
                          Expanded(
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Text(rt.runtimeId, style: AppTypography.bodySmall(context)),
                                Text(
                                  '状态: ${_stateLabel(rt.state)} | 控制: ${_modeLabel(rt.controlMode)}',
                                  style: AppTypography.caption(context),
                                ),
                              ],
                            ),
                          ),
                          AmitiaStatusBadge(
                            label: rt.connected ? (rt.ready ? 'Ready' : 'Connected') : 'Disconnected',
                            type: rt.connected
                                ? (rt.ready ? BadgeType.success : BadgeType.info)
                                : BadgeType.neutral,
                          ),
                        ],
                      ),
                    ),
                  )),
          ],
        ),
      ),
    );
  }

  Widget _buildInfoRow(BuildContext context, String label, String value) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 2),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 100,
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

  String _modeLabel(String mode) {
    switch (mode.toLowerCase()) {
      case 'user':
        return '用户';
      case 'plugin':
        return '插件';
      case 'observe':
        return '观察';
      case 'assist':
        return '辅助';
      case 'shared':
        return '共享';
      case 'suspended':
        return '暂停';
      default:
        return mode;
    }
  }

  Future<void> _pickUpdate(
    BuildContext context,
    WidgetRef ref,
    GameCenterController controller,
    GamePluginDetail detail,
  ) async {
    final result = await FilePicker.platform.pickFiles(
      allowMultiple: false,
      type: FileType.custom,
      allowedExtensions: const ['amitiax'],
    );
    final path = result?.files.single.path;
    if (path == null || path.isEmpty) return;
    final lifecycle = ref.read(gameCenterPackageLifecycleProvider);
    try {
      final preview = await lifecycle.previewPackage(
        path,
        expectedExtensionId: detail.extensionId,
      );
      if (!context.mounted) return;
      final confirmed = await showGamePackagePreviewConfirmation(
        context,
        preview,
        actionLabel: '更新',
      );
      if (!confirmed) return;
      final ok = await controller.runPackageOperation(
        detail.extensionId,
        () => lifecycle.commitPackage(
          preview,
          expectedExtensionId: detail.extensionId,
        ),
      );
      if (!context.mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(ok ? '插件更新完成' : '插件更新失败')),
      );
    } catch (e) {
      if (!context.mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('插件更新失败: $e')),
      );
    }
  }

  Future<void> _confirmUninstall(
    BuildContext context,
    WidgetRef ref,
    GameCenterController controller,
    GamePluginDetail detail,
  ) async {
    final lifecycle = ref.read(gameCenterPackageLifecycleProvider);
    try {
      final preview = await lifecycle.previewUninstall(detail.extensionId);
      if (!context.mounted) return;
      final confirmed = await showGamePackageUninstallConfirmation(
        context,
        preview,
        displayName: detail.name,
      );
      if (!confirmed) return;
      final ok = await controller.runPackageOperation(
        detail.extensionId,
        () => lifecycle.commitUninstall(detail.extensionId, preview),
        clearSelectionAfterSuccess: true,
      );
      if (!context.mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(ok ? '插件卸载完成' : '插件卸载失败')),
      );
      if (ok && context.mounted) Navigator.of(context).maybePop();
    } catch (e) {
      if (!context.mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('插件卸载失败: $e')),
      );
    }
  }
}

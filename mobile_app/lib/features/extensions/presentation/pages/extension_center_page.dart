import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/services/extension_service.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/ui_runtime/mobile_extension_slot.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

final installedExtensionViewProvider =
    FutureProvider.autoDispose<ExtensionCenterView>((ref) async {
      final svc = ref.read(extensionServiceProvider);
      return await svc.getExtensionCenterView();
    });

class ExtensionCenterPage extends ConsumerWidget {
  const ExtensionCenterPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final viewAsync = ref.watch(installedExtensionViewProvider);

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '扩展中心 v1',
        navigation: AmitiaAppBarNavigation.back,
        actions: [
          ConstrainedBox(
            constraints: BoxConstraints(maxWidth: 180),
            child: MobileExtensionSlot(
              slotId: 'extension.center.header.action',
              context: {'surface': 'extension-center'},
            ),
          ),
          AmitiaIconButton(
            icon: Icons.refresh_rounded,
            tooltip: '刷新',
            onPressed: () => ref.invalidate(installedExtensionViewProvider),
          ),
        ],
      ),
      body: viewAsync.when(
        data: (view) => _buildContent(context, ref, view),
        loading: () => const AmitiaLoadingState(message: '正在加载扩展能力…'),
        error: (err, _) => AmitiaErrorState(
          message: '扩展中心加载失败：$err',
          onRetry: () => ref.invalidate(installedExtensionViewProvider),
        ),
      ),
    );
  }

  Widget _buildContent(
    BuildContext context,
    WidgetRef ref,
    ExtensionCenterView view,
  ) {
    return ListView(
      padding: EdgeInsets.all(AppSpacing.pagePadding),
      children: [
        Text(
          '扩展能力',
          style: AppTypography.pageTitle(context).copyWith(fontSize: 16),
        ),
        SizedBox(height: AppSpacing.sm),
        Column(
          children: [
            _CenterEntry(
              label: '扩展包',
              icon: Icons.inventory_2_outlined,
              onTap: () => context.push(AppRoutes.extensionsPackages),
            ),
            _CenterEntry(
              label: 'MCP',
              icon: Icons.hub_outlined,
              onTap: () => context.push(AppRoutes.extensionsMcp),
            ),
            _CenterEntry(
              label: 'Agent Skill',
              icon: Icons.auto_awesome_outlined,
              onTap: () => context.push(AppRoutes.extensionsAgentSkills),
            ),
            _CenterEntry(
              label: '兼容 Skill',
              icon: Icons.psychology_outlined,
              onTap: () => context.push(AppRoutes.extensionsSkills),
            ),
            _CenterEntry(
              label: '执行记录',
              icon: Icons.receipt_long_outlined,
              onTap: () => context.push(AppRoutes.extensionsRuns),
            ),
          ],
        ),
        if (view.all.isEmpty) ...[
          SizedBox(height: AppSpacing.sectionGap),
          const AmitiaEmptyState(
            icon: Icons.extension_off_outlined,
            title: '暂无已发现扩展',
            subtitle: '上方入口仍可用于安装扩展包、连接 MCP 或管理 Skill',
          ),
        ],
        if (view.updates.isNotEmpty) ...[
          SizedBox(height: AppSpacing.lg),
          Text(
            '可更新 (${view.updates.length})',
            style: AppTypography.pageTitle(context).copyWith(fontSize: 16),
          ),
          SizedBox(height: AppSpacing.sm),
          ...view.updates.map(
            (card) => AmitiaExtensionCard(
              extensionId: card.extensionId,
              name: card.displayName,
              description: card.description,
              icon: Icons.system_update_alt,
              typeLabel: card.status,
              isInstalled: true,
              isEnabled: card.enabled,
              onAction: () => context.push(AppRoutes.extensionsPackages),
              onToggle: (enabled) async {
                await ref
                    .read(extensionServiceProvider)
                    .setKernelExtensionEnabled(card.extensionId, enabled);
                ref.invalidate(installedExtensionViewProvider);
              },
            ),
          ),
        ],
        if (view.needsAction.isNotEmpty) ...[
          SizedBox(height: AppSpacing.lg),
          Text(
            '需要处理 (${view.needsAction.length})',
            style: AppTypography.pageTitle(context).copyWith(fontSize: 16),
          ),
          SizedBox(height: AppSpacing.sm),
          ...view.needsAction.map(
            (card) => AmitiaExtensionCard(
              extensionId: card.extensionId,
              name: card.displayName,
              description: card.description,
              icon: Icons.warning_amber_outlined,
              typeLabel: card.status,
              isInstalled: true,
              isEnabled: card.enabled,
              onAction: () => context.push(AppRoutes.extensionsPackages),
              onToggle: (enabled) async {
                await ref
                    .read(extensionServiceProvider)
                    .setKernelExtensionEnabled(card.extensionId, enabled);
                ref.invalidate(installedExtensionViewProvider);
              },
            ),
          ),
        ],
        if (view.installed.isNotEmpty) ...[
          Text(
            '已安装',
            style: AppTypography.pageTitle(context).copyWith(fontSize: 16),
          ),
          SizedBox(height: AppSpacing.sm),
          ...view.installed.map(
            (card) => AmitiaExtensionCard(
              extensionId: card.extensionId,
              name: card.displayName,
              description: card.description,
              icon: Icons.extension_outlined,
              typeLabel: card.status,
              isInstalled: true,
              isEnabled: card.enabled,
              isRecommended: false,
              onToggle: (enabled) async {
                final svc = ref.read(extensionServiceProvider);
                await svc.setKernelExtensionEnabled(card.extensionId, enabled);
                ref.invalidate(installedExtensionViewProvider);
              },
            ),
          ),
        ],
        if (view.discover.isNotEmpty) ...[
          SizedBox(height: AppSpacing.lg),
          Text(
            '发现',
            style: AppTypography.pageTitle(context).copyWith(fontSize: 16),
          ),
          SizedBox(height: AppSpacing.sm),
          ...view.discover.map(
            (card) => AmitiaExtensionCard(
              extensionId: card.extensionId,
              name: card.displayName,
              description: card.description,
              icon: Icons.extension_outlined,
              typeLabel: card.status,
              isInstalled: false,
              isEnabled: false,
              isRecommended: card.contributionTags.contains('recommended'),
              actionLabel: '管理',
              onAction: () => context.push(AppRoutes.extensionsPackages),
            ),
          ),
        ],
      ],
    );
  }
}

class _CenterEntry extends StatelessWidget {
  final String label;
  final IconData icon;
  final VoidCallback onTap;

  const _CenterEntry({
    required this.label,
    required this.icon,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Material(
      color: Colors.transparent,
      child: InkWell(
        onTap: onTap,
        child: Container(
          constraints: const BoxConstraints(minHeight: 56),
          padding: const EdgeInsets.symmetric(vertical: 10),
          decoration: BoxDecoration(
            border: Border(
              bottom: BorderSide(color: context.borderSecondary, width: 0.5),
            ),
          ),
          child: Row(
            children: [
              Icon(icon, size: 22, color: context.accentPrimary),
              const SizedBox(width: 14),
              Expanded(child: Text(label, style: AppTypography.body(context))),
              Icon(Icons.chevron_right, size: 20, color: context.textTertiary),
            ],
          ),
        ),
      ),
    );
  }
}

class AmitiaExtensionCard extends StatelessWidget {
  final String extensionId;
  final String name;
  final String description;
  final IconData icon;
  final String typeLabel;
  final bool isInstalled;
  final bool isEnabled;
  final bool isRecommended;
  final String actionLabel;
  final VoidCallback? onAction;
  final ValueChanged<bool>? onToggle;

  const AmitiaExtensionCard({
    super.key,
    required this.extensionId,
    required this.name,
    required this.description,
    required this.icon,
    required this.typeLabel,
    required this.isInstalled,
    required this.isEnabled,
    this.isRecommended = false,
    this.actionLabel = '安装',
    this.onAction,
    this.onToggle,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(vertical: 12),
      decoration: BoxDecoration(
        border: Border(
          bottom: BorderSide(color: context.borderSecondary, width: 0.5),
        ),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Padding(
            padding: const EdgeInsets.only(top: 2),
            child: Icon(icon, size: 22, color: context.accentPrimary),
          ),
          const SizedBox(width: 14),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  name,
                  style: AppTypography.cardTitle(context),
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                ),
                const SizedBox(height: 3),
                Text(
                  [
                    if (typeLabel.trim().isNotEmpty) typeLabel,
                    if (isRecommended) '推荐',
                  ].join(' · '),
                  style: AppTypography.caption(context),
                ),
                if (description.trim().isNotEmpty) ...[
                  const SizedBox(height: 4),
                  Text(
                    description,
                    style: AppTypography.caption(context),
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                  ),
                ],
                const SizedBox(height: 4),
                MobileExtensionSlot(
                  slotId: 'extension.center.card.badge',
                  context: {
                    'extensionId': extensionId,
                    'extension': {
                      'id': extensionId,
                      'name': name,
                      'status': typeLabel,
                      'installed': isInstalled,
                      'enabled': isEnabled,
                    },
                  },
                ),
              ],
            ),
          ),
          const SizedBox(width: 8),
          if (isInstalled)
            Switch(
              value: isEnabled,
              onChanged: onToggle,
              materialTapTargetSize: MaterialTapTargetSize.padded,
            )
          else
            TextButton(onPressed: onAction, child: Text(actionLabel)),
        ],
      ),
    );
  }
}

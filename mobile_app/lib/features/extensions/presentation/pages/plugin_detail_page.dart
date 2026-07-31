import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../shared/mock_data/mock_data.dart';
import '../../../../shared/models/models.dart';

class PluginDetailPage extends ConsumerWidget {
  final String pluginId;

  const PluginDetailPage({super.key, required this.pluginId});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final plugin = MockData.installedExtensions.where((e) => e.id == pluginId && e.type == ExtensionType.plugin).firstOrNull ??
        MockData.recommendedExtensions.where((e) => e.id == pluginId && e.type == ExtensionType.plugin).firstOrNull;

    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: plugin?.name ?? '插件详情', showBackButton: true),
      body: SafeArea(
        top: false,
        child: ListView(
          padding: const EdgeInsets.all(AppSpacing.pagePadding),
          children: [
            if (plugin != null) ...[
              AmitiaCard(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Container(
                          width: 48,
                          height: 48,
                          decoration: BoxDecoration(color: context.accentSoft, borderRadius: AppRadius.brSmall),
                          child: Icon(plugin.icon, size: 24, color: context.accentPrimary),
                        ),
                        const SizedBox(width: 12),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(plugin.name, style: AppTypography.cardTitle(context)),
                              Text(plugin.description, style: AppTypography.caption(context)),
                            ],
                          ),
                        ),
                      ],
                    ),
                    const SizedBox(height: AppSpacing.md),
                    _InfoRow(label: '类型', value: 'Plugin'),
                    _InfoRow(label: '状态', value: plugin.isInstalled ? '已安装' : '未安装'),
                    _InfoRow(label: '启用', value: plugin.isEnabled ? '已启用' : '已禁用'),
                  ],
                ),
              ),
              const SizedBox(height: AppSpacing.md),
              AmitiaButton(
                label: plugin.isInstalled ? '卸载' : '安装',
                isFullWidth: true,
                onPressed: () {
                  ScaffoldMessenger.of(context).showSnackBar(
                    SnackBar(content: Text(plugin.isInstalled ? '已卸载（Mock）' : '已安装（Mock）')),
                  );
                  context.pop();
                },
              ),
            ] else
              Center(child: Text('未找到插件', style: AppTypography.body(context))),
          ],
        ),
      ),
    );
  }
}

class _InfoRow extends StatelessWidget {
  final String label;
  final String value;

  const _InfoRow({required this.label, required this.value});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Text(label, style: AppTypography.caption(context)),
          Text(value, style: AppTypography.body(context)),
        ],
      ),
    );
  }
}

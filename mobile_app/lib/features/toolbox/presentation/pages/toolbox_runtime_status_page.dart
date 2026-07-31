import 'package:flutter/material.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';

class ToolboxRuntimeStatusPage extends StatelessWidget {
  const ToolboxRuntimeStatusPage({super.key});

  static const _components = <(IconData, String, String, String, String)>[
    (Icons.terminal, 'Go Backend', '运行中', 'v1.0.0 · 监听 :18899', 'ok'),
    (Icons.storage, 'SurrealDB', '运行中', 'v2.1.0 · rocksdb 引擎', 'ok'),
    (Icons.scatter_plot_outlined, 'Qdrant', '运行中', 'v1.12.0 · 1284 向量', 'ok'),
    (Icons.extension_outlined, 'MCP Runtime', '已停止', '无运行中的 MCP 服务', 'warn'),
  ];

  BadgeType _badge(String s) {
    switch (s) {
      case 'ok':
        return BadgeType.success;
      case 'warn':
        return BadgeType.warning;
      case 'error':
        return BadgeType.error;
      default:
        return BadgeType.neutral;
    }
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: 'Runtime 状态', showBackButton: true, fallbackRoute: AppRoutes.settingsToolbox),
      body: ListView(
        padding: const EdgeInsets.all(AppSpacing.pagePadding),
        children: [
          Container(
            padding: const EdgeInsets.all(AppSpacing.cardPadding),
            decoration: BoxDecoration(
              color: context.accentSoft,
              borderRadius: AppRadius.brMedium,
            ),
            child: Row(
              children: [
                Icon(Icons.check_circle, color: context.success, size: 28),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text('运行时整体健康', style: AppTypography.cardTitle(context)),
                      Text('3/4 组件运行中 · MCP Runtime 待启动', style: AppTypography.caption(context)),
                    ],
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: AppSpacing.lg),
          ..._components.map((c) => Padding(
                padding: const EdgeInsets.only(bottom: AppSpacing.md),
                child: Container(
                  padding: const EdgeInsets.all(AppSpacing.cardPadding),
                  decoration: BoxDecoration(
                    color: context.surfacePrimary,
                    borderRadius: AppRadius.brMedium,
                    border: Border.all(color: context.borderPrimary, width: 0.5),
                  ),
                  child: Row(
                    children: [
                      Container(
                        width: 44,
                        height: 44,
                        decoration: BoxDecoration(color: context.accentSoft, borderRadius: AppRadius.brSmall),
                        child: Icon(c.$1, size: 22, color: context.accentPrimary),
                      ),
                      const SizedBox(width: 12),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(c.$2, style: AppTypography.cardTitle(context)),
                            const SizedBox(height: 2),
                            Text(c.$4, style: AppTypography.caption(context)),
                          ],
                        ),
                      ),
                      AmitiaStatusBadge(label: c.$3, type: _badge(c.$5)),
                    ],
                  ),
                ),
              )),
          const SizedBox(height: AppSpacing.xl),
        ],
      ),
    );
  }
}

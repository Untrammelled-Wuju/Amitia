import 'package:flutter/material.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';

class ToolboxDatabaseStatusPage extends StatelessWidget {
  const ToolboxDatabaseStatusPage({super.key});

  static const _dbs = <(IconData, String, String, String, String)>[
    (Icons.table_rows_outlined, 'SQLite', '运行中', '248 MB', '2 分钟前'),
    (Icons.storage, 'SurrealDB', '运行中', '1.2 GB', '2 分钟前'),
    (Icons.scatter_plot_outlined, 'Qdrant', '运行中', '320 MB', '2 分钟前'),
  ];

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '数据库状态', showBackButton: true, fallbackRoute: AppRoutes.settingsToolbox),
      body: ListView(
        padding: const EdgeInsets.all(AppSpacing.pagePadding),
        children: [
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 4),
            child: Text('最后检查：2026-07-31 09:34 · 自动每 5 分钟刷新', style: AppTypography.caption(context)),
          ),
          const SizedBox(height: AppSpacing.md),
          ..._dbs.map((d) => Padding(
                padding: const EdgeInsets.only(bottom: AppSpacing.md),
                child: Container(
                  padding: const EdgeInsets.all(AppSpacing.cardPadding),
                  decoration: BoxDecoration(
                    color: context.surfacePrimary,
                    borderRadius: AppRadius.brMedium,
                    border: Border.all(color: context.borderPrimary, width: 0.5),
                  ),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          Container(
                            width: 40,
                            height: 40,
                            decoration: BoxDecoration(color: context.accentSoft, borderRadius: AppRadius.brSmall),
                            child: Icon(d.$1, size: 20, color: context.accentPrimary),
                          ),
                          const SizedBox(width: 12),
                          Expanded(child: Text(d.$2, style: AppTypography.cardTitle(context))),
                          AmitiaStatusBadge(label: d.$3, type: BadgeType.success),
                        ],
                      ),
                      const SizedBox(height: AppSpacing.sm),
                      Row(
                        children: [
                          Text('占用', style: AppTypography.label(context)),
                          const SizedBox(width: 6),
                          Text(d.$4, style: AppTypography.bodySmall(context)),
                          const Spacer(),
                          Text('最后检查', style: AppTypography.label(context)),
                          const SizedBox(width: 6),
                          Text(d.$5, style: AppTypography.bodySmall(context)),
                        ],
                      ),
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

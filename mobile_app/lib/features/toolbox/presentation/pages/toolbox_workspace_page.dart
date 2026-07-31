import 'package:flutter/material.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class ToolboxWorkspacePage extends StatelessWidget {
  const ToolboxWorkspacePage({super.key});

  static const _workspaces = <(IconData, String, String, int, String)>[
    (Icons.work_outline, '主工作区', '日常对话与任务', 128, '刚刚'),
    (Icons.code, '开发工作区', '代码与调试相关', 56, '2 小时前'),
    (Icons.edit_note_outlined, '写作工作区', '内容创作与整理', 34, '昨天'),
    (Icons.school_outlined, '学习工作区', '知识管理与笔记', 72, '3 天前'),
  ];

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '工作区', showBackButton: true, fallbackRoute: AppRoutes.settingsToolbox),
      body: ListView.separated(
        padding: const EdgeInsets.all(AppSpacing.pagePadding),
        itemCount: _workspaces.length,
        separatorBuilder: (_, _) => const SizedBox(height: AppSpacing.md),
        itemBuilder: (context, i) {
          final w = _workspaces[i];
          return Container(
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
                  child: Icon(w.$1, size: 22, color: context.accentPrimary),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(w.$2, style: AppTypography.cardTitle(context)),
                      const SizedBox(height: 2),
                      Text('${w.$3} · ${w.$4} 项 · 更新于 ${w.$5}', style: AppTypography.caption(context)),
                    ],
                  ),
                ),
                Icon(Icons.chevron_right, size: 20, color: context.textTertiary),
              ],
            ),
          );
        },
      ),
    );
  }
}

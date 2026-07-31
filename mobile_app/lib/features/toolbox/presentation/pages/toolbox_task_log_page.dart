import 'package:flutter/material.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';

class ToolboxTaskLogPage extends StatelessWidget {
  const ToolboxTaskLogPage({super.key});

  static const _logs = <(String, String, String, String)>[
    ('整理下载目录', '成功', '09:32', '已整理 156 个文件，分类为 8 个文件夹'),
    ('生成周报摘要', '成功', '09:00', '已生成本周工作摘要，包含 5 个主要进展'),
    ('备份工作文档', '成功', '昨天 18:00', '已备份 89 个文档到指定目录'),
    ('安装开发工具包', '已取消', '昨天 10:20', '用户取消操作'),
    ('分析产品需求文档', '成功', '前天 16:30', '已提取 3 个模块的关键信息'),
    ('调试 API 接口', '失败', '前天 11:30', '请求头格式错误，已记录详情'),
  ];

  BadgeType _badge(String s) {
    switch (s) {
      case '成功':
        return BadgeType.success;
      case '失败':
        return BadgeType.error;
      default:
        return BadgeType.neutral;
    }
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '任务日志', showBackButton: true, fallbackRoute: AppRoutes.settingsToolbox),
      body: ListView.separated(
        padding: const EdgeInsets.all(AppSpacing.pagePadding),
        itemCount: _logs.length,
        separatorBuilder: (_, _) => const SizedBox(height: AppSpacing.md),
        itemBuilder: (context, i) {
          final l = _logs[i];
          return Container(
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
                    Expanded(child: Text(l.$1, style: AppTypography.cardTitle(context))),
                    AmitiaStatusBadge(label: l.$2, type: _badge(l.$2)),
                  ],
                ),
                const SizedBox(height: 4),
                Text(l.$4, style: AppTypography.caption(context)),
                const SizedBox(height: 4),
                Text('完成于 ${l.$3}', style: AppTypography.label(context)),
              ],
            ),
          );
        },
      ),
    );
  }
}

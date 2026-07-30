import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class ToolboxPage extends ConsumerWidget {
  const ToolboxPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final tools = <(IconData, String, String)>[
      (Icons.folder_outlined, '文件浏览', '浏览设备文件系统'),
      (Icons.work_outline, '工作区', '管理你的工作空间'),
      (Icons.task_alt, '任务日志', '查看 Agent 任务记录'),
      (Icons.terminal, '运行日志', '系统运行日志'),
      (Icons.code, 'Prompt Trace', '提示词调用追踪'),
      (Icons.storage, '数据库状态', '查看数据库健康状态'),
      (Icons.devices, '设备状态', '设备信息与资源'),
      (Icons.developer_mode, '开发者选项', '高级调试选项'),
    ];

    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '工具箱', showBackButton: true),
      body: Padding(
        padding: const EdgeInsets.all(AppSpacing.pagePadding),
        child: GridView.builder(
          gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
            crossAxisCount: 2,
            mainAxisSpacing: AppSpacing.md,
            crossAxisSpacing: AppSpacing.md,
            childAspectRatio: 1.05,
          ),
          itemCount: tools.length,
          itemBuilder: (context, index) {
            final tool = tools[index];
            return _ToolCard(
              icon: tool.$1,
              title: tool.$2,
              subtitle: tool.$3,
              onTap: () {
                ScaffoldMessenger.of(context).showSnackBar(
                  SnackBar(
                    content: Text('${tool.$2} · 功能开发中'),
                    duration: const Duration(seconds: 1),
                  ),
                );
              },
            );
          },
        ),
      ),
    );
  }
}

class _ToolCard extends StatelessWidget {
  final IconData icon;
  final String title;
  final String subtitle;
  final VoidCallback onTap;

  const _ToolCard({
    required this.icon,
    required this.title,
    required this.subtitle,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.all(AppSpacing.lg),
        decoration: BoxDecoration(
          color: context.surfacePrimary,
          borderRadius: AppRadius.brMedium,
          border: Border.all(color: context.borderPrimary, width: 0.5),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Container(
              width: 40,
              height: 40,
              decoration: BoxDecoration(
                color: context.accentSoft,
                borderRadius: AppRadius.brSmall,
              ),
              child: Icon(icon, size: 20, color: context.accentPrimary),
            ),
            const Spacer(),
            Text(
              title,
              style: AppTypography.cardTitle(context),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
            ),
            const SizedBox(height: 2),
            Text(
              subtitle,
              style: AppTypography.label(context),
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
            ),
          ],
        ),
      ),
    );
  }
}

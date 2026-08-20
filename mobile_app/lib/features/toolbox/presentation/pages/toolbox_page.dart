import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_drawer.dart';
import 'toolbox_log_page.dart';
import 'toolbox_prompt_trace_page.dart';
import 'toolbox_runtime_status_page.dart';
import 'toolbox_database_status_page.dart';
import 'toolbox_device_status_page.dart';
import 'toolbox_file_browser_page.dart';
import 'toolbox_workspace_page.dart';
import 'toolbox_task_log_page.dart';

class ToolboxPage extends ConsumerWidget {
  const ToolboxPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final isDev = ref.watch(isDeveloperModeProvider);
    final tools = <(IconData, String, String, VoidCallback)>[
      (Icons.folder_outlined, '文件浏览', '浏览设备文件系统',
          () => context.push(AppRoutes.toolboxFileBrowser)),
      (Icons.work_outline, '工作区', '管理你的工作空间',
          () => context.push(AppRoutes.toolboxWorkspace)),
      (Icons.task_alt, '任务日志', '查看 Agent 任务记录',
          () => context.push(AppRoutes.toolboxTaskLog)),
      (Icons.terminal, '运行日志', '系统运行日志',
          () => context.push(AppRoutes.toolboxLog)),
      (Icons.code, 'Prompt Trace', '提示词调用追踪',
          () => context.push(AppRoutes.toolboxPromptTrace)),
      (Icons.memory, 'Runtime 状态', '运行时组件健康',
          () => context.push(AppRoutes.toolboxRuntimeStatus)),
      (Icons.storage, '数据库状态', '查看数据库健康状态',
          () => context.push(AppRoutes.toolboxDatabaseStatus)),
      (Icons.devices, '设备状态', '设备信息与资源',
          () => context.push(AppRoutes.toolboxDeviceStatus)),
      if (isDev)
        (Icons.developer_mode, '开发者选项', '高级调试选项', () => context.push(AppRoutes.developer)),
    ];

    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '工具箱', showBackButton: true, fallbackRoute: AppRoutes.settings),
      body: Padding(
        padding: EdgeInsets.all(AppSpacing.pagePadding),
        child: GridView.builder(
          gridDelegate: SliverGridDelegateWithFixedCrossAxisCount(
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
              onTap: tool.$4,
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
        padding: EdgeInsets.all(AppSpacing.lg),
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

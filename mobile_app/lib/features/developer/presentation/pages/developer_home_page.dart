import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';

class DeveloperHomePage extends ConsumerWidget {
  const DeveloperHomePage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return AmitiaScaffold(
      appBar: const AmitiaAppBar(
        title: '开发者模式',
        showBackButton: true,
      ),
      body: SafeArea(
        top: false,
        child: ListView(
          padding: const EdgeInsets.symmetric(vertical: AppSpacing.lg),
          children: [
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
              child: AmitiaCard(
                backgroundColor: context.accentSoft,
                border: Border.all(color: context.accentPrimary.withValues(alpha: 0.2), width: 0.5),
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Container(
                      width: 40,
                      height: 40,
                      decoration: BoxDecoration(
                        color: context.accentPrimary,
                        borderRadius: AppRadius.brSmall,
                      ),
                      child: Icon(Icons.code, size: 22, color: Colors.white),
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text('开发者模式已开启', style: AppTypography.cardTitle(context)),
                          const SizedBox(height: 4),
                          Text(
                            '开发者模式提供扩展内核、WASM Runtime、Hook 中心、可信服务等高级功能的管理入口。请谨慎操作。',
                            style: AppTypography.caption(context),
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
              ),
            ),
            const SizedBox(height: AppSpacing.xl),
            const AmitiaSectionHeader(title: '功能入口'),
            const SizedBox(height: AppSpacing.sm),
            ..._buildEntries(context),
          ],
        ),
      ),
    );
  }

  List<Widget> _buildEntries(BuildContext context) {
    final entries = <_DevEntry>[
      _DevEntry(
        title: '扩展内核首页',
        subtitle: '内核状态与扩展包管理',
        icon: Icons.memory,
        route: AppRoutes.developerKernel,
      ),
      _DevEntry(
        title: 'WASM Runtime',
        subtitle: '模块定义、加载与配额管理',
        icon: Icons.extension_outlined,
        route: AppRoutes.kernelPage('wasm'),
      ),
      _DevEntry(
        title: 'Hook 中心',
        subtitle: 'Hook 点、贡献与熔断器管理',
        icon: Icons.account_tree_outlined,
        route: AppRoutes.kernelPage('hooks'),
      ),
      _DevEntry(
        title: '可信服务',
        subtitle: '服务注册、隔离与调用',
        icon: Icons.verified_user_outlined,
        route: AppRoutes.kernelPage('trusted-services'),
      ),
      _DevEntry(
        title: '任务运行时',
        subtitle: '任务执行、取消与检查点',
        icon: Icons.play_circle_outline,
        route: AppRoutes.kernelPage('tasks'),
      ),
      _DevEntry(
        title: '事件中心',
        subtitle: '事件订阅、历史与死信',
        icon: Icons.notifications_active_outlined,
        route: AppRoutes.kernelPage('events'),
      ),
      _DevEntry(
        title: '调度中心',
        subtitle: '定时任务与调度执行',
        icon: Icons.schedule_outlined,
        route: AppRoutes.kernelPage('schedules'),
      ),
      _DevEntry(
        title: '桌面贡献中心',
        subtitle: '快捷键、菜单与窗口贡献',
        icon: Icons.desktop_windows_outlined,
        route: AppRoutes.kernelPage('desktop-contributions'),
      ),
      _DevEntry(
        title: '更新中心',
        subtitle: '可用更新与更新历史',
        icon: Icons.system_update_outlined,
        route: AppRoutes.kernelPage('updates'),
      ),
      _DevEntry(
        title: '诊断控制台',
        subtitle: '日志流与级别筛选',
        icon: Icons.terminal,
        route: AppRoutes.kernelPage('dev-console'),
      ),
      _DevEntry(
        title: '迁移与灰度',
        subtitle: '迁移计划与灰度发布管理',
        icon: Icons.merge_type_outlined,
        route: AppRoutes.kernelPage('migrations'),
      ),
      _DevEntry(
        title: '开发模式',
        subtitle: '开发工作区与热重载',
        icon: Icons.developer_mode_outlined,
        route: AppRoutes.kernelPage('dev-mode'),
      ),
    ];

    return entries.map((e) {
      return Padding(
        padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: 2),
        child: AmitiaListTile(
          leading: Container(
            width: 36,
            height: 36,
            decoration: BoxDecoration(
              color: context.accentSoft,
              borderRadius: AppRadius.brSmall,
            ),
            child: Icon(e.icon, size: 20, color: context.accentPrimary),
          ),
          title: e.title,
          subtitle: e.subtitle,
          trailing: Icon(Icons.chevron_right, color: context.textTertiary, size: 20),
          onTap: () => context.push(e.route),
        ),
      );
    }).toList();
  }
}

class _DevEntry {
  final String title;
  final String subtitle;
  final IconData icon;
  final String route;

  _DevEntry({
    required this.title,
    required this.subtitle,
    required this.icon,
    required this.route,
  });
}

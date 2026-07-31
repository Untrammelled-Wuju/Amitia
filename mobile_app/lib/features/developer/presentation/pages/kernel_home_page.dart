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

class KernelHomePage extends ConsumerWidget {
  const KernelHomePage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return AmitiaScaffold(
      appBar: const AmitiaAppBar(
        title: '扩展内核',
        showBackButton: true,
        fallbackRoute: AppRoutes.developer,
      ),
      body: SafeArea(
        top: false,
        child: ListView(
          padding: const EdgeInsets.symmetric(vertical: AppSpacing.lg),
          children: [
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
              child: _buildKernelStatusCard(context),
            ),
            const SizedBox(height: AppSpacing.xl),
            const AmitiaSectionHeader(title: '内核模块'),
            const SizedBox(height: AppSpacing.sm),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
              child: _buildModuleGrid(context),
            ),
            const SizedBox(height: AppSpacing.xl),
            const AmitiaSectionHeader(title: '更多入口'),
            const SizedBox(height: AppSpacing.sm),
            ..._buildMoreEntries(context),
          ],
        ),
      ),
    );
  }

  Widget _buildKernelStatusCard(BuildContext context) {
    return AmitiaCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                width: 44,
                height: 44,
                decoration: BoxDecoration(
                  color: context.accentSoft,
                  borderRadius: AppRadius.brSmall,
                ),
                child: Icon(Icons.memory, size: 24, color: context.accentPrimary),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text('扩展内核', style: AppTypography.cardTitle(context)),
                    const SizedBox(height: 2),
                    Text('v1.2.0 · 运行中', style: AppTypography.caption(context)),
                  ],
                ),
              ),
              const AmitiaStatusBadge(label: '运行中', type: BadgeType.success),
            ],
          ),
          const SizedBox(height: AppSpacing.md),
          Row(
            children: [
              Expanded(
                child: _buildStatItem(context, 'WASM 模块', '3', Icons.extension_outlined),
              ),
              Container(width: 1, height: 36, color: context.borderPrimary),
              Expanded(
                child: _buildStatItem(context, 'Hook 点', '5', Icons.account_tree_outlined),
              ),
              Container(width: 1, height: 36, color: context.borderPrimary),
              Expanded(
                child: _buildStatItem(context, '可信服务', '4', Icons.verified_user_outlined),
              ),
            ],
          ),
        ],
      ),
    );
  }

  Widget _buildStatItem(BuildContext context, String label, String value, IconData icon) {
    return Column(
      children: [
        Icon(icon, size: 18, color: context.accentPrimary),
        const SizedBox(height: 4),
        Text(value, style: AppTypography.cardTitle(context).copyWith(fontSize: 18)),
        const SizedBox(height: 2),
        Text(label, style: AppTypography.label(context)),
      ],
    );
  }

  Widget _buildModuleGrid(BuildContext context) {
    final modules = <_KernelModule>[
      _KernelModule(title: 'WASM Runtime', subtitle: '3 个模块', icon: Icons.extension_outlined, route: AppRoutes.kernelPage('wasm')),
      _KernelModule(title: 'Hook 中心', subtitle: '5 个 Hook 点', icon: Icons.account_tree_outlined, route: AppRoutes.kernelPage('hooks')),
      _KernelModule(title: '可信服务', subtitle: '4 个服务', icon: Icons.verified_user_outlined, route: AppRoutes.kernelPage('trusted-services')),
      _KernelModule(title: '任务运行时', subtitle: '4 个任务', icon: Icons.play_circle_outline, route: AppRoutes.kernelPage('tasks')),
    ];

    return GridView.builder(
      shrinkWrap: true,
      physics: const NeverScrollableScrollPhysics(),
      gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
        crossAxisCount: 2,
        mainAxisSpacing: AppSpacing.sm,
        crossAxisSpacing: AppSpacing.sm,
        childAspectRatio: 1.6,
      ),
      itemCount: modules.length,
      itemBuilder: (context, index) {
        final m = modules[index];
        return AmitiaCard(
          padding: const EdgeInsets.all(AppSpacing.md),
          onTap: () => context.push(m.route),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Container(
                width: 32,
                height: 32,
                decoration: BoxDecoration(
                  color: context.accentSoft,
                  borderRadius: AppRadius.brSmall,
                ),
                child: Icon(m.icon, size: 18, color: context.accentPrimary),
              ),
              const SizedBox(height: AppSpacing.sm),
              Text(m.title, style: AppTypography.cardTitle(context).copyWith(fontSize: 14)),
              const SizedBox(height: 2),
              Text(m.subtitle, style: AppTypography.label(context)),
            ],
          ),
        );
      },
    );
  }

  List<Widget> _buildMoreEntries(BuildContext context) {
    final entries = <_DevEntry>[
      _DevEntry(title: '事件中心', subtitle: '事件订阅与历史', icon: Icons.notifications_active_outlined, route: AppRoutes.kernelPage('events')),
      _DevEntry(title: '调度中心', subtitle: '定时任务管理', icon: Icons.schedule_outlined, route: AppRoutes.kernelPage('schedules')),
      _DevEntry(title: '桌面贡献中心', subtitle: '快捷键与窗口', icon: Icons.desktop_windows_outlined, route: AppRoutes.kernelPage('desktop-contributions')),
      _DevEntry(title: '更新中心', subtitle: '版本更新与回滚', icon: Icons.system_update_outlined, route: AppRoutes.kernelPage('updates')),
      _DevEntry(title: '诊断控制台', subtitle: '日志与诊断', icon: Icons.terminal, route: AppRoutes.kernelPage('dev-console')),
      _DevEntry(title: '迁移与灰度', subtitle: '数据迁移与灰度发布', icon: Icons.merge_type_outlined, route: AppRoutes.kernelPage('migrations')),
      _DevEntry(title: '开发模式', subtitle: '工作区与热重载', icon: Icons.developer_mode_outlined, route: AppRoutes.kernelPage('dev-mode')),
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

class _KernelModule {
  final String title;
  final String subtitle;
  final IconData icon;
  final String route;

  _KernelModule({required this.title, required this.subtitle, required this.icon, required this.route});
}

class _DevEntry {
  final String title;
  final String subtitle;
  final IconData icon;
  final String route;

  _DevEntry({required this.title, required this.subtitle, required this.icon, required this.route});
}

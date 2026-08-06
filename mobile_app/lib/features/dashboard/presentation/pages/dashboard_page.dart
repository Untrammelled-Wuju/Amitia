import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/services/providers.dart';

final dashboardHealthProvider = FutureProvider<Map<String, dynamic>?>((ref) async {
  final svc = ref.read(systemServiceProvider);
  return svc.health();
});

final dashboardStatsProvider = FutureProvider<Map<String, dynamic>?>((ref) async {
  final svc = ref.read(systemServiceProvider);
  return svc.chatStats();
});

class DashboardPage extends ConsumerStatefulWidget {
  const DashboardPage({super.key});

  @override
  ConsumerState<DashboardPage> createState() => _DashboardPageState();
}

class _DashboardPageState extends ConsumerState<DashboardPage> {
  int _selectedTab = 0;

  BadgeType _statusBadgeType(String status) {
    if (status.contains('运行') || status.contains('已连接') || status.contains('正常') || status.contains('alive')) {
      return BadgeType.success;
    } else if (status.contains('空闲') || status.contains('低')) {
      return BadgeType.accent;
    } else if (status.contains('停止') || status.contains('失败') || status.contains('高')) {
      return BadgeType.error;
    }
    return BadgeType.neutral;
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '概览', navigation: AmitiaAppBarNavigation.back),
      body: Column(
        children: [
          Padding(
            padding: const EdgeInsets.all(AppSpacing.pagePadding),
            child: AmitiaSegmentedControl(
              segments: const ['运行概览', '数据概览'],
              selectedIndex: _selectedTab,
              onChanged: (i) => setState(() => _selectedTab = i),
            ),
          ),
          Expanded(
            child: _selectedTab == 0 ? _buildRunOverview() : _buildDataOverview(),
          ),
        ],
      ),
    );
  }

  Widget _buildRunOverview() {
    final healthAsync = ref.watch(dashboardHealthProvider);
    return healthAsync.when(
      data: (health) {
        final status = health?['status'] as Map<String, dynamic>?;
        final backendStatus = status?['database'] == true ? '运行中' : '已停止';
        final modelStatus = status?['orchestratorReady'] == true ? '就绪' : '未就绪';
        return ListView(
          padding: const EdgeInsets.fromLTRB(AppSpacing.pagePadding, 0, AppSpacing.pagePadding, AppSpacing.pagePadding),
          children: [
            AmitiaSectionHeader(title: '系统状态'),
            const SizedBox(height: AppSpacing.md),
            _StatusGrid(
              items: [
                _StatusItem(label: '后端', value: backendStatus, icon: Icons.dns_outlined, type: _statusBadgeType(backendStatus)),
                _StatusItem(label: 'Agent Runtime', value: modelStatus, icon: Icons.auto_awesome, type: _statusBadgeType(modelStatus)),
                _StatusItem(label: '模型', value: status?['readinessReady'] == true ? '已加载' : '未加载', icon: Icons.psychology_outlined, type: _statusBadgeType(status?['readinessReady'] == true ? '正常' : '未加载')),
                _StatusItem(label: '数据库', value: status?['database'] == true ? '正常' : '异常', icon: Icons.storage, type: _statusBadgeType(status?['database'] == true ? '正常' : '异常')),
                _StatusItem(label: '渠道', value: '未连接', icon: Icons.wechat_outlined, type: BadgeType.neutral),
                _StatusItem(label: '访问风险', value: '安全', icon: Icons.shield_outlined, type: BadgeType.success),
              ],
            ),
            const SizedBox(height: AppSpacing.sectionGap),
            AmitiaSectionHeader(title: '应用信息'),
            const SizedBox(height: AppSpacing.md),
            AmitiaCard(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      Text('状态', style: AppTypography.bodySmall(context)),
                      Text(status?['status']?.toString() ?? 'unknown', style: AppTypography.bodySmall(context).copyWith(fontWeight: FontWeight.w600)),
                    ],
                  ),
                  const SizedBox(height: AppSpacing.sm),
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      Text('就绪组件', style: AppTypography.bodySmall(context)),
                      Text('${status?['readyCount'] ?? 0}', style: AppTypography.bodySmall(context).copyWith(fontWeight: FontWeight.w600)),
                    ],
                  ),
                  const SizedBox(height: AppSpacing.sm),
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      Text('检查时间', style: AppTypography.bodySmall(context)),
                      Text(DateTime.now().toString().substring(0, 19), style: AppTypography.bodySmall(context).copyWith(fontWeight: FontWeight.w600)),
                    ],
                  ),
                ],
              ),
            ),
            const SizedBox(height: AppSpacing.sectionGap),
            AmitiaButton(
              label: '刷新状态',
              icon: Icons.refresh,
              isFullWidth: true,
              isSecondary: true,
              onPressed: () => ref.invalidate(dashboardHealthProvider),
            ),
          ],
        );
      },
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (err, _) => Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(Icons.error_outline, size: 48, color: context.error),
            const SizedBox(height: AppSpacing.md),
            Text('无法连接到后端', style: AppTypography.body(context)),
            const SizedBox(height: AppSpacing.sm),
            Text(err.toString(), style: AppTypography.caption(context)),
            const SizedBox(height: AppSpacing.md),
            AmitiaButton(
              label: '重试',
              icon: Icons.refresh,
              isSecondary: true,
              onPressed: () => ref.invalidate(dashboardHealthProvider),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildDataOverview() {
    final statsAsync = ref.watch(dashboardStatsProvider);
    final characterCount = ref.watch(characterListProvider);
    final memoryCount = ref.watch(memoryListProvider);
    return ListView(
      padding: const EdgeInsets.fromLTRB(AppSpacing.pagePadding, 0, AppSpacing.pagePadding, AppSpacing.pagePadding),
      children: [
        AmitiaSectionHeader(title: '数据统计'),
        const SizedBox(height: AppSpacing.md),
        GridView.count(
          shrinkWrap: true,
          physics: const NeverScrollableScrollPhysics(),
          crossAxisCount: 3,
          mainAxisSpacing: AppSpacing.md,
          crossAxisSpacing: AppSpacing.md,
          childAspectRatio: 1.0,
          children: [
            _StatCard(stat: _StatItem(label: '角色', value: characterCount.valueOrNull?.length ?? 0, icon: Icons.people_outline)),
            _StatCard(stat: _StatItem(label: '记忆', value: memoryCount.valueOrNull?.length ?? 0, icon: Icons.memory)),
            _StatCard(stat: _StatItem(label: '对话', value: statsAsync.valueOrNull?['conversationCount'] ?? 0, icon: Icons.chat_bubble_outline)),
          ],
        ),
        const SizedBox(height: AppSpacing.sectionGap),
        AmitiaButton(
          label: '刷新数据',
          icon: Icons.refresh,
          isFullWidth: true,
          isSecondary: true,
          onPressed: () {
            ref.invalidate(dashboardStatsProvider);
            ref.invalidate(characterListProvider);
            ref.invalidate(memoryListProvider);
          },
        ),
      ],
    );
  }
}

class _StatusItem {
  final String label;
  final String value;
  final IconData icon;
  final BadgeType type;

  _StatusItem({required this.label, required this.value, required this.icon, required this.type});
}

class _StatusGrid extends StatelessWidget {
  final List<_StatusItem> items;

  const _StatusGrid({required this.items});

  @override
  Widget build(BuildContext context) {
    return GridView.count(
      shrinkWrap: true,
      physics: const NeverScrollableScrollPhysics(),
      crossAxisCount: 2,
      mainAxisSpacing: AppSpacing.md,
      crossAxisSpacing: AppSpacing.md,
      childAspectRatio: 2.4,
      children: items.map((item) => _StatusCard(item: item)).toList(),
    );
  }
}

class _StatusCard extends StatelessWidget {
  final _StatusItem item;

  const _StatusCard({required this.item});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(AppSpacing.md),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Row(
        children: [
          Container(
            width: 40,
            height: 40,
            decoration: BoxDecoration(
              color: context.accentSoft,
              borderRadius: AppRadius.brSmall,
            ),
            child: Icon(item.icon, size: 20, color: context.accentPrimary),
          ),
          const SizedBox(width: AppSpacing.md),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                Text(item.label, style: AppTypography.label(context)),
                const SizedBox(height: 2),
                Text(item.value, style: AppTypography.bodySmall(context).copyWith(fontWeight: FontWeight.w500)),
              ],
            ),
          ),
          AmitiaStatusBadge(
            label: item.value.contains('运行') || item.value.contains('正常') || item.value.contains('就绪') || item.value == '已加载' ? '正常' : '注意',
            type: item.type,
          ),
        ],
      ),
    );
  }
}

class _StatItem {
  final String label;
  final int value;
  final IconData icon;

  _StatItem({required this.label, required this.value, required this.icon});
}

class _StatCard extends StatelessWidget {
  final _StatItem stat;

  const _StatCard({required this.stat});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(AppSpacing.md),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(stat.icon, size: 24, color: context.accentPrimary),
          const SizedBox(height: AppSpacing.sm),
          Text(
            stat.value.toString(),
            style: AppTypography.cardTitle(context).copyWith(fontSize: 22, fontWeight: FontWeight.w700),
          ),
          const SizedBox(height: 2),
          Text(stat.label, style: AppTypography.label(context)),
        ],
      ),
    );
  }
}

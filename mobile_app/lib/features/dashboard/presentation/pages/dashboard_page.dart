import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../shared/mock_data/mock_data.dart';

class DashboardPage extends ConsumerStatefulWidget {
  const DashboardPage({super.key});

  @override
  ConsumerState<DashboardPage> createState() => _DashboardPageState();
}

class _DashboardPageState extends ConsumerState<DashboardPage> {
  int _selectedTab = 0;

  void _snack(String message) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(message), duration: const Duration(seconds: 1)),
    );
  }

  BadgeType _statusBadgeType(String status) {
    if (status.contains('运行') || status.contains('已连接') || status.contains('正常')) {
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
    final info = MockDashboard.runInfo;
    return ListView(
      padding: const EdgeInsets.fromLTRB(AppSpacing.pagePadding, 0, AppSpacing.pagePadding, AppSpacing.pagePadding),
      children: [
        AmitiaSectionHeader(title: '系统状态'),
        const SizedBox(height: AppSpacing.md),
        _StatusGrid(
          items: [
            _StatusItem(label: '后端', value: info.backendStatus, icon: Icons.dns_outlined, type: _statusBadgeType(info.backendStatus)),
            _StatusItem(label: 'Agent Runtime', value: info.agentRuntimeStatus, icon: Icons.auto_awesome, type: _statusBadgeType(info.agentRuntimeStatus)),
            _StatusItem(label: '模型', value: info.modelStatus, icon: Icons.psychology_outlined, type: _statusBadgeType(info.modelStatus)),
            _StatusItem(label: '数据库', value: info.databaseStatus, icon: Icons.storage, type: _statusBadgeType(info.databaseStatus)),
            _StatusItem(label: '渠道', value: info.channelStatus, icon: Icons.wechat_outlined, type: _statusBadgeType(info.channelStatus)),
            _StatusItem(label: '访问风险', value: info.accessRisk, icon: Icons.shield_outlined, type: _statusBadgeType(info.accessRisk)),
          ],
        ),
        const SizedBox(height: AppSpacing.sectionGap),
        AmitiaSectionHeader(title: '最近错误'),
        const SizedBox(height: AppSpacing.md),
        Container(
          decoration: BoxDecoration(
            color: context.surfacePrimary,
            borderRadius: AppRadius.brMedium,
            border: Border.all(color: context.borderPrimary, width: 0.5),
          ),
          child: Column(
            children: [
              for (int i = 0; i < info.recentErrors.length; i++) ...[
                _ErrorTile(message: info.recentErrors[i]),
                if (i < info.recentErrors.length - 1)
                  Divider(height: 1, indent: AppSpacing.lg, color: context.borderSecondary),
              ],
            ],
          ),
        ),
        const SizedBox(height: AppSpacing.sectionGap),
        AmitiaSectionHeader(title: '最近任务'),
        const SizedBox(height: AppSpacing.md),
        Container(
          decoration: BoxDecoration(
            color: context.surfacePrimary,
            borderRadius: AppRadius.brMedium,
            border: Border.all(color: context.borderPrimary, width: 0.5),
          ),
          child: Column(
            children: [
              for (int i = 0; i < info.recentTasks.length; i++) ...[
                _TaskTile(task: info.recentTasks[i]),
                if (i < info.recentTasks.length - 1)
                  Divider(height: 1, indent: AppSpacing.lg, color: context.borderSecondary),
              ],
            ],
          ),
        ),
        const SizedBox(height: AppSpacing.sectionGap),
        AmitiaSectionHeader(title: '心理状态摘要'),
        const SizedBox(height: AppSpacing.md),
        AmitiaCard(
          child: Row(
            children: [
              Container(
                width: 44,
                height: 44,
                decoration: BoxDecoration(
                  color: context.accentSoft,
                  borderRadius: AppRadius.brSmall,
                ),
                child: Icon(Icons.mood, size: 24, color: context.accentPrimary),
              ),
              const SizedBox(width: AppSpacing.md),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text('Amitia', style: AppTypography.cardTitle(context)),
                    Text(info.psycheSummary, style: AppTypography.caption(context)),
                  ],
                ),
              ),
              AmitiaStatusBadge(label: '稳定', type: BadgeType.success),
            ],
          ),
        ),
        const SizedBox(height: AppSpacing.sectionGap),
        AmitiaButton(
          label: '进入诊断',
          icon: Icons.health_and_safety_outlined,
          isFullWidth: true,
          isSecondary: true,
          onPressed: () => _snack('正在打开诊断工具...'),
        ),
      ],
    );
  }

  Widget _buildDataOverview() {
    final info = MockDashboard.dataInfo;
    final stats = [
      _StatItem(label: '对话', value: info.conversationCount, icon: Icons.chat_bubble_outline),
      _StatItem(label: '角色', value: info.characterCount, icon: Icons.people_outline),
      _StatItem(label: '记忆', value: info.memoryCount, icon: Icons.memory),
      _StatItem(label: '主动消息', value: info.proactiveMessageCount, icon: Icons.campaign_outlined),
      _StatItem(label: '扩展', value: info.extensionCount, icon: Icons.extension_outlined),
      _StatItem(label: '错误', value: info.errorCount, icon: Icons.error_outline),
    ];
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
          children: stats.map((s) => _StatCard(stat: s)).toList(),
        ),
        const SizedBox(height: AppSpacing.sectionGap),
        AmitiaSectionHeader(title: '存储占用'),
        const SizedBox(height: AppSpacing.md),
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Text('已使用', style: AppTypography.caption(context)),
                  Text(info.storageUsage, style: AppTypography.cardTitle(context).copyWith(color: context.accentPrimary)),
                ],
              ),
              const SizedBox(height: AppSpacing.md),
              AmitiaProgressBar(progress: 0.35, height: 8),
              const SizedBox(height: AppSpacing.sm),
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Text('总容量 10 GB', style: AppTypography.label(context)),
                  Text('35%', style: AppTypography.label(context)),
                ],
              ),
            ],
          ),
        ),
        const SizedBox(height: AppSpacing.sectionGap),
        AmitiaSectionHeader(title: '使用趋势'),
        const SizedBox(height: AppSpacing.md),
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('近 7 天对话量', style: AppTypography.caption(context)),
              const SizedBox(height: AppSpacing.lg),
              _BarChart(),
            ],
          ),
        ),
        const SizedBox(height: AppSpacing.sectionGap),
        AmitiaSectionHeader(title: '最近导入'),
        const SizedBox(height: AppSpacing.md),
        Container(
          decoration: BoxDecoration(
            color: context.surfacePrimary,
            borderRadius: AppRadius.brMedium,
            border: Border.all(color: context.borderPrimary, width: 0.5),
          ),
          child: Column(
            children: [
              for (int i = 0; i < info.recentImports.length; i++) ...[
                _ImportTile(text: info.recentImports[i]),
                if (i < info.recentImports.length - 1)
                  Divider(height: 1, indent: AppSpacing.lg, color: context.borderSecondary),
              ],
            ],
          ),
        ),
        const SizedBox(height: AppSpacing.sectionGap),
        AmitiaSectionHeader(title: '错误统计'),
        const SizedBox(height: AppSpacing.md),
        AmitiaCard(
          child: Column(
            children: [
              _ErrorStatRow(label: '模型调用错误', count: 2, color: context.warning),
              const SizedBox(height: AppSpacing.sm),
              _ErrorStatRow(label: '渠道连接错误', count: 1, color: context.error),
              const SizedBox(height: AppSpacing.sm),
              _ErrorStatRow(label: 'Agent 执行错误', count: 0, color: context.success),
            ],
          ),
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
          AmitiaStatusBadge(label: item.value.contains('运行') || item.value.contains('已连接') || item.value.contains('正常') ? '正常' : item.value.contains('低') ? '安全' : '注意', type: item.type),
        ],
      ),
    );
  }
}

class _ErrorTile extends StatelessWidget {
  final String message;

  const _ErrorTile({required this.message});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.lg, vertical: 13),
      child: Row(
        children: [
          Icon(Icons.error_outline, size: 18, color: context.error),
          const SizedBox(width: AppSpacing.md),
          Expanded(child: Text(message, style: AppTypography.bodySmall(context))),
          Icon(Icons.chevron_right, size: 18, color: context.textTertiary),
        ],
      ),
    );
  }
}

class _TaskTile extends StatelessWidget {
  final String task;

  const _TaskTile({required this.task});

  @override
  Widget build(BuildContext context) {
    final isRunning = task.contains('进行中');
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.lg, vertical: 13),
      child: Row(
        children: [
          Icon(
            isRunning ? Icons.play_circle_outline : Icons.check_circle_outline,
            size: 18,
            color: isRunning ? context.accentPrimary : context.success,
          ),
          const SizedBox(width: AppSpacing.md),
          Expanded(child: Text(task, style: AppTypography.bodySmall(context))),
          if (isRunning)
            SizedBox(
              width: 14,
              height: 14,
              child: CircularProgressIndicator(strokeWidth: 2, color: context.accentPrimary),
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

class _BarChart extends StatelessWidget {
  final _data = [
    ('周一', 0.45),
    ('周二', 0.62),
    ('周三', 0.38),
    ('周四', 0.75),
    ('周五', 0.55),
    ('周六', 0.88),
    ('周日', 0.70),
  ];

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      height: 120,
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.end,
        children: _data.map((d) {
          return Expanded(
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: 4),
              child: Column(
                mainAxisAlignment: MainAxisAlignment.end,
                children: [
                  Flexible(
                    child: FractionallySizedBox(
                      heightFactor: d.$2,
                      child: Container(
                        decoration: BoxDecoration(
                          color: context.accentPrimary.withValues(alpha: 0.15 + d.$2 * 0.5),
                          borderRadius: const BorderRadius.only(
                            topLeft: Radius.circular(4),
                            topRight: Radius.circular(4),
                          ),
                        ),
                      ),
                    ),
                  ),
                  const SizedBox(height: 4),
                  Text(d.$1, style: AppTypography.label(context)),
                ],
              ),
            ),
          );
        }).toList(),
      ),
    );
  }
}

class _ImportTile extends StatelessWidget {
  final String text;

  const _ImportTile({required this.text});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.lg, vertical: 13),
      child: Row(
        children: [
          Icon(Icons.file_download_outlined, size: 18, color: context.accentPrimary),
          const SizedBox(width: AppSpacing.md),
          Expanded(child: Text(text, style: AppTypography.bodySmall(context))),
          AmitiaStatusBadge(label: '已完成', type: BadgeType.success),
        ],
      ),
    );
  }
}

class _ErrorStatRow extends StatelessWidget {
  final String label;
  final int count;
  final Color color;

  const _ErrorStatRow({required this.label, required this.count, required this.color});

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Container(
          width: 8,
          height: 8,
          decoration: BoxDecoration(color: color, shape: BoxShape.circle),
        ),
        const SizedBox(width: AppSpacing.md),
        Expanded(child: Text(label, style: AppTypography.bodySmall(context))),
        Text(
          '$count 次',
          style: AppTypography.bodySmall(context).copyWith(
            color: color,
            fontWeight: FontWeight.w600,
          ),
        ),
      ],
    );
  }
}

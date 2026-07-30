import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';

class CharacterDebugPage extends ConsumerStatefulWidget {
  final String characterId;

  const CharacterDebugPage({super.key, required this.characterId});

  @override
  ConsumerState<CharacterDebugPage> createState() => _CharacterDebugPageState();
}

class _CharacterDebugPageState extends ConsumerState<CharacterDebugPage> {
  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '调试模式',
        showBackButton: true,
        actions: [
          AmitiaIconButton(
            icon: Icons.warning_amber,
            color: context.warning,
            onPressed: () {
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(content: Text('开发者模式 - 所有操作均为Mock'), duration: Duration(seconds: 2)),
              );
            },
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: ListView(
          padding: const EdgeInsets.all(AppSpacing.pagePadding),
          children: [
            _buildWarningBanner(context),
            const SizedBox(height: AppSpacing.sectionGap),
            AmitiaSectionHeader(title: '调度操作'),
            const SizedBox(height: AppSpacing.sm),
            _buildDebugAction(
              context,
              '重新生成今日作息',
              '根据当前时间和规则重新生成今日作息计划',
              Icons.refresh,
              context.accentPrimary,
              () => _showConfirmDialog(
                context,
                '重新生成今日作息',
                '这将覆盖当前的作息计划，根据生活规则重新生成。确定继续吗？',
                () => _showResultDialog(context, '重新生成今日作息', '已成功重新生成今日作息计划，包含6个时段。'),
              ),
            ),
            _buildDebugAction(
              context,
              '触发主动消息处理',
              '立即执行一次主动消息规则检查和触发',
              Icons.notifications_active_outlined,
              context.info,
              () => _showConfirmDialog(
                context,
                '触发主动消息处理',
                '将立即检查所有主动消息规则并尝试触发。确定继续吗？',
                () => _showResultDialog(context, '触发主动消息处理', '已检查5条规则，其中2条满足触发条件，已发送主动消息。'),
              ),
            ),
            _buildDebugAction(
              context,
              '处理延迟回复',
              '处理因网络或其他原因延迟的回复消息',
              Icons.schedule_send_outlined,
              context.warning,
              () => _showConfirmDialog(
                context,
                '处理延迟回复',
                '将处理所有待发送的延迟回复消息。确定继续吗？',
                () => _showResultDialog(context, '处理延迟回复', '已处理3条延迟回复，全部发送成功。'),
              ),
            ),
            _buildDebugAction(
              context,
              '触发每日重生',
              '执行每日角色重生流程，重置部分状态',
              Icons.auto_awesome_outlined,
              context.accentSecondary,
              () => _showConfirmDialog(
                context,
                '触发每日重生',
                '这将执行角色每日重生流程，包括重置情绪、更新作息等。此操作不可撤销，确定继续吗？',
                () => _showResultDialog(context, '触发每日重生', '每日重生已完成。情绪已重置，作息已更新，记忆已归档。'),
              ),
            ),
            const SizedBox(height: AppSpacing.sectionGap),
            AmitiaSectionHeader(title: '任务管理'),
            const SizedBox(height: AppSpacing.sm),
            _buildDebugAction(
              context,
              '查看任务',
              '查看当前角色的所有调度任务',
              Icons.list_alt,
              context.success,
              () => _showTaskListDialog(context),
            ),
            _buildDebugAction(
              context,
              '取消所有任务',
              '取消该角色的所有待执行调度任务',
              Icons.cancel_outlined,
              context.error,
              () => _showConfirmDialog(
                context,
                '取消所有任务',
                '这将取消该角色的所有待执行调度任务，已执行的任务不受影响。确定继续吗？',
                () => _showResultDialog(context, '取消所有任务', '已取消4个待执行任务。'),
              ),
            ),
            const SizedBox(height: AppSpacing.xxl),
          ],
        ),
      ),
    );
  }

  Widget _buildWarningBanner(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(AppSpacing.lg),
      decoration: BoxDecoration(
        color: context.warning.withValues(alpha: 0.08),
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.warning.withValues(alpha: 0.3), width: 1),
      ),
      child: Row(
        children: [
          Icon(Icons.warning_amber_rounded, size: 24, color: context.warning),
          const SizedBox(width: AppSpacing.md),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('开发者模式', style: AppTypography.cardTitle(context).copyWith(color: context.warning)),
                const SizedBox(height: 2),
                Text('以下操作将直接影响角色运行状态，请谨慎操作。所有操作需二次确认。', style: AppTypography.caption(context)),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildDebugAction(
    BuildContext context,
    String title,
    String description,
    IconData icon,
    Color color,
    VoidCallback onTap,
  ) {
    return Padding(
      padding: const EdgeInsets.only(bottom: AppSpacing.sm),
      child: AmitiaCard(
        child: Row(
          children: [
            Container(
              width: 44,
              height: 44,
              decoration: BoxDecoration(
                color: color.withValues(alpha: 0.12),
                borderRadius: AppRadius.brSmall,
              ),
              child: Icon(icon, size: 22, color: color),
            ),
            const SizedBox(width: AppSpacing.md),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(title, style: AppTypography.cardTitle(context)),
                  const SizedBox(height: 2),
                  Text(description, style: AppTypography.caption(context)),
                ],
              ),
            ),
            Icon(Icons.chevron_right, color: context.textTertiary, size: 20),
          ],
        ),
        onTap: onTap,
      ),
    );
  }

  void _showConfirmDialog(
    BuildContext context,
    String title,
    String message,
    VoidCallback onConfirm,
  ) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Row(
          children: [
            Icon(Icons.warning_amber, color: context.warning, size: 22),
            const SizedBox(width: AppSpacing.sm),
            Text(title, style: AppTypography.cardTitle(context)),
          ],
        ),
        content: Text(message, style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              onConfirm();
            },
            child: Text('确认执行', style: TextStyle(color: context.accentPrimary)),
          ),
        ],
      ),
    );
  }

  void _showResultDialog(BuildContext context, String title, String result) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Row(
          children: [
            Icon(Icons.check_circle, color: context.success, size: 22),
            const SizedBox(width: AppSpacing.sm),
            Text(title, style: AppTypography.cardTitle(context)),
          ],
        ),
        content: Container(
          padding: const EdgeInsets.all(AppSpacing.lg),
          decoration: BoxDecoration(
            color: context.success.withValues(alpha: 0.06),
            borderRadius: AppRadius.brSmall,
          ),
          child: Text(result, style: AppTypography.bodySmall(context)),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('关闭')),
        ],
      ),
    );
  }

  void _showTaskListDialog(BuildContext context) {
    final tasks = [
      {'name': '早安问候', 'time': '07:00', 'status': '已完成'},
      {'name': '午餐提醒', 'time': '12:00', 'status': '待执行'},
      {'name': '午休问候', 'time': '13:30', 'status': '待执行'},
      {'name': '晚安问候', 'time': '23:00', 'status': '待执行'},
    ];

    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('调度任务列表', style: AppTypography.cardTitle(context)),
        content: SizedBox(
          width: double.maxFinite,
          child: ListView.separated(
            shrinkWrap: true,
            itemCount: tasks.length,
            separatorBuilder: (_, _) => const Divider(height: 1),
            itemBuilder: (context, index) {
              final task = tasks[index];
              final isDone = task['status'] == '已完成';
              return ListTile(
                contentPadding: EdgeInsets.zero,
                leading: Icon(
                  isDone ? Icons.check_circle : Icons.schedule,
                  size: 20,
                  color: isDone ? context.success : context.accentPrimary,
                ),
                title: Text(task['name']!, style: AppTypography.bodySmall(context)),
                subtitle: Text('计划时间：${task['time']}', style: AppTypography.label(context)),
                trailing: AmitiaStatusBadge(
                  label: task['status']!,
                  type: isDone ? BadgeType.success : BadgeType.info,
                ),
              );
            },
          ),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('关闭')),
        ],
      ),
    );
  }
}

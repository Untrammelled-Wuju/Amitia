import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../shared/models/models.dart';
import '../../../../shared/mock_data/mock_data.dart';

class RemindersPage extends ConsumerStatefulWidget {
  const RemindersPage({super.key});

  @override
  ConsumerState<RemindersPage> createState() => _RemindersPageState();
}

class _RemindersPageState extends ConsumerState<RemindersPage> {
  late List<Reminder> _reminders;
  int _selectedSegment = 0;
  final _segments = ['今天', '未来', '已完成'];

  @override
  void initState() {
    super.initState();
    _reminders = List.from(MockMemory.reminders);
  }

  List<Reminder> get _filteredReminders {
    switch (_selectedSegment) {
      case 0:
        return _reminders.where((r) => r.isToday && !r.isCompleted).toList();
      case 1:
        return _reminders.where((r) => !r.isToday && !r.isCompleted).toList();
      case 2:
        return _reminders.where((r) => r.isCompleted).toList();
      default:
        return _reminders;
    }
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '日程提醒',
        showBackButton: true,
        actions: [
          AmitiaIconButton(
            icon: Icons.add,
            onPressed: () => _showReminderEditor(context, null),
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: Column(
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.sm, AppSpacing.pagePadding, AppSpacing.sm),
              child: AmitiaSegmentedControl(
                segments: _segments,
                selectedIndex: _selectedSegment,
                onChanged: (i) => setState(() => _selectedSegment = i),
              ),
            ),
            Expanded(
              child: _filteredReminders.isEmpty
                  ? AmitiaEmptyState(
                      icon: Icons.notifications_none,
                      title: '暂无提醒',
                      subtitle: '点击右上角添加新提醒',
                      actionText: '新建提醒',
                      onAction: () => _showReminderEditor(context, null),
                    )
                  : ListView.separated(
                      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
                      itemCount: _filteredReminders.length,
                      separatorBuilder: (_, _) => const SizedBox(height: AppSpacing.sm),
                      itemBuilder: (context, index) => _buildReminderCard(context, _filteredReminders[index]),
                    ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildReminderCard(BuildContext context, Reminder reminder) {
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
                  color: _getCategoryColor(context, reminder.category).withValues(alpha: 0.12),
                  borderRadius: AppRadius.brSmall,
                ),
                child: Icon(
                  _getCategoryIcon(reminder.category),
                  size: 22,
                  color: _getCategoryColor(context, reminder.category),
                ),
              ),
              const SizedBox(width: AppSpacing.md),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(reminder.title, style: AppTypography.cardTitle(context)),
                    const SizedBox(height: 2),
                    Text(reminder.description, style: AppTypography.caption(context), maxLines: 1, overflow: TextOverflow.ellipsis),
                  ],
                ),
              ),
              AmitiaStatusBadge(
                label: reminder.category,
                type: _getCategoryBadge(reminder.category),
              ),
            ],
          ),
          const SizedBox(height: AppSpacing.sm),
          Row(
            children: [
              Icon(Icons.access_time, size: 14, color: context.textTertiary),
              const SizedBox(width: 4),
              Text(_formatReminderTime(reminder.time), style: AppTypography.label(context)),
              const Spacer(),
              if (reminder.isCompleted)
                AmitiaStatusBadge(label: '已完成', type: BadgeType.success)
              else if (!reminder.isEnabled)
                AmitiaStatusBadge(label: '已停用', type: BadgeType.neutral)
              else
                AmitiaStatusBadge(label: '已启用', type: BadgeType.accent),
            ],
          ),
          const SizedBox(height: AppSpacing.sm),
          Row(
            children: [
              if (!reminder.isCompleted) ...[
                GestureDetector(
                  onTap: () => _showTestResult(context, reminder),
                  child: _buildActionButton(context, '测试', Icons.science_outlined, context.success),
                ),
                const SizedBox(width: AppSpacing.sm),
                GestureDetector(
                  onTap: () {
                    setState(() {
                      final idx = _reminders.indexWhere((r) => r.id == reminder.id);
                      _reminders[idx] = Reminder(
                        id: reminder.id,
                        title: reminder.title,
                        description: reminder.description,
                        time: reminder.time,
                        isCompleted: reminder.isCompleted,
                        isEnabled: !reminder.isEnabled,
                        category: reminder.category,
                        isToday: reminder.isToday,
                      );
                    });
                    ScaffoldMessenger.of(context).showSnackBar(
                      SnackBar(content: Text('已${reminder.isEnabled ? '停用' : '启用'}提醒'), duration: const Duration(seconds: 1)),
                    );
                  },
                  child: _buildActionButton(
                    context,
                    reminder.isEnabled ? '停用' : '启用',
                    reminder.isEnabled ? Icons.pause_circle_outline : Icons.play_circle_outline,
                    context.warning,
                  ),
                ),
                const SizedBox(width: AppSpacing.sm),
              ],
              GestureDetector(
                onTap: () => _showReminderEditor(context, reminder),
                child: _buildActionButton(context, '编辑', Icons.edit_outlined, context.accentPrimary),
              ),
              const Spacer(),
              GestureDetector(
                onTap: () => _showDeleteConfirm(context, reminder),
                child: _buildActionButton(context, '删除', Icons.delete_outline, context.error),
              ),
            ],
          ),
        ],
      ),
    );
  }

  Widget _buildActionButton(BuildContext context, String label, IconData icon, Color color) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.1),
        borderRadius: AppRadius.brTag,
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 14, color: color),
          const SizedBox(width: 4),
          Text(label, style: TextStyle(fontSize: 12, color: color)),
        ],
      ),
    );
  }

  Color _getCategoryColor(BuildContext context, String category) {
    switch (category) {
      case '工作':
        return context.accentPrimary;
      case '社交':
        return context.info;
      case '生活':
        return context.success;
      case '健康':
        return context.error;
      default:
        return context.warning;
    }
  }

  IconData _getCategoryIcon(String category) {
    switch (category) {
      case '工作':
        return Icons.work_outline;
      case '社交':
        return Icons.people_outline;
      case '生活':
        return Icons.home_outlined;
      case '健康':
        return Icons.health_and_safety_outlined;
      default:
        return Icons.notifications_outlined;
    }
  }

  BadgeType _getCategoryBadge(String category) {
    switch (category) {
      case '工作':
        return BadgeType.accent;
      case '社交':
        return BadgeType.info;
      case '生活':
        return BadgeType.success;
      case '健康':
        return BadgeType.error;
      default:
        return BadgeType.warning;
    }
  }

  String _formatReminderTime(DateTime time) {
    return '${time.month}月${time.day}日 ${time.hour.toString().padLeft(2, '0')}:${time.minute.toString().padLeft(2, '0')}';
  }

  void _showReminderEditor(BuildContext context, Reminder? existing) {
    final isEdit = existing != null;
    final titleCtrl = TextEditingController(text: existing?.title ?? '');
    final descCtrl = TextEditingController(text: existing?.description ?? '');
    String category = existing?.category ?? '日常';

    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setSheetState) => Padding(
          padding: EdgeInsets.fromLTRB(
            AppSpacing.xl, AppSpacing.lg, AppSpacing.xl,
            MediaQuery.of(ctx).viewInsets.bottom + AppSpacing.xl,
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Center(child: Container(width: 40, height: 4, decoration: BoxDecoration(color: context.borderPrimary, borderRadius: BorderRadius.circular(2)))),
              const SizedBox(height: AppSpacing.lg),
              Text(isEdit ? '编辑提醒' : '新建提醒', style: AppTypography.sectionTitle(context)),
              const SizedBox(height: AppSpacing.lg),
              Text('标题', style: AppTypography.label(context)),
              const SizedBox(height: AppSpacing.xs),
              AmitiaTextField(controller: titleCtrl, hintText: '输入提醒标题'),
              const SizedBox(height: AppSpacing.md),
              Text('描述', style: AppTypography.label(context)),
              const SizedBox(height: AppSpacing.xs),
              AmitiaTextField(controller: descCtrl, maxLines: 2, hintText: '输入提醒描述'),
              const SizedBox(height: AppSpacing.md),
              Text('分类', style: AppTypography.label(context)),
              const SizedBox(height: AppSpacing.xs),
              Wrap(
                spacing: AppSpacing.sm,
                children: ['工作', '社交', '生活', '健康', '日常'].map((c) {
                  final isSelected = category == c;
                  return GestureDetector(
                    onTap: () => setSheetState(() => category = c),
                    child: Container(
                      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
                      decoration: BoxDecoration(
                        color: isSelected ? context.accentPrimary : context.surfaceSecondary,
                        borderRadius: AppRadius.brTag,
                      ),
                      child: Text(c, style: TextStyle(fontSize: 13, color: isSelected ? Colors.white : context.textSecondary)),
                    ),
                  );
                }).toList(),
              ),
              const SizedBox(height: AppSpacing.xl),
              AmitiaButton(
                label: isEdit ? '保存' : '创建',
                isFullWidth: true,
                onPressed: () {
                  if (titleCtrl.text.trim().isEmpty) return;
                  Navigator.pop(ctx);
                  setState(() {
                    if (isEdit) {
                      final idx = _reminders.indexWhere((r) => r.id == existing.id);
                      _reminders[idx] = Reminder(
                        id: existing.id,
                        title: titleCtrl.text.trim(),
                        description: descCtrl.text.trim(),
                        time: existing.time,
                        isCompleted: existing.isCompleted,
                        isEnabled: existing.isEnabled,
                        category: category,
                        isToday: existing.isToday,
                      );
                    } else {
                      _reminders.add(Reminder(
                        id: 'r${DateTime.now().millisecondsSinceEpoch}',
                        title: titleCtrl.text.trim(),
                        description: descCtrl.text.trim(),
                        time: DateTime.now().add(const Duration(days: 1)),
                        category: category,
                        isToday: false,
                      ));
                    }
                  });
                  ScaffoldMessenger.of(context).showSnackBar(
                    SnackBar(content: Text(isEdit ? '提醒已更新' : '提醒已创建'), duration: const Duration(seconds: 1)),
                  );
                },
              ),
            ],
          ),
        ),
      ),
    );
  }

  void _showTestResult(BuildContext context, Reminder reminder) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('测试结果', style: AppTypography.cardTitle(context)),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Container(
              width: double.infinity,
              padding: const EdgeInsets.all(AppSpacing.lg),
              decoration: BoxDecoration(
                color: context.success.withValues(alpha: 0.08),
                borderRadius: AppRadius.brMedium,
              ),
              child: Column(
                children: [
                  Icon(Icons.check_circle, size: 36, color: context.success),
                  const SizedBox(height: AppSpacing.sm),
                  Text('提醒发送成功', style: AppTypography.cardTitle(context).copyWith(color: context.success)),
                ],
              ),
            ),
            const SizedBox(height: AppSpacing.md),
            Text('提醒标题：${reminder.title}', style: AppTypography.bodySmall(context)),
            const SizedBox(height: 4),
            Text('通知渠道：系统通知', style: AppTypography.caption(context)),
            Text('声音：默认', style: AppTypography.caption(context)),
            Text('震动：已开启', style: AppTypography.caption(context)),
          ],
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('关闭')),
        ],
      ),
    );
  }

  void _showDeleteConfirm(BuildContext context, Reminder reminder) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('删除提醒', style: AppTypography.cardTitle(context)),
        content: Text('确定要删除「${reminder.title}」吗？', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              setState(() {
                _reminders.removeWhere((r) => r.id == reminder.id);
              });
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(content: Text('提醒已删除'), duration: Duration(seconds: 1)),
              );
            },
            child: Text('删除', style: TextStyle(color: context.error)),
          ),
        ],
      ),
    );
  }
}

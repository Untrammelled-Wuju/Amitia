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

class RemindersPage extends ConsumerStatefulWidget {
  const RemindersPage({super.key});

  @override
  ConsumerState<RemindersPage> createState() => _RemindersPageState();
}

class _RemindersPageState extends ConsumerState<RemindersPage> {
  List<Map<String, dynamic>> _reminders = [];
  int _selectedSegment = 0;
  final _segments = ['今天', '未来', '已完成'];
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _loadReminders();
  }

  Future<void> _loadReminders() async {
    setState(() { _loading = true; _error = null; });
    try {
      final svc = ref.read(reminderServiceProvider);
      final list = await svc.list();
      if (mounted) setState(() { _reminders = list.map((r) => r.toJson()).toList(); _loading = false; });
    } catch (e) {
      if (mounted) setState(() { _error = e.toString(); _loading = false; });
    }
  }

  List<Map<String, dynamic>> get _filteredReminders {
    switch (_selectedSegment) {
      case 0:
        return _reminders.where((r) => r['isToday'] == true && r['isCompleted'] != true).toList();
      case 1:
        return _reminders.where((r) => r['isToday'] != true && r['isCompleted'] != true).toList();
      case 2:
        return _reminders.where((r) => r['isCompleted'] == true).toList();
      default:
        return _reminders;
    }
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '日程提醒',
        navigation: AmitiaAppBarNavigation.back,
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
              padding: EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.sm, AppSpacing.pagePadding, AppSpacing.sm),
              child: AmitiaSegmentedControl(
                segments: _segments,
                selectedIndex: _selectedSegment,
                onChanged: (i) => setState(() => _selectedSegment = i),
              ),
            ),
            Expanded(
              child: _loading
                  ? const AmitiaLoadingState(message: '加载中...')
                  : _error != null
                      ? AmitiaErrorState(message: _error!, onRetry: _loadReminders)
                      : _filteredReminders.isEmpty
                          ? AmitiaEmptyState(
                              icon: Icons.notifications_none,
                              title: '暂无提醒',
                              subtitle: '点击右上角添加新提醒',
                              actionText: '新建提醒',
                              onAction: () => _showReminderEditor(context, null),
                            )
                          : ListView.separated(
                              padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
                              itemCount: _filteredReminders.length,
                              separatorBuilder: (_, _) => SizedBox(height: AppSpacing.sm),
                              itemBuilder: (context, index) => _buildReminderCard(context, _filteredReminders[index]),
                            ),
            ),
          ],
        ),
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

  String _formatTime(String? timeStr) {
    if (timeStr == null || timeStr.isEmpty) return '未知';
    try {
      final dt = DateTime.parse(timeStr);
      return '${dt.month}月${dt.day}日 ${dt.hour.toString().padLeft(2, '0')}:${dt.minute.toString().padLeft(2, '0')}';
    } catch (_) {
      return timeStr;
    }
  }

  Widget _buildReminderCard(BuildContext context, Map<String, dynamic> reminder) {
    final id = (reminder['id'] ?? '').toString();
    final title = (reminder['title'] ?? '').toString();
    final description = (reminder['content'] ?? reminder['description'] ?? '').toString();
    final category = (reminder['category'] ?? '日常').toString();
    final isCompleted = (reminder['isCompleted'] == true);
    final isEnabled = (reminder['enabled'] is int)
        ? (reminder['enabled'] == 1)
        : (reminder['isEnabled'] as bool? ?? true);
    final timeStr = (reminder['cronExpr'] ?? reminder['time'] ?? reminder['createdAt'] ?? '').toString();

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
                  color: _getCategoryColor(context, category).withValues(alpha: 0.12),
                  borderRadius: AppRadius.brSmall,
                ),
                child: Icon(
                  _getCategoryIcon(category),
                  size: 22,
                  color: _getCategoryColor(context, category),
                ),
              ),
              SizedBox(width: AppSpacing.md),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(title, style: AppTypography.cardTitle(context)),
                    const SizedBox(height: 2),
                    Text(description, style: AppTypography.caption(context), maxLines: 1, overflow: TextOverflow.ellipsis),
                  ],
                ),
              ),
              AmitiaStatusBadge(
                label: category,
                type: _getCategoryBadge(category),
              ),
            ],
          ),
          SizedBox(height: AppSpacing.sm),
          Row(
            children: [
              Icon(Icons.access_time, size: 14, color: context.textTertiary),
              const SizedBox(width: 4),
              Text(_formatTime(timeStr), style: AppTypography.label(context)),
              const Spacer(),
              if (isCompleted)
                AmitiaStatusBadge(label: '已完成', type: BadgeType.success)
              else if (!isEnabled)
                AmitiaStatusBadge(label: '已停用', type: BadgeType.neutral)
              else
                AmitiaStatusBadge(label: '已启用', type: BadgeType.accent),
            ],
          ),
          SizedBox(height: AppSpacing.sm),
          Row(
            children: [
              if (!isCompleted) ...[
                GestureDetector(
                  onTap: () => _showTestResult(context, title),
                  child: _buildActionButton(context, '测试', Icons.science_outlined, context.success),
                ),
                SizedBox(width: AppSpacing.sm),
                GestureDetector(
                  onTap: () => _toggleReminder(id, !isEnabled),
                  child: _buildActionButton(
                    context,
                    isEnabled ? '停用' : '启用',
                    isEnabled ? Icons.pause_circle_outline : Icons.play_circle_outline,
                    context.warning,
                  ),
                ),
                SizedBox(width: AppSpacing.sm),
              ],
              GestureDetector(
                onTap: () => _showReminderEditor(context, reminder),
                child: _buildActionButton(context, '编辑', Icons.edit_outlined, context.accentPrimary),
              ),
              const Spacer(),
              GestureDetector(
                onTap: () => _showDeleteConfirm(context, id, title),
                child: _buildActionButton(context, '删除', Icons.delete_outline, context.error),
              ),
            ],
          ),
        ],
      ),
    );
  }

  Future<void> _toggleReminder(String id, bool enable) async {
    try {
      final svc = ref.read(reminderServiceProvider);
      await svc.toggle(id, enable);
      _loadReminders();
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(enable ? '已启用提醒' : '已停用提醒'), duration: const Duration(seconds: 1)),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('操作失败: $e'), backgroundColor: context.error));
      }
    }
  }

  void _showReminderEditor(BuildContext context, Map<String, dynamic>? existing) {
    final isEdit = existing != null;
    final titleCtrl = TextEditingController(text: (existing?['title'] ?? '').toString());
    final descCtrl = TextEditingController(text: (existing?['content'] ?? existing?['description'] ?? '').toString());
    String category = (existing?['category'] ?? '日常').toString();
    String cronExpr = (existing?['cronExpr'] ?? existing?['time'] ?? '').toString();

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
              SizedBox(height: AppSpacing.lg),
              Text(isEdit ? '编辑提醒' : '新建提醒', style: AppTypography.sectionTitle(context)),
              SizedBox(height: AppSpacing.lg),
              Text('标题', style: AppTypography.label(context)),
              SizedBox(height: AppSpacing.xs),
              AmitiaTextField(controller: titleCtrl, hintText: '输入提醒标题'),
              SizedBox(height: AppSpacing.md),
              Text('描述', style: AppTypography.label(context)),
              SizedBox(height: AppSpacing.xs),
              AmitiaTextField(controller: descCtrl, maxLines: 2, hintText: '输入提醒描述'),
              SizedBox(height: AppSpacing.md),
              Text('Cron 表达式', style: AppTypography.label(context)),
              SizedBox(height: AppSpacing.xs),
              AmitiaTextField(controller: TextEditingController(text: cronExpr), hintText: '例如：0 9 * * *'),
              SizedBox(height: AppSpacing.md),
              Text('分类', style: AppTypography.label(context)),
              SizedBox(height: AppSpacing.xs),
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
              SizedBox(height: AppSpacing.xl),
              AmitiaButton(
                label: isEdit ? '保存' : '创建',
                isFullWidth: true,
                onPressed: () async {
                  if (titleCtrl.text.trim().isEmpty) return;
                  try {
                    final svc = ref.read(reminderServiceProvider);
                    final data = {
                      'title': titleCtrl.text.trim(),
                      'content': descCtrl.text.trim(),
                      'cronExpr': cronExpr,
                      'category': category,
                      'enabled': 1,
                    };
                    if (isEdit) {
                      await svc.update(existing!['id'].toString(), data);
                    } else {
                      await svc.create(data);
                    }
                    if (ctx.mounted) Navigator.pop(ctx);
                    _loadReminders();
                    if (mounted) {
                      ScaffoldMessenger.of(context).showSnackBar(
                        SnackBar(content: Text(isEdit ? '提醒已更新' : '提醒已创建'), duration: const Duration(seconds: 1)),
                      );
                    }
                  } catch (e) {
                    if (mounted) {
                      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('保存失败: $e'), backgroundColor: context.error));
                    }
                  }
                },
              ),
            ],
          ),
        ),
      ),
    );
  }

  void _showTestResult(BuildContext context, String title) {
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
              padding: EdgeInsets.all(AppSpacing.lg),
              decoration: BoxDecoration(
                color: context.success.withValues(alpha: 0.08),
                borderRadius: AppRadius.brMedium,
              ),
              child: Column(
                children: [
                  Icon(Icons.check_circle, size: 36, color: context.success),
                  SizedBox(height: AppSpacing.sm),
                  Text('提醒发送成功', style: AppTypography.cardTitle(context).copyWith(color: context.success)),
                ],
              ),
            ),
            SizedBox(height: AppSpacing.md),
            Text('提醒标题：$title', style: AppTypography.bodySmall(context)),
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

  void _showDeleteConfirm(BuildContext context, String id, String title) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('删除提醒', style: AppTypography.cardTitle(context)),
        content: Text('确定要删除「$title」吗？', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () async {
              try {
                final svc = ref.read(reminderServiceProvider);
                await svc.delete(id);
              } catch (_) {}
              if (ctx.mounted) Navigator.pop(ctx);
              _loadReminders();
              if (mounted) {
                ScaffoldMessenger.of(context).showSnackBar(
                  const SnackBar(content: Text('提醒已删除'), duration: Duration(seconds: 1)),
                );
              }
            },
            child: Text('删除', style: TextStyle(color: context.error)),
          ),
        ],
      ),
    );
  }
}

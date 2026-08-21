import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/services/providers.dart';

class CharacterDebugPage extends ConsumerWidget {
  final String characterId;

  const CharacterDebugPage({super.key, required this.characterId});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final companionAsync = ref.watch(companionStateByCharacterProvider(characterId));

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '调试模式',
        showBackButton: true,
        fallbackRoute: AppRoutes.characters,
        actions: [
          AmitiaIconButton(
            icon: Icons.warning_amber,
            color: context.warning,
            onPressed: () {
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(content: Text('开发者模式'), duration: Duration(seconds: 2)),
              );
            },
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: ListView(
          padding: EdgeInsets.all(AppSpacing.pagePadding),
          children: [
            _buildWarningBanner(context),
            SizedBox(height: AppSpacing.sectionGap),
            AmitiaSectionHeader(title: '角色状态'),
            SizedBox(height: AppSpacing.sm),
            companionAsync.when(
              loading: () => const Center(child: CircularProgressIndicator()),
              error: (err, _) => Text('加载失败: $err', style: AppTypography.bodySmall(context)),
              data: (state) {
                if (state == null) {
                  return Text('暂无状态数据', style: AppTypography.caption(context));
                }
                return Column(
                  children: [
                    _buildStatusRow(context, '状态', state['state']?.toString() ?? '-', Icons.info_outline),
                    _buildStatusRow(context, '睡眠中', (state['isSleeping'] == true) ? '是' : '否', Icons.bedtime_outlined),
                    _buildStatusRow(context, '当前活动', state['currentActivity']?.toString() ?? '-', Icons.play_circle_outline),
                    _buildStatusRow(context, '下次活动', state['nextActivity']?.toString() ?? '-', Icons.schedule),
                    _buildStatusRow(context, '醒来时间', state['wakeTime']?.toString() ?? '-', Icons.wb_sunny_outlined),
                    _buildStatusRow(context, '睡眠时间', state['sleepTime']?.toString() ?? '-', Icons.nights_stay_outlined),
                  ],
                );
              },
            ),
            SizedBox(height: AppSpacing.sectionGap),
            AmitiaSectionHeader(title: '调试操作'),
            SizedBox(height: AppSpacing.sm),
            _buildDebugAction(
              context,
              '测试角色对话',
              '发送测试消息验证角色响应',
              Icons.chat_bubble_outline,
              context.accentPrimary,
              () => _testCharacter(context, ref),
            ),
            _buildDebugAction(
              context,
              '重新生成今日作息',
              '根据当前时间和规则重新生成今日作息计划',
              Icons.refresh,
              context.accentPrimary,
              () => _regenerateSchedule(context, ref),
            ),
            _buildDebugAction(
              context,
              '重建生活时间线',
              '按当前角色重新生成生活时间线',
              Icons.timeline_outlined,
              context.accentPrimary,
              () => _regenerateTimeline(context, ref),
            ),
            _buildDebugAction(
              context,
              '日程冲突',
              '查看当前角色的日程冲突检测结果',
              Icons.event_busy_outlined,
              context.accentPrimary,
              () => _showScheduleConflicts(context, ref),
            ),
            _buildDebugAction(
              context,
              '课程与调课',
              '查看生效课程和临时调课记录',
              Icons.school_outlined,
              context.accentPrimary,
              () => _showClassState(context, ref),
            ),
            _buildDebugAction(
              context,
              '主动消息任务',
              '查看、运行、取消或重新生成今日主动消息任务',
              Icons.mark_chat_unread_outlined,
              context.accentPrimary,
              () => _showActiveTasks(context, ref),
            ),
            _buildDebugAction(
              context,
              '处理延迟回复',
              '立即处理当前角色等待中的延迟回复',
              Icons.schedule_send_outlined,
              context.accentPrimary,
              () => _processDelayedReplies(context, ref),
            ),
            _buildDebugAction(
              context,
              '规则日志',
              '查看主动消息和生活规则的最近执行记录',
              Icons.receipt_long_outlined,
              context.accentPrimary,
              () => _showRuleLogs(context, ref),
            ),
            _buildDebugAction(
              context,
              '重置所有状态',
              '重置角色的运行状态',
              Icons.restart_alt,
              context.warning,
              () => _showResetConfirm(context, ref),
            ),
            SizedBox(height: AppSpacing.xxl),
          ],
        ),
      ),
    );
  }

  Widget _buildStatusRow(BuildContext context, String label, String value, IconData icon) {
    return Padding(
      padding: EdgeInsets.only(bottom: AppSpacing.sm),
      child: AmitiaCard(
        child: Row(
          children: [
            Icon(icon, size: 20, color: context.accentPrimary),
            SizedBox(width: AppSpacing.md),
            Text(label, style: AppTypography.body(context)),
            const Spacer(),
            Text(value, style: AppTypography.bodySmall(context).copyWith(color: context.accentPrimary)),
          ],
        ),
      ),
    );
  }

  Widget _buildWarningBanner(BuildContext context) {
    return Container(
      padding: EdgeInsets.all(AppSpacing.lg),
      decoration: BoxDecoration(
        color: context.warning.withValues(alpha: 0.08),
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.warning.withValues(alpha: 0.3), width: 1),
      ),
      child: Row(
        children: [
          Icon(Icons.warning_amber_rounded, size: 24, color: context.warning),
          SizedBox(width: AppSpacing.md),
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
      padding: EdgeInsets.only(bottom: AppSpacing.sm),
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
            SizedBox(width: AppSpacing.md),
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

  Future<void> _testCharacter(BuildContext context, WidgetRef ref) async {
    final confirmed = await _showConfirmDialog(context, '测试角色对话', '将发送一条测试消息验证角色响应。确定继续吗？');
    if (confirmed != true) return;
    try {
      final svc = ref.read(characterDetailServiceProvider);
      final result = await svc.test(characterId);
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('测试成功: ${result?.name ?? characterId}')),
        );
      }
    } catch (e) {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('测试失败: $e')),
        );
      }
    }
  }

  Future<void> _regenerateSchedule(BuildContext context, WidgetRef ref) async {
    final confirmed = await _showConfirmDialog(context, '重新生成今日作息', '这将覆盖当前的作息计划。确定继续吗？');
    if (confirmed != true) return;
    try {
      final svc = ref.read(companionServiceProvider);
      await svc.regenerateSchedule(characterId: characterId);
      ref.invalidate(companionStateByCharacterProvider(characterId));
      ref.invalidate(companionStateProvider);
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('已重新生成今日作息')),
        );
      }
    } catch (e) {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('操作失败: $e')),
        );
      }
    }
  }

  Future<void> _regenerateTimeline(BuildContext context, WidgetRef ref) async {
    final confirmed = await _showConfirmDialog(context, '重建生活时间线', '将按当前角色的最新规则重新生成生活时间线。确定继续吗？');
    if (confirmed != true) return;
    try {
      final result = await ref.read(companionServiceProvider).regenerateTimeline(characterId: characterId);
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('时间线已重建${result == null ? '' : '：${_compact(result)}'}')),
        );
      }
    } catch (e) {
      _showError(context, e);
    }
  }

  Future<void> _showScheduleConflicts(BuildContext context, WidgetRef ref) async {
    try {
      final items = await ref.read(companionServiceProvider).scheduleConflicts(characterId: characterId);
      if (context.mounted) await _showDataDialog(context, '日程冲突', items);
    } catch (e) {
      _showError(context, e);
    }
  }

  Future<void> _showClassState(BuildContext context, WidgetRef ref) async {
    try {
      final svc = ref.read(companionServiceProvider);
      var effective = await svc.effectiveClasses(characterId: characterId);
      var adjustments = await svc.classAdjustments(characterId: characterId);
      if (!context.mounted) return;
      await showDialog<void>(
        context: context,
        builder: (dialogContext) => StatefulBuilder(
          builder: (dialogContext, setDialogState) => AlertDialog(
            title: const Text('课程与调课'),
            content: SizedBox(
              width: 700,
              height: 480,
              child: ListView(
                children: [
                  Text('今日生效课程', style: AppTypography.cardTitle(dialogContext)),
                  const SizedBox(height: 8),
                  if (effective.isEmpty)
                    const Text('暂无生效课程')
                  else
                    ...effective.map((item) => ListTile(
                          dense: true,
                          contentPadding: EdgeInsets.zero,
                          leading: const Icon(Icons.school_outlined),
                          title: Text((item['title'] ?? item['className'] ?? '课程').toString()),
                          subtitle: Text(_compact(item)),
                        )),
                  const Divider(height: 28),
                  Row(
                    children: [
                      Expanded(child: Text('临时调课', style: AppTypography.cardTitle(dialogContext))),
                      TextButton.icon(
                        onPressed: () async {
                          final changed = await _editClassAdjustment(dialogContext, ref);
                          if (changed == true) {
                            effective = await svc.effectiveClasses(characterId: characterId);
                            adjustments = await svc.classAdjustments(characterId: characterId);
                            if (dialogContext.mounted) setDialogState(() {});
                          }
                        },
                        icon: const Icon(Icons.add),
                        label: const Text('新增'),
                      ),
                    ],
                  ),
                  if (adjustments.isEmpty)
                    const Text('暂无调课记录')
                  else
                    ...adjustments.map((item) {
                      final id = int.tryParse((item['id'] ?? '').toString());
                      return ListTile(
                        dense: true,
                        contentPadding: EdgeInsets.zero,
                        leading: const Icon(Icons.swap_horiz_outlined),
                        title: Text((item['className'] ?? '调课').toString()),
                        subtitle: Text(_compact(item)),
                        trailing: id == null
                            ? null
                            : Wrap(
                                spacing: 2,
                                children: [
                                  IconButton(
                                    tooltip: '编辑',
                                    icon: const Icon(Icons.edit_outlined),
                                    onPressed: () async {
                                      final changed = await _editClassAdjustment(dialogContext, ref, current: item);
                                      if (changed == true) {
                                        effective = await svc.effectiveClasses(characterId: characterId);
                                        adjustments = await svc.classAdjustments(characterId: characterId);
                                        if (dialogContext.mounted) setDialogState(() {});
                                      }
                                    },
                                  ),
                                  IconButton(
                                    tooltip: '删除',
                                    icon: const Icon(Icons.delete_outline),
                                    onPressed: () async {
                                      await svc.deleteClassAdjustment(id, characterId: characterId);
                                      effective = await svc.effectiveClasses(characterId: characterId);
                                      adjustments = await svc.classAdjustments(characterId: characterId);
                                      if (dialogContext.mounted) setDialogState(() {});
                                    },
                                  ),
                                ],
                              ),
                      );
                    }),
                ],
              ),
            ),
            actions: [TextButton(onPressed: () => Navigator.pop(dialogContext), child: const Text('关闭'))],
          ),
        ),
      );
    } catch (e) {
      _showError(context, e);
    }
  }

  Future<bool?> _editClassAdjustment(
    BuildContext context,
    WidgetRef ref, {
    Map<String, dynamic>? current,
  }) async {
    final now = DateTime.now();
    final defaultDate = '${now.year.toString().padLeft(4, '0')}-${now.month.toString().padLeft(2, '0')}-${now.day.toString().padLeft(2, '0')}';
    final dateController = TextEditingController(text: (current?['date'] ?? defaultDate).toString());
    final slotController = TextEditingController(text: (current?['slotIndex'] ?? 0).toString());
    final classController = TextEditingController(text: (current?['className'] ?? '').toString());
    final typeController = TextEditingController(text: (current?['adjustType'] ?? 'swap').toString());
    final descriptionController = TextEditingController(text: (current?['description'] ?? '').toString());
    final data = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: Text(current == null ? '新增调课' : '编辑调课'),
        content: SizedBox(
          width: 520,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TextField(controller: dateController, decoration: const InputDecoration(labelText: '日期 YYYY-MM-DD')),
              TextField(controller: slotController, keyboardType: TextInputType.number, decoration: const InputDecoration(labelText: '课程序号')),
              TextField(controller: classController, decoration: const InputDecoration(labelText: '课程名称')),
              TextField(controller: typeController, decoration: const InputDecoration(labelText: '调整类型（swap / canceled）')),
              TextField(controller: descriptionController, decoration: const InputDecoration(labelText: '说明')),
            ],
          ),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(dialogContext), child: const Text('取消')),
          TextButton(
            onPressed: () {
              final slot = int.tryParse(slotController.text.trim());
              if (dateController.text.trim().isEmpty || classController.text.trim().isEmpty || slot == null) return;
              Navigator.pop(dialogContext, <String, dynamic>{
                'date': dateController.text.trim(),
                'slotIndex': slot,
                'className': classController.text.trim(),
                'adjustType': typeController.text.trim().isEmpty ? 'swap' : typeController.text.trim(),
                'description': descriptionController.text.trim(),
              });
            },
            child: const Text('保存'),
          ),
        ],
      ),
    );
    dateController.dispose();
    slotController.dispose();
    classController.dispose();
    typeController.dispose();
    descriptionController.dispose();
    if (data == null) return false;
    final svc = ref.read(companionServiceProvider);
    final id = int.tryParse((current?['id'] ?? '').toString());
    if (id == null) {
      await svc.createClassAdjustment(data, characterId: characterId);
    } else {
      await svc.updateClassAdjustment(id, data, characterId: characterId);
    }
    return true;
  }

  Future<void> _showActiveTasks(BuildContext context, WidgetRef ref) async {
    try {
      final svc = ref.read(companionServiceProvider);
      var items = await svc.activeMessageTasksToday(characterId: characterId);
      if (!context.mounted) return;
      await showDialog<void>(
        context: context,
        builder: (dialogContext) => StatefulBuilder(
          builder: (dialogContext, setDialogState) => AlertDialog(
            title: const Text('主动消息任务'),
            content: SizedBox(
              width: 620,
              height: 420,
              child: items.isEmpty
                  ? const Center(child: Text('今日暂无主动消息任务'))
                  : ListView.separated(
                      itemCount: items.length,
                      separatorBuilder: (_, _) => const Divider(height: 1),
                      itemBuilder: (_, index) {
                        final item = items[index];
                        final id = int.tryParse((item['id'] ?? '').toString());
                        return ListTile(
                          contentPadding: EdgeInsets.zero,
                          title: Text((item['type'] ?? item['taskType'] ?? '任务 ${id ?? '-'}').toString()),
                          subtitle: Text(_compact(item)),
                          trailing: id == null
                              ? null
                              : Wrap(
                                  spacing: 4,
                                  children: [
                                    IconButton(
                                      tooltip: '立即运行',
                                      icon: const Icon(Icons.play_arrow),
                                      onPressed: () async {
                                        await svc.runActiveMessageTask(id, characterId: characterId);
                                        items = await svc.activeMessageTasksToday(characterId: characterId);
                                        if (dialogContext.mounted) setDialogState(() {});
                                      },
                                    ),
                                    IconButton(
                                      tooltip: '取消',
                                      icon: const Icon(Icons.cancel_outlined),
                                      onPressed: () async {
                                        await svc.cancelActiveMessageTask(id, characterId: characterId);
                                        items = await svc.activeMessageTasksToday(characterId: characterId);
                                        if (dialogContext.mounted) setDialogState(() {});
                                      },
                                    ),
                                  ],
                                ),
                        );
                      },
                    ),
            ),
            actions: [
              TextButton(
                onPressed: () async {
                  await svc.regenerateActiveMessageTasks(characterId: characterId);
                  items = await svc.activeMessageTasksToday(characterId: characterId);
                  if (dialogContext.mounted) setDialogState(() {});
                },
                child: const Text('重新生成'),
              ),
              TextButton(onPressed: () => Navigator.pop(dialogContext), child: const Text('关闭')),
            ],
          ),
        ),
      );
    } catch (e) {
      _showError(context, e);
    }
  }

  Future<void> _processDelayedReplies(BuildContext context, WidgetRef ref) async {
    final confirmed = await _showConfirmDialog(context, '处理延迟回复', '立即处理当前角色所有到期的延迟回复。确定继续吗？');
    if (confirmed != true) return;
    try {
      final result = await ref.read(companionServiceProvider).processDelayedReplies(characterId: characterId);
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('处理完成：${_compact(result ?? const {})}')));
      }
    } catch (e) {
      _showError(context, e);
    }
  }

  Future<void> _showRuleLogs(BuildContext context, WidgetRef ref) async {
    try {
      final items = await ref.read(companionServiceProvider).ruleLogs(characterId: characterId);
      if (context.mounted) await _showDataDialog(context, '规则日志', items);
    } catch (e) {
      _showError(context, e);
    }
  }

  Future<void> _showDataDialog(BuildContext context, String title, List<Map<String, dynamic>> items) {
    return showDialog<void>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: Text(title),
        content: SizedBox(
          width: 620,
          height: 420,
          child: items.isEmpty
              ? const Center(child: Text('暂无数据'))
              : ListView.separated(
                  itemCount: items.length,
                  separatorBuilder: (_, _) => const Divider(height: 1),
                  itemBuilder: (_, index) => Padding(
                    padding: const EdgeInsets.symmetric(vertical: 8),
                    child: SelectableText(_compact(items[index])),
                  ),
                ),
        ),
        actions: [TextButton(onPressed: () => Navigator.pop(dialogContext), child: const Text('关闭'))],
      ),
    );
  }

  String _compact(Map<String, dynamic> value) {
    return value.entries.map((entry) => '${entry.key}: ${entry.value}').join(' · ');
  }

  void _showError(BuildContext context, Object error) {
    if (!context.mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('操作失败: $error')));
  }

  Future<void> _showResetConfirm(BuildContext context, WidgetRef ref) async {
    final confirmed = await _showConfirmDialog(context, '重置所有状态', '将重置角色所有运行状态。此操作不可撤销，确定继续吗？');
    if (confirmed != true) return;
    try {
      final svc = ref.read(companionServiceProvider);
      await svc.regenerateAll(characterId: characterId);
      ref.invalidate(companionStateByCharacterProvider(characterId));
      ref.invalidate(companionStateProvider);
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('已重置所有状态')),
        );
      }
    } catch (e) {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('操作失败: $e')),
        );
      }
    }
  }

  Future<bool?> _showConfirmDialog(BuildContext context, String title, String message) {
    return showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Row(
          children: [
            Icon(Icons.warning_amber, color: context.warning, size: 22),
            SizedBox(width: AppSpacing.sm),
            Text(title, style: AppTypography.cardTitle(context)),
          ],
        ),
        content: Text(message, style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('取消')),
          TextButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: Text('确认执行', style: TextStyle(color: context.accentPrimary)),
          ),
        ],
      ),
    );
  }
}

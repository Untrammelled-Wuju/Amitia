import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/models/reminder.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_icons.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/hub_entry_card.dart';

class TaskHubPage extends ConsumerStatefulWidget {
  const TaskHubPage({super.key});

  @override
  ConsumerState<TaskHubPage> createState() => _TaskHubPageState();
}

class _TaskHubPageState extends ConsumerState<TaskHubPage> {
  static const _pageSize = 10;
  final _scrollController = ScrollController();
  int _visibleCount = _pageSize;

  @override
  void initState() {
    super.initState();
    _scrollController.addListener(_handleScroll);
  }

  @override
  void dispose() {
    _scrollController.removeListener(_handleScroll);
    _scrollController.dispose();
    super.dispose();
  }

  void _handleScroll() {
    if (!_scrollController.hasClients ||
        _scrollController.position.extentAfter > 180) {
      return;
    }
    final tasks = _plannedTasks(
      ref.read(reminderListProvider).valueOrNull ?? const <ReminderDto>[],
    );
    if (_visibleCount >= tasks.length) return;
    setState(() {
      _visibleCount = (_visibleCount + _pageSize).clamp(0, tasks.length);
    });
  }

  List<ReminderDto> _plannedTasks(List<ReminderDto> reminders) {
    final tasks = reminders
        .where((task) => task.isEnabled && !task.paused)
        .toList(growable: false);
    tasks.sort((left, right) {
      final leftDate = _taskDate(left);
      final rightDate = _taskDate(right);
      if (leftDate == null && rightDate == null) return 0;
      if (leftDate == null) return 1;
      if (rightDate == null) return -1;
      return leftDate.compareTo(rightDate);
    });
    return tasks;
  }

  DateTime? _taskDate(ReminderDto task) {
    final value = task.localRemindAt.isNotEmpty
        ? task.localRemindAt
        : task.remindAt.isNotEmpty
        ? task.remindAt
        : task.updatedAt;
    return DateTime.tryParse(value);
  }

  String _formatTaskTime(ReminderDto task) {
    final date = _taskDate(task);
    if (date == null) return '时间待定';
    final value =
        '${date.year}年${date.month}月${date.day}日 ${date.hour.toString().padLeft(2, '0')}:${date.minute.toString().padLeft(2, '0')}';
    final repeat = switch (task.repeatRule) {
      'none' || '' => '',
      'daily' => '每天重复',
      'weekly' => '每周重复',
      'monthly' => '每月重复',
      _ => task.repeatRule,
    };
    return repeat.isEmpty ? value : '$value · $repeat';
  }

  Widget _buildPlannedTasks(AsyncValue<List<ReminderDto>> asyncTasks) {
    if (asyncTasks.isLoading && !asyncTasks.hasValue) {
      return const HubSectionPlaceholder(message: '正在加载任务…');
    }
    if (asyncTasks.hasError && !asyncTasks.hasValue) {
      return Padding(
        padding: EdgeInsets.symmetric(vertical: AppSpacing.md),
        child: Center(
          child: TextButton(
            onPressed: () => ref.invalidate(reminderListProvider),
            child: const Text('任务加载失败，点击重试'),
          ),
        ),
      );
    }
    final tasks = _plannedTasks(
      asyncTasks.valueOrNull ?? const <ReminderDto>[],
    );
    if (tasks.isEmpty) {
      return const HubSectionPlaceholder(message: '暂无任务');
    }
    final visible = tasks.take(_visibleCount).toList(growable: false);
    return Container(
      clipBehavior: Clip.antiAlias,
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Column(
        children: [
          for (var index = 0; index < visible.length; index++)
            HubEntryCard(
              key: ValueKey('planned-task-${visible[index].id}'),
              icon: AmitiaIconGlyph.reminder,
              title: visible[index].title.trim().isEmpty
                  ? '未命名任务'
                  : visible[index].title.trim(),
              description: _formatTaskTime(visible[index]),
              onTap: () => context.push(AppRoutes.reminders),
              showDivider: index < visible.length - 1,
            ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final tasksAsync = ref.watch(reminderListProvider);
    return AmitiaScaffold(
      handlesSystemBack: false,
      appBar: AmitiaAppBar(
        title: '任务',
        backgroundColor: context.surfacePrimary,
        toolbarHeight: AmitiaAppBar.rootPageToolbarHeight,
      ),
      body: ListView(
        controller: _scrollController,
        padding: EdgeInsets.fromLTRB(
          AppSpacing.pagePadding,
          AppSpacing.md,
          AppSpacing.pagePadding,
          AppSpacing.xl,
        ),
        children: [
          Container(
            clipBehavior: Clip.antiAlias,
            decoration: BoxDecoration(
              color: context.surfacePrimary,
              borderRadius: AppRadius.brMedium,
              border: Border.all(color: context.borderPrimary, width: 0.5),
            ),
            child: Column(
              children: [
                HubEntryCard(
                  icon: AmitiaIconGlyph.reminder,
                  title: '主动任务',
                  onTap: () => context.push(AppRoutes.reminders),
                  showDivider: true,
                ),
                HubEntryCard(
                  icon: AmitiaIconGlyph.continuity,
                  title: '持续事项',
                  onTap: () => context.push(AppRoutes.continuity),
                ),
              ],
            ),
          ),
          SizedBox(height: AppSpacing.lg),
          Text('计划中的任务', style: AppTypography.label(context)),
          SizedBox(height: AppSpacing.xs),
          _buildPlannedTasks(tasksAsync),
        ],
      ),
    );
  }
}

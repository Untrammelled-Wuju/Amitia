import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/models/reminder.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class RemindersPage extends ConsumerStatefulWidget {
  const RemindersPage({super.key});

  @override
  ConsumerState<RemindersPage> createState() => _RemindersPageState();
}

class _RemindersPageState extends ConsumerState<RemindersPage> {
  List<ReminderDto> _reminders = const [];
  Map<String, dynamic> _status = const {};
  int _selectedSegment = 0;
  bool _loading = true;
  String? _error;
  Timer? _refreshTimer;
  bool _refreshing = false;

  @override
  void initState() {
    super.initState();
    _load();
    _refreshTimer = Timer.periodic(const Duration(seconds: 5), (_) {
      if (!_refreshing && mounted) unawaited(_load(showLoading: false));
    });
  }

  @override
  void dispose() {
    _refreshTimer?.cancel();
    super.dispose();
  }

  Future<void> _load({bool showLoading = true}) async {
    if (_refreshing) return;
    _refreshing = true;
    if (showLoading && mounted) setState(() { _loading = true; _error = null; });
    try {
      final service = ref.read(reminderServiceProvider);
      final values = await Future.wait<dynamic>([service.list(), service.status()]);
      if (!mounted) return;
      setState(() {
        _reminders = values[0] as List<ReminderDto>;
        _status = values[1] as Map<String, dynamic>;
        _loading = false;
        _error = null;
      });
    } catch (error) {
      if (showLoading && mounted) setState(() { _error = error.toString(); _loading = false; });
    } finally {
      _refreshing = false;
    }
  }

  List<ReminderDto> get _visible {
    switch (_selectedSegment) {
      case 1:
        return _reminders.where((item) => item.isEnabled).toList(growable: false);
      case 2:
        return _reminders.where((item) => !item.isEnabled).toList(growable: false);
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
          AmitiaIconButton(icon: Icons.monitor_heart_outlined, onPressed: _showSchedulerPanel),
          AmitiaIconButton(icon: Icons.add, onPressed: () => _showEditor(null)),
        ],
      ),
      body: SafeArea(
        top: false,
        child: Column(
          children: [
            _buildSchedulerStatus(),
            Padding(
              padding: EdgeInsets.fromLTRB(AppSpacing.pagePadding, 0, AppSpacing.pagePadding, AppSpacing.sm),
              child: AmitiaSegmentedControl(
                segments: const ['全部', '待触发', '已停用/已触发'],
                selectedIndex: _selectedSegment,
                onChanged: (index) => setState(() => _selectedSegment = index),
              ),
            ),
            Expanded(
              child: _loading
                  ? const AmitiaLoadingState(message: '加载中…')
                  : _error != null
                      ? AmitiaErrorState(message: _error!, onRetry: _load)
                      : _visible.isEmpty
                          ? AmitiaEmptyState(
                              icon: Icons.notifications_none,
                              title: '暂无提醒',
                              subtitle: '创建一个一次性或重复提醒',
                              actionText: '新建提醒',
                              onAction: () => _showEditor(null),
                            )
                          : RefreshIndicator(
                              onRefresh: _load,
                              child: ListView.separated(
                                padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
                                itemCount: _visible.length,
                                separatorBuilder: (_, _) => SizedBox(height: AppSpacing.sm),
                                itemBuilder: (context, index) => _buildCard(_visible[index]),
                              ),
                            ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildSchedulerStatus() {
    final running = _status['schedulerRunning'] == true;
    final total = (_status['total'] as num?)?.toInt() ?? _reminders.length;
    final due = (_status['dueNow'] as num?)?.toInt() ?? 0;
    return Padding(
      padding: EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.sm, AppSpacing.pagePadding, AppSpacing.sm),
      child: Row(
        children: [
          AmitiaStatusBadge(label: running ? '调度器运行中' : '调度器未运行', type: running ? BadgeType.success : BadgeType.error),
          SizedBox(width: AppSpacing.sm),
          AmitiaStatusBadge(label: '总计 $total', type: BadgeType.neutral),
          SizedBox(width: AppSpacing.sm),
          if (due > 0) AmitiaStatusBadge(label: '已到期 $due', type: BadgeType.warning),
        ],
      ),
    );
  }

  Widget _buildCard(ReminderDto reminder) {
    final repeatLabel = _repeatLabels[reminder.repeatRule] ?? reminder.repeatRule;
    return AmitiaCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Container(
                width: 42,
                height: 42,
                decoration: BoxDecoration(color: context.accentSoft, borderRadius: AppRadius.brSmall),
                child: Icon(Icons.notifications_active_outlined, size: 21, color: context.accentPrimary),
              ),
              SizedBox(width: AppSpacing.md),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(reminder.title, style: AppTypography.cardTitle(context)),
                    if (reminder.content.isNotEmpty) ...[
                      const SizedBox(height: 2),
                      Text(reminder.content, style: AppTypography.caption(context), maxLines: 2, overflow: TextOverflow.ellipsis),
                    ],
                  ],
                ),
              ),
              AmitiaStatusBadge(label: reminder.isEnabled ? '启用' : '停用', type: reminder.isEnabled ? BadgeType.success : BadgeType.neutral),
            ],
          ),
          SizedBox(height: AppSpacing.sm),
          Wrap(
            spacing: AppSpacing.sm,
            runSpacing: AppSpacing.xs,
            children: [
              AmitiaStatusBadge(label: _formatTime(reminder.remindAt), type: BadgeType.accent),
              AmitiaStatusBadge(label: repeatLabel, type: BadgeType.info),
              AmitiaStatusBadge(label: reminder.channel, type: BadgeType.neutral),
              if (reminder.characterName.isNotEmpty) AmitiaStatusBadge(label: reminder.characterName, type: BadgeType.warning),
              if (reminder.conversationTitle.isNotEmpty) AmitiaStatusBadge(label: reminder.conversationTitle, type: BadgeType.neutral),
            ],
          ),
          SizedBox(height: AppSpacing.sm),
          Wrap(
            spacing: AppSpacing.sm,
            children: [
              TextButton.icon(onPressed: () => _test(reminder), icon: const Icon(Icons.science_outlined, size: 16), label: const Text('测试')),
              TextButton.icon(onPressed: () => _trigger(reminder), icon: const Icon(Icons.send_outlined, size: 16), label: const Text('立即触发')),
              TextButton.icon(
                onPressed: () => _toggle(reminder),
                icon: Icon(reminder.isEnabled ? Icons.pause_circle_outline : Icons.play_circle_outline, size: 16),
                label: Text(reminder.isEnabled ? '停用' : '启用'),
              ),
              TextButton.icon(onPressed: () => _showEditor(reminder), icon: const Icon(Icons.edit_outlined, size: 16), label: const Text('编辑')),
              TextButton.icon(
                onPressed: () => _delete(reminder),
                icon: Icon(Icons.delete_outline, size: 16, color: context.error),
                label: Text('删除', style: TextStyle(color: context.error)),
              ),
            ],
          ),
        ],
      ),
    );
  }

  static const _repeatLabels = <String, String>{
    'none': '不重复',
    'daily': '每天',
    'weekly': '每周',
    'monthly': '每月',
    'yearly': '每年',
  };

  Future<void> _showEditor(ReminderDto? existing) async {
    final titleController = TextEditingController(text: existing?.title ?? '');
    final contentController = TextEditingController(text: existing?.content ?? '');
    final conversationController = TextEditingController(text: existing?.conversationId ?? '');
    final characterController = TextEditingController(text: existing?.characterId ?? '');
    var channel = existing?.channel ?? 'web';
    var repeatRule = existing?.repeatRule ?? 'none';
    var remindAt = _parseBackendTime(existing?.remindAt) ?? DateTime.now().add(const Duration(minutes: 10));

    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      builder: (sheetContext) => StatefulBuilder(
        builder: (sheetContext, setSheetState) => Padding(
          padding: EdgeInsets.fromLTRB(
            AppSpacing.xl,
            AppSpacing.lg,
            AppSpacing.xl,
            MediaQuery.of(sheetContext).viewInsets.bottom + AppSpacing.xl,
          ),
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(existing == null ? '新建提醒' : '编辑提醒', style: AppTypography.sectionTitle(context)),
                SizedBox(height: AppSpacing.lg),
                AmitiaTextField(controller: titleController, hintText: '提醒标题'),
                SizedBox(height: AppSpacing.md),
                AmitiaTextField(controller: contentController, maxLines: 3, hintText: '提醒内容（可选）'),
                SizedBox(height: AppSpacing.md),
                ListTile(
                  contentPadding: EdgeInsets.zero,
                  title: Text('提醒时间', style: AppTypography.label(context)),
                  subtitle: Text(_formatBackendTime(remindAt), style: AppTypography.body(context)),
                  trailing: const Icon(Icons.calendar_month_outlined),
                  onTap: () async {
                    final date = await showDatePicker(
                      context: sheetContext,
                      initialDate: remindAt,
                      firstDate: DateTime.now(),
                      lastDate: DateTime.now().add(const Duration(days: 3650)),
                    );
                    if (date == null || !sheetContext.mounted) return;
                    final time = await showTimePicker(context: sheetContext, initialTime: TimeOfDay.fromDateTime(remindAt));
                    if (time == null) return;
                    setSheetState(() {
                      remindAt = DateTime(date.year, date.month, date.day, time.hour, time.minute);
                    });
                  },
                ),
                SizedBox(height: AppSpacing.md),
                DropdownButtonFormField<String>(
                  value: repeatRule,
                  decoration: const InputDecoration(labelText: '重复规则', border: OutlineInputBorder()),
                  items: _repeatLabels.entries.map((e) => DropdownMenuItem(value: e.key, child: Text(e.value))).toList(growable: false),
                  onChanged: (value) { if (value != null) setSheetState(() => repeatRule = value); },
                ),
                SizedBox(height: AppSpacing.md),
                DropdownButtonFormField<String>(
                  value: channel,
                  decoration: const InputDecoration(labelText: '渠道', border: OutlineInputBorder()),
                  items: const [
                    DropdownMenuItem(value: 'web', child: Text('Web / App')),
                    DropdownMenuItem(value: 'wechat', child: Text('微信')),
                    DropdownMenuItem(value: 'qq', child: Text('QQ')),
                  ],
                  onChanged: (value) { if (value != null) setSheetState(() => channel = value); },
                ),
                SizedBox(height: AppSpacing.md),
                AmitiaTextField(controller: characterController, hintText: '角色 ID（可选）'),
                SizedBox(height: AppSpacing.md),
                AmitiaTextField(controller: conversationController, hintText: '目标会话 ID（可选）'),
                SizedBox(height: AppSpacing.lg),
                AmitiaButton(
                  label: existing == null ? '创建' : '保存',
                  isFullWidth: true,
                  onPressed: () async {
                    if (titleController.text.trim().isEmpty) {
                      ScaffoldMessenger.of(sheetContext).showSnackBar(const SnackBar(content: Text('标题不能为空')));
                      return;
                    }
                    if (!remindAt.isAfter(DateTime.now()) && existing == null) {
                      ScaffoldMessenger.of(sheetContext).showSnackBar(const SnackBar(content: Text('提醒时间必须晚于当前时间')));
                      return;
                    }
                    final data = <String, dynamic>{
                      'title': titleController.text.trim(),
                      'content': contentController.text.trim(),
                      'channel': channel,
                      'conversationId': conversationController.text.trim(),
                      'characterId': characterController.text.trim(),
                      'remindAt': _formatBackendTime(remindAt),
                      'repeatRule': repeatRule,
                    };
                    try {
                      final service = ref.read(reminderServiceProvider);
                      if (existing == null) {
                        await service.create(data);
                      } else {
                        data['enabled'] = existing.isEnabled;
                        await service.update(existing.id, data);
                      }
                      if (sheetContext.mounted) Navigator.pop(sheetContext);
                      await _load();
                    } catch (error) {
                      if (sheetContext.mounted) ScaffoldMessenger.of(sheetContext).showSnackBar(SnackBar(content: Text('保存失败：$error')));
                    }
                  },
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Future<void> _test(ReminderDto reminder) async {
    try {
      final result = await ref.read(reminderServiceProvider).test(reminder.id);
      if (!mounted) return;
      await showDialog<void>(
        context: context,
        builder: (dialogContext) => AlertDialog(
          title: const Text('提醒测试结果'),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _detail('标题', (result['title'] ?? reminder.title).toString()),
              _detail('渠道', (result['channel'] ?? reminder.channel).toString()),
              _detail('目标会话', (result['conversationId'] ?? '').toString()),
              _detail('消息内容', (result['messageContent'] ?? '').toString()),
              _detail('提醒时间', (result['remindAt'] ?? reminder.remindAt).toString()),
            ],
          ),
          actions: [TextButton(onPressed: () => Navigator.pop(dialogContext), child: const Text('关闭'))],
        ),
      );
    } catch (error) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('测试失败：$error')));
    }
  }

  Widget _detail(String label, String value) => Padding(
        padding: const EdgeInsets.only(bottom: 6),
        child: Text('$label：${value.isEmpty ? '—' : value}'),
      );

  Future<void> _trigger(ReminderDto reminder) async {
    try {
      final result = await ref.read(reminderServiceProvider).trigger(reminder.id);
      await _load();
      if (mounted) {
        final conversationId = (result['conversationId'] ?? '').toString();
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(conversationId.isEmpty ? '提醒已触发' : '提醒已触发到会话 $conversationId')));
      }
    } catch (error) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('触发失败：$error')));
    }
  }

  Future<void> _toggle(ReminderDto reminder) async {
    try {
      await ref.read(reminderServiceProvider).toggle(reminder.id);
      await _load();
    } catch (error) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('操作失败：$error')));
    }
  }

  Future<void> _delete(ReminderDto reminder) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('删除提醒'),
        content: Text('确定删除“${reminder.title}”吗？'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(dialogContext, false), child: const Text('取消')),
          TextButton(onPressed: () => Navigator.pop(dialogContext, true), child: Text('删除', style: TextStyle(color: context.error))),
        ],
      ),
    );
    if (confirmed != true) return;
    await ref.read(reminderServiceProvider).delete(reminder.id);
    await _load();
  }

  Future<void> _showSchedulerPanel() async {
    final service = ref.read(reminderServiceProvider);
    Map<String, dynamic> status = const {};
    Map<String, dynamic> queue = const {};
    Map<String, dynamic> cleanup = const {};
    List<Map<String, dynamic>> prospective = const [];
    Map<String, dynamic> history = const {};
    try {
      final values = await Future.wait<dynamic>([
        service.status(),
        service.queueSummary(),
        service.cleanupConfig(),
        service.prospective(),
        service.triggerHistory(pageSize: 10),
      ]);
      status = values[0] as Map<String, dynamic>;
      queue = values[1] as Map<String, dynamic>;
      cleanup = values[2] as Map<String, dynamic>;
      prospective = values[3] as List<Map<String, dynamic>>;
      history = values[4] as Map<String, dynamic>;
    } catch (error) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('加载调度器状态失败：$error')));
      return;
    }
    if (!mounted) return;
    final cleanupController = TextEditingController(text: (cleanup['cleanupDays'] ?? '0').toString());
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      builder: (sheetContext) => DraggableScrollableSheet(
        initialChildSize: 0.85,
        minChildSize: 0.55,
        maxChildSize: 0.96,
        expand: false,
        builder: (sheetContext, controller) => ListView(
          controller: controller,
          padding: EdgeInsets.all(AppSpacing.xl),
          children: [
            Text('提醒调度器', style: AppTypography.sectionTitle(context)),
            SizedBox(height: AppSpacing.md),
            Wrap(
              spacing: AppSpacing.sm,
              runSpacing: AppSpacing.sm,
              children: [
                AmitiaStatusBadge(label: status['schedulerRunning'] == true ? '调度器运行中' : '调度器未运行', type: status['schedulerRunning'] == true ? BadgeType.success : BadgeType.error),
                AmitiaStatusBadge(label: '队列深度 ${queue['depth'] ?? 0}', type: BadgeType.neutral),
                AmitiaStatusBadge(label: '失败 ${queue['recentFailures'] ?? 0}', type: BadgeType.warning),
                AmitiaStatusBadge(label: queue['backpressure'] == true ? '存在背压' : '队列正常', type: queue['backpressure'] == true ? BadgeType.error : BadgeType.success),
              ],
            ),
            if (queue['backpressure'] == true) ...[
              SizedBox(height: AppSpacing.md),
              AmitiaButton(
                label: '清除背压标记',
                isSecondary: true,
                onPressed: () async {
                  await service.clearBackpressure();
                  if (sheetContext.mounted) ScaffoldMessenger.of(sheetContext).showSnackBar(const SnackBar(content: Text('背压标记已清除')));
                },
              ),
            ],
            SizedBox(height: AppSpacing.lg),
            Text('自动清理', style: AppTypography.cardTitle(context)),
            SizedBox(height: AppSpacing.xs),
            AmitiaTextField(controller: cleanupController, keyboardType: TextInputType.number, hintText: '触发后保留天数；0 表示不自动清理'),
            SizedBox(height: AppSpacing.sm),
            AmitiaButton(
              label: '保存清理策略',
              isSecondary: true,
              onPressed: () async {
                await service.setCleanupConfig(cleanupController.text.trim().isEmpty ? '0' : cleanupController.text.trim());
                if (sheetContext.mounted) ScaffoldMessenger.of(sheetContext).showSnackBar(const SnackBar(content: Text('清理策略已更新')));
              },
            ),
            SizedBox(height: AppSpacing.lg),
            Text('前瞻记忆', style: AppTypography.cardTitle(context)),
            SizedBox(height: AppSpacing.sm),
            if (prospective.isEmpty)
              Text('暂无待触发前瞻记忆', style: AppTypography.caption(context))
            else
              ...prospective.map((item) => ListTile(
                    contentPadding: EdgeInsets.zero,
                    title: Text((item['title'] ?? '').toString()),
                    subtitle: Text((item['remindAt'] ?? '').toString()),
                    trailing: Text((item['status'] ?? '').toString()),
                  )),
            SizedBox(height: AppSpacing.lg),
            Text('最近触发历史', style: AppTypography.cardTitle(context)),
            SizedBox(height: AppSpacing.sm),
            ...((history['items'] as List?) ?? const []).whereType<Map>().map((item) => ListTile(
                  contentPadding: EdgeInsets.zero,
                  title: Text((item['title'] ?? '').toString()),
                  subtitle: Text('${item['createdAt'] ?? ''}${(item['lastError'] ?? '').toString().isNotEmpty ? ' · ${item['lastError']}' : ''}'),
                  trailing: Text((item['state'] ?? '').toString()),
                )),
          ],
        ),
      ),
    );
  }

  static DateTime? _parseBackendTime(String? value) {
    final raw = value?.trim() ?? '';
    if (raw.isEmpty) return null;
    return DateTime.tryParse(raw.replaceFirst(' ', 'T'));
  }

  static String _formatBackendTime(DateTime value) =>
      '${value.year.toString().padLeft(4, '0')}-${value.month.toString().padLeft(2, '0')}-${value.day.toString().padLeft(2, '0')} '
      '${value.hour.toString().padLeft(2, '0')}:${value.minute.toString().padLeft(2, '0')}:${value.second.toString().padLeft(2, '0')}';

  static String _formatTime(String value) {
    final parsed = _parseBackendTime(value);
    if (parsed == null) return value;
    return '${parsed.month}月${parsed.day}日 ${parsed.hour.toString().padLeft(2, '0')}:${parsed.minute.toString().padLeft(2, '0')}';
  }
}

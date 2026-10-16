import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/models/reminder.dart';
import '../../../../core/models/character.dart';
import '../../../../core/models/conversation.dart';
import '../../../../core/services/providers.dart';
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
    if (showLoading && mounted)
      setState(() {
        _loading = true;
        _error = null;
      });
    try {
      final service = ref.read(reminderServiceProvider);
      final values = await Future.wait<dynamic>([
        service.list(),
        service.status(),
      ]);
      if (!mounted) return;
      setState(() {
        _reminders = values[0] as List<ReminderDto>;
        _status = values[1] as Map<String, dynamic>;
        _loading = false;
        _error = null;
      });
    } catch (error) {
      if (mounted)
        setState(() {
          _error = error.toString();
          _reminders = const [];
          _loading = false;
        });
    } finally {
      _refreshing = false;
    }
  }

  List<ReminderDto> get _visible {
    switch (_selectedSegment) {
      case 1:
        return _reminders
            .where((item) => item.isEnabled)
            .toList(growable: false);
      case 2:
        return _reminders
            .where((item) => !item.isEnabled)
            .toList(growable: false);
      default:
        return _reminders;
    }
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '主动任务',
        navigation: AmitiaAppBarNavigation.back,
        actions: [
          AmitiaIconButton(
            icon: Icons.monitor_heart_outlined,
            onPressed: _showSchedulerPanel,
          ),
          AmitiaIconButton(icon: Icons.add, onPressed: () => _openEditor(null)),
        ],
      ),
      body: SafeArea(
        top: false,
        child: Column(
          children: [
            _buildSchedulerStatus(),
            Padding(
              padding: EdgeInsets.fromLTRB(
                AppSpacing.pagePadding,
                0,
                AppSpacing.pagePadding,
                AppSpacing.sm,
              ),
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
                      title: '暂无主动任务',
                      subtitle: '创建一个一次性或重复任务',
                      actionText: '新建任务',
                      onAction: () => _openEditor(null),
                    )
                  : RefreshIndicator(
                      onRefresh: _load,
                      child: ListView.separated(
                        padding: EdgeInsets.symmetric(
                          horizontal: AppSpacing.pagePadding,
                        ),
                        itemCount: _visible.length,
                        separatorBuilder: (_, _) =>
                            SizedBox(height: AppSpacing.sm),
                        itemBuilder: (context, index) =>
                            _buildCard(_visible[index]),
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
      padding: EdgeInsets.fromLTRB(
        AppSpacing.pagePadding,
        AppSpacing.sm,
        AppSpacing.pagePadding,
        AppSpacing.sm,
      ),
      child: Row(
        children: [
          AmitiaStatusBadge(
            label: running ? '调度器运行中' : '调度器未运行',
            type: running ? BadgeType.success : BadgeType.error,
          ),
          SizedBox(width: AppSpacing.sm),
          AmitiaStatusBadge(label: '总计 $total', type: BadgeType.neutral),
          SizedBox(width: AppSpacing.sm),
          if (due > 0)
            AmitiaStatusBadge(label: '已到期 $due', type: BadgeType.warning),
        ],
      ),
    );
  }

  Widget _buildCard(ReminderDto reminder) {
    final repeatLabel =
        _repeatLabels[reminder.repeatRule] ?? reminder.repeatRule;
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
                decoration: BoxDecoration(
                  color: context.accentSoft,
                  borderRadius: AppRadius.brSmall,
                ),
                child: Icon(
                  Icons.notifications_active_outlined,
                  size: 21,
                  color: context.accentPrimary,
                ),
              ),
              SizedBox(width: AppSpacing.md),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      reminder.title,
                      style: AppTypography.cardTitle(context),
                    ),
                    if (reminder.content.isNotEmpty) ...[
                      const SizedBox(height: 2),
                      Text(
                        reminder.content,
                        style: AppTypography.caption(context),
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                      ),
                    ],
                  ],
                ),
              ),
              AmitiaStatusBadge(
                label: reminder.paused
                    ? '已暂停'
                    : reminder.isEnabled
                    ? '启用'
                    : reminder.state,
                type: reminder.isEnabled
                    ? BadgeType.success
                    : BadgeType.neutral,
              ),
            ],
          ),
          SizedBox(height: AppSpacing.sm),
          Wrap(
            spacing: AppSpacing.sm,
            runSpacing: AppSpacing.xs,
            children: [
              AmitiaStatusBadge(
                label:
                    '${_formatTime(reminder.localRemindAt.isEmpty ? reminder.remindAt : reminder.localRemindAt)} ${reminder.timezone}',
                type: BadgeType.accent,
              ),
              AmitiaStatusBadge(label: repeatLabel, type: BadgeType.info),
              AmitiaStatusBadge(
                label: reminder.channel,
                type: BadgeType.neutral,
              ),
              if (reminder.characterName.isNotEmpty)
                AmitiaStatusBadge(
                  label: reminder.characterName,
                  type: BadgeType.warning,
                ),
              if (reminder.conversationTitle.isNotEmpty)
                AmitiaStatusBadge(
                  label: reminder.conversationTitle,
                  type: BadgeType.neutral,
                ),
            ],
          ),
          SizedBox(height: AppSpacing.sm),
          Text(
            '执行核心：${reminder.ownerId} · ${reminder.executionMode} · 来源：${reminder.source}',
            style: AppTypography.caption(context),
          ),
          if (reminder.endAt.isNotEmpty || reminder.maxOccurrences > 0)
            Text(
              '结束：${reminder.endAt.isEmpty ? '不限时间' : reminder.endAt} · 次数：${reminder.maxOccurrences == 0 ? '不限' : reminder.maxOccurrences}',
              style: AppTypography.caption(context),
            ),
          Wrap(
            spacing: AppSpacing.sm,
            children: [
              TextButton.icon(
                onPressed: () => _test(reminder),
                icon: const Icon(Icons.science_outlined, size: 16),
                label: const Text('测试'),
              ),
              TextButton.icon(
                onPressed: () => _trigger(reminder),
                icon: const Icon(Icons.send_outlined, size: 16),
                label: const Text('立即触发'),
              ),
              TextButton.icon(
                onPressed: () => _toggle(reminder),
                icon: Icon(
                  reminder.isEnabled
                      ? Icons.pause_circle_outline
                      : Icons.play_circle_outline,
                  size: 16,
                ),
                label: Text(reminder.isEnabled ? '停用' : '启用'),
              ),
              TextButton.icon(
                onPressed: () => _openEditor(reminder),
                icon: const Icon(Icons.edit_outlined, size: 16),
                label: const Text('编辑'),
              ),
              TextButton.icon(
                onPressed: () => _pause(reminder),
                icon: const Icon(Icons.pause, size: 16),
                label: Text(reminder.paused ? '恢复' : '暂停'),
              ),
              TextButton.icon(
                onPressed: () => _delete(reminder),
                icon: Icon(
                  Icons.delete_outline,
                  size: 16,
                  color: context.error,
                ),
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
    'weekdays': '指定星期',
    'interval': '自定义间隔',
  };

  Future<void> _openEditor(ReminderDto? existing) async {
    try {
      await _showEditor(existing);
    } catch (error) {
      if (mounted)
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('打开编辑失败：$error')));
    }
  }

  Future<void> _showEditor(ReminderDto? existing) async {
    final titleController = TextEditingController(text: existing?.title ?? '');
    final contentController = TextEditingController(
      text: existing?.content ?? '',
    );
    final requestId =
        'mobile-reminder-${DateTime.now().microsecondsSinceEpoch}';
    final timezoneController = TextEditingController(
      text: existing?.timezone ?? 'Asia/Shanghai',
    );
    final intervalController = TextEditingController(
      text: (existing?.recurrence['intervalSeconds'] ?? 3600).toString(),
    );
    final endController = TextEditingController(text: existing?.endAt ?? '');
    final countController = TextEditingController(
      text: (existing?.maxOccurrences ?? 0).toString(),
    );
    final weekdays = <int>{
      ...(existing?.recurrence['weekdays'] as List? ?? []).cast<int>(),
    };
    var systemNotification = existing?.systemNotification ?? true;
    var selectedConversationId = existing?.conversationId ?? '';
    var selectedCharacterId = existing?.characterId ?? '';
    const channel = 'web';
    final service = ref.read(reminderServiceProvider);
    final availableChannels = await service.channels();
    final peers = <String, TextEditingController>{};
    for (final target in existing?.channelTargets ?? []) {
      if (target is Map)
        peers[target['channelId'].toString()] = TextEditingController(
          text: target['peerId'].toString(),
        );
    }
    final selectedChannels = peers.keys.toSet();
    var repeatRule = existing?.repeatRule ?? 'none';
    var remindAt =
        DateTime.tryParse(existing?.localAnchorAt ?? '') ??
        DateTime.now().add(const Duration(minutes: 10));

    final owned = ref.read(chatServiceProvider).owned;
    await owned.refresh();
    final editorCore = owned.coreId;
    final characters = owned.roles
        .map(
          (item) => CharacterDto(
            id: item['id'].toString(),
            name: (item['name'] ?? item['id']).toString(),
          ),
        )
        .toList();
    if (selectedCharacterId.isEmpty && characters.length == 1)
      selectedCharacterId = characters.single.id;
    if (!characters.any((item) => item.id == selectedCharacterId)) {
      selectedCharacterId = '';
    }
    List<ConversationDto> conversations = selectedCharacterId.isEmpty
        ? []
        : (await owned.conversations(
            characterId: selectedCharacterId,
          )).where((item) => item.sourceOwnerId == editorCore).toList();
    if (!mounted) return;
    for (final item in conversations) {
      if (item.sourceResourceId == selectedConversationId)
        selectedConversationId = item.id;
    }
    final knownCharacterIds = characters.map((item) => item.id).toSet();
    if (selectedCharacterId.isNotEmpty &&
        !knownCharacterIds.contains(selectedCharacterId)) {
      selectedCharacterId = '';
    }
    final knownConversationIds = conversations.map((item) => item.id).toSet();
    if (selectedConversationId.isNotEmpty &&
        !knownConversationIds.contains(selectedConversationId)) {
      selectedConversationId = '';
    }

    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      builder: (sheetContext) => StatefulBuilder(
        builder: (sheetContext, setSheetState) {
          final visibleConversations = conversations;
          if (selectedConversationId.isNotEmpty &&
              !visibleConversations.any(
                (item) => item.id == selectedConversationId,
              )) {
            selectedConversationId = '';
          }
          return Padding(
            padding: EdgeInsets.fromLTRB(
              AppSpacing.xl,
              0,
              AppSpacing.xl,
              MediaQuery.of(sheetContext).viewInsets.bottom + AppSpacing.xl,
            ),
            child: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    existing == null ? '新建任务' : '编辑任务',
                    style: AppTypography.sectionTitle(context),
                  ),
                  SizedBox(height: AppSpacing.lg),
                  AmitiaTextField(
                    controller: titleController,
                    hintText: '任务标题',
                  ),
                  SizedBox(height: AppSpacing.md),
                  AmitiaTextField(
                    controller: contentController,
                    maxLines: 3,
                    hintText: '消息内容（可选）',
                  ),
                  SizedBox(height: AppSpacing.md),
                  ListTile(
                    contentPadding: EdgeInsets.zero,
                    title: Text('执行时间', style: AppTypography.label(context)),
                    subtitle: Text(
                      _formatBackendTime(remindAt),
                      style: AppTypography.body(context),
                    ),
                    trailing: const Icon(Icons.calendar_month_outlined),
                    onTap: () async {
                      final date = await showDatePicker(
                        context: sheetContext,
                        initialDate: remindAt,
                        firstDate: DateTime(2000),
                        lastDate: DateTime.now().add(
                          const Duration(days: 3650),
                        ),
                      );
                      if (date == null || !sheetContext.mounted) return;
                      final time = await showTimePicker(
                        context: sheetContext,
                        initialTime: TimeOfDay.fromDateTime(remindAt),
                      );
                      if (time == null) return;
                      setSheetState(() {
                        remindAt = DateTime(
                          date.year,
                          date.month,
                          date.day,
                          time.hour,
                          time.minute,
                        );
                      });
                    },
                  ),
                  SizedBox(height: AppSpacing.md),
                  DropdownButtonFormField<String>(
                    value: repeatRule,
                    decoration: const InputDecoration(
                      labelText: '重复规则',
                      border: OutlineInputBorder(),
                    ),
                    items: _repeatLabels.entries
                        .map(
                          (e) => DropdownMenuItem(
                            value: e.key,
                            child: Text(e.value),
                          ),
                        )
                        .toList(growable: false),
                    onChanged: (value) {
                      if (value != null)
                        setSheetState(() => repeatRule = value);
                    },
                  ),
                  SizedBox(height: AppSpacing.md),
                  const Text('默认发送至角色对话，可附加已接入渠道'),
                  ...{
                    ...availableChannels.where((item) => item != 'web'),
                    ...selectedChannels,
                  }.map(
                    (id) => Column(
                      children: [
                        CheckboxListTile(
                          title: Text(
                            availableChannels.contains(id) ? id : '$id（已断开）',
                          ),
                          value: selectedChannels.contains(id),
                          onChanged: (value) => setSheetState(() {
                            if (value == true) {
                              selectedChannels.add(id);
                              peers.putIfAbsent(
                                id,
                                () => TextEditingController(),
                              );
                            } else {
                              selectedChannels.remove(id);
                            }
                          }),
                        ),
                        if (selectedChannels.contains(id))
                          AmitiaTextField(
                            controller: peers[id],
                            hintText: '$id 接收方标识（必填）',
                          ),
                      ],
                    ),
                  ),
                  SizedBox(height: AppSpacing.md),
                  DropdownButtonFormField<String>(
                    value: selectedCharacterId,
                    decoration: const InputDecoration(
                      labelText: '所属角色（必选）',
                      border: OutlineInputBorder(),
                    ),
                    items: [
                      const DropdownMenuItem(value: '', child: Text('不指定角色')),
                      ...characters.map(
                        (item) => DropdownMenuItem(
                          value: item.id,
                          child: Text(item.name.isEmpty ? item.id : item.name),
                        ),
                      ),
                    ],
                    onChanged: (value) async {
                      final role = value ?? '';
                      setSheetState(() {
                        selectedCharacterId = role;
                        selectedConversationId = '';
                        conversations = [];
                      });
                      try {
                        final next = role.isEmpty
                            ? <ConversationDto>[]
                            : await owned.conversations(characterId: role);
                        if (sheetContext.mounted && selectedCharacterId == role)
                          setSheetState(
                            () => conversations = next
                                .where(
                                  (item) => item.sourceOwnerId == owned.coreId,
                                )
                                .toList(),
                          );
                      } catch (error) {
                        if (sheetContext.mounted)
                          ScaffoldMessenger.of(sheetContext).showSnackBar(
                            SnackBar(content: Text('加载会话失败：$error')),
                          );
                      }
                    },
                  ),
                  SizedBox(height: AppSpacing.md),
                  DropdownButtonFormField<String>(
                    value: selectedConversationId,
                    decoration: const InputDecoration(
                      labelText: '目标会话（可选）',
                      border: OutlineInputBorder(),
                    ),
                    items: [
                      const DropdownMenuItem(value: '', child: Text('不指定会话')),
                      ...visibleConversations.map((item) {
                        final title = item.title.trim().isEmpty
                            ? item.id
                            : item.title.trim();
                        return DropdownMenuItem(
                          value: item.id,
                          child: Text(title),
                        );
                      }),
                    ],
                    onChanged: (value) => setSheetState(() {
                      selectedConversationId = value ?? '';
                      if (selectedConversationId.isNotEmpty) {}
                    }),
                  ),
                  SizedBox(height: AppSpacing.lg),
                  AmitiaTextField(
                    controller: timezoneController,
                    hintText: 'IANA 时区，如 Asia/Shanghai',
                  ),
                  if (repeatRule == 'interval')
                    AmitiaTextField(
                      controller: intervalController,
                      hintText: '重复间隔（秒）',
                    ),
                  if (repeatRule == 'weekdays')
                    Wrap(
                      children: List.generate(7, (index) {
                        final day = index + 1;
                        return FilterChip(
                          label: Text(
                            ['一', '二', '三', '四', '五', '六', '日'][index],
                          ),
                          selected: weekdays.contains(day),
                          onSelected: (value) => setSheetState(() {
                            if (value) {
                              weekdays.add(day);
                            } else {
                              weekdays.remove(day);
                            }
                          }),
                        );
                      }),
                    ),
                  AmitiaTextField(
                    controller: endController,
                    hintText: '结束时间（可选，带时区的 ISO 8601）',
                  ),
                  AmitiaTextField(
                    controller: countController,
                    hintText: '最多执行次数，0 表示不限制',
                  ),
                  SwitchListTile(
                    title: const Text('系统通知'),
                    value: systemNotification,
                    onChanged: (value) =>
                        setSheetState(() => systemNotification = value),
                  ),
                  AmitiaButton(
                    label: existing == null ? '创建' : '保存',
                    isFullWidth: true,
                    onPressed: () async {
                      if (titleController.text.trim().isEmpty) {
                        ScaffoldMessenger.of(
                          sheetContext,
                        ).showSnackBar(const SnackBar(content: Text('标题不能为空')));
                        return;
                      }
                      if (selectedCharacterId.isEmpty) {
                        ScaffoldMessenger.of(sheetContext).showSnackBar(
                          const SnackBar(content: Text('请选择所属角色')),
                        );
                        return;
                      }
                      try {
                        await owned.refresh();
                      } catch (error) {
                        if (sheetContext.mounted)
                          ScaffoldMessenger.of(sheetContext).showSnackBar(
                            SnackBar(content: Text('执行核心校验失败：$error')),
                          );
                        return;
                      }
                      if (!sheetContext.mounted) return;
                      if (owned.coreId != editorCore) {
                        ScaffoldMessenger.of(sheetContext).showSnackBar(
                          const SnackBar(content: Text('执行核心已变化，请重新打开编辑页面')),
                        );
                        return;
                      }
                      if (selectedChannels.any(
                        (id) =>
                            !availableChannels.contains(id) ||
                            peers[id]!.text.trim().isEmpty,
                      )) {
                        if (sheetContext.mounted)
                          ScaffoldMessenger.of(sheetContext).showSnackBar(
                            const SnackBar(content: Text('请选择已接入渠道并填写接收方标识')),
                          );
                        return;
                      }
                      final data = <String, dynamic>{
                        'title': titleController.text.trim(),
                        'content': contentController.text.trim(),
                        'channel': channel,
                        'conversationId': selectedConversationId.isEmpty
                            ? ''
                            : conversations
                                  .firstWhere(
                                    (item) => item.id == selectedConversationId,
                                  )
                                  .sourceResourceId,
                        'characterId': selectedCharacterId,
                        'remindAt': _formatBackendTime(remindAt),
                        'repeatRule': repeatRule,
                        'requestId': requestId,
                        'generation': existing?.generation ?? 0,
                        'timezone': timezoneController.text.trim(),
                        'recurrence': {
                          'type': repeatRule,
                          if (repeatRule == 'weekdays')
                            'weekdays': weekdays.toList()..sort(),
                          if (repeatRule == 'interval')
                            'intervalSeconds':
                                int.tryParse(intervalController.text) ?? 0,
                        },
                        'endAt': endController.text.trim(),
                        'maxOccurrences':
                            int.tryParse(countController.text) ?? -1,
                        'systemNotification': systemNotification,
                        'channelTargets': selectedChannels
                            .map(
                              (id) => {
                                'channelId': id,
                                'peerId': peers[id]!.text.trim(),
                              },
                            )
                            .toList(),
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
                        if (sheetContext.mounted)
                          ScaffoldMessenger.of(sheetContext).showSnackBar(
                            SnackBar(content: Text('保存失败：$error')),
                          );
                      }
                    },
                  ),
                ],
              ),
            ),
          );
        },
      ),
    );
    titleController.dispose();
    contentController.dispose();
    timezoneController.dispose();
    intervalController.dispose();
    endController.dispose();
    countController.dispose();
    for (final peer in peers.values) {
      peer.dispose();
    }
  }

  Future<void> _pause(ReminderDto reminder) async {
    try {
      await ref
          .read(reminderServiceProvider)
          .action(
            reminder.id,
            reminder.generation,
            reminder.paused ? 'resume' : 'pause',
          );
      await _load();
    } catch (error) {
      if (mounted)
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('操作失败：$error')));
    }
  }

  Future<void> _test(ReminderDto reminder) async {
    try {
      final result = await ref.read(reminderServiceProvider).test(reminder.id);
      if (!mounted) return;
      await showDialog<void>(
        context: context,
        builder: (dialogContext) => AlertDialog(
          title: const Text('任务测试结果'),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _detail('标题', (result['title'] ?? reminder.title).toString()),
              _detail('渠道', (result['channel'] ?? reminder.channel).toString()),
              _detail('目标会话', (result['conversationId'] ?? '').toString()),
              _detail('消息内容', (result['messageContent'] ?? '').toString()),
              _detail(
                '执行时间',
                (result['remindAt'] ?? reminder.remindAt).toString(),
              ),
            ],
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialogContext),
              child: const Text('关闭'),
            ),
          ],
        ),
      );
    } catch (error) {
      if (mounted)
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('测试失败：$error')));
    }
  }

  Widget _detail(String label, String value) => Padding(
    padding: const EdgeInsets.only(bottom: 6),
    child: Text('$label：${value.isEmpty ? '—' : value}'),
  );

  Future<void> _trigger(ReminderDto reminder) async {
    try {
      await ref
          .read(reminderServiceProvider)
          .trigger(reminder.id, reminder.generation);
      await _load();
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('任务已受理，请在执行历史查看发送结果')));
      }
    } catch (error) {
      if (mounted)
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('触发失败：$error')));
    }
  }

  Future<void> _toggle(ReminderDto reminder) async {
    try {
      await ref
          .read(reminderServiceProvider)
          .toggle(reminder.id, reminder.generation);
      await _load();
    } catch (error) {
      if (mounted)
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('操作失败：$error')));
    }
  }

  Future<void> _delete(ReminderDto reminder) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('删除任务'),
        content: Text('确定删除“${reminder.title}”吗？'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext, false),
            child: const Text('取消'),
          ),
          TextButton(
            onPressed: () => Navigator.pop(dialogContext, true),
            child: Text('删除', style: TextStyle(color: context.error)),
          ),
        ],
      ),
    );
    if (confirmed != true) return;
    try {
      await ref
          .read(reminderServiceProvider)
          .delete(reminder.id, reminder.generation);
      await _load();
    } catch (error) {
      if (mounted)
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('删除失败：$error')));
    }
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
      if (mounted)
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('加载调度器状态失败：$error')));
      return;
    }
    if (!mounted) return;
    final cleanupController = TextEditingController(
      text: (cleanup['cleanupDays'] ?? '0').toString(),
    );
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
          padding: EdgeInsets.fromLTRB(
            AppSpacing.xl,
            0,
            AppSpacing.xl,
            AppSpacing.xl,
          ),
          children: [
            Text('任务调度器', style: AppTypography.sectionTitle(context)),
            SizedBox(height: AppSpacing.md),
            Wrap(
              spacing: AppSpacing.sm,
              runSpacing: AppSpacing.sm,
              children: [
                AmitiaStatusBadge(
                  label: status['schedulerRunning'] == true
                      ? '调度器运行中'
                      : '调度器未运行',
                  type: status['schedulerRunning'] == true
                      ? BadgeType.success
                      : BadgeType.error,
                ),
                AmitiaStatusBadge(
                  label: '队列深度 ${queue['depth'] ?? 0}',
                  type: BadgeType.neutral,
                ),
                AmitiaStatusBadge(
                  label: '失败 ${queue['recentFailures'] ?? 0}',
                  type: BadgeType.warning,
                ),
                AmitiaStatusBadge(
                  label: queue['backpressure'] == true ? '存在背压' : '队列正常',
                  type: queue['backpressure'] == true
                      ? BadgeType.error
                      : BadgeType.success,
                ),
              ],
            ),
            if (queue['backpressure'] == true) ...[
              SizedBox(height: AppSpacing.md),
              AmitiaButton(
                label: '清除背压标记',
                isSecondary: true,
                onPressed: () async {
                  try {
                    await service.clearBackpressure();
                    if (sheetContext.mounted)
                      ScaffoldMessenger.of(sheetContext).showSnackBar(
                        const SnackBar(content: Text('已重置熔断，请检查阻断及结果未知的任务')),
                      );
                  } catch (error) {
                    if (sheetContext.mounted)
                      ScaffoldMessenger.of(
                        sheetContext,
                      ).showSnackBar(SnackBar(content: Text('清理失败：$error')));
                  }
                },
              ),
            ],
            SizedBox(height: AppSpacing.lg),
            Text('自动清理', style: AppTypography.cardTitle(context)),
            SizedBox(height: AppSpacing.xs),
            AmitiaTextField(
              controller: cleanupController,
              keyboardType: TextInputType.number,
              hintText: '触发后保留天数；0 表示不自动清理',
            ),
            SizedBox(height: AppSpacing.sm),
            AmitiaButton(
              label: '保存清理策略',
              isSecondary: true,
              onPressed: () async {
                try {
                  await service.setCleanupConfig(
                    cleanupController.text.trim().isEmpty
                        ? '0'
                        : cleanupController.text.trim(),
                  );
                  if (sheetContext.mounted)
                    ScaffoldMessenger.of(
                      sheetContext,
                    ).showSnackBar(const SnackBar(content: Text('清理策略已更新')));
                } catch (error) {
                  if (sheetContext.mounted)
                    ScaffoldMessenger.of(
                      sheetContext,
                    ).showSnackBar(SnackBar(content: Text('保存失败：$error')));
                }
              },
            ),
            SizedBox(height: AppSpacing.lg),
            Text('待执行任务', style: AppTypography.cardTitle(context)),
            SizedBox(height: AppSpacing.sm),
            if (prospective.isEmpty)
              Text('暂无待执行任务', style: AppTypography.caption(context))
            else
              ...prospective.map(
                (item) => ListTile(
                  contentPadding: EdgeInsets.zero,
                  title: Text((item['title'] ?? '').toString()),
                  subtitle: Text((item['remindAt'] ?? '').toString()),
                  trailing: Text((item['status'] ?? '').toString()),
                ),
              ),
            SizedBox(height: AppSpacing.lg),
            Text('最近触发历史', style: AppTypography.cardTitle(context)),
            TextButton(
              onPressed: _showHistory,
              child: const Text('查看全部执行历史与分目标结果'),
            ),
            SizedBox(height: AppSpacing.sm),
            ...((history['items'] as List?) ?? const []).whereType<Map>().map(
              (item) => ListTile(
                contentPadding: EdgeInsets.zero,
                title: Text((item['title'] ?? '').toString()),
                subtitle: Text(
                  '${item['createdAt'] ?? ''}${(item['lastError'] ?? '').toString().isNotEmpty ? ' · ${item['lastError']}' : ''}',
                ),
                trailing: Text((item['state'] ?? '').toString()),
              ),
            ),
          ],
        ),
      ),
    );
    cleanupController.dispose();
  }

  Future<void> _showHistory() async {
    var page = 1;
    var state = '';
    var loading = false;
    var initialLoad = true;
    var total = 0;
    var rows = <Map<String, dynamic>>[];
    String? error;
    final service = ref.read(reminderServiceProvider);
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      builder: (sheetContext) => StatefulBuilder(
        builder: (sheetContext, update) {
          Future<void> load() async {
            update(() {
              loading = true;
              error = null;
            });
            try {
              final result = await service.triggerHistory(
                page: page,
                state: state,
              );
              if (!sheetContext.mounted) return;
              update(() {
                rows = (result['items'] as List)
                    .map((item) => Map<String, dynamic>.from(item as Map))
                    .toList();
                total = (result['total'] as num).toInt();
              });
            } catch (value) {
              if (sheetContext.mounted)
                update(() {
                  error = value.toString();
                  rows = [];
                });
            } finally {
              if (sheetContext.mounted) update(() => loading = false);
            }
          }

          if (initialLoad) {
            initialLoad = false;
            scheduleMicrotask(load);
          }
          return SafeArea(
            child: SizedBox(
              height: MediaQuery.of(sheetContext).size.height * .85,
              child: Column(
                children: [
                  const ListTile(title: Text('执行历史')),
                  DropdownButton<String>(
                    value: state,
                    items:
                        const {
                              '': '全部',
                              'waiting': '待处理',
                              'running': '执行中',
                              'completed': '已完成',
                              'partial': '部分完成',
                              'blocked': '已阻断',
                              'skipped': '已跳过',
                              'failed': '失败',
                              'cancelled': '已取消',
                            }.entries
                            .map(
                              (entry) => DropdownMenuItem(
                                value: entry.key,
                                child: Text(entry.value),
                              ),
                            )
                            .toList(),
                    onChanged: loading
                        ? null
                        : (value) {
                            state = value ?? '';
                            page = 1;
                            unawaited(load());
                          },
                  ),
                  TextButton(
                    onPressed: loading ? null : load,
                    child: const Text('加载 / 刷新'),
                  ),
                  if (error != null)
                    Text(error!, style: TextStyle(color: context.error)),
                  if (loading) const LinearProgressIndicator(),
                  Expanded(
                    child: ListView(
                      children: rows.map((item) {
                        final result = item['result'] is Map
                            ? item['result'] as Map
                            : const {};
                        final targets = result['targets'] is List
                            ? result['targets'] as List
                            : const [];
                        return ListTile(
                          title: Text('${item['title']} · ${item['state']}'),
                          subtitle: Text(
                            '${item['createdAt']}\n${item['lastError'] ?? ''}\n${targets.map((target) => '${target['kind']} ${target['id']}：${target['state']} ${target['error'] ?? ''}').join('\n')}',
                          ),
                        );
                      }).toList(),
                    ),
                  ),
                  Row(
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      TextButton(
                        onPressed: loading || page <= 1
                            ? null
                            : () {
                                page--;
                                unawaited(load());
                              },
                        child: const Text('上一页'),
                      ),
                      Text('$page / ${total == 0 ? 1 : (total / 20).ceil()}'),
                      TextButton(
                        onPressed: loading || page * 20 >= total
                            ? null
                            : () {
                                page++;
                                unawaited(load());
                              },
                        child: const Text('下一页'),
                      ),
                    ],
                  ),
                ],
              ),
            ),
          );
        },
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

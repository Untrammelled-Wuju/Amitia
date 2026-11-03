import 'dart:async';
import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/app_routes.dart';
import '../../../../core/models/reminder.dart';
import '../../../../core/models/attention.dart';
import '../../../../core/models/character.dart';
import '../../../../core/models/conversation.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../chat/runtime/conversation_runtime_controller.dart';

class RemindersPage extends ConsumerStatefulWidget {
  const RemindersPage({super.key, this.initialCharacterId = ''});

  final String initialCharacterId;

  @override
  ConsumerState<RemindersPage> createState() => _RemindersPageState();
}

class _RemindersPageState extends ConsumerState<RemindersPage> {
  List<ReminderDto> _reminders = const [];
  List<AttentionItem> _attentionItems = const [];
  AttentionBudgetStats? _attentionStats;
  String? _attentionError;
  List<AgentSourcePolicy> _sourcePolicies = const [];
  String? _sourcePolicyError;
  Map<String, dynamic> _status = const {};
  int _selectedSegment = 0;
  String _taskKind = 'user_reminder';
  bool _loading = true;
  String? _error;
  Timer? _refreshTimer;
  bool _refreshing = false;
  bool get _isProactiveTask => _taskKind == 'proactive_task';

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
        service.list(kind: _taskKind),
        service.status(kind: _taskKind),
      ]);
      final attentionService = ref.read(attentionServiceProvider);
      List<AttentionItem> attentionItems = const [];
      AttentionBudgetStats? attentionStats;
      String? attentionError;
      List<AgentSourcePolicy> sourcePolicies = const [];
      String? sourcePolicyError;
      try {
        final attentionValues = await Future.wait<dynamic>([
          attentionService.items(),
          attentionService.budget(),
        ]);
        attentionItems = attentionValues[0] as List<AttentionItem>;
        attentionStats = attentionValues[1] as AttentionBudgetStats;
        try {
          sourcePolicies = await attentionService.policies();
        } catch (policyFailure) {
          sourcePolicyError = policyFailure.toString();
        }
      } catch (attentionFailure) {
        attentionError = attentionFailure.toString();
      }
      if (!mounted) return;
      setState(() {
        _reminders = values[0] as List<ReminderDto>;
        _status = values[1] as Map<String, dynamic>;
        _attentionItems = attentionItems;
        _attentionStats = attentionStats;
        _attentionError = attentionError;
        _sourcePolicies = sourcePolicies;
        _sourcePolicyError = sourcePolicyError;
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
    final filtered = widget.initialCharacterId.isEmpty
        ? _reminders
        : _reminders
              .where((item) => item.characterId == widget.initialCharacterId)
              .toList(growable: false);
    switch (_selectedSegment) {
      case 1:
        return filtered.where((item) => item.isEnabled).toList(growable: false);
      case 2:
        return filtered
            .where((item) => !item.isEnabled)
            .toList(growable: false);
      default:
        return filtered;
    }
  }

  Future<void> _setTaskKind(String kind) async {
    if (_taskKind == kind) return;
    setState(() {
      _taskKind = kind;
      _selectedSegment = 0;
      _reminders = const [];
      _loading = true;
    });
    await _load();
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
            if (_attentionError != null)
              Padding(
                padding: EdgeInsets.fromLTRB(
                  AppSpacing.pagePadding,
                  0,
                  AppSpacing.pagePadding,
                  AppSpacing.sm,
                ),
                child: AmitiaErrorState(
                  message: '待处理事项读取失败：$_attentionError',
                  onRetry: () => _load(showLoading: false),
                ),
              )
            else if (_attentionItems.isNotEmpty)
              _buildAttentionSection(),
            Padding(
              padding: EdgeInsets.fromLTRB(
                AppSpacing.pagePadding,
                0,
                AppSpacing.pagePadding,
                AppSpacing.sm,
              ),
              child: AmitiaSegmentedControl(
                segments: const ['定时任务', '主动消息'],
                selectedIndex: _taskKind == 'user_reminder' ? 0 : 1,
                onChanged: (index) => _setTaskKind(
                  index == 0 ? 'user_reminder' : 'proactive_task',
                ),
              ),
            ),
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

  Widget _buildAttentionSection() {
    final visible = _attentionItems.take(5).toList(growable: false);
    return Padding(
      padding: EdgeInsets.fromLTRB(
        AppSpacing.pagePadding,
        0,
        AppSpacing.pagePadding,
        AppSpacing.sm,
      ),
      child: AmitiaCard(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                const Icon(Icons.priority_high_rounded, size: 20),
                SizedBox(width: AppSpacing.xs),
                Expanded(
                  child: Text(
                    '现在需要我处理',
                    style: AppTypography.cardTitle(context),
                  ),
                ),
                AmitiaStatusBadge(
                  label: '${_attentionItems.length} 项',
                  type: BadgeType.warning,
                ),
                SizedBox(width: AppSpacing.xs),
                AmitiaIconButton(
                  icon: Icons.tune,
                  onPressed: _showAttentionBudget,
                ),
                AmitiaIconButton(
                  icon: Icons.shield_outlined,
                  onPressed: _showSourcePolicies,
                ),
              ],
            ),
            if (_attentionStats != null) ...[
              SizedBox(height: AppSpacing.xs),
              Text(
                '今日打断 ${_attentionStats!.todayInterruptions} · '
                '抑制 ${_attentionStats!.todaySuppressed} · '
                '影子观察 ${_attentionStats!.todayObserved}',
                style: AppTypography.caption(context),
              ),
            ],
            if (_sourcePolicyError != null)
              Text(
                '来源策略读取失败：$_sourcePolicyError',
                style: AppTypography.caption(context),
              ),
            SizedBox(height: AppSpacing.sm),
            for (final item in visible)
              InkWell(
                borderRadius: BorderRadius.circular(AppRadius.medium),
                onTap: () => _openAttentionItem(item),
                child: Padding(
                  padding: EdgeInsets.symmetric(vertical: AppSpacing.xs),
                  child: Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Icon(
                        item.kind == 'task_failed'
                            ? Icons.error_outline
                            : item.kind == 'task_result'
                            ? Icons.check_circle_outline
                            : Icons.schedule_outlined,
                        size: 18,
                      ),
                      SizedBox(width: AppSpacing.xs),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              item.title,
                              style: AppTypography.bodySmall(context),
                            ),
                            if (item.summary.isNotEmpty)
                              Text(
                                item.summary,
                                maxLines: 2,
                                overflow: TextOverflow.ellipsis,
                                style: AppTypography.caption(context),
                              ),
                            Text(
                              item.reason,
                              maxLines: 2,
                              overflow: TextOverflow.ellipsis,
                              style: AppTypography.caption(context),
                            ),
                          ],
                        ),
                      ),
                      const Icon(Icons.chevron_right, size: 18),
                    ],
                  ),
                ),
              ),
            if (_attentionItems.length > visible.length)
              TextButton(
                onPressed: () => _showAllAttentionItems(),
                child: Text(
                  '查看其余 ${_attentionItems.length - visible.length} 项',
                ),
              ),
          ],
        ),
      ),
    );
  }

  Future<void> _showAllAttentionItems() async {
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      builder: (sheetContext) => SafeArea(
        child: DraggableScrollableSheet(
          expand: false,
          initialChildSize: .72,
          builder: (_, controller) => ListView.separated(
            controller: controller,
            padding: EdgeInsets.all(AppSpacing.pagePadding),
            itemCount: _attentionItems.length,
            separatorBuilder: (_, _) => SizedBox(height: AppSpacing.sm),
            itemBuilder: (_, index) {
              final item = _attentionItems[index];
              return ListTile(
                leading: const Icon(Icons.task_alt),
                title: Text(item.title),
                subtitle: Text(
                  item.summary.isNotEmpty ? item.summary : item.reason,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                ),
                onTap: () {
                  Navigator.of(sheetContext).pop();
                  unawaited(_openAttentionItem(item));
                },
              );
            },
          ),
        ),
      ),
    );
  }

  Future<void> _openAttentionItem(AttentionItem item) async {
    final service = ref.read(attentionServiceProvider);
    List<AttentionTimelineEvent> timeline = const [];
    String timelineError = '';
    try {
      timeline = await service.timeline(
        requestId: item.requestId,
        threadId: item.threadId,
        coordinationId: item.coordinationId,
        approvalId: item.approvalId,
        workflowRunId: item.workflowRunId,
      );
    } catch (error) {
      timelineError = error.toString();
    }
    if (!mounted) return;
    final action = await showModalBottomSheet<String>(
      context: context,
      isScrollControlled: true,
      builder: (sheetContext) => SafeArea(
        child: Padding(
          padding: EdgeInsets.all(AppSpacing.pagePadding),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(item.title, style: AppTypography.cardTitle(context)),
              if (item.summary.isNotEmpty) Text(item.summary),
              SizedBox(height: AppSpacing.sm),
              Text(item.reason, style: AppTypography.caption(context)),
              SizedBox(height: AppSpacing.md),
              Row(
                children: [
                  if (item.kind == 'agent_approval' ||
                      item.kind == 'task_approval') ...[
                    TextButton(
                      onPressed: () => Navigator.pop(sheetContext, 'reject'),
                      child: Text(
                        '拒绝',
                        style: TextStyle(color: sheetContext.error),
                      ),
                    ),
                    FilledButton(
                      onPressed: () => Navigator.pop(sheetContext, 'approve'),
                      child: Text(
                        item.kind == 'agent_approval' ? '本次允许' : '批准本次执行',
                      ),
                    ),
                  ],
                  if (item.threadId.isNotEmpty)
                    TextButton(
                      onPressed: () {
                        Navigator.of(sheetContext).pop();
                        context.push(AppRoutes.continuityDetail(item.threadId));
                      },
                      child: const Text('打开持续事项'),
                    ),
                  if (item.workflowId.isNotEmpty)
                    TextButton(
                      onPressed: () {
                        Navigator.of(sheetContext).pop();
                        final deviceId = (item.payload['deviceId'] ?? '')
                            .toString();
                        context.push(
                          AppRoutes.workflowEditor(
                            item.workflowId,
                            location: deviceId.isEmpty ? 'cloud' : 'device',
                            deviceId: deviceId,
                          ),
                        );
                      },
                      child: const Text('打开自动化'),
                    ),
                  if (item.conversationId.isNotEmpty)
                    TextButton(
                      onPressed: () {
                        Navigator.of(sheetContext).pop();
                        context.push(
                          Uri(
                            path: AppRoutes.chat,
                            queryParameters: {
                              'conversationId': item.conversationId,
                            },
                          ).toString(),
                        );
                      },
                      child: const Text('打开对话'),
                    ),
                ],
              ),
              SizedBox(height: AppSpacing.sm),
              Text('决策时间线', style: AppTypography.cardTitle(context)),
              SizedBox(height: AppSpacing.xs),
              if (timelineError.isNotEmpty)
                Text(timelineError, style: AppTypography.caption(context))
              else if (timeline.isEmpty)
                Text('暂无记录', style: AppTypography.caption(context))
              else
                ConstrainedBox(
                  constraints: const BoxConstraints(maxHeight: 320),
                  child: ListView.builder(
                    shrinkWrap: true,
                    itemCount: timeline.length,
                    itemBuilder: (_, index) {
                      final event = timeline[index];
                      final details = _timelineDetails(event.details);
                      return ListTile(
                        dense: true,
                        leading: const Icon(Icons.circle, size: 8),
                        title: Text(event.title),
                        subtitle: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              [
                                if (event.summary.isNotEmpty) event.summary,
                                event.source,
                                event.status,
                              ].where((value) => value.isNotEmpty).join(' · '),
                            ),
                            if (details.isNotEmpty)
                              Padding(
                                padding: const EdgeInsets.only(top: 6),
                                child: SelectableText(
                                  details,
                                  style: AppTypography.caption(
                                    context,
                                  ).copyWith(fontFamily: 'monospace'),
                                ),
                              ),
                          ],
                        ),
                      );
                    },
                  ),
                ),
            ],
          ),
        ),
      ),
    );
    if (action == null || !mounted) return;
    try {
      await _resolveAttentionApproval(item, action == 'approve');
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(action == 'approve' ? '已批准本次执行' : '已拒绝本次执行')),
        );
      }
      await _load(showLoading: false);
    } catch (error, stackTrace) {
      print("[original-error] $error\n$stackTrace");
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(error.toString())));
      }
    }
  }

  Future<void> _resolveAttentionApproval(
    AttentionItem item,
    bool approved,
  ) async {
    switch (item.kind) {
      case 'agent_approval':
        if (item.conversationId.isEmpty || item.approvalId.isEmpty) {
          throw StateError('工具审批缺少会话或审批标识');
        }
        await ref
            .read(conversationRuntimeRegistryProvider)
            .sessionForConversation(item.conversationId)
            .resolveApproval(item.approvalId, approved);
      case 'task_approval':
        final approvals = await ref
            .read(extensionTaskServiceProvider)
            .listSourceTaskApprovals();
        final approval = approvals
            .where((candidate) => candidate['id'] == item.approvalId)
            .firstOrNull;
        if (approval == null) {
          throw StateError('设备审批已不存在或已处理');
        }
        await ref
            .read(extensionTaskServiceProvider)
            .decideSourceTaskApproval(approval, approved);
      default:
        throw StateError('当前事项不支持审批操作');
    }
  }

  Future<void> _showAttentionBudget() async {
    final service = ref.read(attentionServiceProvider);
    var policy = _attentionStats?.policy ?? const AttentionBudgetPolicy();
    var saving = false;
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      builder: (sheetContext) => StatefulBuilder(
        builder: (context, setSheetState) => SafeArea(
          child: Padding(
            padding: EdgeInsets.all(AppSpacing.pagePadding),
            child: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text('注意力预算', style: AppTypography.cardTitle(context)),
                  SwitchListTile(
                    title: const Text('启用预算'),
                    value: policy.enabled,
                    onChanged: (value) => setSheetState(
                      () => policy = policy.copyWith(enabled: value),
                    ),
                  ),
                  SwitchListTile(
                    title: const Text('影子模式'),
                    subtitle: const Text('只记录本应抑制的提醒，不改变实际发送'),
                    value: policy.shadowMode,
                    onChanged: (value) => setSheetState(
                      () => policy = policy.copyWith(shadowMode: value),
                    ),
                  ),
                  _numberSetting(
                    '每日打断上限',
                    policy.maxInterruptionsPerDay,
                    (value) => setSheetState(
                      () => policy = policy.copyWith(
                        maxInterruptionsPerDay: value,
                      ),
                    ),
                  ),
                  _numberSetting(
                    '重复确认上限',
                    policy.maxRepeatedConfirmations,
                    (value) => setSheetState(
                      () => policy = policy.copyWith(
                        maxRepeatedConfirmations: value,
                      ),
                    ),
                  ),
                  _numberSetting(
                    '连续追问上限',
                    policy.maxConsecutiveFollowups,
                    (value) => setSheetState(
                      () => policy = policy.copyWith(
                        maxConsecutiveFollowups: value,
                      ),
                    ),
                  ),
                  _numberSetting(
                    '无变化通知上限',
                    policy.maxUnchangedNotifications,
                    (value) => setSheetState(
                      () => policy = policy.copyWith(
                        maxUnchangedNotifications: value,
                      ),
                    ),
                  ),
                  _numberSetting(
                    '去重窗口分钟',
                    policy.dedupeWindowMinutes,
                    (value) => setSheetState(
                      () =>
                          policy = policy.copyWith(dedupeWindowMinutes: value),
                    ),
                  ),
                  SizedBox(height: AppSpacing.sm),
                  SizedBox(
                    width: double.infinity,
                    child: FilledButton(
                      onPressed: saving
                          ? null
                          : () async {
                              setSheetState(() => saving = true);
                              try {
                                final saved = await service.saveBudget(policy);
                                if (mounted) {
                                  setState(() {
                                    policy = saved;
                                    _attentionStats = AttentionBudgetStats(
                                      policy: saved,
                                      todayInterruptions:
                                          _attentionStats?.todayInterruptions ??
                                          0,
                                      todaySuppressed:
                                          _attentionStats?.todaySuppressed ?? 0,
                                      todayObserved:
                                          _attentionStats?.todayObserved ?? 0,
                                    );
                                  });
                                }
                                if (sheetContext.mounted)
                                  Navigator.of(sheetContext).pop();
                                unawaited(_load(showLoading: false));
                              } catch (error) {
                                if (mounted)
                                  ScaffoldMessenger.of(
                                    this.context,
                                  ).showSnackBar(
                                    SnackBar(content: Text(error.toString())),
                                  );
                              } finally {
                                if (sheetContext.mounted)
                                  setSheetState(() => saving = false);
                              }
                            },
                      child: Text(saving ? '保存中…' : '保存预算'),
                    ),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }

  Future<void> _showSourcePolicies() async {
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      builder: (sheetContext) => SafeArea(
        child: DraggableScrollableSheet(
          expand: false,
          initialChildSize: .76,
          builder: (_, controller) => Column(
            children: [
              Padding(
                padding: EdgeInsets.fromLTRB(
                  AppSpacing.pagePadding,
                  AppSpacing.md,
                  AppSpacing.pagePadding,
                  AppSpacing.sm,
                ),
                child: Row(
                  children: [
                    Expanded(
                      child: Text(
                        '来源授权策略',
                        style: AppTypography.cardTitle(context),
                      ),
                    ),
                    FilledButton.icon(
                      onPressed: () {
                        Navigator.pop(sheetContext);
                        unawaited(_editSourcePolicy(null));
                      },
                      icon: const Icon(Icons.add, size: 18),
                      label: const Text('新增'),
                    ),
                  ],
                ),
              ),
              Expanded(
                child: _sourcePolicies.isEmpty
                    ? const AmitiaEmptyState(
                        icon: Icons.shield_outlined,
                        title: '暂无更具体的来源策略',
                        subtitle: '未命中具体策略时，明确请求放行，推断请求仅记录',
                      )
                    : ListView.separated(
                        controller: controller,
                        padding: EdgeInsets.fromLTRB(
                          AppSpacing.pagePadding,
                          0,
                          AppSpacing.pagePadding,
                          AppSpacing.pagePadding,
                        ),
                        itemCount: _sourcePolicies.length,
                        separatorBuilder: (_, _) =>
                            SizedBox(height: AppSpacing.sm),
                        itemBuilder: (_, index) {
                          final policy = _sourcePolicies[index];
                          return AmitiaCard(
                            child: InkWell(
                              onTap: () {
                                Navigator.pop(sheetContext);
                                unawaited(_editSourcePolicy(policy));
                              },
                              child: Padding(
                                padding: EdgeInsets.all(AppSpacing.sm),
                                child: Column(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  children: [
                                    Row(
                                      children: [
                                        Expanded(
                                          child: Text(
                                            _policyScopeLabel(policy),
                                            style: AppTypography.bodySmall(
                                              context,
                                            ),
                                          ),
                                        ),
                                        AmitiaStatusBadge(
                                          label: policy.enabled ? '已启用' : '已停用',
                                          type: policy.enabled
                                              ? BadgeType.success
                                              : BadgeType.neutral,
                                        ),
                                      ],
                                    ),
                                    SizedBox(height: AppSpacing.xs),
                                    Text(
                                      '明确 ${_policyModeLabel(policy.explicitMode)} · '
                                      '推断 ${_policyModeLabel(policy.inferredMode)} · '
                                      '权限 ${_policyPermissionLabel(policy.permissionCeiling)}',
                                      style: AppTypography.caption(context),
                                    ),
                                  ],
                                ),
                              ),
                            ),
                          );
                        },
                      ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Future<void> _editSourcePolicy(AgentSourcePolicy? existing) async {
    var scopeType = existing?.scopeType ?? 'channel';
    var explicitMode = existing?.explicitMode ?? 'allow';
    var inferredMode = existing?.inferredMode ?? 'observe';
    var triggerMode = existing?.triggerMode ?? 'direct_or_mention';
    var permissionCeiling = existing?.permissionCeiling ?? 'request_approval';
    var enabled = existing?.enabled ?? true;
    var saving = false;
    final channel = TextEditingController(text: existing?.channelId ?? '');
    final account = TextEditingController(text: existing?.accountId ?? '');
    final conversation = TextEditingController(
      text: existing?.conversationId ?? '',
    );
    final sender = TextEditingController(text: existing?.senderId ?? '');
    try {
      await showModalBottomSheet<void>(
        context: context,
        isScrollControlled: true,
        builder: (sheetContext) => StatefulBuilder(
          builder: (context, setSheetState) => SafeArea(
            child: Padding(
              padding: EdgeInsets.fromLTRB(
                AppSpacing.pagePadding,
                AppSpacing.md,
                AppSpacing.pagePadding,
                MediaQuery.viewInsetsOf(context).bottom + AppSpacing.md,
              ),
              child: SingleChildScrollView(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      existing == null ? '新增来源策略' : '编辑来源策略',
                      style: AppTypography.cardTitle(context),
                    ),
                    SizedBox(height: AppSpacing.md),
                    DropdownButtonFormField<String>(
                      initialValue: scopeType,
                      decoration: const InputDecoration(labelText: '作用范围'),
                      items: const [
                        DropdownMenuItem(value: 'space', child: Text('空间')),
                        DropdownMenuItem(value: 'channel', child: Text('渠道')),
                        DropdownMenuItem(value: 'account', child: Text('账号')),
                        DropdownMenuItem(
                          value: 'conversation',
                          child: Text('会话'),
                        ),
                        DropdownMenuItem(value: 'sender', child: Text('发送者')),
                      ],
                      onChanged: (value) =>
                          setSheetState(() => scopeType = value ?? 'channel'),
                    ),
                    TextField(
                      controller: channel,
                      decoration: const InputDecoration(labelText: '渠道 ID'),
                    ),
                    TextField(
                      controller: account,
                      decoration: const InputDecoration(labelText: '账号 ID'),
                    ),
                    TextField(
                      controller: conversation,
                      decoration: const InputDecoration(labelText: '会话 ID'),
                    ),
                    TextField(
                      controller: sender,
                      decoration: const InputDecoration(labelText: '发送者 ID'),
                    ),
                    DropdownButtonFormField<String>(
                      initialValue: triggerMode,
                      decoration: const InputDecoration(labelText: '触发方式'),
                      items: const [
                        DropdownMenuItem(
                          value: 'direct_or_mention',
                          child: Text('私聊或明确提及'),
                        ),
                        DropdownMenuItem(value: 'direct', child: Text('仅私聊')),
                        DropdownMenuItem(
                          value: 'mention',
                          child: Text('仅明确提及'),
                        ),
                        DropdownMenuItem(value: 'all', child: Text('全部消息')),
                        DropdownMenuItem(
                          value: 'inferred_only',
                          child: Text('仅推断请求'),
                        ),
                      ],
                      onChanged: (value) => setSheetState(
                        () => triggerMode = value ?? 'direct_or_mention',
                      ),
                    ),
                    _policyModeField(
                      context,
                      label: '明确请求',
                      value: explicitMode,
                      onChanged: (value) =>
                          setSheetState(() => explicitMode = value),
                    ),
                    _policyModeField(
                      context,
                      label: '推断请求',
                      value: inferredMode,
                      onChanged: (value) =>
                          setSheetState(() => inferredMode = value),
                    ),
                    DropdownButtonFormField<String>(
                      initialValue: permissionCeiling,
                      decoration: const InputDecoration(labelText: '权限上限'),
                      items: const [
                        DropdownMenuItem(
                          value: 'request_approval',
                          child: Text('请求批准'),
                        ),
                        DropdownMenuItem(
                          value: 'full_access',
                          child: Text('完全权限'),
                        ),
                      ],
                      onChanged: (value) => setSheetState(
                        () => permissionCeiling = value ?? 'request_approval',
                      ),
                    ),
                    SwitchListTile(
                      contentPadding: EdgeInsets.zero,
                      title: const Text('启用策略'),
                      value: enabled,
                      onChanged: (value) =>
                          setSheetState(() => enabled = value),
                    ),
                    SizedBox(height: AppSpacing.sm),
                    Row(
                      children: [
                        if (existing != null)
                          TextButton(
                            onPressed: saving
                                ? null
                                : () async {
                                    final confirmed = await showDialog<bool>(
                                      context: context,
                                      builder: (dialogContext) => AlertDialog(
                                        title: const Text('删除来源策略？'),
                                        content: const Text('删除后该范围会回退到上一级策略。'),
                                        actions: [
                                          TextButton(
                                            onPressed: () => Navigator.pop(
                                              dialogContext,
                                              false,
                                            ),
                                            child: const Text('取消'),
                                          ),
                                          FilledButton(
                                            onPressed: () => Navigator.pop(
                                              dialogContext,
                                              true,
                                            ),
                                            child: const Text('删除'),
                                          ),
                                        ],
                                      ),
                                    );
                                    if (confirmed != true) return;
                                    setSheetState(() => saving = true);
                                    try {
                                      await ref
                                          .read(attentionServiceProvider)
                                          .deletePolicy(
                                            existing.id,
                                            existing.revision,
                                          );
                                      if (sheetContext.mounted)
                                        Navigator.pop(sheetContext);
                                      await _load(showLoading: false);
                                    } catch (error, stackTrace) {
                                      print(
                                        "[original-error] $error\n$stackTrace",
                                      );
                                      if (context.mounted) {
                                        ScaffoldMessenger.of(
                                          context,
                                        ).showSnackBar(
                                          SnackBar(
                                            content: Text(error.toString()),
                                          ),
                                        );
                                      }
                                    } finally {
                                      if (sheetContext.mounted)
                                        setSheetState(() => saving = false);
                                    }
                                  },
                            child: Text(
                              '删除',
                              style: TextStyle(color: context.error),
                            ),
                          ),
                        const Spacer(),
                        TextButton(
                          onPressed: saving
                              ? null
                              : () => Navigator.pop(sheetContext),
                          child: const Text('取消'),
                        ),
                        SizedBox(width: AppSpacing.xs),
                        FilledButton(
                          onPressed: saving
                              ? null
                              : () async {
                                  setSheetState(() => saving = true);
                                  try {
                                    await ref
                                        .read(attentionServiceProvider)
                                        .savePolicy(
                                          AgentSourcePolicy(
                                            id: existing?.id ?? '',
                                            spaceId: existing?.spaceId ?? '',
                                            scopeType: scopeType,
                                            channelId: channel.text,
                                            accountId: account.text,
                                            conversationId: conversation.text,
                                            senderId: sender.text,
                                            explicitMode: explicitMode,
                                            inferredMode: inferredMode,
                                            triggerMode: triggerMode,
                                            permissionCeiling:
                                                permissionCeiling,
                                            enabled: enabled,
                                            revision: existing?.revision ?? 0,
                                          ),
                                        );
                                    if (sheetContext.mounted)
                                      Navigator.pop(sheetContext);
                                    await _load(showLoading: false);
                                  } catch (error, stackTrace) {
                                    print(
                                      "[original-error] $error\n$stackTrace",
                                    );
                                    if (context.mounted) {
                                      ScaffoldMessenger.of(
                                        context,
                                      ).showSnackBar(
                                        SnackBar(
                                          content: Text(error.toString()),
                                        ),
                                      );
                                    }
                                  } finally {
                                    if (sheetContext.mounted)
                                      setSheetState(() => saving = false);
                                  }
                                },
                          child: Text(saving ? '保存中…' : '保存'),
                        ),
                      ],
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      );
    } finally {
      channel.dispose();
      account.dispose();
      conversation.dispose();
      sender.dispose();
    }
  }

  Widget _policyModeField(
    BuildContext context, {
    required String label,
    required String value,
    required ValueChanged<String> onChanged,
  }) {
    return DropdownButtonFormField<String>(
      initialValue: value,
      decoration: InputDecoration(labelText: label),
      items: const [
        DropdownMenuItem(value: 'allow', child: Text('允许执行')),
        DropdownMenuItem(value: 'observe', child: Text('仅记录')),
        DropdownMenuItem(value: 'deny', child: Text('拒绝')),
      ],
      onChanged: (next) => onChanged(next ?? value),
    );
  }

  String _policyScopeLabel(AgentSourcePolicy policy) {
    final scope = switch (policy.scopeType) {
      'space' => '空间',
      'account' => '账号',
      'conversation' => '会话',
      'sender' => '发送者',
      _ => '渠道',
    };
    final target = [
      policy.channelId,
      policy.accountId,
      policy.conversationId,
      policy.senderId,
    ].where((value) => value.isNotEmpty).join(' / ');
    return target.isEmpty ? scope : '$scope · $target';
  }

  String _policyModeLabel(String mode) {
    return switch (mode) {
      'allow' => '允许',
      'deny' => '拒绝',
      _ => '仅记录',
    };
  }

  String _policyPermissionLabel(String mode) {
    return mode == 'full_access' ? '完全权限' : '请求批准';
  }

  Widget _numberSetting(String label, int value, ValueChanged<int> onChanged) {
    return Padding(
      padding: EdgeInsets.symmetric(vertical: AppSpacing.xs),
      child: Row(
        children: [
          Expanded(child: Text(label)),
          IconButton(
            onPressed: value > 1 ? () => onChanged(value - 1) : null,
            icon: const Icon(Icons.remove_circle_outline),
          ),
          Text('$value'),
          IconButton(
            onPressed: value < 1000 ? () => onChanged(value + 1) : null,
            icon: const Icon(Icons.add_circle_outline),
          ),
        ],
      ),
    );
  }

  String _timelineDetails(Map<String, dynamic> details) {
    if (details.isEmpty) return '';
    return const JsonEncoder.withIndent('  ').convert(details);
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
    'cron': 'Cron 计划',
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
    final cronController = TextEditingController(
      text: (existing?.recurrence['cron'] ?? '0 9 * * *').toString(),
    );
    final minIntervalController = TextEditingController(
      text: (existing?.policy['minInterval'] ?? 60).toString(),
    );
    final maxPerDayController = TextEditingController(
      text: (existing?.policy['maxPerDay'] ?? 6).toString(),
    );
    final maxDailyCallsController = TextEditingController(
      text: (existing?.policy['maxDailyCalls'] ?? 10).toString(),
    );
    final ruleMaxPerDayController = TextEditingController(
      text: (existing?.policy['ruleMaxPerDay'] ?? 1).toString(),
    );
    final randomMinutesController = TextEditingController(
      text: (existing?.policy['randomMinutes'] ?? 0).toString(),
    );
    final activeLevelController = TextEditingController(
      text: (existing?.policy['activeLevel'] ?? 40).toString(),
    );
    final quietStartController = TextEditingController(
      text: (existing?.policy['quietStart'] ?? '23:00').toString(),
    );
    final quietEndController = TextEditingController(
      text: (existing?.policy['quietEnd'] ?? '07:00').toString(),
    );
    final ruleTypeController = TextEditingController(
      text: (existing?.policy['ruleType'] ?? 'custom').toString(),
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
    var repeatRule =
        existing?.repeatRule ?? (_isProactiveTask ? 'cron' : 'none');
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
                  if (_isProactiveTask) ...[
                    SizedBox(height: AppSpacing.md),
                    AmitiaTextField(
                      controller: cronController,
                      hintText: 'Cron 表达式，例如 0 9 * * *',
                    ),
                    SizedBox(height: AppSpacing.md),
                    AmitiaTextField(
                      controller: ruleTypeController,
                      hintText: '规则类型，例如 custom / daily_greeting',
                    ),
                    SizedBox(height: AppSpacing.md),
                    AmitiaTextField(
                      controller: activeLevelController,
                      keyboardType: TextInputType.number,
                      hintText: '主动频率 0-100',
                    ),
                    SizedBox(height: AppSpacing.md),
                    AmitiaTextField(
                      controller: quietStartController,
                      hintText: '安静时段开始，例如 23:00',
                    ),
                    SizedBox(height: AppSpacing.md),
                    AmitiaTextField(
                      controller: quietEndController,
                      hintText: '安静时段结束，例如 07:00',
                    ),
                    SizedBox(height: AppSpacing.md),
                    AmitiaTextField(
                      controller: minIntervalController,
                      keyboardType: TextInputType.number,
                      hintText: '最小间隔（分钟）',
                    ),
                    SizedBox(height: AppSpacing.md),
                    AmitiaTextField(
                      controller: maxPerDayController,
                      keyboardType: TextInputType.number,
                      hintText: '每日总上限',
                    ),
                    SizedBox(height: AppSpacing.md),
                    AmitiaTextField(
                      controller: maxDailyCallsController,
                      keyboardType: TextInputType.number,
                      hintText: '每日模型调用上限',
                    ),
                    SizedBox(height: AppSpacing.md),
                    AmitiaTextField(
                      controller: ruleMaxPerDayController,
                      keyboardType: TextInputType.number,
                      hintText: '单条规则每日上限',
                    ),
                    SizedBox(height: AppSpacing.md),
                    AmitiaTextField(
                      controller: randomMinutesController,
                      keyboardType: TextInputType.number,
                      hintText: '随机延迟（分钟）',
                    ),
                  ],
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
                      if (_isProactiveTask &&
                          cronController.text.trim().isEmpty) {
                        ScaffoldMessenger.of(sheetContext).showSnackBar(
                          const SnackBar(content: Text('请输入 Cron 表达式')),
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
                        'kind': _taskKind,
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
                          if (repeatRule == 'cron')
                            'cron': cronController.text.trim(),
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
                        if (_isProactiveTask)
                          'policy': {
                            'enabled': true,
                            'activeLevel':
                                int.tryParse(activeLevelController.text) ?? 40,
                            'quietStart': quietStartController.text.trim(),
                            'quietEnd': quietEndController.text.trim(),
                            'minInterval':
                                int.tryParse(minIntervalController.text) ?? 60,
                            'maxPerDay':
                                int.tryParse(maxPerDayController.text) ?? 6,
                            'maxDailyCalls':
                                int.tryParse(maxDailyCallsController.text) ??
                                10,
                            'unrepliedSlowdownEnabled': true,
                            'unrepliedSlowdownAfter': 2,
                            'unrepliedCooldownMultiplier': 2,
                            'unrepliedRecoveryOnReply': true,
                            'ruleMaxPerDay':
                                int.tryParse(ruleMaxPerDayController.text) ?? 1,
                            'ruleType': ruleTypeController.text.trim().isEmpty
                                ? 'custom'
                                : ruleTypeController.text.trim(),
                            'randomMinutes':
                                int.tryParse(randomMinutesController.text) ?? 0,
                            'ruleQuietStart': '',
                            'ruleQuietEnd': '',
                          },
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
    cronController.dispose();
    minIntervalController.dispose();
    maxPerDayController.dispose();
    maxDailyCallsController.dispose();
    ruleMaxPerDayController.dispose();
    randomMinutesController.dispose();
    activeLevelController.dispose();
    quietStartController.dispose();
    quietEndController.dispose();
    ruleTypeController.dispose();
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
        service.status(kind: _taskKind),
        service.queueSummary(kind: _taskKind),
        service.cleanupConfig(),
        service.prospective(kind: _taskKind),
        service.triggerHistory(pageSize: 10, kind: _taskKind),
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
                    await service.clearBackpressure(kind: _taskKind);
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
                kind: _taskKind,
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

import 'dart:async';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/app_routes.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../data/workbench_task_service.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import 'workbench_task_snapshot.dart';
import 'workbench_task_checkpoints.dart';
import 'workbench_task_link.dart';
import 'workbench_task_dependency_view.dart';
import 'workbench_task_budget.dart';
import 'workbench_budget_evidence_panel.dart';
import '../../../agent/presentation/providers/agent_tasks_provider.dart';
import '../../../../core/services/providers.dart';
import '../../runtime/workbench_mobile_events.dart';

class WorkbenchTasksView extends ConsumerStatefulWidget {
  const WorkbenchTasksView({super.key, required this.conversationId, this.characterId = '', this.active = true});
  final String conversationId;
  final String characterId;
  final bool active;
  @override
  ConsumerState<WorkbenchTasksView> createState() => _WorkbenchTasksViewState();
}

class _WorkbenchTasksViewState extends ConsumerState<WorkbenchTasksView> with WidgetsBindingObserver {
  WorkbenchTaskSnapshot snapshot = WorkbenchTaskSnapshot.fromJson(null);
  bool busy = false;
  bool snapshotStale = false;
  bool linkedTaskBusy = false;
  int linkedTaskGeneration = 0;
  bool foreground = true;
  String? error;
  int generation = 0;
  Timer? poller;
  final Set<String> expandedTasks = <String>{};

  bool get canQuery => widget.active && foreground && widget.conversationId.isNotEmpty && widget.conversationId != '__draft__';

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    WorkbenchMobileEvents.taskChanges.addListener(_taskChanged);
    _syncPolling();
  }

  void _taskChanged() {
    final change = WorkbenchMobileEvents.taskChanges.value;
    if (change != null && change.matches(conversationId: widget.conversationId, characterId: widget.characterId, api: ref.read(rawBackendServiceApiProvider))) {
      unawaited(refresh(force: true));
    }
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    final wasForeground = foreground;
    foreground = state == AppLifecycleState.resumed;
    if (wasForeground != foreground && mounted) {
      generation++;
      setState(() { snapshot = WorkbenchTaskSnapshot.fromJson(null); snapshotStale = false; expandedTasks.clear(); busy = false; linkedTaskBusy = false; linkedTaskGeneration++; error = null; });
    }
    _syncPolling();
  }

  @override
  void didUpdateWidget(covariant WorkbenchTasksView old) {
    super.didUpdateWidget(old);
    if (old.conversationId != widget.conversationId || old.characterId != widget.characterId) {
      generation++;
      snapshot = WorkbenchTaskSnapshot.fromJson(null);
      snapshotStale = false;
      expandedTasks.clear();
      linkedTaskBusy = false;
      linkedTaskGeneration++;
      error = null;
      busy = false;
    }
    if (old.conversationId != widget.conversationId || old.characterId != widget.characterId || old.active != widget.active) _syncPolling();
  }

  void _syncPolling() {
    poller?.cancel();
    poller = null;
    if (!canQuery) { generation++; linkedTaskGeneration++; linkedTaskBusy = false; busy = false; return; }
    Future.microtask(refresh);
    poller = Timer.periodic(const Duration(seconds: 15), (_) { if (!busy && canQuery) refresh(); });
  }

  @override
  void dispose() {
    generation++;
    linkedTaskGeneration++;
    poller?.cancel();
    WidgetsBinding.instance.removeObserver(this);
    WorkbenchMobileEvents.taskChanges.removeListener(_taskChanged);
    super.dispose();
  }

  Future<void> refresh({bool force = false}) async {
    if (!canQuery || (busy && !force)) return;
    final sequence = ++generation;
    linkedTaskGeneration++;
    final id = widget.conversationId;
    if (mounted) setState(() { busy = true; linkedTaskBusy = false; error = null; });
    try {
      final response = await ref.read(workbenchTaskServiceProvider).read(id, widget.characterId).timeout(const Duration(seconds: 25));
      if (!mounted || sequence != generation || !canQuery || id != widget.conversationId) return;
      late final WorkbenchTaskSnapshot confirmed;
      try {
        confirmed = WorkbenchTaskSnapshot.fromResponse(response, id);
      } on FormatException {
        throw WorkbenchTaskReadFailure('任务响应证据无效，已丢弃缓存', invalidatesSnapshot: true);
      }
      setState(() { snapshot = confirmed; busy = false; snapshotStale = false; error = null; });
    } catch (e) {
      if (!mounted || sequence != generation || !canQuery) return;
      setState(() {
        error = e.toString(); busy = false;
        if ((e is WorkbenchTaskReadFailure && e.invalidatesSnapshot) || (e is StateError && e is! WorkbenchTaskReadFailure)) {
          snapshot = WorkbenchTaskSnapshot.fromJson(null);
          snapshotStale = false;
          expandedTasks.clear();
        } else {
          snapshotStale = snapshot.tasks.isNotEmpty;
          expandedTasks.clear();
        }
      });
    }
  }

  void invalidateAuthorization() {
    generation++;
    linkedTaskGeneration++;
    if (!mounted) return;
    setState(() {
      snapshot = WorkbenchTaskSnapshot.fromJson(null);
      snapshotStale = false;
      expandedTasks.clear();
      busy = false;
      linkedTaskBusy = false;
      error = '检查点权限已变化，已清除先前的任务快照，请重新验证';
    });
  }

  Future<void> openLinkedTaskRun(Map task) async {
    final rawId = task['taskRunId'];
    if (rawId is! String || !mayOpenWorkbenchTaskRun(
      live: canQuery,
      stale: snapshotStale,
      loading: linkedTaskBusy,
      recoveryState: task['recoveryState']?.toString() ?? '',
    )) return;
    final requestGeneration = generation;
    final requestLinkGeneration = ++linkedTaskGeneration;
    final conversationId = widget.conversationId;
    final characterId = widget.characterId;
    final updatedAt = task['updatedAt'];
    setState(() => linkedTaskBusy = true);
    try {
      final runs = await ref.read(extensionTaskServiceProvider).listRuns(limit: 200);
      if (!mounted || requestGeneration != generation ||
          conversationId != widget.conversationId || characterId != widget.characterId ||
          snapshotStale || !canQuery) return;
      final current = snapshot.tasks.where((row) =>
          row['coordinationId'] == task['coordinationId'] &&
          row['taskRunId'] == rawId && row['updatedAt'] == updatedAt &&
          row['recoveryState'] != 'needs-reconciliation').toList();
      if (current.length != 1) throw StateError('关联任务快照已变化，请刷新后重试');
      final authorized = resolveAuthorizedWorkbenchTaskRun(rawId, runs);
      if (authorized == null) throw StateError('当前授权任务列表中不存在该 TaskRun，请在任务中心重新确认');
      ref.invalidate(agentTasksProvider);
      if (!mounted || requestGeneration != generation || snapshotStale) return;
      context.push(AppRoutes.agentTask(rawId), extra: authorized);
    } catch (cause) {
      if (mounted && requestGeneration == generation) {
        setState(() => error = cause.toString().replaceFirst('Bad state: ', ''));
      }
    } finally {
      if (mounted && requestGeneration == generation && requestLinkGeneration == linkedTaskGeneration) setState(() => linkedTaskBusy = false);
    }
  }

  String statusName(String state) => switch (state) {
    'planning' => '规划中', 'running' => '运行中', 'waiting' => '等待中',
    'aggregating' => '整理结果', 'succeeded' => '已结束', 'failed' => '失败',
    'cancelled' => '已取消', 'paused' => '已暂停', _ => state,
  };
  String dependencyStatuses(Map child, List children) {
    if (child['dependencies'] is! List) return '';
    return (child['dependencies'] as List).map((dependency) {
      for (final predecessor in children) {
        if (predecessor is Map && predecessor['id'] == dependency) {
          return '$dependency（${statusName(predecessor['status']?.toString() ?? '')}）';
        }
      }
      return '$dependency（状态未知）';
    }).join('、');
  }
  String recoveryName(String state) => switch (state) {
    'runtime-observed' => '已确认 Runtime', 'needs-reconciliation' => '需协调',
    _ => '历史检查点',
  };

  @override
  Widget build(BuildContext context) {
    ref.listen(rawBackendServiceApiProvider, (previous, next) {
      generation++;
      setState(() { snapshot = WorkbenchTaskSnapshot.fromJson(null); snapshotStale = false; expandedTasks.clear(); busy = false; linkedTaskBusy = false; linkedTaskGeneration++; error = null; });
      if (canQuery) Future.microtask(refresh);
    });
    return Column(children: [
    ListTile(
      dense: true,
      title: const Text('多 Agent 任务状态'),
      trailing: IconButton(icon: const Icon(Icons.refresh), onPressed: !canQuery ? null : () => refresh(force: true)),
    ),
    const Padding(
      padding: EdgeInsets.symmetric(horizontal: 12, vertical: 4),
      child: Text('历史检查点不代表实时运行；子任务结束不等于测试和合并已通过。', style: TextStyle(fontSize: 11)),
    ),
    Padding(
      padding: const EdgeInsets.symmetric(horizontal: 12),
      child: TextButton(
        onPressed: () => context.push(AppRoutes.agent),
        child: const Text('前往 Agent 任务中心管理正式任务（暂停、恢复、取消、重试）'),
      ),
    ),
    Wrap(
      alignment: WrapAlignment.center,
      children: [
        TextButton(onPressed: () => context.push(AppRoutes.characters), child: const Text('角色配置')),
        TextButton(onPressed: () => context.push(AppRoutes.continuity), child: const Text('长期目标')),
      ],
    ),
    Padding(
      padding: const EdgeInsets.symmetric(horizontal: 8),
      child: Wrap(spacing: 6, runSpacing: 4, children: [
        Chip(label: Text('总数 ${snapshot.summary.total}')),
        Chip(label: Text('进行中/待核实 ${snapshot.summary.active}')),
        Chip(label: Text('完成 ${snapshot.summary.succeeded}')),
        Chip(label: Text('失败/取消 ${snapshot.summary.failed}')),
        if (snapshot.summary.needsReconciliation > 0) Chip(label: Text('需协调 ${snapshot.summary.needsReconciliation}')),
      ]),
    ),
    if (busy) const LinearProgressIndicator(),
    if (error != null) Padding(padding: const EdgeInsets.all(12), child: Text(error!, style: TextStyle(color: Theme.of(context).colorScheme.error))),
    if (snapshotStale && snapshot.tasks.isNotEmpty) const Padding(padding: EdgeInsets.all(8), child: Text('连接中断，仅显示当前授权会话最后确认的只读快照；任务可能已变化，请勿据此判断已完成。', style: TextStyle(fontSize: 11))),
    if (snapshot.liveObservationTruncated) const Padding(padding: EdgeInsets.all(8), child: Text('实时快照仅检查最近 8 个任务，其余显示历史检查点。', style: TextStyle(fontSize: 11))),
    if (snapshot.truncated) const Padding(padding: EdgeInsets.all(8), child: Text('仅展示最近 30 项，较旧任务未计入摘要。', style: TextStyle(fontSize: 11))),
    Expanded(child: RefreshIndicator(
      onRefresh: () => refresh(force: true),
      child: ListView.builder(
        itemCount: snapshot.tasks.isEmpty ? 1 : snapshot.tasks.length,
        itemBuilder: (context,index) {
          if (snapshot.tasks.isEmpty) return const ListTile(title: Text('暂无任务检查点'));
          final task = snapshot.tasks[index];
          final children = task['assignments'] is List ? task['assignments'] as List : const [];
          final dependencyRows = WorkbenchTaskDependencyView.project(children);
          final recovery = task['recoveryState']?.toString() ?? 'checkpoint-only';
          final coordinationId = task['coordinationId']?.toString() ?? '';
          return Card(child: ExpansionTile(
            key: ValueKey('${widget.conversationId}:$coordinationId'),
            onExpansionChanged: (expanded) => setState(() { if (expanded) { expandedTasks.add(coordinationId); } else { expandedTasks.remove(coordinationId); } }),
            title: Text(recovery == 'needs-reconciliation' ? '待对账（不可确认终态）' : statusName(task['status']?.toString() ?? '')),
            subtitle: Text('结束 ${task['completedCount'] ?? 0}/${task['assignmentCount'] ?? 0} · ${recoveryName(recovery)}${task['goalId'] == null ? '' : '\n目标 ${task['goalId']} · 修订 ${task['goalRevision'] ?? 0}'}${task['deviceId'] != null ? '\n设备 ${task['deviceId']}' : task['executionCoreId'] == null ? '' : '\nCore ${task['executionCoreId']}'}'),
            children: [
              if (task['taskRunId'] != null) ListTile(
                dense: true,
                title: Text('关联正式 TaskRun：${task['taskRunId']}'),
                subtitle: Text(recovery == 'needs-reconciliation' || snapshotStale
                    ? '状态待核实，刷新可信快照后才能进入任务中心。'
                    : '重新读取当前授权任务列表后打开详情，控制仍由正式 TaskRuntime 鉴权。'),
                trailing: const Icon(Icons.open_in_new),
                enabled: mayOpenWorkbenchTaskRun(
                  live: canQuery,
                  stale: snapshotStale,
                  loading: linkedTaskBusy,
                  recoveryState: recovery,
                ),
                onTap: () => openLinkedTaskRun(task),
              ),
              if (task['cancelRequested'] == true) const ListTile(dense: true, title: Text('已请求取消，但不代表外部工具动作已经停止。')),
              if (task['deadlineAt'] != null) ListTile(dense: true, title: Text('协调截止 ${task['deadlineAt']}')),
              ListTile(dense: true, title: Text(WorkbenchTaskBudget.notice(task, now: DateTime.now(), stale: snapshotStale))),
              if (recovery == 'needs-reconciliation') ListTile(dense: true, title: Text(task['recoveryReason'] == 'parent-state-conflict' ? '父任务已结束，但检查点尚未结束。' : task['recoveryReason'] == 'parent-terminal-child-pending' ? '父任务已结束但子任务尚未确认终态，需要对账。' : task['recoveryReason'] == 'terminal-child-deadline-unverified' ? '子任务未证明在截止时间内完成，成功结果需要对账。' : task['recoveryReason']?.toString().startsWith('terminal-') == true ? '子任务终态缺少可信回执，暂不计为已完成。' : 'Runtime 快照与持久化检查点冲突。')),
              if (!snapshotStale && canQuery && task['parentInteractionId'] is String && expandedTasks.contains(coordinationId)) WorkbenchBudgetEvidencePanel(key: ValueKey('budget:${widget.conversationId}:$coordinationId:${task['updatedAt']}'), conversationId: widget.conversationId, characterId: widget.characterId, coordinationId: coordinationId, parentInteractionId: task['parentInteractionId'] as String, onAuthorizationInvalidated: invalidateAuthorization),
              if (coordinationId.isNotEmpty && expandedTasks.contains(coordinationId)) WorkbenchTaskCheckpoints(key: ValueKey('${widget.conversationId}:$coordinationId:${task['updatedAt']}'), conversationId: widget.conversationId, characterId: widget.characterId, coordinationId: coordinationId, onAuthorizationInvalidated: invalidateAuthorization),
              ...dependencyRows.take(8).map((view) {
                final child = view['assignment'] as Map;
                return ListTile(
                  dense: true,
                  title: Text(child['objective']?.toString() ?? ''),
                  subtitle: Text('第 ${(view['depth'] as int) + 1} 层 · ${statusName(child['status']?.toString() ?? '')}${child['reviewRequired'] == true ? ' · 须独立审查' : ''}${(view['notice'] as String).isNotEmpty ? '\n${view['notice']}' : ''}${child['dependencies'] is List && (child['dependencies'] as List).isNotEmpty ? '\n前置任务：${dependencyStatuses(child, children)}' : ''}'),
                );
              }),
            ],
          ));
        },
      ),
    )),
  ]);
  }
}

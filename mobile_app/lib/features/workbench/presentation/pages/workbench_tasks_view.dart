import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../data/workbench_task_service.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import 'workbench_task_snapshot.dart';
import 'workbench_task_checkpoints.dart';
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
    foreground = state == AppLifecycleState.resumed;
    _syncPolling();
  }

  @override
  void didUpdateWidget(covariant WorkbenchTasksView old) {
    super.didUpdateWidget(old);
    if (old.conversationId != widget.conversationId || old.characterId != widget.characterId) {
      generation++;
      snapshot = WorkbenchTaskSnapshot.fromJson(null);
      expandedTasks.clear();
      error = null;
      busy = false;
    }
    if (old.conversationId != widget.conversationId || old.characterId != widget.characterId || old.active != widget.active) _syncPolling();
  }

  void _syncPolling() {
    poller?.cancel();
    poller = null;
    if (!canQuery) { generation++; busy = false; return; }
    Future.microtask(refresh);
    poller = Timer.periodic(const Duration(seconds: 15), (_) { if (!busy && canQuery) refresh(); });
  }

  @override
  void dispose() {
    generation++;
    poller?.cancel();
    WidgetsBinding.instance.removeObserver(this);
    WorkbenchMobileEvents.taskChanges.removeListener(_taskChanged);
    super.dispose();
  }

  Future<void> refresh({bool force = false}) async {
    if (!canQuery || (busy && !force)) return;
    final sequence = ++generation;
    final id = widget.conversationId;
    if (mounted) setState(() { busy = true; error = null; });
    try {
      final response = await ref.read(workbenchTaskServiceProvider).read(id, widget.characterId).timeout(const Duration(seconds: 25));
      if (!mounted || sequence != generation || !canQuery || id != widget.conversationId) return;
      setState(() { snapshot = WorkbenchTaskSnapshot.fromResponse(response, id); busy = false; });
    } catch (e) {
      if (!mounted || sequence != generation || !canQuery) return;
      setState(() { error = e.toString(); busy = false; snapshot = WorkbenchTaskSnapshot.fromJson(null); });
    }
  }

  String statusName(String state) => switch (state) {
    'planning' => '规划中', 'running' => '运行中', 'waiting' => '等待中',
    'aggregating' => '整理结果', 'succeeded' => '已结束', 'failed' => '失败',
    'cancelled' => '已取消', 'paused' => '已暂停', _ => state,
  };
  String recoveryName(String state) => switch (state) {
    'runtime-observed' => '已确认 Runtime', 'needs-reconciliation' => '需协调',
    _ => '历史检查点',
  };

  @override
  Widget build(BuildContext context) {
    ref.listen(rawBackendServiceApiProvider, (previous, next) {
      generation++;
      setState(() { snapshot = WorkbenchTaskSnapshot.fromJson(null); expandedTasks.clear(); busy = false; error = null; });
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
      padding: const EdgeInsets.symmetric(horizontal: 8),
      child: Wrap(spacing: 6, runSpacing: 4, children: [
        Chip(label: Text('总数 ${snapshot.summary.total}')),
        Chip(label: Text('进行中 ${snapshot.summary.active}')),
        Chip(label: Text('完成 ${snapshot.summary.succeeded}')),
        Chip(label: Text('失败/取消 ${snapshot.summary.failed}')),
        if (snapshot.summary.needsReconciliation > 0) Chip(label: Text('需协调 ${snapshot.summary.needsReconciliation}')),
      ]),
    ),
    if (busy) const LinearProgressIndicator(),
    if (error != null) Padding(padding: const EdgeInsets.all(12), child: Text(error!, style: TextStyle(color: Theme.of(context).colorScheme.error))),
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
          final recovery = task['recoveryState']?.toString() ?? 'checkpoint-only';
          final coordinationId = task['coordinationId']?.toString() ?? '';
          return Card(child: ExpansionTile(
            key: ValueKey('${widget.conversationId}:$coordinationId'),
            onExpansionChanged: (expanded) => setState(() { if (expanded) { expandedTasks.add(coordinationId); } else { expandedTasks.remove(coordinationId); } }),
            title: Text(statusName(task['status']?.toString() ?? '')),
            subtitle: Text('结束 ${task['completedCount'] ?? 0}/${task['assignmentCount'] ?? 0} · ${recoveryName(recovery)}'),
            children: [
              if (recovery == 'needs-reconciliation') ListTile(dense: true, title: Text(task['recoveryReason'] == 'parent-state-conflict' ? '父任务已结束，但检查点尚未结束。' : 'Runtime 快照与持久化检查点冲突。')),
              if (coordinationId.isNotEmpty && expandedTasks.contains(coordinationId)) WorkbenchTaskCheckpoints(key: ValueKey('${widget.conversationId}:$coordinationId:${task['updatedAt']}'), conversationId: widget.conversationId, characterId: widget.characterId, coordinationId: coordinationId),
              ...children.take(8).map((child) {
                if (child is! Map) return const SizedBox.shrink();
                return ListTile(
                  dense: true,
                  title: Text(child['objective']?.toString() ?? ''),
                  subtitle: Text('${statusName(child['status']?.toString() ?? '')}${child['reviewRequired'] == true ? ' · 须独立审查' : ''}'),
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

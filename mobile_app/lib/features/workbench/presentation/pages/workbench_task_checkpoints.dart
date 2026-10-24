import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../data/workbench_task_service.dart';

class WorkbenchTaskCheckpoints extends ConsumerStatefulWidget {
  const WorkbenchTaskCheckpoints({super.key, required this.conversationId, required this.coordinationId, this.characterId = '', this.onAuthorizationInvalidated});
  final String conversationId;
  final String characterId;
  final String coordinationId;
  final VoidCallback? onAuthorizationInvalidated;
  @override
  ConsumerState<WorkbenchTaskCheckpoints> createState() => _WorkbenchTaskCheckpointsState();
}

class _WorkbenchTaskCheckpointsState extends ConsumerState<WorkbenchTaskCheckpoints> {
  bool busy = true;
  bool truncated = false;
  String? error;
  List<Map<String, dynamic>> records = const [];
  int generation = 0;

  @override
  void initState() {
    super.initState();
    Future.microtask(refresh);
  }

  @override
  void didUpdateWidget(covariant WorkbenchTaskCheckpoints oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.coordinationId != widget.coordinationId || oldWidget.conversationId != widget.conversationId || oldWidget.characterId != widget.characterId) {
      generation++;
      busy = true;
      records = const [];
      Future.microtask(refresh);
    }
  }

  @override
  void dispose() {
    generation++;
    super.dispose();
  }

  Future<void> refresh() async {
    if (!mounted) return;
    final requestId = ++generation;
    final conversationId = widget.conversationId;
    final coordinationId = widget.coordinationId;
    setState(() { busy = true; error = null; });
    try {
      final response = await ref.read(workbenchTaskServiceProvider).read(conversationId, widget.characterId, coordinationId: coordinationId).timeout(const Duration(seconds: 25));
      if (!mounted || requestId != generation || conversationId != widget.conversationId || coordinationId != widget.coordinationId) return;
      if (response['protocolVersion'] != 1 || response['conversationId'] != conversationId || response['coordinationId'] != coordinationId || response['source'] != 'sampled-durable-checkpoints' || response['records'] is! List || response['truncated'] is! bool) throw const FormatException('检查点响应与当前任务不匹配');
      final raw = response['records'] as List;
      if (raw.length > 20) throw const FormatException('检查点数量异常');
      final parsed = <Map<String, dynamic>>[];
      for (final item in raw) {
        if (item is! Map || item['updatedAt'] is! String || item['coordinationStatus'] is! String || item['parentTerminal'] is! bool || item['consistent'] is! bool || item['assignmentCount'] is! int || item['completedCount'] is! int || item['failedCount'] is! int) throw const FormatException('检查点记录无效');
        final counts = [item['assignmentCount'] as int, item['completedCount'] as int, item['failedCount'] as int];
        if (counts[0] < 1 || counts[0] > 8 || counts[1] < 0 || counts[1] > counts[0] || counts[2] < 0 || counts[2] > counts[1]) throw const FormatException('检查点统计无效');
        if ((item['updatedAt'] as String).length > 80 || DateTime.tryParse(item['updatedAt'] as String) == null ||
            (item['interactionStatus'] is! String) || (item['interactionStatus'] as String).isEmpty || (item['interactionStatus'] as String).length > 64 ||
            !const {'planning', 'running', 'waiting', 'aggregating', 'succeeded', 'failed', 'cancelled', 'paused'}.contains(item['coordinationStatus'])) throw const FormatException('检查点状态无效');
        parsed.add(Map<String, dynamic>.from(item));
      }
      setState(() { records = parsed; truncated = response['truncated'] == true; busy = false; });
    } catch (cause) {
      if (!mounted || requestId != generation) return;
      setState(() { busy = false; error = cause.toString(); records = const []; });
      if ((cause is WorkbenchTaskReadFailure && cause.invalidatesSnapshot) || cause is FormatException) {
        widget.onAuthorizationInvalidated?.call();
      }
    }
  }

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
    child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
      const Text('仅展示已保存检查点样本，不是完整事件日志或测试通过证明。', style: TextStyle(fontSize: 11)),
      if (busy) const LinearProgressIndicator(),
      if (error != null) Text(error!, style: TextStyle(color: Theme.of(context).colorScheme.error)),
      for (final point in records)
        ListTile(
          dense: true,
          title: Text('${point['coordinationStatus']} · ${point['completedCount']}/${point['assignmentCount']} 已结束'),
          subtitle: Text(point['updatedAt']?.toString() ?? ''),
          trailing: point['consistent'] == false ? const Icon(Icons.warning_amber) : null,
        ),
      if (truncated) const Text('样本已截断，不能还原完整执行历史。', style: TextStyle(fontSize: 11)),
    ]),
  );
}

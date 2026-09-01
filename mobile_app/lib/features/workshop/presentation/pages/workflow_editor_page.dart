import 'dart:async';
import 'dart:convert';
import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/services/extension_service.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class WorkflowEditorPage extends ConsumerStatefulWidget {
  final String workflowId;
  final String location;
  final String deviceId;

  const WorkflowEditorPage({
    super.key,
    required this.workflowId,
    this.location = 'cloud',
    this.deviceId = '',
  });

  @override
  ConsumerState<WorkflowEditorPage> createState() => _WorkflowEditorPageState();
}

class _WorkflowEditorPageState extends ConsumerState<WorkflowEditorPage> {
  static const double _canvasWidth = 2400;
  static const double _canvasHeight = 1800;
  static const double _nodeWidth = 190;
  static const double _nodeHeight = 108;

  final TransformationController _transform = TransformationController();
  Map<String, dynamic>? _workflow;
  List<Map<String, dynamic>> _nodes = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> _edges = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> _triggers = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> _catalog = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> _ownedWorkflows = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> _devices = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> _triggerCapabilities = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> _triggerAppCatalog = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> _triggerWakeConfigs = <Map<String, dynamic>>[];
  Map<String, Map<String, dynamic>> _stepRuns = <String, Map<String, dynamic>>{};
  List<Map<String, dynamic>> _stepAttempts = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> _checkpoints = <Map<String, dynamic>>[];
  Map<String, dynamic> _workflowStats = <String, dynamic>{};
  List<Map<String, dynamic>> _runHistory = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> _revisions = <Map<String, dynamic>>[];
  bool _revisionBusy = false;
  String? _connectFrom;
  String? _activeRunId;
  String _activeRunStatus = '';
  bool _loading = true;
  bool _saving = false;
  bool _aiWorking = false;
  bool _dirty = false;
  bool _disposed = false;
  Timer? _pollTimer;
  Timer? _syncTimer;
  int? _syncCursor;
  bool _syncBusy = false;
  int _conflictNoticeRevision = 0;
  int _deviceSyncTicks = 0;

  WorkflowApiTarget get _target {
    if (widget.location == 'local') return const WorkflowApiTarget.local();
    if (widget.location == 'device') return WorkflowApiTarget.device(widget.deviceId);
    return const WorkflowApiTarget.cloud();
  }

  bool get _isCloud => widget.location == 'cloud';
  bool get _isDevice => widget.location == 'device';
  String get _locationLabel => _isCloud ? '云端' : (_isDevice ? '设备 · ${widget.deviceId}' : '当前设备');

  @override
  void initState() {
    super.initState();
    unawaited(_initialize());
  }

  Future<void> _initialize() async {
    await _load();
    if (_disposed) return;
    await _primeSyncCursor();
    if (_disposed) return;
    _syncTimer = Timer.periodic(const Duration(seconds: 2), (_) => unawaited(_pollSyncEvents()));
  }

  @override
  void dispose() {
    _disposed = true;
    _pollTimer?.cancel();
    _syncTimer?.cancel();
    _transform.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    setState(() => _loading = true);
    try {
      final service = ref.read(extensionServiceProvider);
      final workflow = await service.getWorkflow(widget.workflowId, target: _target);
      _normalize(workflow);
      if (_isDevice) {
        _catalog = <Map<String, dynamic>>[];
      } else {
        try {
          _catalog = await service.workflowCatalog(target: _target);
        } catch (_) {
          _catalog = <Map<String, dynamic>>[];
        }
      }
      try {
        _ownedWorkflows = (await service.workflows(limit: 200, target: _target))
            .where((item) => (item['id'] ?? '').toString() != widget.workflowId)
            .toList(growable: false);
      } catch (_) {
        _ownedWorkflows = <Map<String, dynamic>>[];
      }
      if (_isCloud) {
        try {
          _devices = await service.workflowDevices();
        } catch (_) {
          _devices = <Map<String, dynamic>>[];
        }
        _triggerCapabilities = <Map<String, dynamic>>[];
        _triggerAppCatalog = <Map<String, dynamic>>[];
        _triggerWakeConfigs = <Map<String, dynamic>>[];
      } else {
        try {
          _triggerCapabilities = await service.workflowTriggerCapabilities(target: _target);
        } catch (_) {
          _triggerCapabilities = <Map<String, dynamic>>[];
        }
        try {
          _triggerAppCatalog = await service.workflowTriggerAppCatalog(target: _target);
        } catch (_) {
          _triggerAppCatalog = <Map<String, dynamic>>[];
        }
        try {
          _triggerWakeConfigs = await service.workflowTriggerWakeConfigs(target: _target);
        } catch (_) {
          _triggerWakeConfigs = <Map<String, dynamic>>[];
        }
      }
      if (_isDevice) {
        _runHistory = <Map<String, dynamic>>[];
        _revisions = <Map<String, dynamic>>[];
        _workflowStats = <String, dynamic>{};
      } else {
        await Future.wait(<Future<void>>[_loadRuns(), _loadRevisions(), _loadStats()]);
      }
      if (!mounted) return;
      setState(() => _loading = false);
      WidgetsBinding.instance.addPostFrameCallback((_) => _fitView());
    } catch (error) {
      if (!mounted) return;
      setState(() => _loading = false);
      _show('加载失败：${_message(error)}');
    }
  }

  void _normalize(Map<String, dynamic> raw) {
    _workflow = Map<String, dynamic>.from(raw);
    _workflow!['schemaVersion'] = (_workflow!['schemaVersion'] ?? 'workflow-v2').toString();
    _workflow!['name'] = (_workflow!['name'] ?? '未命名工作流').toString();
    _workflow!['description'] = (_workflow!['description'] ?? '').toString();
    _workflow!['inputSchema'] = _asMap(_workflow!['inputSchema'], fallback: <String, dynamic>{'type': 'object'});
    _workflow!['outputSchema'] = _asMap(_workflow!['outputSchema'], fallback: <String, dynamic>{'type': 'object'});
    _workflow!['metadata'] = _asMap(_workflow!['metadata']);
    _workflow!['callableByAgent'] = _workflow!['callableByAgent'] == true;
    _workflow!['agentTool'] = _asMap(_workflow!['agentTool']);
    _workflow!['enabled'] = _workflow!['enabled'] != false;
    _nodes = _asMapList(_workflow!['nodes']);
    _edges = _asMapList(_workflow!['edges']);
    _triggers = _asMapList(_workflow!['triggers']);
    for (var i = 0; i < _nodes.length; i++) {
      final node = _nodes[i];
      node['id'] = (node['id'] ?? 'node-${i + 1}').toString();
      node['label'] = (node['label'] ?? node['type'] ?? 'Node').toString();
      node['type'] = (node['type'] ?? 'tool').toString();
      node['position'] = _asMap(
        node['position'],
        fallback: <String, dynamic>{'x': 160.0 + (i % 4) * 280.0, 'y': 160.0 + (i ~/ 4) * 190.0},
      );
      node['runtime'] = _asMap(node['runtime']);
      var executionTarget = _asMap(node['executionTarget']);
      if (_isCloud) {
        var placement = (executionTarget['placement'] ?? 'cloud').toString();
        if (!<String>{'cloud', 'device', 'auto'}.contains(placement)) placement = 'cloud';
        executionTarget = <String, dynamic>{
          'placement': placement,
          'deviceId': (executionTarget['deviceId'] ?? '').toString(),
          'runtimeId': (executionTarget['runtimeId'] ?? '').toString(),
          'providerId': (executionTarget['providerId'] ?? '').toString(),
          'providerInstanceId': (executionTarget['providerInstanceId'] ?? '').toString(),
          'offlinePolicy': <String>{'fail', 'wait'}.contains((executionTarget['offlinePolicy'] ?? '').toString())
              ? executionTarget['offlinePolicy'].toString()
              : 'fail',
        };
      } else {
        executionTarget = <String, dynamic>{'placement': 'local', 'offlinePolicy': 'fail'};
      }
      node['executionTarget'] = executionTarget;
      node['step'] = _asMap(node['step']);
      final step = node['step'] as Map<String, dynamic>;
      step['onError'] = _asMap(step['onError'], fallback: <String, dynamic>{'mode': 'fail'});
      node['permissions'] = ((node['permissions'] as List?) ?? const <dynamic>[]).map((e) => e.toString()).toList();
    }
    for (var i = 0; i < _edges.length; i++) {
      _edges[i]['id'] = (_edges[i]['id'] ?? 'edge-${i + 1}').toString();
      _edges[i]['label'] = (_edges[i]['label'] ?? '').toString();
    }
    if (_triggers.isEmpty) {
      _triggers.add(<String, dynamic>{'id': 'manual', 'type': 'manual', 'config': <String, dynamic>{}, 'enabled': true});
    }
    for (var i = 0; i < _triggers.length; i++) {
      _triggers[i]['id'] = (_triggers[i]['id'] ?? 'trigger-${i + 1}').toString();
      _triggers[i]['type'] = (_triggers[i]['type'] ?? 'manual').toString();
      _triggers[i]['config'] = _asMap(_triggers[i]['config']);
      _triggers[i]['enabled'] = _triggers[i]['enabled'] != false;
    }
    _dirty = false;
  }

  Map<String, dynamic> _asMap(dynamic value, {Map<String, dynamic>? fallback}) {
    if (value is Map<String, dynamic>) return Map<String, dynamic>.from(value);
    if (value is Map) return value.map((k, v) => MapEntry(k.toString(), v));
    return Map<String, dynamic>.from(fallback ?? const <String, dynamic>{});
  }

  List<Map<String, dynamic>> _asMapList(dynamic value) {
    if (value is! List) return <Map<String, dynamic>>[];
    return value.whereType<Map>().map((e) => e.map((k, v) => MapEntry(k.toString(), v))).toList();
  }

  String _message(Object error) => error.toString().replaceFirst('Exception: ', '');

  void _show(String message) {
    if (!mounted) return;
    ScaffoldMessenger.of(context)
      ..clearSnackBars()
      ..showSnackBar(SnackBar(content: Text(message)));
  }

  void _markDirty() {
    if (!_dirty && mounted) setState(() => _dirty = true);
  }

  int _revisionOf(Map<String, dynamic>? definition) {
    final installation = definition?['installation'];
    if (installation is! Map) return 0;
    final raw = installation['revision'];
    return raw is int ? raw : int.tryParse(raw?.toString() ?? '') ?? 0;
  }

  bool _syncEventMatches(Map<String, dynamic> event) {
    if (!(event['type'] ?? '').toString().startsWith('workflow.installation.')) return false;
    final payload = _asMap(event['payload']);
    if ((payload['workflowId'] ?? '').toString() != widget.workflowId) return false;
    if (_isDevice) return (payload['hostDeviceId'] ?? '').toString() == widget.deviceId;
    if (_isCloud) return (payload['location'] ?? '').toString() == 'cloud';
    return true;
  }

  Future<void> _primeSyncCursor() async {
    try {
      final page = await ref.read(extensionServiceProvider).workflowSyncEvents(target: _target);
      if (_disposed) return;
      final raw = page['cursor'];
      _syncCursor = raw is int ? raw : int.tryParse(raw?.toString() ?? '') ?? 0;
    } catch (_) {
      _syncCursor = null;
    }
  }

  Future<void> _refreshDefinitionFromSync() async {
    try {
      final latest = await ref.read(extensionServiceProvider).getWorkflow(widget.workflowId, target: _target);
      if (_disposed || !mounted) return;
      final latestRevision = _revisionOf(latest);
      final localRevision = _revisionOf(_workflow);
      if (_dirty) {
        if (latestRevision > localRevision && latestRevision != _conflictNoticeRevision) {
          _conflictNoticeRevision = latestRevision;
          _show('该工作流已在其他客户端更新到 revision $latestRevision。当前草稿不会被覆盖，保存时会执行冲突校验。');
        }
        return;
      }
      if (latestRevision > 0 && latestRevision == localRevision) return;
      setState(() {
        _normalize(latest);
        _conflictNoticeRevision = 0;
      });
      if (!_isDevice) {
        await Future.wait(<Future<void>>[_loadRuns(), _loadRevisions(), _loadStats()]);
      }
    } catch (_) {
      // A transient sync failure must not make the editor unusable.
    }
  }

  Future<void> _pollSyncEvents() async {
    if (_disposed || _syncBusy) return;
    _syncBusy = true;
    try {
      if (_syncCursor == null) {
        await _primeSyncCursor();
        return;
      }
      final page = await ref.read(extensionServiceProvider).workflowSyncEvents(target: _target, afterCursor: _syncCursor);
      if (_disposed) return;
      final rawCursor = page['cursor'];
      _syncCursor = rawCursor is int ? rawCursor : int.tryParse(rawCursor?.toString() ?? '') ?? _syncCursor;
      final events = ((page['items'] as List?) ?? const <dynamic>[])
          .whereType<Map>()
          .map((item) => item.map((key, value) => MapEntry(key.toString(), value)))
          .toList(growable: false);
      if (events.any(_syncEventMatches)) await _refreshDefinitionFromSync();
      _deviceSyncTicks++;
      if (_isDevice && _deviceSyncTicks % 8 == 0) await _refreshDefinitionFromSync();
    } catch (_) {
      // Keep the durable cursor and retry on the next tick.
    } finally {
      _syncBusy = false;
    }
  }

  Map<String, dynamic> _definition() {
    final out = Map<String, dynamic>.from(_workflow ?? <String, dynamic>{});
    out['nodes'] = _nodes;
    out['edges'] = _edges;
    out['triggers'] = _triggers;
    return out;
  }

  bool _hasEnabledDeviceTrigger() {
    final workflowEnabled = _workflow?['enabled'] != false;
    if (!workflowEnabled) return false;
    const deviceEvents = <String>{
      'device.android.intent',
      'device.android.tasker',
      'voice.wake.detected',
      'voice.asr.final',
      'device.app.foreground',
    };
    return _triggers.any((trigger) =>
        trigger['enabled'] != false &&
        trigger['type'] == 'event' &&
        deviceEvents.contains((trigger['eventType'] ?? '').toString()));
  }

  bool _hasHighImpactWorkflowNodes() {
    if (_workflow?['hasSideEffects'] == true) return true;
    const highImpactTypes = <String>{'tool', 'task', 'mcp', 'javascript', 'wasm', 'trusted_service'};
    return _nodes.any((node) => highImpactTypes.contains((node['type'] ?? '').toString().toLowerCase()));
  }

  Future<bool> _confirmDeviceTriggerRisk() async {
    if (!_hasEnabledDeviceTrigger() || !_hasHighImpactWorkflowNodes()) return true;
    if (!mounted) return false;
    return await showDialog<bool>(
          context: context,
          barrierDismissible: false,
          builder: (context) => AlertDialog(
            title: const Text('确认设备自动执行'),
            content: const Text('此工作流包含可产生副作用或调用外部能力的节点，并启用了设备自动触发器。触发条件满足时可能在无人交互情况下执行。请确认触发来源、权限、输入过滤与幂等策略均符合预期。'),
            actions: [
              TextButton(onPressed: () => Navigator.pop(context, false), child: const Text('取消')),
              FilledButton(onPressed: () => Navigator.pop(context, true), child: const Text('确认并保存')),
            ],
          ),
        ) ??
        false;
  }

  Future<bool> _save({bool notify = true}) async {
    if (_workflow == null || _saving) return false;
    if (!await _confirmDeviceTriggerRisk()) return false;
    setState(() => _saving = true);
    try {
      if (!_isDevice) {
        final validation = await ref.read(extensionServiceProvider).validateWorkflow(_definition(), target: _target);
        if (validation['valid'] != true) {
          throw StateError((validation['error'] ?? '工作流校验失败').toString());
        }
      }
      final saved = await ref.read(extensionServiceProvider).updateWorkflow(widget.workflowId, _definition(), target: _target);
      _normalize(saved);
      if (!mounted) return true;
      setState(() {
        _saving = false;
        _dirty = false;
      });
      if (!_isDevice) await _loadRevisions();
      if (notify) _show(_isDevice ? '已保存到目标设备' : '已保存并通过 DAG 校验');
      return true;
    } catch (error) {
      if (!mounted) return false;
      setState(() => _saving = false);
      _show('保存失败：${_message(error)}');
      return false;
    }
  }

  Future<void> _validate() async {
    if (_isDevice) {
      _show('远程设备工作流由设备本地 Kernel 在保存/运行时校验');
      return;
    }
    try {
      final result = await ref.read(extensionServiceProvider).validateWorkflow(_definition(), target: _target);
      if (result['valid'] == true) {
        final order = ((result['topologicalOrder'] as List?) ?? const <dynamic>[]).join(' → ');
        _show(order.isEmpty ? 'DAG 校验通过' : 'DAG 校验通过：$order');
      } else {
        _show('校验失败：${result['error'] ?? 'unknown error'}');
      }
    } catch (error) {
      _show('校验失败：${_message(error)}');
    }
  }

  Future<void> _run() async {
    if (_dirty && !await _save(notify: false)) return;
    try {
      final result = await ref.read(extensionServiceProvider).runWorkflow(widget.workflowId, target: _target);
      final id = (result['executionId'] ?? '').toString();
      if (id.isEmpty) throw StateError('后端没有返回 executionId');
      setState(() {
        _activeRunId = id;
        _activeRunStatus = (result['status'] ?? 'running').toString();
        _stepRuns = <String, Map<String, dynamic>>{};
        _stepAttempts = <Map<String, dynamic>>[];
        _checkpoints = <Map<String, dynamic>>[];
      });
      if (!_isDevice) _startPolling();
      _show(_isDevice ? '已提交到目标设备运行' : '工作流已开始运行');
    } catch (error) {
      _show('运行失败：${_message(error)}');
    }
  }

  void _startPolling() {
    _pollTimer?.cancel();
    _pollTimer = Timer.periodic(const Duration(milliseconds: 900), (_) => _pollRun());
    _pollRun();
  }

  Future<void> _pollRun() async {
    final id = _activeRunId;
    if (id == null || id.isEmpty || _disposed) return;
    try {
      final detail = await ref.read(extensionServiceProvider).getWorkflowRun(id, target: _target);
      final run = _asMap(detail['run']);
      final steps = _asMapList(detail['stepRuns']);
      final attempts = _asMapList(detail['attempts']);
      final checkpoints = _asMapList(detail['checkpoints']);
      final status = (run['status'] ?? '').toString();
      if (!mounted) return;
      setState(() {
        _activeRunStatus = status;
        _stepRuns = <String, Map<String, dynamic>>{
          for (final step in steps) (step['nodeId'] ?? '').toString(): step,
        };
        _stepAttempts = attempts;
        _checkpoints = checkpoints;
      });
      if (_terminal(status)) {
        _pollTimer?.cancel();
        await Future.wait(<Future<void>>[_loadRuns(), _loadStats()]);
      }
    } catch (_) {
      // Keep the editor usable if one polling request is interrupted.
    }
  }

  bool _terminal(String status) => <String>{'succeeded', 'failed', 'cancelled', 'completed', 'compensated'}.contains(status.toLowerCase());

  Future<void> _loadRuns() async {
    if (_workflow == null || _isDevice) return;
    try {
      final result = await ref.read(extensionServiceProvider).workflowRuns(widget.workflowId, limit: 30, target: _target);
      final items = _asMapList(result['items']);
      if (mounted) setState(() => _runHistory = items);
    } catch (_) {}
  }

  Future<void> _loadStats() async {
    if (_isDevice) return;
    try {
      final stats = await ref.read(extensionServiceProvider).workflowStats(widget.workflowId, target: _target);
      if (mounted) setState(() => _workflowStats = stats);
    } catch (_) {}
  }

  Future<void> _pauseRun() async {
    final id = _activeRunId;
    if (id == null) return;
    await ref.read(extensionServiceProvider).pauseWorkflowRun(id, target: _target);
    await _pollRun();
  }

  Future<void> _resumeRun() async {
    final id = _activeRunId;
    if (id == null) return;
    await ref.read(extensionServiceProvider).resumeWorkflowRun(id, target: _target);
    await _pollRun();
  }

  Future<void> _rerunActiveRun() async {
    final id = _activeRunId;
    if (id == null || !_terminal(_activeRunStatus) || _workflow?['enabled'] == false) return;
    try {
      final result = await ref.read(extensionServiceProvider).rerunWorkflowRun(id, target: _target);
      final newId = (result['executionId'] ?? '').toString();
      if (newId.isEmpty) throw StateError('后端没有返回 executionId');
      if (!mounted) return;
      setState(() {
        _activeRunId = newId;
        _activeRunStatus = (result['status'] ?? 'running').toString();
        _stepRuns = <String, Map<String, dynamic>>{};
        _stepAttempts = <Map<String, dynamic>>[];
        _checkpoints = <Map<String, dynamic>>[];
      });
      _startPolling();
      _show('已使用原运行输入重新执行当前已保存工作流');
    } catch (error) {
      _show('重新运行失败：${_message(error)}');
    }
  }

  Future<void> _recoverActiveRun() async {
    final id = _activeRunId;
    if (id == null || !<String>{'failed', 'cancelled'}.contains(_activeRunStatus.toLowerCase()) || _checkpoints.isEmpty || _workflow?['enabled'] == false) return;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('从 Checkpoint 恢复'),
        content: Text('将复用当前运行的 ${_checkpoints.length} 个 Checkpoint，并按当前已保存工作流继续执行。不会绕过 DAG 单独执行某个节点。'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context, false), child: const Text('取消')),
          FilledButton(onPressed: () => Navigator.pop(context, true), child: const Text('恢复')),
        ],
      ),
    );
    if (confirmed != true) return;
    try {
      final result = await ref.read(extensionServiceProvider).recoverWorkflowRun(id, target: _target);
      if (!mounted) return;
      setState(() => _activeRunStatus = (result['status'] ?? 'running').toString());
      _startPolling();
      _show('已从 ${result['checkpointCount'] ?? _checkpoints.length} 个 Checkpoint 恢复执行');
    } catch (error) {
      _show('Checkpoint 恢复失败：${_message(error)}');
    }
  }

  Future<void> _cancelRun() async {
    final id = _activeRunId;
    if (id == null) return;
    await ref.read(extensionServiceProvider).cancelWorkflowRun(id, target: _target);
    await _pollRun();
  }

  void _fitView() {
    if (_nodes.isEmpty) {
      _transform.value = Matrix4.identity()..translate(-40.0, -40.0);
      return;
    }
    var minX = double.infinity;
    var minY = double.infinity;
    var maxX = -double.infinity;
    var maxY = -double.infinity;
    for (final node in _nodes) {
      final p = _asMap(node['position']);
      final x = _number(p['x']);
      final y = _number(p['y']);
      minX = math.min(minX, x);
      minY = math.min(minY, y);
      maxX = math.max(maxX, x + _nodeWidth);
      maxY = math.max(maxY, y + _nodeHeight);
    }
    final box = context.findRenderObject() as RenderBox?;
    final viewport = box?.size ?? const Size(800, 600);
    final contentW = math.max(1.0, maxX - minX + 100);
    final contentH = math.max(1.0, maxY - minY + 160);
    final scale = math.min(1.15, math.max(0.35, math.min(viewport.width / contentW, viewport.height / contentH)));
    final tx = 36 - minX * scale;
    final ty = 48 - minY * scale;
    _transform.value = Matrix4.identity()..translate(tx, ty)..scale(scale);
  }

  double _number(dynamic value, [double fallback = 0]) => value is num ? value.toDouble() : double.tryParse('$value') ?? fallback;

  void _autoLayout() {
    final incoming = <String, int>{for (final node in _nodes) (node['id'] ?? '').toString(): 0};
    final outgoing = <String, List<String>>{for (final id in incoming.keys) id: <String>[]};
    for (final edge in _edges) {
      final source = (edge['source'] ?? '').toString();
      final target = (edge['target'] ?? '').toString();
      if (incoming.containsKey(target) && outgoing.containsKey(source)) {
        incoming[target] = (incoming[target] ?? 0) + 1;
        outgoing[source]!.add(target);
      }
    }
    final queue = incoming.entries.where((e) => e.value == 0).map((e) => e.key).toList();
    final level = <String, int>{for (final id in queue) id: 0};
    while (queue.isNotEmpty) {
      final id = queue.removeAt(0);
      for (final next in outgoing[id] ?? const <String>[]) {
        level[next] = math.max(level[next] ?? 0, (level[id] ?? 0) + 1);
        incoming[next] = (incoming[next] ?? 1) - 1;
        if (incoming[next] == 0) queue.add(next);
      }
    }
    final rows = <int, int>{};
    setState(() {
      for (final node in _nodes) {
        final id = (node['id'] ?? '').toString();
        final l = level[id] ?? 0;
        final row = rows[l] ?? 0;
        rows[l] = row + 1;
        node['position'] = <String, dynamic>{'x': 150.0 + l * 310.0, 'y': 130.0 + row * 180.0};
      }
      _dirty = true;
    });
    WidgetsBinding.instance.addPostFrameCallback((_) => _fitView());
  }

  Future<void> _addNode() async {
    final type = await showModalBottomSheet<String>(
      context: context,
      showDragHandle: true,
      builder: (context) => SafeArea(
        child: ListView(
          shrinkWrap: true,
          children: _nodeTypes
              .map(
                (entry) => ListTile(
                  leading: Icon(entry.icon),
                  title: Text(entry.label),
                  subtitle: Text(entry.description),
                  onTap: () => Navigator.pop(context, entry.type),
                ),
              )
              .toList(),
        ),
      ),
    );
    if (type == null) return;
    final index = _nodes.length + 1;
    final id = '${type}_${DateTime.now().microsecondsSinceEpoch}';
    final nodeType = _nodeTypes.firstWhere((e) => e.type == type);
    final node = <String, dynamic>{
      'id': id,
      'type': type,
      'label': '${nodeType.label} $index',
      'targetId': '',
      'dependsOn': <dynamic>[],
      'runtime': <String, dynamic>{'runtimeType': _defaultRuntimeType(type), 'runtimeId': '', 'handlerName': '', 'metadata': <String, dynamic>{}},
      'executionTarget': _isCloud
          ? <String, dynamic>{'placement': 'cloud', 'deviceId': '', 'offlinePolicy': 'fail'}
          : <String, dynamic>{'placement': 'local', 'offlinePolicy': 'fail'},
      'permissions': <dynamic>[],
      'position': <String, dynamic>{'x': 220.0 + (index % 5) * 80, 'y': 180.0 + (index % 6) * 80},
      'step': <String, dynamic>{
        'input': <String, dynamic>{},
        'onError': <String, dynamic>{'mode': 'fail'},
      },
    };
    if (type == 'wait') {
      (node['runtime'] as Map<String, dynamic>)['metadata'] = <String, dynamic>{'durationMs': 1000};
    }
    setState(() {
      _nodes.add(node);
      _dirty = true;
    });
    await _editNode(node);
  }

  bool _hasPath(String from, String to) {
    if (from == to) return true;
    final seen = <String>{};
    final stack = <String>[from];
    while (stack.isNotEmpty) {
      final current = stack.removeLast();
      if (!seen.add(current)) continue;
      for (final edge in _edges.where((e) => (e['source'] ?? '').toString() == current)) {
        final next = (edge['target'] ?? '').toString();
        if (next == to) return true;
        stack.add(next);
      }
    }
    return false;
  }

  void _connectTo(String target) {
    final source = _connectFrom;
    if (source == null || source == target) {
      setState(() => _connectFrom = null);
      return;
    }
    if (_edges.any((e) => e['source'] == source && e['target'] == target)) {
      _show('这两个节点已经连接');
      setState(() => _connectFrom = null);
      return;
    }
    if (_hasPath(target, source)) {
      _show('不能创建环：Workflow 必须保持 DAG');
      setState(() => _connectFrom = null);
      return;
    }
    setState(() {
      _edges.add(<String, dynamic>{
        'id': 'edge-${DateTime.now().microsecondsSinceEpoch}',
        'source': source,
        'target': target,
        'sourceHandle': 'output',
        'targetHandle': 'input',
        'label': '',
      });
      _connectFrom = null;
      _dirty = true;
    });
  }

  List<String> _schemaLeafPaths(dynamic schema, [String prefix = '']) {
    final map = _asMap(schema);
    final properties = map['properties'];
    if (properties is! Map) return prefix.isEmpty ? const <String>[] : <String>[prefix];
    final result = <String>[];
    for (final entry in properties.entries) {
      final path = prefix.isEmpty ? entry.key.toString() : '$prefix.${entry.key}';
      final nested = _schemaLeafPaths(entry.value, path);
      result.addAll(nested.isEmpty ? <String>[path] : nested);
    }
    return result;
  }

  Map<String, dynamic>? _catalogForNode(Map<String, dynamic> node) {
    final targetId = (node['targetId'] ?? '').toString();
    final runtimeId = (_asMap(node['runtime'])['runtimeId'] ?? '').toString();
    for (final item in _catalog) {
      final itemRuntime = _asMap(item['runtime']);
      if ((item['id'] ?? '').toString() == targetId ||
          (item['modelName'] ?? '').toString() == targetId ||
          (runtimeId.isNotEmpty && (itemRuntime['runtimeId'] ?? '').toString() == runtimeId)) {
        return item;
      }
    }
    return null;
  }

  List<Map<String, String>> _mappingSources(Map<String, dynamic> targetNode) {
    final sources = <Map<String, String>>[];
    for (final path in _schemaLeafPaths(_workflow?['inputSchema'])) {
      sources.add(<String, String>{'label': '工作流输入 · $path', 'ref': 'input.$path'});
    }
    sources.addAll(const <Map<String, String>>[
      <String, String>{'label': 'Runtime · userId', 'ref': 'runtime.userId'},
      <String, String>{'label': 'Runtime · conversationId', 'ref': 'runtime.conversationId'},
      <String, String>{'label': 'Runtime · characterId', 'ref': 'runtime.characterId'},
      <String, String>{'label': 'Runtime · traceId', 'ref': 'runtime.traceId'},
    ]);
    final targetId = (targetNode['id'] ?? '').toString();
    for (final node in _nodes) {
      final nodeId = (node['id'] ?? '').toString();
      if (nodeId.isEmpty || nodeId == targetId || _hasPath(targetId, nodeId)) continue;
      var paths = _schemaLeafPaths(_catalogForNode(node)?['outputSchema']);
      if (paths.isEmpty) {
        paths = _valueLeafPaths(_stepRuns[nodeId]?['output']);
      }
      for (final path in paths) {
        sources.add(<String, String>{
          'label': '${node['label'] ?? nodeId} · $path',
          'ref': 'steps.$nodeId.$path',
        });
      }
    }
    return sources;
  }

  List<String> _valueLeafPaths(dynamic value, [String prefix = '']) {
    if (value is Map) {
      final result = <String>[];
      for (final entry in value.entries) {
        final path = prefix.isEmpty ? entry.key.toString() : '$prefix.${entry.key}';
        final nested = _valueLeafPaths(entry.value, path);
        result.addAll(nested.isEmpty ? <String>[path] : nested);
      }
      return result;
    }
    return prefix.isEmpty ? const <String>[] : <String>[prefix];
  }

  void _setMapPath(Map<String, dynamic> root, String path, dynamic value) {
    final parts = path.split('.').map((e) => e.trim()).where((e) => e.isNotEmpty).toList();
    if (parts.isEmpty) return;
    var current = root;
    for (var i = 0; i < parts.length - 1; i++) {
      final key = parts[i];
      final child = current[key];
      if (child is Map<String, dynamic>) {
        current = child;
      } else if (child is Map) {
        final mapped = child.map((k, v) => MapEntry(k.toString(), v));
        current[key] = mapped;
        current = mapped;
      } else {
        final mapped = <String, dynamic>{};
        current[key] = mapped;
        current = mapped;
      }
    }
    current[parts.last] = value;
  }

  Future<void> _showDataMappingDialog(
    BuildContext dialogParentContext,
    Map<String, dynamic> node,
    TextEditingController inputController,
  ) async {
    final targetFields = _schemaLeafPaths(_catalogForNode(node)?['inputSchema']);
    final sources = _mappingSources(node);
    final targetController = TextEditingController(text: targetFields.isNotEmpty ? targetFields.first : '');
    String? sourceRef = sources.isNotEmpty ? sources.first['ref'] : null;
    final applied = await showDialog<bool>(
      context: dialogParentContext,
      builder: (context) => StatefulBuilder(
        builder: (context, setDialogState) => AlertDialog(
          title: const Text('可视化数据映射'),
          content: SizedBox(
            width: 520,
            child: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  TextField(controller: targetController, decoration: const InputDecoration(labelText: '目标输入字段路径')),
                  if (targetFields.isNotEmpty) ...[
                    const SizedBox(height: 8),
                    Wrap(
                      spacing: 6,
                      runSpacing: 6,
                      children: targetFields.take(16).map((path) => ActionChip(label: Text(path), onPressed: () => targetController.text = path)).toList(),
                    ),
                  ],
                  const SizedBox(height: 14),
                  if (sources.isEmpty)
                    const Text('当前没有可映射的数据源。请先配置 Workflow Input Schema、节点输出 Schema，或运行一次上游节点。')
                  else
                    DropdownButtonFormField<String>(
                      initialValue: sourceRef,
                      isExpanded: true,
                      decoration: const InputDecoration(labelText: '数据来源'),
                      items: sources.map((item) => DropdownMenuItem(value: item['ref'], child: Text(item['label'] ?? item['ref'] ?? '', overflow: TextOverflow.ellipsis))).toList(),
                      onChanged: (value) => setDialogState(() => sourceRef = value),
                    ),
                  const SizedBox(height: 8),
                  const Text('跨节点绑定会自动添加依赖边；如果会形成循环则拒绝绑定。'),
                ],
              ),
            ),
          ),
          actions: [
            TextButton(onPressed: () => Navigator.pop(context, false), child: const Text('取消')),
            FilledButton(onPressed: sources.isEmpty ? null : () => Navigator.pop(context, true), child: const Text('绑定')),
          ],
        ),
      ),
    );
    if (applied == true) {
      try {
        final targetPath = targetController.text.trim();
        final refValue = (sourceRef ?? '').trim();
        if (targetPath.isEmpty || refValue.isEmpty) throw const FormatException('字段和数据来源不能为空');
        final decoded = inputController.text.trim().isEmpty ? <String, dynamic>{} : jsonDecode(inputController.text);
        if (decoded is! Map) throw const FormatException('Input JSON 必须是 object 才能使用可视化映射');
        final mappedInput = decoded.map((k, v) => MapEntry(k.toString(), v));
        _setMapPath(mappedInput, targetPath, refValue);
        final match = RegExp(r'^steps\.([^.]+)\.').firstMatch(refValue);
        final sourceId = match?.group(1);
        final targetId = (node['id'] ?? '').toString();
        if (sourceId != null && sourceId.isNotEmpty && !_hasPath(sourceId, targetId)) {
          if (_hasPath(targetId, sourceId)) throw const FormatException('该数据映射会形成 DAG 循环');
          setState(() {
            _edges.add(<String, dynamic>{
              'id': 'edge-map-${DateTime.now().microsecondsSinceEpoch}',
              'source': sourceId,
              'target': targetId,
              'sourceHandle': 'output',
              'targetHandle': 'input',
              'label': 'data',
            });
            _dirty = true;
          });
        }
        inputController.text = _pretty(mappedInput);
        setState(() {
          final step = _asMap(node['step']);
          step['input'] = mappedInput;
          node['step'] = step;
          _dirty = true;
        });
        _show('数据映射已绑定');
      } catch (error) {
        _show('数据映射失败：${_message(error)}');
      }
    }
    targetController.dispose();
  }

  Future<String?> _promptAI(String title, String hint, {bool allowEmpty = false}) async {
    final controller = TextEditingController();
    final value = await showDialog<String>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: Text(title),
        content: TextField(
          controller: controller,
          autofocus: true,
          minLines: 4,
          maxLines: 8,
          decoration: InputDecoration(hintText: hint),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(dialogContext), child: const Text('取消')),
          FilledButton(
            onPressed: () {
              final text = controller.text.trim();
              if (allowEmpty || text.isNotEmpty) Navigator.pop(dialogContext, text);
            },
            child: const Text('继续'),
          ),
        ],
      ),
    );
    controller.dispose();
    return value;
  }

  Future<bool> _ensureSavedForAI() async {
    if (!_dirty) return true;
    return _save(notify: false);
  }

  void _applyAIProposal(Map<String, dynamic> proposal) {
    final raw = proposal['definition'];
    if (raw is! Map) throw StateError('AI 没有返回 workflow-v2 definition');
    final definition = raw.map((k, v) => MapEntry(k.toString(), v));
    definition['id'] = widget.workflowId;
    setState(() {
      _normalize(definition);
      _dirty = true;
    });
    _autoLayout();
  }

  Future<void> _showAIProposalResult(Map<String, dynamic> result, String title) async {
    if (!mounted) return;
    final summary = (result['summary'] ?? '').toString();
    final changes = ((result['changes'] as List?) ?? const <dynamic>[]).map((e) => e.toString()).toList();
    final warnings = ((result['warnings'] as List?) ?? const <dynamic>[]).map((e) => e.toString()).toList();
    await showModalBottomSheet<void>(
      context: context,
      showDragHandle: true,
      builder: (context) => SafeArea(
        child: Padding(
          padding: const EdgeInsets.fromLTRB(20, 0, 20, 24),
          child: SingleChildScrollView(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(title, style: Theme.of(context).textTheme.titleMedium),
                if (summary.isNotEmpty) ...[const SizedBox(height: 8), Text(summary)],
                for (final item in changes) Padding(padding: const EdgeInsets.only(top: 8), child: Text('• $item')),
                for (final item in warnings) Padding(padding: const EdgeInsets.only(top: 8), child: Text('⚠ $item')),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Future<void> _aiEdit() async {
    if (_isDevice) { _show('远程设备工作流暂不提供云端 AI 编辑入口'); return; }
    if (_aiWorking || _workflow == null) return;
    final instruction = await _promptAI('AI 修改工作流', '例如：在天气节点后增加条件，只有下雨才通知；失败重试三次。');
    if (instruction == null || instruction.isEmpty || !await _ensureSavedForAI()) return;
    setState(() => _aiWorking = true);
    try {
      final result = await ref.read(extensionServiceProvider).editWorkflowWithAI(widget.workflowId, instruction, target: _target);
      _applyAIProposal(result);
      await _showAIProposalResult(result, 'AI 修改已应用到草稿');
    } catch (error) {
      _show('AI 修改失败：${_message(error)}');
    } finally {
      if (mounted) setState(() => _aiWorking = false);
    }
  }

  Future<void> _aiRepair() async {
    if (_isDevice) { _show('远程设备工作流暂不提供云端 AI 修复入口'); return; }
    if (_aiWorking || _workflow == null) return;
    final instruction = await _promptAI('AI 修复工作流', '可选：描述当前问题；留空则自动检查并修复 DAG。', allowEmpty: true);
    if (instruction == null || !await _ensureSavedForAI()) return;
    setState(() => _aiWorking = true);
    try {
      final result = await ref.read(extensionServiceProvider).repairWorkflowWithAI(widget.workflowId, instruction: instruction, target: _target);
      _applyAIProposal(result);
      await _showAIProposalResult(result, 'AI 修复已应用到草稿');
    } catch (error) {
      _show('AI 修复失败：${_message(error)}');
    } finally {
      if (mounted) setState(() => _aiWorking = false);
    }
  }

  Future<void> _aiExplain() async {
    if (_isDevice) { _show('远程设备工作流暂不提供云端 AI 解释入口'); return; }
    if (_aiWorking || _workflow == null) return;
    final instruction = await _promptAI('AI 解释工作流', '可选：例如“重点解释失败路径和权限风险”。', allowEmpty: true);
    if (instruction == null || !await _ensureSavedForAI()) return;
    setState(() => _aiWorking = true);
    try {
      final result = await ref.read(extensionServiceProvider).explainWorkflowWithAI(widget.workflowId, instruction: instruction, target: _target);
      if (!mounted) return;
      final flow = ((result['flow'] as List?) ?? const <dynamic>[]).map((e) => e.toString()).toList();
      final issues = ((result['issues'] as List?) ?? const <dynamic>[]).map((e) => e.toString()).toList();
      final suggestions = ((result['suggestions'] as List?) ?? const <dynamic>[]).map((e) => e.toString()).toList();
      await showModalBottomSheet<void>(
        context: context,
        isScrollControlled: true,
        showDragHandle: true,
        builder: (context) => SafeArea(
          child: Padding(
            padding: const EdgeInsets.fromLTRB(20, 0, 20, 24),
            child: SingleChildScrollView(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('AI 工作流解释', style: Theme.of(context).textTheme.titleMedium),
                  const SizedBox(height: 10),
                  Text((result['summary'] ?? '').toString()),
                  if (flow.isNotEmpty) ...[const SizedBox(height: 16), const Text('流程'), ...flow.map((e) => Padding(padding: const EdgeInsets.only(top: 6), child: Text('• $e')))],
                  if (issues.isNotEmpty) ...[const SizedBox(height: 16), const Text('问题'), ...issues.map((e) => Padding(padding: const EdgeInsets.only(top: 6), child: Text('• $e')))],
                  if (suggestions.isNotEmpty) ...[const SizedBox(height: 16), const Text('建议'), ...suggestions.map((e) => Padding(padding: const EdgeInsets.only(top: 6), child: Text('• $e')))],
                ],
              ),
            ),
          ),
        ),
      );
    } catch (error) {
      _show('AI 解释失败：${_message(error)}');
    } finally {
      if (mounted) setState(() => _aiWorking = false);
    }
  }

  Future<void> _loadRevisions() async {
    if (_isDevice) {
      if (mounted) setState(() => _revisions = <Map<String, dynamic>>[]);
      return;
    }
    try {
      final items = await ref.read(extensionServiceProvider).workflowRevisions(widget.workflowId, limit: 50, target: _target);
      if (mounted) setState(() => _revisions = items);
    } catch (_) {
      if (mounted) setState(() => _revisions = <Map<String, dynamic>>[]);
    }
  }

  Future<void> _createRevisionSnapshot() async {
    if (_isDevice) { _show('远程设备工作流版本历史请在目标设备查看'); return; }
    if (_revisionBusy) return;
    if (_dirty && !await _save(notify: false)) return;
    final controller = TextEditingController();
    final note = await showDialog<String>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('保存版本快照'),
        content: TextField(controller: controller, autofocus: true, decoration: const InputDecoration(labelText: '备注（可选）', hintText: '例如：调整天气分支前')),
        actions: [
          TextButton(onPressed: () => Navigator.pop(dialogContext), child: const Text('取消')),
          FilledButton(onPressed: () => Navigator.pop(dialogContext, controller.text.trim()), child: const Text('保存')),
        ],
      ),
    );
    controller.dispose();
    if (note == null || !mounted) return;
    setState(() => _revisionBusy = true);
    try {
      await ref.read(extensionServiceProvider).createWorkflowRevision(widget.workflowId, note: note, target: _target);
      await _loadRevisions();
      _show('版本快照已保存');
    } catch (error) {
      _show('保存版本失败：${_message(error)}');
    } finally {
      if (mounted) setState(() => _revisionBusy = false);
    }
  }

  Future<void> _rollbackRevision(Map<String, dynamic> item) async {
    if (_isDevice) return;
    if (_revisionBusy) return;
    final revisionNo = (item['revisionNo'] ?? '').toString();
    final revisionId = (item['revisionId'] ?? '').toString();
    if (revisionId.isEmpty) return;
    final confirmed = await showDialog<bool>(
          context: context,
          builder: (dialogContext) => AlertDialog(
            title: const Text('版本回滚'),
            content: Text('回滚到版本 #$revisionNo？当前状态会先自动保存，随后恢复当时的 DAG、触发器和设置。'),
            actions: [
              TextButton(onPressed: () => Navigator.pop(dialogContext, false), child: const Text('取消')),
              FilledButton(onPressed: () => Navigator.pop(dialogContext, true), child: const Text('回滚')),
            ],
          ),
        ) ??
        false;
    if (!confirmed) return;
    setState(() => _revisionBusy = true);
    try {
      final restored = await ref.read(extensionServiceProvider).rollbackWorkflowRevision(widget.workflowId, revisionId, target: _target);
      _normalize(restored);
      await Future.wait(<Future<void>>[_loadRevisions(), _loadRuns()]);
      if (mounted) WidgetsBinding.instance.addPostFrameCallback((_) => _fitView());
      _show('已回滚到版本 #$revisionNo');
    } catch (error) {
      _show('版本回滚失败：${_message(error)}');
    } finally {
      if (mounted) setState(() => _revisionBusy = false);
    }
  }

  Future<void> _showRevisions() async {
    if (_isDevice) { _show('远程设备工作流版本历史请在目标设备查看'); return; }
    await _loadRevisions();
    if (!mounted) return;
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      showDragHandle: true,
      builder: (sheetContext) => StatefulBuilder(
        builder: (context, setSheetState) => SafeArea(
          child: SizedBox(
            height: MediaQuery.sizeOf(context).height * 0.72,
            child: Column(
              children: [
                Padding(
                  padding: const EdgeInsets.fromLTRB(20, 0, 12, 8),
                  child: Row(
                    children: [
                      Expanded(child: Text('版本历史', style: Theme.of(context).textTheme.titleMedium)),
                      IconButton(
                        tooltip: '保存快照',
                        onPressed: _revisionBusy
                            ? null
                            : () async {
                                Navigator.pop(sheetContext);
                                await _createRevisionSnapshot();
                              },
                        icon: const Icon(Icons.add_circle_outline),
                      ),
                    ],
                  ),
                ),
                Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 20),
                  child: Text('保存修改前会自动记录旧版本，最多保留最近 50 个版本快照；回滚前也会先保存当前状态。', style: AppTypography.caption(context)),
                ),
                const SizedBox(height: 8),
                Expanded(
                  child: _revisions.isEmpty
                      ? const Center(child: Text('暂无版本快照'))
                      : ListView.separated(
                          padding: const EdgeInsets.all(12),
                          itemCount: _revisions.length,
                          separatorBuilder: (_, __) => const Divider(height: 1),
                          itemBuilder: (context, index) {
                            final item = _revisions[index];
                            final note = (item['note'] ?? '').toString();
                            final hash = (item['definitionHash'] ?? '').toString();
                            final createdAt = (item['createdAt'] ?? '').toString();
                            return ListTile(
                              title: Text('#${item['revisionNo']} · ${note.isEmpty ? '自动快照' : note}'),
                              subtitle: Text('${createdAt.isEmpty ? '' : createdAt}\n${hash.length > 12 ? hash.substring(0, 12) : hash}'),
                              isThreeLine: true,
                              trailing: TextButton(
                                onPressed: _revisionBusy
                                    ? null
                                    : () async {
                                        Navigator.pop(sheetContext);
                                        await _rollbackRevision(item);
                                      },
                                child: const Text('回滚'),
                              ),
                            );
                          },
                        ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Future<void> _editMetadata() async {
    if (_workflow == null) return;
    final name = TextEditingController(text: (_workflow!['name'] ?? '').toString());
    final desc = TextEditingController(text: (_workflow!['description'] ?? '').toString());
    final agentTool = _asMap(_workflow!['agentTool']);
    final agentToolName = TextEditingController(text: (agentTool['name'] ?? '').toString());
    final agentToolDescription = TextEditingController(text: (agentTool['description'] ?? '').toString());
    var callableByAgent = _workflow!['callableByAgent'] == true;
    var enabled = _workflow!['enabled'] != false;
    final result = await showModalBottomSheet<bool>(
      context: context,
      isScrollControlled: true,
      showDragHandle: true,
      builder: (context) => StatefulBuilder(
        builder: (context, setSheetState) => Padding(
          padding: EdgeInsets.fromLTRB(20, 0, 20, MediaQuery.viewInsetsOf(context).bottom + 24),
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
              Text('工作流设置', style: Theme.of(context).textTheme.titleMedium),
              const SizedBox(height: 16),
              TextField(controller: name, decoration: const InputDecoration(labelText: '名称')),
              const SizedBox(height: 12),
              TextField(controller: desc, minLines: 2, maxLines: 4, decoration: const InputDecoration(labelText: '描述')),
              SwitchListTile(
                contentPadding: EdgeInsets.zero,
                title: const Text('启用工作流'),
                value: enabled,
                onChanged: (v) => setSheetState(() => enabled = v),
              ),
              SwitchListTile(
                contentPadding: EdgeInsets.zero,
                title: const Text('允许 AI 调用'),
                subtitle: const Text('保存后按当前用户隔离注册为 Agent Tool'),
                value: callableByAgent,
                onChanged: (v) => setSheetState(() => callableByAgent = v),
              ),
              if (callableByAgent) ...[
                TextField(controller: agentToolName, decoration: const InputDecoration(labelText: 'Agent Tool 名称（可空）')),
                const SizedBox(height: 10),
                TextField(controller: agentToolDescription, minLines: 2, maxLines: 4, decoration: const InputDecoration(labelText: 'Agent Tool 描述')),
                const SizedBox(height: 10),
              ],
              const SizedBox(height: 10),
              SizedBox(width: double.infinity, child: FilledButton(onPressed: () => Navigator.pop(context, true), child: const Text('应用'))),
              ],
            ),
          ),
        ),
      ),
    );
    if (result == true && mounted) {
      setState(() {
        _workflow!['name'] = name.text.trim().isEmpty ? '未命名工作流' : name.text.trim();
        _workflow!['description'] = desc.text.trim();
        _workflow!['callableByAgent'] = callableByAgent;
        _workflow!['agentTool'] = <String, dynamic>{
          'name': agentToolName.text.trim(),
          'description': agentToolDescription.text.trim(),
        };
        _workflow!['enabled'] = enabled;
        _dirty = true;
      });
    }
    name.dispose();
    desc.dispose();
    agentToolName.dispose();
    agentToolDescription.dispose();
  }

  Future<void> _editNode(Map<String, dynamic> node) async {
    final label = TextEditingController(text: (node['label'] ?? '').toString());
    final target = TextEditingController(text: (node['targetId'] ?? '').toString());
    final runtime = _asMap(node['runtime']);
    final runtimeType = TextEditingController(text: (runtime['runtimeType'] ?? '').toString());
    final runtimeId = TextEditingController(text: (runtime['runtimeId'] ?? '').toString());
    final handlerName = TextEditingController(text: (runtime['handlerName'] ?? '').toString());
    final runtimeMetadata = TextEditingController(text: _pretty(_asMap(runtime['metadata'])));
    final step = _asMap(node['step']);
    final input = TextEditingController(text: _pretty(step['input'] ?? <String, dynamic>{}));
    final when = TextEditingController(text: step['when'] == null ? '' : _pretty(step['when']));
    final permissions = TextEditingController(text: ((node['permissions'] as List?) ?? const <dynamic>[]).join(', '));
    final onErrorConfig = _asMap(step['onError']);
    var onError = (onErrorConfig['mode'] ?? 'fail').toString();
    final onErrorDefault = TextEditingController(text: onErrorConfig['default'] == null ? '' : _pretty(onErrorConfig['default']));
    final retryConfig = _asMap(node['retry']);
    var retryEnabled = node['retry'] is Map;
    final timeoutMs = TextEditingController(text: _number(node['timeoutMs']).round().toString());
    final retryMaxAttempts = TextEditingController(text: _number(retryConfig['maxAttempts'], 3).round().toString());
    final retryInitialBackoffMs = TextEditingController(text: _number(retryConfig['initialBackoffMs'], 200).round().toString());
    final retryMaxBackoffMs = TextEditingController(text: _number(retryConfig['maxBackoffMs'], 30000).round().toString());
    final retryMultiplier = TextEditingController(text: _number(retryConfig['multiplier'], 2).toString());
    final retryJitter = TextEditingController(text: _number(retryConfig['jitter'], 0.2).toString());
    final nodeType = (node['type'] ?? '').toString();
    final runtimeConfigurable = _needsRuntime(nodeType);
    final executionTarget = _asMap(node['executionTarget']);
    var executionPlacement = _isCloud ? (executionTarget['placement'] ?? 'cloud').toString() : 'local';
    if (_isCloud && !<String>{'cloud', 'device', 'auto'}.contains(executionPlacement)) executionPlacement = 'cloud';
    var executionDeviceId = (executionTarget['deviceId'] ?? '').toString();
    var offlinePolicy = <String>{'fail', 'wait'}.contains((executionTarget['offlinePolicy'] ?? '').toString())
        ? executionTarget['offlinePolicy'].toString()
        : 'fail';
    var nestedWorkflows = List<Map<String, dynamic>>.from(_ownedWorkflows);
    if (_isCloud && nodeType == 'nested_workflow' && executionPlacement == 'device' && executionDeviceId.isNotEmpty) {
      try {
        nestedWorkflows = await ref.read(extensionServiceProvider).workflows(
              limit: 200,
              target: WorkflowApiTarget.device(executionDeviceId),
            );
      } catch (_) {
        nestedWorkflows = <Map<String, dynamic>>[];
      }
    }
    var delete = false;
    final applied = await showModalBottomSheet<bool>(
      context: context,
      isScrollControlled: true,
      showDragHandle: true,
      builder: (context) => StatefulBuilder(
        builder: (context, setSheetState) => SafeArea(
          child: Padding(
            padding: EdgeInsets.fromLTRB(20, 0, 20, MediaQuery.viewInsetsOf(context).bottom + 16),
            child: SizedBox(
              height: MediaQuery.sizeOf(context).height * 0.78,
              child: Column(
                children: [
                  Row(
                    children: [
                      Expanded(child: Text('节点 · ${node['type']}', style: Theme.of(context).textTheme.titleMedium)),
                      IconButton(
                        tooltip: '删除节点',
                        onPressed: () {
                          delete = true;
                          Navigator.pop(context, true);
                        },
                        icon: const Icon(Icons.delete_outline),
                      ),
                    ],
                  ),
                  Expanded(
                    child: ListView(
                      children: [
                        TextField(controller: label, decoration: const InputDecoration(labelText: '显示名称')),
                        const SizedBox(height: 10),
                        if (_isCloud && _supportsExecutionTarget(nodeType)) ...[
                          Text('执行位置', style: Theme.of(context).textTheme.titleSmall),
                          const SizedBox(height: 8),
                          DropdownButtonFormField<String>(
                            initialValue: executionPlacement,
                            decoration: const InputDecoration(labelText: 'Placement'),
                            items: <DropdownMenuItem<String>>[
                              const DropdownMenuItem(value: 'cloud', child: Text('云端')),
                              if (nodeType != 'nested_workflow') const DropdownMenuItem(value: 'auto', child: Text('自动选择设备')),
                              const DropdownMenuItem(value: 'device', child: Text('指定设备')),
                            ],
                            onChanged: (value) async {
                              final next = value ?? 'cloud';
                              setSheetState(() {
                                executionPlacement = next;
                                if (next != 'device') executionDeviceId = '';
                                if (nodeType == 'nested_workflow') {
                                  nestedWorkflows = List<Map<String, dynamic>>.from(_ownedWorkflows);
                                  target.text = '';
                                }
                              });
                            },
                          ),
                          if (executionPlacement == 'device') ...[
                            const SizedBox(height: 10),
                            DropdownButtonFormField<String>(
                              initialValue: _devices.any((item) => (item['deviceId'] ?? '').toString() == executionDeviceId) ? executionDeviceId : null,
                              decoration: const InputDecoration(labelText: '目标设备'),
                              hint: const Text('选择账号下设备'),
                              items: _devices.map((item) {
                                final id = (item['deviceId'] ?? '').toString();
                                final labelText = (item['label'] ?? item['name'] ?? id).toString();
                                final online = item['online'] == true;
                                return DropdownMenuItem<String>(value: id, child: Text('$labelText${online ? '' : ' · 离线'}', overflow: TextOverflow.ellipsis));
                              }).toList(growable: false),
                              onChanged: (value) async {
                                final next = value ?? '';
                                setSheetState(() {
                                  executionDeviceId = next;
                                  if (nodeType == 'nested_workflow') {
                                    nestedWorkflows = <Map<String, dynamic>>[];
                                    target.text = '';
                                  }
                                });
                                if (nodeType == 'nested_workflow' && next.isNotEmpty) {
                                  try {
                                    final items = await ref.read(extensionServiceProvider).workflows(
                                          limit: 200,
                                          target: WorkflowApiTarget.device(next),
                                        );
                                    if (context.mounted) setSheetState(() => nestedWorkflows = items);
                                  } catch (_) {
                                    if (context.mounted) setSheetState(() => nestedWorkflows = <Map<String, dynamic>>[]);
                                  }
                                }
                              },
                            ),
                            const SizedBox(height: 10),
                            DropdownButtonFormField<String>(
                              initialValue: offlinePolicy,
                              decoration: const InputDecoration(labelText: '设备离线时'),
                              items: const [
                                DropdownMenuItem(value: 'fail', child: Text('失败')),
                                DropdownMenuItem(value: 'wait', child: Text('等待设备上线')),
                              ],
                              onChanged: (value) => setSheetState(() => offlinePolicy = value ?? 'fail'),
                            ),
                          ],
                          const SizedBox(height: 12),
                        ] else if (!_isCloud) ...[
                          ListTile(
                            contentPadding: EdgeInsets.zero,
                            leading: const Icon(Icons.memory_outlined),
                            title: const Text('执行位置'),
                            subtitle: Text(_isDevice ? '目标设备本地 Runtime' : '当前设备本地 Runtime'),
                          ),
                        ],
                        if (nodeType == 'nested_workflow')
                          DropdownButtonFormField<String>(
                            key: ValueKey<String>('nested-$executionPlacement-$executionDeviceId-${nestedWorkflows.length}'),
                            initialValue: nestedWorkflows.any((item) => (item['id'] ?? item['workflowId'] ?? '').toString() == target.text) ? target.text : null,
                            decoration: const InputDecoration(labelText: '子工作流'),
                            hint: Text(executionPlacement == 'device' ? '选择该设备上的本地工作流' : '选择我的另一个工作流'),
                            items: nestedWorkflows
                                .map((item) {
                                  final id = (item['id'] ?? item['workflowId'] ?? '').toString();
                                  return DropdownMenuItem<String>(value: id, child: Text((item['name'] ?? id).toString(), overflow: TextOverflow.ellipsis));
                                })
                                .where((item) => item.value?.isNotEmpty == true)
                                .toList(growable: false),
                            onChanged: (value) => setSheetState(() => target.text = value ?? ''),
                          )
                        else
                          TextField(controller: target, decoration: const InputDecoration(labelText: 'Target ID')),
                        const SizedBox(height: 10),
                        if (runtimeConfigurable && (!_isCloud || executionPlacement == 'cloud')) ...[
                          TextField(controller: runtimeType, decoration: const InputDecoration(labelText: 'Runtime Type')),
                          const SizedBox(height: 10),
                          TextField(controller: runtimeId, decoration: const InputDecoration(labelText: 'Runtime ID')),
                          const SizedBox(height: 10),
                          TextField(controller: handlerName, decoration: const InputDecoration(labelText: 'Handler Name')),
                          const SizedBox(height: 10),
                          TextField(
                            controller: runtimeMetadata,
                            minLines: 3,
                            maxLines: 6,
                            style: const TextStyle(fontFamily: 'monospace', fontSize: 13),
                            decoration: const InputDecoration(labelText: 'Runtime Metadata JSON', alignLabelWithHint: true),
                          ),
                          const SizedBox(height: 10),
                        ],
                        Text('执行可靠性', style: Theme.of(context).textTheme.titleSmall),
                        const SizedBox(height: 8),
                        TextField(
                          controller: timeoutMs,
                          keyboardType: TextInputType.number,
                          decoration: const InputDecoration(labelText: '节点超时（毫秒，0=继承工作流上限）'),
                        ),
                        const SizedBox(height: 6),
                        SwitchListTile.adaptive(
                          contentPadding: EdgeInsets.zero,
                          title: const Text('自定义重试'),
                          subtitle: const Text('未启用时节点默认只尝试 1 次'),
                          value: retryEnabled,
                          onChanged: (value) => setSheetState(() => retryEnabled = value),
                        ),
                        if (retryEnabled) ...[
                          TextField(controller: retryMaxAttempts, keyboardType: TextInputType.number, decoration: const InputDecoration(labelText: '最大尝试次数（1~10）')),
                          const SizedBox(height: 8),
                          TextField(controller: retryInitialBackoffMs, keyboardType: TextInputType.number, decoration: const InputDecoration(labelText: '首次退避（毫秒，0=默认 200）')),
                          const SizedBox(height: 8),
                          TextField(controller: retryMaxBackoffMs, keyboardType: TextInputType.number, decoration: const InputDecoration(labelText: '最大退避（毫秒，0=默认 30000）')),
                          const SizedBox(height: 8),
                          TextField(controller: retryMultiplier, keyboardType: const TextInputType.numberWithOptions(decimal: true), decoration: const InputDecoration(labelText: '退避倍率（>1，≤10）')),
                          const SizedBox(height: 8),
                          TextField(controller: retryJitter, keyboardType: const TextInputType.numberWithOptions(decimal: true), decoration: const InputDecoration(labelText: '随机抖动（0~1）')),
                          const SizedBox(height: 10),
                        ],
                        DropdownButtonFormField<String>(
                          initialValue: <String>{'fail', 'continue', 'use_default'}.contains(onError) ? onError : 'fail',
                          decoration: const InputDecoration(labelText: '失败策略'),
                          items: const [
                            DropdownMenuItem(value: 'fail', child: Text('Fail workflow')),
                            DropdownMenuItem(value: 'continue', child: Text('Continue')),
                            DropdownMenuItem(value: 'use_default', child: Text('Use default')),
                          ],
                          onChanged: (v) => setSheetState(() => onError = v ?? 'fail'),
                        ),
                        const SizedBox(height: 10),
                        TextField(
                          controller: onErrorDefault,
                          minLines: 2,
                          maxLines: 5,
                          style: const TextStyle(fontFamily: 'monospace', fontSize: 13),
                          decoration: const InputDecoration(labelText: '失败默认值 JSON（仅 Use default）', alignLabelWithHint: true),
                        ),
                        const SizedBox(height: 10),
                        TextField(controller: permissions, decoration: const InputDecoration(labelText: '权限（逗号分隔）')),
                        const SizedBox(height: 10),
                        TextField(
                          controller: input,
                          minLines: 4,
                          maxLines: 8,
                          style: const TextStyle(fontFamily: 'monospace', fontSize: 13),
                          decoration: const InputDecoration(labelText: 'Input JSON', alignLabelWithHint: true),
                        ),
                        const SizedBox(height: 8),
                        OutlinedButton.icon(
                          onPressed: () => _showDataMappingDialog(context, node, input),
                          icon: const Icon(Icons.data_object_outlined),
                          label: const Text('可视化数据映射'),
                        ),
                        const SizedBox(height: 10),
                        TextField(
                          controller: when,
                          minLines: 3,
                          maxLines: 6,
                          style: const TextStyle(fontFamily: 'monospace', fontSize: 13),
                          decoration: const InputDecoration(labelText: 'When / Condition JSON（可空）', alignLabelWithHint: true),
                        ),
                        const SizedBox(height: 16),
                        _NodeTraceCard(
                          stepRun: _stepRuns[(node['id'] ?? '').toString()],
                          attempts: _stepAttempts.where((item) => (item['nodeId'] ?? '').toString() == (node['id'] ?? '').toString()).toList(growable: false),
                          checkpoint: _checkpoints.any((item) => (item['nodeId'] ?? '').toString() == (node['id'] ?? '').toString()),
                        ),
                      ],
                    ),
                  ),
                  SizedBox(width: double.infinity, child: FilledButton(onPressed: () => Navigator.pop(context, true), child: const Text('应用节点配置'))),
                ],
              ),
            ),
          ),
        ),
      ),
    );
    if (applied == true && mounted) {
      if (delete) {
        final id = (node['id'] ?? '').toString();
        setState(() {
          _nodes.removeWhere((e) => (e['id'] ?? '').toString() == id);
          _edges.removeWhere((e) => e['source'] == id || e['target'] == id);
          if (_connectFrom == id) _connectFrom = null;
          _dirty = true;
        });
      } else {
        try {
          final parsedInput = _decodeJson(input.text, empty: <String, dynamic>{});
          final parsedWhen = when.text.trim().isEmpty ? null : _decodeJson(when.text);
          final parsedRuntimeMetadata = runtimeMetadata.text.trim().isEmpty ? <String, dynamic>{} : _decodeJson(runtimeMetadata.text);
          if (parsedRuntimeMetadata is! Map) {
            throw const FormatException('Runtime Metadata 必须是 JSON object');
          }
          final parsedDefault = onErrorDefault.text.trim().isEmpty ? null : _decodeJson(onErrorDefault.text);
          final parsedTimeoutMs = int.tryParse(timeoutMs.text.trim()) ?? -1;
          if (parsedTimeoutMs < 0) throw const FormatException('节点超时必须是大于等于 0 的毫秒数');
          Map<String, dynamic>? parsedRetry;
          if (retryEnabled) {
            final attempts = int.tryParse(retryMaxAttempts.text.trim()) ?? 0;
            final initialBackoff = int.tryParse(retryInitialBackoffMs.text.trim()) ?? -1;
            final maxBackoff = int.tryParse(retryMaxBackoffMs.text.trim()) ?? -1;
            final multiplier = double.tryParse(retryMultiplier.text.trim()) ?? 0;
            final jitter = double.tryParse(retryJitter.text.trim()) ?? -1;
            if (attempts < 1 || attempts > 10) throw const FormatException('最大尝试次数必须在 1~10 之间');
            if (initialBackoff < 0 || maxBackoff < 0 || initialBackoff > 600000 || maxBackoff > 600000) {
              throw const FormatException('重试退避必须在 0~600000 毫秒之间');
            }
            if (initialBackoff > 0 && maxBackoff > 0 && maxBackoff < initialBackoff) {
              throw const FormatException('最大退避不能小于首次退避');
            }
            if (multiplier <= 1 || multiplier > 10) throw const FormatException('退避倍率必须大于 1 且不超过 10');
            if (jitter < 0 || jitter > 1) throw const FormatException('随机抖动必须在 0~1 之间');
            parsedRetry = <String, dynamic>{
              'maxAttempts': attempts,
              'initialBackoffMs': initialBackoff,
              'maxBackoffMs': maxBackoff,
              'multiplier': multiplier,
              'jitter': jitter,
            };
          }
          if (_isCloud && executionPlacement == 'device' && executionDeviceId.isEmpty) {
            throw const FormatException('指定设备执行时必须选择目标设备');
          }
          if (nodeType == 'nested_workflow' && target.text.trim().isEmpty) {
            throw const FormatException('Nested Workflow 必须选择目标工作流');
          }
          setState(() {
            node['label'] = label.text.trim().isEmpty ? (node['type'] ?? 'Node').toString() : label.text.trim();
            node['targetId'] = target.text.trim();
            node['executionTarget'] = _isCloud
                ? <String, dynamic>{
                    'placement': executionPlacement,
                    'deviceId': executionPlacement == 'device' ? executionDeviceId : '',
                    'runtimeId': '',
                    'providerId': '',
                    'providerInstanceId': '',
                    'offlinePolicy': executionPlacement == 'device' ? offlinePolicy : 'fail',
                  }
                : <String, dynamic>{'placement': 'local', 'offlinePolicy': 'fail'};
            node['runtime'] = <String, dynamic>{
              ...runtime,
              'runtimeType': runtimeConfigurable ? runtimeType.text.trim() : (runtime['runtimeType'] ?? '').toString(),
              'runtimeId': runtimeConfigurable ? runtimeId.text.trim() : (runtime['runtimeId'] ?? '').toString(),
              'handlerName': runtimeConfigurable ? handlerName.text.trim() : (runtime['handlerName'] ?? '').toString(),
              'metadata': Map<String, dynamic>.from(parsedRuntimeMetadata),
            };
            node['permissions'] = permissions.text.split(',').map((e) => e.trim()).where((e) => e.isNotEmpty).toList();
            if (parsedTimeoutMs > 0) {
              node['timeoutMs'] = parsedTimeoutMs;
            } else {
              node.remove('timeoutMs');
            }
            if (parsedRetry != null) {
              node['retry'] = parsedRetry;
            } else {
              node.remove('retry');
            }
            node['step'] = <String, dynamic>{
              ...step,
              'input': parsedInput,
              'onError': <String, dynamic>{'mode': onError, if (parsedDefault != null) 'default': parsedDefault},
              if (parsedWhen != null) 'when': parsedWhen,
            }..removeWhere((key, value) => key == 'when' && parsedWhen == null);
            _dirty = true;
          });
        } catch (error) {
          _show('JSON 无效：${_message(error)}');
        }
      }
    }
    for (final controller in <TextEditingController>[label, target, runtimeType, runtimeId, handlerName, runtimeMetadata, onErrorDefault, timeoutMs, retryMaxAttempts, retryInitialBackoffMs, retryMaxBackoffMs, retryMultiplier, retryJitter, input, when, permissions]) {
      controller.dispose();
    }
  }

  static bool _supportsExecutionTarget(String type) => const <String>{'tool', 'mcp', 'task', 'javascript', 'wasm', 'trusted_service', 'nested_workflow'}.contains(type);

  static bool _needsRuntime(String type) => const <String>{'mcp', 'task', 'javascript', 'wasm', 'trusted_service'}.contains(type);

  static String _defaultRuntimeType(String type) {
    const mapping = <String, String>{
      'mcp': 'mcp',
      'task': 'task',
      'javascript': 'javascript',
      'wasm': 'wasm',
      'trusted_service': 'trusted_service',
    };
    return mapping[type] ?? '';
  }

  dynamic _decodeJson(String text, {dynamic empty}) {
    final value = text.trim();
    if (value.isEmpty) return empty;
    return jsonDecode(value);
  }

  String _pretty(dynamic value) {
    try {
      return const JsonEncoder.withIndent('  ').convert(value);
    } catch (_) {
      return '$value';
    }
  }

  Future<void> _editEdges() async {
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      showDragHandle: true,
      builder: (sheetContext) => StatefulBuilder(
        builder: (context, setSheetState) => SafeArea(
          child: SizedBox(
            height: MediaQuery.sizeOf(context).height * 0.72,
            child: Column(
              children: [
                Padding(
                  padding: const EdgeInsets.fromLTRB(20, 0, 12, 8),
                  child: Row(
                    children: [
                      Expanded(child: Text('连线配置', style: Theme.of(context).textTheme.titleMedium)),
                      Text('${_edges.length} edges'),
                    ],
                  ),
                ),
                Expanded(
                  child: _edges.isEmpty
                      ? const Center(child: Text('暂无连线。点击节点右侧端口，再点击目标节点左侧端口创建。'))
                      : ListView.separated(
                          padding: const EdgeInsets.symmetric(horizontal: 12),
                          itemCount: _edges.length,
                          separatorBuilder: (_, __) => const Divider(height: 1),
                          itemBuilder: (context, index) {
                            final edge = _edges[index];
                            final condition = edge['condition'];
                            return ListTile(
                              title: Text('${edge['source']} → ${edge['target']}'),
                              subtitle: Text(condition == null ? ((edge['label'] ?? '').toString().isEmpty ? '无条件' : edge['label'].toString()) : '条件：${_pretty(condition)}', maxLines: 2, overflow: TextOverflow.ellipsis),
                              trailing: IconButton(
                                onPressed: () {
                                  setState(() {
                                    _edges.removeAt(index);
                                    _dirty = true;
                                  });
                                  setSheetState(() {});
                                },
                                icon: const Icon(Icons.delete_outline),
                              ),
                              onTap: () async {
                                await _editEdge(edge);
                                setSheetState(() {});
                              },
                            );
                          },
                        ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Future<void> _editEdge(Map<String, dynamic> edge) async {
    final label = TextEditingController(text: (edge['label'] ?? '').toString());
    final condition = TextEditingController(text: edge['condition'] == null ? '' : _pretty(edge['condition']));
    final applied = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text('${edge['source']} → ${edge['target']}'),
        content: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TextField(controller: label, decoration: const InputDecoration(labelText: 'Edge Label')),
              const SizedBox(height: 12),
              TextField(controller: condition, minLines: 4, maxLines: 8, style: const TextStyle(fontFamily: 'monospace', fontSize: 13), decoration: const InputDecoration(labelText: 'Condition JSON（可空）')),
            ],
          ),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context, false), child: const Text('取消')),
          FilledButton(onPressed: () => Navigator.pop(context, true), child: const Text('应用')),
        ],
      ),
    );
    if (applied == true) {
      try {
        final parsed = condition.text.trim().isEmpty ? null : _decodeJson(condition.text);
        setState(() {
          edge['label'] = label.text.trim();
          if (parsed == null) {
            edge.remove('condition');
          } else {
            edge['condition'] = parsed;
          }
          _dirty = true;
        });
      } catch (error) {
        _show('Condition JSON 无效：${_message(error)}');
      }
    }
    label.dispose();
    condition.dispose();
  }

  Future<void> _editTriggers() async {
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      showDragHandle: true,
      builder: (sheetContext) => StatefulBuilder(
        builder: (context, setSheetState) => SafeArea(
          child: SizedBox(
            height: MediaQuery.sizeOf(context).height * 0.78,
            child: Column(
              children: [
                Padding(
                  padding: const EdgeInsets.fromLTRB(20, 0, 8, 6),
                  child: Row(
                    children: [
                      Expanded(child: Text('Trigger Center', style: Theme.of(context).textTheme.titleMedium)),
                      IconButton(
                        tooltip: '添加 Trigger',
                        onPressed: () async {
                          final type = await _chooseTriggerType();
                          if (type == null) return;
                          setState(() {
                            _triggers.add(<String, dynamic>{
                              'id': 'trigger-${DateTime.now().microsecondsSinceEpoch}',
                              'type': type,
                              'eventType': '',
                              'config': <String, dynamic>{},
                              'enabled': true,
                            });
                            _dirty = true;
                          });
                          setSheetState(() {});
                        },
                        icon: const Icon(Icons.add),
                      ),
                    ],
                  ),
                ),
                Expanded(
                  child: ListView.separated(
                    padding: const EdgeInsets.fromLTRB(12, 4, 12, 20),
                    itemCount: _triggers.length,
                    separatorBuilder: (_, __) => const SizedBox(height: 8),
                    itemBuilder: (context, index) {
                      final trigger = _triggers[index];
                      final type = (trigger['type'] ?? 'manual').toString();
                      return Container(
                        decoration: BoxDecoration(color: context.surfaceSecondary, borderRadius: AppRadius.brMedium),
                        child: ListTile(
                          leading: Icon(_triggerIcon(type)),
                          title: Text(_triggerLabel(type)),
                          subtitle: Text(_triggerSummary(trigger), maxLines: 2, overflow: TextOverflow.ellipsis),
                          trailing: Row(
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              Switch(
                                value: trigger['enabled'] != false,
                                onChanged: (v) {
                                  setState(() {
                                    trigger['enabled'] = v;
                                    _dirty = true;
                                  });
                                  setSheetState(() {});
                                },
                              ),
                              IconButton(
                                tooltip: '删除',
                                onPressed: () {
                                  setState(() {
                                    _triggers.removeAt(index);
                                    _dirty = true;
                                  });
                                  setSheetState(() {});
                                },
                                icon: const Icon(Icons.delete_outline),
                              ),
                            ],
                          ),
                          onTap: () async {
                            await _editTrigger(trigger);
                            setSheetState(() {});
                          },
                        ),
                      );
                    },
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Future<String?> _chooseTriggerType() {
    return showModalBottomSheet<String>(
      context: context,
      showDragHandle: true,
      builder: (context) => SafeArea(
        child: ListView(
          shrinkWrap: true,
          children: _triggerTypes
              .map((type) => ListTile(leading: Icon(_triggerIcon(type)), title: Text(_triggerLabel(type)), onTap: () => Navigator.pop(context, type)))
              .toList(),
        ),
      ),
    );
  }


  String _triggerCapabilityId(String preset) {
    return switch (preset) {
      'android_intent' => 'workflow.trigger.android_intent.v1',
      'tasker' => 'workflow.trigger.tasker.v1',
      'voice_wake' => 'workflow.trigger.voice_wake.v1',
      'voice_phrase' => 'workflow.trigger.voice_phrase.v1',
      'app_foreground' => 'workflow.trigger.app_foreground.v1',
      _ => '',
    };
  }

  Map<String, dynamic>? _triggerCapability(String preset) {
    final id = _triggerCapabilityId(preset);
    if (id.isEmpty) return null;
    for (final item in _triggerCapabilities) {
      if ((item['id'] ?? '').toString() == id) return item;
    }
    return null;
  }

  bool _canUseDeviceTrigger(String preset) {
    if (preset == 'advanced') return true;
    if (_isCloud) return false;
    return _triggerCapability(preset)?['supported'] == true;
  }

  String _triggerCapabilityLabel(String preset) {
    if (_isCloud) return '仅可用于本地或指定设备 Workflow';
    final item = _triggerCapability(preset);
    if (item == null) return '未取得目标设备 Capability 状态';
    if (item['supported'] != true) return '目标设备不支持该触发器';
    if (item['available'] == true) return '目标设备当前可用';
    final reason = (item['reason'] ?? '').toString().trim();
    final permission = (item['permission'] ?? '').toString().trim();
    if (reason.isNotEmpty) return reason;
    if (permission.isNotEmpty) return '需要权限：$permission';
    return '目标设备当前不可用';
  }

  Future<void> _chooseWakeConfig(TextEditingController controller) async {
    if (_triggerWakeConfigs.isEmpty) {
      _show('目标设备没有启用的 Wake Config，请先创建并启用唤醒配置');
      return;
    }
    final selected = await showModalBottomSheet<String>(
      context: context,
      showDragHandle: true,
      builder: (context) => SafeArea(
        child: ListView.builder(
          itemCount: _triggerWakeConfigs.length,
          itemBuilder: (context, index) {
            final item = _triggerWakeConfigs[index];
            final id = (item['id'] ?? '').toString().trim();
            final name = (item['name'] ?? '').toString().trim();
            final backend = (item['backend'] ?? '').toString().trim();
            return ListTile(
              title: Text(name.isEmpty ? id : name),
              subtitle: Text([if (backend.isNotEmpty) backend, id].join(' · ')),
              onTap: id.isEmpty ? null : () => Navigator.pop(context, id),
            );
          },
        ),
      ),
    );
    if (selected == null || selected.trim().isEmpty) return;
    controller.text = selected.trim();
  }

  Future<void> _appendAppPackageFromCatalog(TextEditingController controller) async {
    if (_triggerAppCatalog.isEmpty) {
      _show('目标设备尚未上报可启动应用目录，可继续手动填写 Package Name');
      return;
    }
    final selected = await showModalBottomSheet<String>(
      context: context,
      showDragHandle: true,
      builder: (context) => SafeArea(
        child: ListView.builder(
          itemCount: _triggerAppCatalog.length,
          itemBuilder: (context, index) {
            final app = _triggerAppCatalog[index];
            final packageName = (app['packageName'] ?? '').toString();
            final label = (app['label'] ?? '').toString().trim();
            return ListTile(
              title: Text(label.isEmpty ? packageName : label),
              subtitle: label.isEmpty ? null : Text(packageName),
              onTap: packageName.isEmpty ? null : () => Navigator.pop(context, packageName),
            );
          },
        ),
      ),
    );
    if (selected == null || selected.trim().isEmpty) return;
    final values = _splitValues(controller.text).toSet();
    values.add(selected.trim());
    controller.text = values.join(', ');
  }

  Future<void> _generateTaskerSecret(TextEditingController controller) async {
    if (!_canUseDeviceTrigger('tasker')) {
      _show(_triggerCapabilityLabel('tasker'));
      return;
    }
    try {
      final value = await ref.read(extensionServiceProvider).createWorkflowTaskerSecret(target: _target);
      final secretRef = (value['secretRef'] ?? '').toString().trim();
      final secret = (value['secret'] ?? '').toString();
      if (secretRef.isEmpty || secret.isEmpty) throw StateError('Tasker Secret 返回数据无效');
      controller.text = secretRef;
      if (!mounted) return;
      await showDialog<void>(
        context: context,
        barrierDismissible: false,
        builder: (context) => AlertDialog(
          title: const Text('Tasker Secret（仅显示一次）'),
          content: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Text('请立即复制并保存到 Tasker。Workflow Definition 只保存 Secret Ref。'),
                const SizedBox(height: 12),
                const Text('Action：com.amitia.workflow.TASKER'),
                const SizedBox(height: 8),
                SelectableText(secret),
              ],
            ),
          ),
          actions: [FilledButton(onPressed: () => Navigator.pop(context), child: const Text('我已保存'))],
        ),
      );
    } catch (error) {
      _show('Tasker Secret 生成失败：${_message(error)}');
    }
  }

  Future<void> _editTrigger(Map<String, dynamic> trigger) async {
    final type = (trigger['type'] ?? 'manual').toString();
    final eventType = TextEditingController(text: (trigger['eventType'] ?? '').toString());
    final config = _asMap(trigger['config']);
    final cron = TextEditingController(text: (config['cronExpression'] ?? '').toString());
    final timezone = TextEditingController(text: (config['timezone'] ?? '').toString());
    final interval = TextEditingController(text: (config['intervalSeconds'] ?? config['seconds'] ?? '').toString());
    final runAt = TextEditingController(text: (config['runAt'] ?? '').toString());
    final actions = TextEditingController(text: _stringList(config['actions']).join(', '));
    final categories = TextEditingController(text: _stringList(config['categories']).join(', '));
    final dataSchemes = TextEditingController(text: _stringList(config['dataSchemes']).join(', '));
    final mimeTypes = TextEditingController(text: _stringList(config['mimeTypes']).join(', '));
    final dedupWindowMs = TextEditingController(text: (config['dedupWindowMs'] ?? 2000).toString());
    final taskerEventName = TextEditingController(text: (config['eventName'] ?? '').toString());
    final taskerSecretRef = TextEditingController(text: (config['secretRef'] ?? '').toString());
    final taskerVariables = TextEditingController(text: _stringList(config['allowedVariables']).join(', '));
    final wakeConfigId = TextEditingController(text: (config['wakeConfigId'] ?? '').toString());
    final phrases = TextEditingController(text: _stringList(config['phrases']).join('\n'));
    final packages = TextEditingController(text: _stringList(config['packages']).join(', '));
    final cooldownMs = TextEditingController(text: (config['cooldownMs'] ?? 30000).toString());
    var preset = type == 'event' ? _deviceEventPresetFor(eventType.text) : 'advanced';
    var phraseMatchMode = <String>{'exact', 'normalized'}.contains((config['matchMode'] ?? '').toString()) ? config['matchMode'].toString() : 'normalized';
    final applied = await showDialog<bool>(
      context: context,
      builder: (context) => StatefulBuilder(
        builder: (context, setDialogState) => AlertDialog(
          title: Text(_triggerLabel(type)),
          content: SizedBox(
            width: 520,
            child: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  if (type == 'event') ...[
                    DropdownButtonFormField<String>(
                      initialValue: preset,
                      decoration: const InputDecoration(labelText: '触发方式'),
                      items: [
                        const DropdownMenuItem(value: 'advanced', child: Text('高级 Event Trigger')),
                        DropdownMenuItem(value: 'android_intent', enabled: _canUseDeviceTrigger('android_intent'), child: const Text('Android Intent')),
                        DropdownMenuItem(value: 'tasker', enabled: _canUseDeviceTrigger('tasker'), child: const Text('Tasker')),
                        DropdownMenuItem(value: 'voice_wake', enabled: _canUseDeviceTrigger('voice_wake'), child: const Text('Voice / Wake')),
                        DropdownMenuItem(value: 'voice_phrase', enabled: _canUseDeviceTrigger('voice_phrase'), child: const Text('Voice Phrase')),
                        DropdownMenuItem(value: 'app_foreground', enabled: _canUseDeviceTrigger('app_foreground'), child: const Text('App Launch / Foreground')),
                      ],
                      onChanged: (value) {
                        final next = value ?? 'advanced';
                        if (!_canUseDeviceTrigger(next)) {
                          _show(_triggerCapabilityLabel(next));
                          return;
                        }
                        setDialogState(() {
                          preset = next;
                          eventType.text = switch (next) {
                            'android_intent' => 'device.android.intent',
                            'tasker' => 'device.android.tasker',
                            'voice_wake' => 'voice.wake.detected',
                            'voice_phrase' => 'voice.asr.final',
                            'app_foreground' => 'device.app.foreground',
                            _ => eventType.text,
                          };
                        });
                      },
                    ),
                    const SizedBox(height: 10),
                    if (preset != 'advanced') ...[
                      Text(_triggerCapabilityLabel(preset), style: Theme.of(context).textTheme.bodySmall),
                      const SizedBox(height: 10),
                    ],
                    if (preset == 'advanced')
                      TextField(controller: eventType, decoration: const InputDecoration(labelText: 'Event Type')),
                    if (preset == 'android_intent') ...[
                      TextField(controller: actions, decoration: const InputDecoration(labelText: 'Intent Actions', hintText: 'com.example.EVENT')),
                      const SizedBox(height: 10),
                      TextField(controller: categories, decoration: const InputDecoration(labelText: 'Categories（可空，逗号分隔）')),
                      const SizedBox(height: 10),
                      TextField(controller: dataSchemes, decoration: const InputDecoration(labelText: 'Data Schemes（可空，逗号分隔）')),
                      const SizedBox(height: 10),
                      TextField(controller: mimeTypes, decoration: const InputDecoration(labelText: 'MIME Types（可空，逗号分隔）')),
                      const SizedBox(height: 10),
                      TextField(controller: dedupWindowMs, keyboardType: TextInputType.number, decoration: const InputDecoration(labelText: '去重窗口（毫秒，最大 600000）')),
                      const SizedBox(height: 8),
                      const Text('第三方 App 需向 Amitia Receiver 发送显式 Broadcast；Release Component=com.amitia.amitia_app/com.amitia.amitia_app.workflow.WorkflowIntentReceiver，Debug/变体构建请使用实际 applicationId。Action 使用上方配置值。'),
                    ],
                    if (preset == 'tasker') ...[
                      TextField(controller: taskerEventName, decoration: const InputDecoration(labelText: 'Tasker Event Name', hintText: 'home_arrived')),
                      const SizedBox(height: 10),
                      TextField(controller: taskerSecretRef, readOnly: true, decoration: const InputDecoration(labelText: 'Secret Ref', hintText: '点击下方按钮生成')),
                      const SizedBox(height: 8),
                      OutlinedButton.icon(
                        onPressed: _canUseDeviceTrigger('tasker') ? () => _generateTaskerSecret(taskerSecretRef) : null,
                        icon: const Icon(Icons.key_outlined),
                        label: const Text('生成 Tasker Secret'),
                      ),
                      const SizedBox(height: 10),
                      TextField(controller: taskerVariables, decoration: const InputDecoration(labelText: '允许变量（逗号分隔）', hintText: 'battery, location')),
                      const SizedBox(height: 8),
                      const Text('Tasker 使用显式 Broadcast：Action=com.amitia.workflow.TASKER；Release Component=com.amitia.amitia_app/com.amitia.amitia_app.workflow.WorkflowIntentReceiver，Debug/变体构建请使用实际 applicationId；Secret 实值只显示一次。'),
                    ],
                    if (preset == 'voice_wake') ...[
                      TextField(controller: wakeConfigId, readOnly: true, decoration: const InputDecoration(labelText: 'Wake Config', hintText: '必须选择已启用配置')),
                      const SizedBox(height: 8),
                      OutlinedButton.icon(
                        onPressed: () => _chooseWakeConfig(wakeConfigId),
                        icon: const Icon(Icons.record_voice_over_outlined),
                        label: Text(_triggerWakeConfigs.isEmpty ? '没有可用 Wake Config' : '选择 Wake Config（${_triggerWakeConfigs.length}）'),
                      ),
                    ],
                    if (preset == 'voice_phrase') ...[
                      TextField(controller: phrases, minLines: 2, maxLines: 5, decoration: const InputDecoration(labelText: '短语（每行一个）', hintText: '开始回家模式')),
                      const SizedBox(height: 10),
                      DropdownButtonFormField<String>(
                        initialValue: phraseMatchMode,
                        decoration: const InputDecoration(labelText: '匹配模式'),
                        items: const [
                          DropdownMenuItem(value: 'normalized', child: Text('Normalized')),
                          DropdownMenuItem(value: 'exact', child: Text('Exact')),
                        ],
                        onChanged: (value) => setDialogState(() => phraseMatchMode = value ?? 'normalized'),
                      ),
                    ],
                    if (preset == 'app_foreground') ...[
                      TextField(controller: packages, decoration: const InputDecoration(labelText: 'Package Names', hintText: 'com.example.app')),
                      const SizedBox(height: 8),
                      OutlinedButton.icon(
                        onPressed: () => _appendAppPackageFromCatalog(packages),
                        icon: const Icon(Icons.apps_outlined),
                        label: Text(_triggerAppCatalog.isEmpty ? '设备应用目录未就绪' : '从设备应用选择（${_triggerAppCatalog.length}）'),
                      ),
                      const SizedBox(height: 10),
                      TextField(controller: cooldownMs, keyboardType: TextInputType.number, decoration: const InputDecoration(labelText: '冷却时间（毫秒）')),
                      const SizedBox(height: 8),
                      const Text('语义为 package becomes foreground；同一 Package 内 Activity 切换不会重复触发。'),
                    ],
                  ] else if (!<String>{'manual', 'schedule', 'cron', 'interval', 'one_shot'}.contains(type))
                    TextField(controller: eventType, decoration: const InputDecoration(labelText: 'Event Type')),
                  if (type == 'cron' || type == 'schedule') ...[
                    TextField(controller: cron, decoration: const InputDecoration(labelText: 'Cron Expression', hintText: '0 8 * * *')),
                    const SizedBox(height: 10),
                    TextField(controller: timezone, decoration: const InputDecoration(labelText: 'Timezone', hintText: 'Asia/Shanghai')),
                  ],
                  if (type == 'interval')
                    TextField(controller: interval, keyboardType: TextInputType.number, decoration: const InputDecoration(labelText: 'Interval Seconds')),
                  if (type == 'one_shot')
                    TextField(controller: runAt, decoration: const InputDecoration(labelText: 'Run At (RFC3339)')),
                ],
              ),
            ),
          ),
          actions: [
            TextButton(onPressed: () => Navigator.pop(context, false), child: const Text('取消')),
            FilledButton(onPressed: () => Navigator.pop(context, true), child: const Text('应用')),
          ],
        ),
      ),
    );
    if (applied == true && mounted) {
      setState(() {
        trigger['eventType'] = eventType.text.trim();
        if (type == 'event') {
          trigger['config'] = switch (preset) {
            'android_intent' => <String, dynamic>{
                'actions': _splitValues(actions.text),
                'categories': _splitValues(categories.text),
                'dataSchemes': _splitValues(dataSchemes.text),
                'mimeTypes': _splitValues(mimeTypes.text),
                'dedupWindowMs': (int.tryParse(dedupWindowMs.text.trim()) ?? 2000).clamp(0, 600000).toInt(),
              },
            'tasker' => <String, dynamic>{
                'eventName': taskerEventName.text.trim(),
                'secretRef': taskerSecretRef.text.trim(),
                'allowedVariables': _splitValues(taskerVariables.text),
              },
            'voice_wake' => <String, dynamic>{'mode': 'wake', 'wakeConfigId': wakeConfigId.text.trim()},
            'voice_phrase' => <String, dynamic>{'mode': 'phrase', 'phrases': _splitValues(phrases.text), 'matchMode': phraseMatchMode},
            'app_foreground' => <String, dynamic>{
                'packages': _splitValues(packages.text),
                'cooldownMs': (int.tryParse(cooldownMs.text.trim()) ?? 30000).clamp(0, 86400000).toInt(),
              },
            _ => config,
          };
        } else {
          trigger['config'] = <String, dynamic>{
            ...config,
            if (cron.text.trim().isNotEmpty) 'cronExpression': cron.text.trim(),
            if (timezone.text.trim().isNotEmpty) 'timezone': timezone.text.trim(),
            if (int.tryParse(interval.text.trim()) != null) 'intervalSeconds': int.parse(interval.text.trim()),
            if (runAt.text.trim().isNotEmpty) 'runAt': runAt.text.trim(),
          };
        }
        _dirty = true;
      });
    }
    for (final c in <TextEditingController>[
      eventType,
      cron,
      timezone,
      interval,
      runAt,
      actions,
      categories,
      dataSchemes,
      mimeTypes,
      dedupWindowMs,
      taskerEventName,
      taskerSecretRef,
      taskerVariables,
      wakeConfigId,
      phrases,
      packages,
      cooldownMs,
    ]) {
      c.dispose();
    }
  }

  String _deviceEventPresetFor(String eventType) => switch (eventType.trim()) {
        'device.android.intent' => 'android_intent',
        'device.android.tasker' => 'tasker',
        'voice.wake.detected' => 'voice_wake',
        'voice.asr.final' => 'voice_phrase',
        'device.app.foreground' => 'app_foreground',
        _ => 'advanced',
      };

  List<String> _stringList(dynamic value) {
    if (value is! List) return <String>[];
    return value.map((item) => item.toString().trim()).where((item) => item.isNotEmpty).toList(growable: false);
  }

  List<String> _splitValues(String value) => value
      .split(RegExp(r'[,\n]'))
      .map((item) => item.trim())
      .where((item) => item.isNotEmpty)
      .toSet()
      .toList(growable: false);

  String _triggerSummary(Map<String, dynamic> trigger) {
    final type = (trigger['type'] ?? 'manual').toString();
    final config = _asMap(trigger['config']);
    if (type == 'cron' || type == 'schedule') return '${config['cronExpression'] ?? '未配置'} · ${config['timezone'] ?? 'local'}';
    if (type == 'interval') return '每 ${config['intervalSeconds'] ?? '?'} 秒';
    if (type == 'one_shot') return (config['runAt'] ?? '未配置时间').toString();
    if (type == 'manual') return '用户手动运行';
    return (trigger['eventType'] ?? type).toString();
  }

  String _triggerLabel(String type) => switch (type) {
        'manual' => 'Manual',
        'schedule' => 'Schedule',
        'cron' => 'Cron',
        'interval' => 'Interval',
        'one_shot' => 'One Shot',
        'event' => 'System / Device Event',
        _ => type,
      };

  IconData _triggerIcon(String type) => switch (type) {
        'manual' => Icons.touch_app_outlined,
        'schedule' || 'cron' || 'interval' || 'one_shot' => Icons.schedule_outlined,
        _ => Icons.bolt_outlined,
      };

  Future<void> _showRuns() async {
    await Future.wait(<Future<void>>[_loadRuns(), _loadStats()]);
    if (!mounted) return;
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      showDragHandle: true,
      builder: (context) => SafeArea(
        child: SizedBox(
          height: MediaQuery.sizeOf(context).height * 0.72,
          child: Column(
            children: [
              Padding(
                padding: const EdgeInsets.fromLTRB(20, 0, 12, 8),
                child: Row(
                  children: [
                    Expanded(child: Text('Execution Trace', style: Theme.of(context).textTheme.titleMedium)),
                    if (_activeRunId != null) _RunStatusBadge(status: _activeRunStatus),
                  ],
                ),
              ),
              if (_workflowStats.isNotEmpty)
                Padding(
                  padding: const EdgeInsets.fromLTRB(16, 0, 16, 10),
                  child: Row(
                    children: [
                      Expanded(child: _WorkflowStatTile(label: '运行', value: '${_workflowStats['runCount'] ?? 0}')),
                      const SizedBox(width: 6),
                      Expanded(child: _WorkflowStatTile(label: '成功率', value: '${((_number(_workflowStats['successRate']) * 100).round())}%')),
                      const SizedBox(width: 6),
                      Expanded(child: _WorkflowStatTile(label: '平均耗时', value: _formatDuration(_number(_workflowStats['averageRunMs'])))),
                      const SizedBox(width: 6),
                      Expanded(child: _WorkflowStatTile(label: '失败', value: '${_workflowStats['failed'] ?? 0}')),
                    ],
                  ),
                ),
              if (_activeRunId != null)
                Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 16),
                  child: Wrap(
                    spacing: 8,
                    runSpacing: 8,
                    children: [
                      OutlinedButton.icon(onPressed: _activeRunStatus == 'paused' ? null : _pauseRun, icon: const Icon(Icons.pause), label: const Text('暂停')),
                      OutlinedButton.icon(onPressed: _activeRunStatus == 'paused' ? _resumeRun : null, icon: const Icon(Icons.play_arrow), label: const Text('恢复')),
                      OutlinedButton.icon(
                        onPressed: <String>{'failed', 'cancelled'}.contains(_activeRunStatus.toLowerCase()) && _checkpoints.isNotEmpty && _workflow?['enabled'] != false ? _recoverActiveRun : null,
                        icon: const Icon(Icons.restore_page_outlined),
                        label: const Text('Checkpoint 恢复'),
                      ),
                      OutlinedButton.icon(
                        onPressed: _terminal(_activeRunStatus) && _workflow?['enabled'] != false ? _rerunActiveRun : null,
                        icon: const Icon(Icons.replay),
                        label: const Text('原输入重跑'),
                      ),
                      OutlinedButton.icon(onPressed: _terminal(_activeRunStatus) ? null : _cancelRun, icon: const Icon(Icons.stop), label: const Text('取消')),
                    ],
                  ),
                ),
              const SizedBox(height: 8),
              Expanded(
                child: _runHistory.isEmpty
                    ? const Center(child: Text('暂无运行记录'))
                    : ListView.separated(
                        itemCount: _runHistory.length,
                        separatorBuilder: (_, __) => const Divider(height: 1),
                        itemBuilder: (context, index) {
                          final run = _runHistory[index];
                          return ListTile(
                            leading: _RunStatusIcon(status: (run['status'] ?? '').toString()),
                            title: Text((run['executionId'] ?? '').toString(), maxLines: 1, overflow: TextOverflow.ellipsis),
                            subtitle: Text('${run['startedAt'] ?? ''}${(run['error'] ?? '').toString().isEmpty ? '' : '\n${run['error']}'}', maxLines: 3, overflow: TextOverflow.ellipsis),
                            trailing: _RunStatusBadge(status: (run['status'] ?? '').toString()),
                            onTap: () async {
                              final id = (run['executionId'] ?? '').toString();
                              if (id.isEmpty) return;
                              setState(() {
                                _activeRunId = id;
                                _activeRunStatus = (run['status'] ?? '').toString();
                              });
                              await _pollRun();
                              if (mounted) Navigator.pop(context);
                            },
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

  Future<void> _showSafetyAnalysis() async {
    if (_isDevice) { _show('远程设备工作流的安全分析在设备本地执行'); return; }
    try {
      final analysis = await ref.read(extensionServiceProvider).workflowAnalysis(widget.workflowId, target: _target);
      if (!mounted) return;
      final permissions = ((analysis['declaredPermissions'] as List?) ?? const <dynamic>[]).map((e) => e.toString()).toList(growable: false);
      final secrets = ((analysis['secretReferences'] as List?) ?? const <dynamic>[]).map((e) => e.toString()).toList(growable: false);
      final dependencies = _asMapList(analysis['nestedDependencies']);
      final risks = _asMapList(analysis['risks']);
      await showModalBottomSheet<void>(
        context: context,
        isScrollControlled: true,
        showDragHandle: true,
        builder: (context) => SafeArea(
          child: SizedBox(
            height: MediaQuery.sizeOf(context).height * 0.72,
            child: ListView(
              padding: const EdgeInsets.fromLTRB(20, 0, 20, 24),
              children: [
                Row(children: [Expanded(child: Text('权限与风险摘要', style: Theme.of(context).textTheme.titleMedium)), _RiskBadge(level: (analysis['riskLevel'] ?? 'low').toString())]),
                const SizedBox(height: 8),
                Text('这是静态摘要，不代表相关权限已经授予；真实执行仍由 Kernel Capability / Scope 策略决定。', style: AppTypography.caption(context)),
                const SizedBox(height: 16),
                _SafetySection(title: '声明权限', emptyText: '无显式权限声明', items: permissions),
                const SizedBox(height: 12),
                _SafetySection(title: 'Secret / Credential 引用', emptyText: '未发现引用', items: secrets),
                const SizedBox(height: 12),
                Text('子工作流依赖', style: Theme.of(context).textTheme.titleSmall),
                const SizedBox(height: 6),
                if (dependencies.isEmpty) Text('无', style: AppTypography.caption(context)) else ...dependencies.map((dep) => ListTile(contentPadding: EdgeInsets.zero, dense: true, title: Text((dep['name'] ?? dep['workflowId'] ?? '').toString()), subtitle: Text((dep['workflowId'] ?? '').toString()), trailing: _RiskBadge(level: (dep['status'] ?? 'missing').toString()))),
                const SizedBox(height: 12),
                Text('风险提示', style: Theme.of(context).textTheme.titleSmall),
                const SizedBox(height: 6),
                if (risks.isEmpty) Text('未发现需要特别提示的静态风险', style: AppTypography.caption(context)) else ...risks.map((risk) => ListTile(contentPadding: EdgeInsets.zero, dense: true, leading: _RiskBadge(level: (risk['level'] ?? 'low').toString()), title: Text((risk['message'] ?? '').toString()), subtitle: (risk['nodeId'] ?? '').toString().isEmpty ? null : Text('节点：${risk['nodeId']}'))),
              ],
            ),
          ),
        ),
      );
    } catch (error) {
      _show('安全摘要加载失败：${_message(error)}');
    }
  }

  String _formatDuration(double ms) {
    if (ms <= 0) return '0ms';
    if (ms < 1000) return '${ms.round()}ms';
    if (ms < 60000) return '${(ms / 1000).toStringAsFixed(1)}s';
    return '${(ms / 60000).toStringAsFixed(1)}m';
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      resizeToAvoidBottomInset: false,
      appBar: AmitiaAppBar(
        titleWidget: GestureDetector(
          onTap: _workflow == null ? null : _editMetadata,
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Flexible(child: Text((_workflow?['name'] ?? '工作流').toString(), overflow: TextOverflow.ellipsis, style: AppTypography.pageTitle(context))),
              const SizedBox(width: 6),
              Text(_locationLabel, style: AppTypography.caption(context)),
              if (_dirty) Padding(padding: const EdgeInsets.only(left: 6), child: Icon(Icons.circle, size: 7, color: context.warning)),
            ],
          ),
        ),
        showBackButton: true,
        fallbackRoute: AppRoutes.workshopWorkflows,
        actions: [
          if (_aiWorking) const Padding(padding: EdgeInsets.symmetric(horizontal: 10), child: SizedBox(width: 18, height: 18, child: CircularProgressIndicator(strokeWidth: 2))),
          IconButton(tooltip: '校验', onPressed: _workflow == null ? null : _validate, icon: const Icon(Icons.fact_check_outlined)),
          IconButton(tooltip: '保存', onPressed: _workflow == null || _saving ? null : _save, icon: _saving ? const SizedBox(width: 20, height: 20, child: CircularProgressIndicator(strokeWidth: 2)) : const Icon(Icons.save_outlined)),
          IconButton(tooltip: '运行', onPressed: _workflow == null ? null : _run, icon: const Icon(Icons.play_arrow)),
          PopupMenuButton<String>(
            onSelected: (value) {
              switch (value) {
                case 'settings':
                  _editMetadata();
                  break;
                case 'ai_edit':
                  _aiEdit();
                  break;
                case 'ai_repair':
                  _aiRepair();
                  break;
                case 'ai_explain':
                  _aiExplain();
                  break;
                case 'triggers':
                  _editTriggers();
                  break;
                case 'edges':
                  _editEdges();
                  break;
                case 'runs':
                  _showRuns();
                  break;
                case 'versions':
                  _showRevisions();
                  break;
                case 'security':
                  _showSafetyAnalysis();
                  break;
                case 'layout':
                  _autoLayout();
                  break;
              }
            },
            itemBuilder: (context) => <PopupMenuEntry<String>>[
              if (!_isDevice) const PopupMenuItem(value: 'ai_edit', child: ListTile(leading: Icon(Icons.auto_awesome_outlined), title: Text('AI 修改工作流'))),
              if (!_isDevice) const PopupMenuItem(value: 'ai_repair', child: ListTile(leading: Icon(Icons.build_circle_outlined), title: Text('AI 修复工作流'))),
              if (!_isDevice) const PopupMenuItem(value: 'ai_explain', child: ListTile(leading: Icon(Icons.psychology_alt_outlined), title: Text('AI 解释工作流'))),
              const PopupMenuItem(value: 'settings', child: ListTile(leading: Icon(Icons.settings_outlined), title: Text('工作流设置'))),
              const PopupMenuItem(value: 'triggers', child: ListTile(leading: Icon(Icons.bolt_outlined), title: Text('Trigger Center'))),
              const PopupMenuItem(value: 'edges', child: ListTile(leading: Icon(Icons.route_outlined), title: Text('连线配置'))),
              if (!_isDevice) const PopupMenuItem(value: 'runs', child: ListTile(leading: Icon(Icons.timeline_outlined), title: Text('Execution Trace'))),
              if (!_isDevice) const PopupMenuItem(value: 'versions', child: ListTile(leading: Icon(Icons.history_outlined), title: Text('版本历史'))),
              if (!_isDevice) const PopupMenuItem(value: 'security', child: ListTile(leading: Icon(Icons.security_outlined), title: Text('权限与风险摘要'))),
              const PopupMenuItem(value: 'layout', child: ListTile(leading: Icon(Icons.auto_fix_high_outlined), title: Text('自动布局'))),
            ],
          ),
        ],
      ),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _workflow == null
              ? Center(child: FilledButton(onPressed: _load, child: const Text('重新加载')))
              : Column(
                  children: [
                    _Toolbar(
                      nodeCount: _nodes.length,
                      edgeCount: _edges.length,
                      triggerCount: _triggers.where((e) => e['enabled'] != false).length,
                      connectFrom: _connectFrom,
                      runStatus: _activeRunStatus,
                      onAdd: _addNode,
                      onFit: _fitView,
                      onTriggers: _editTriggers,
                      onRuns: _showRuns,
                      onCancelConnect: () => setState(() => _connectFrom = null),
                    ),
                    Expanded(
                      child: ClipRect(
                        child: InteractiveViewer(
                          transformationController: _transform,
                          constrained: false,
                          minScale: 0.25,
                          maxScale: 2.2,
                          boundaryMargin: const EdgeInsets.all(500),
                          child: SizedBox(
                            width: _canvasWidth,
                            height: _canvasHeight,
                            child: Stack(
                              clipBehavior: Clip.none,
                              children: [
                                Positioned.fill(
                                  child: CustomPaint(
                                    painter: _WorkflowGraphPainter(
                                      nodes: _nodes,
                                      edges: _edges,
                                      stepRuns: _stepRuns,
                                      nodeWidth: _nodeWidth,
                                      nodeHeight: _nodeHeight,
                                      lineColor: context.borderPrimary,
                                      activeColor: context.accentPrimary,
                                      successColor: context.success,
                                      errorColor: context.error,
                                    ),
                                  ),
                                ),
                                ..._nodes.map((node) => _nodeWidget(context, node)),
                              ],
                            ),
                          ),
                        ),
                      ),
                    ),
                  ],
                ),
      floatingActionButton: _loading ? null : FloatingActionButton.small(onPressed: _addNode, tooltip: '添加节点', child: const Icon(Icons.add)),
    );
  }

  Widget _nodeWidget(BuildContext context, Map<String, dynamic> node) {
    final id = (node['id'] ?? '').toString();
    final p = _asMap(node['position']);
    final x = _number(p['x'], 100);
    final y = _number(p['y'], 100);
    final status = (_stepRuns[id]?['status'] ?? '').toString().toLowerCase();
    final connecting = _connectFrom == id;
    Color statusColor = context.textTertiary;
    if (status == 'running' || status == 'retry_wait') statusColor = context.info;
    if (status == 'succeeded' || status == 'completed') statusColor = context.success;
    if (status == 'failed') statusColor = context.error;
    if (status == 'cancelled') statusColor = context.warning;
    final type = (node['type'] ?? 'tool').toString();
    final nodeType = _nodeTypes.firstWhere((e) => e.type == type, orElse: () => _NodeType(type, type, 'Kernel node', Icons.extension_outlined));
    return Positioned(
      left: x,
      top: y,
      width: _nodeWidth,
      height: _nodeHeight,
      child: GestureDetector(
        behavior: HitTestBehavior.opaque,
        onPanUpdate: (detail) {
          setState(() {
            final position = _asMap(node['position']);
            position['x'] = (_number(position['x']) + detail.delta.dx).clamp(0.0, _canvasWidth - _nodeWidth);
            position['y'] = (_number(position['y']) + detail.delta.dy).clamp(0.0, _canvasHeight - _nodeHeight);
            node['position'] = position;
            _dirty = true;
          });
        },
        onDoubleTap: () => _editNode(node),
        child: Stack(
          clipBehavior: Clip.none,
          children: [
            Positioned.fill(
              child: Container(
                padding: const EdgeInsets.fromLTRB(16, 12, 16, 10),
                decoration: BoxDecoration(
                  color: context.surfacePrimary,
                  borderRadius: AppRadius.brMedium,
                  border: Border.all(color: connecting ? context.accentPrimary : (status.isEmpty ? context.borderPrimary : statusColor), width: connecting || status.isNotEmpty ? 1.8 : 1),
                  boxShadow: [BoxShadow(color: Colors.black.withValues(alpha: context.isDark ? 0.22 : 0.06), blurRadius: 10, offset: const Offset(0, 3))],
                ),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Icon(nodeType.icon, size: 18, color: context.accentPrimary),
                        const SizedBox(width: 7),
                        Expanded(child: Text((node['label'] ?? type).toString(), maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(fontWeight: FontWeight.w600, fontSize: 13))),
                        if (status.isNotEmpty) Container(width: 8, height: 8, decoration: BoxDecoration(color: statusColor, shape: BoxShape.circle)),
                      ],
                    ),
                    const SizedBox(height: 8),
                    Text(type, style: TextStyle(fontSize: 11, color: context.textTertiary)),
                    const Spacer(),
                    Row(
                      children: [
                        Expanded(child: Text((node['targetId'] ?? '').toString().isEmpty ? '双击配置' : (node['targetId'] ?? '').toString(), maxLines: 1, overflow: TextOverflow.ellipsis, style: TextStyle(fontSize: 10, color: context.textSecondary))),
                        IconButton(
                          visualDensity: VisualDensity.compact,
                          tooltip: '编辑',
                          onPressed: () => _editNode(node),
                          icon: const Icon(Icons.tune, size: 16),
                        ),
                      ],
                    ),
                  ],
                ),
              ),
            ),
            Positioned(
              left: -18,
              top: _nodeHeight / 2 - 18,
              child: Tooltip(
                message: _connectFrom == null ? '输入端口' : '连接到这里',
                child: GestureDetector(
                  onTap: () => _connectTo(id),
                  child: Container(
                    width: 36,
                    height: 36,
                    alignment: Alignment.center,
                    color: Colors.transparent,
                    child: Container(width: 13, height: 13, decoration: BoxDecoration(color: _connectFrom == null ? context.surfacePrimary : context.accentSoft, shape: BoxShape.circle, border: Border.all(color: context.accentPrimary, width: 2))),
                  ),
                ),
              ),
            ),
            Positioned(
              right: -18,
              top: _nodeHeight / 2 - 18,
              child: Tooltip(
                message: '从此节点开始连线',
                child: GestureDetector(
                  onTap: () => setState(() => _connectFrom = _connectFrom == id ? null : id),
                  child: Container(
                    width: 36,
                    height: 36,
                    alignment: Alignment.center,
                    color: Colors.transparent,
                    child: Container(width: 13, height: 13, decoration: BoxDecoration(color: connecting ? context.accentPrimary : context.surfacePrimary, shape: BoxShape.circle, border: Border.all(color: context.accentPrimary, width: 2))),
                  ),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _Toolbar extends StatelessWidget {
  final int nodeCount;
  final int edgeCount;
  final int triggerCount;
  final String? connectFrom;
  final String runStatus;
  final VoidCallback onAdd;
  final VoidCallback onFit;
  final VoidCallback onTriggers;
  final VoidCallback onRuns;
  final VoidCallback onCancelConnect;

  const _Toolbar({
    required this.nodeCount,
    required this.edgeCount,
    required this.triggerCount,
    required this.connectFrom,
    required this.runStatus,
    required this.onAdd,
    required this.onFit,
    required this.onTriggers,
    required this.onRuns,
    required this.onCancelConnect,
  });

  @override
  Widget build(BuildContext context) {
    return Material(
      color: context.surfacePrimary,
      child: SizedBox(
        height: 52,
        child: ListView(
          scrollDirection: Axis.horizontal,
          padding: const EdgeInsets.symmetric(horizontal: 8),
          children: [
            TextButton.icon(onPressed: onAdd, icon: const Icon(Icons.add, size: 18), label: const Text('节点')),
            TextButton.icon(onPressed: onTriggers, icon: const Icon(Icons.bolt_outlined, size: 18), label: Text('Trigger $triggerCount')),
            TextButton.icon(onPressed: onRuns, icon: const Icon(Icons.timeline_outlined, size: 18), label: Text(runStatus.isEmpty ? '运行记录' : runStatus)),
            IconButton(onPressed: onFit, tooltip: '适应画布', icon: const Icon(Icons.center_focus_strong)),
            Center(child: Padding(padding: const EdgeInsets.symmetric(horizontal: 8), child: Text('$nodeCount 节点 · $edgeCount 连线', style: AppTypography.caption(context)))),
            if (connectFrom != null)
              TextButton.icon(onPressed: onCancelConnect, icon: const Icon(Icons.link_off, size: 18), label: Text('取消连线 · $connectFrom')),
          ],
        ),
      ),
    );
  }
}

class _WorkflowGraphPainter extends CustomPainter {
  final List<Map<String, dynamic>> nodes;
  final List<Map<String, dynamic>> edges;
  final Map<String, Map<String, dynamic>> stepRuns;
  final double nodeWidth;
  final double nodeHeight;
  final Color lineColor;
  final Color activeColor;
  final Color successColor;
  final Color errorColor;

  _WorkflowGraphPainter({
    required this.nodes,
    required this.edges,
    required this.stepRuns,
    required this.nodeWidth,
    required this.nodeHeight,
    required this.lineColor,
    required this.activeColor,
    required this.successColor,
    required this.errorColor,
  });

  double _n(dynamic value) => value is num ? value.toDouble() : double.tryParse('$value') ?? 0;

  @override
  void paint(Canvas canvas, Size size) {
    final grid = Paint()..color = lineColor.withValues(alpha: 0.28)..strokeWidth = 0.6;
    for (double x = 0; x < size.width; x += 32) {
      canvas.drawLine(Offset(x, 0), Offset(x, size.height), grid);
    }
    for (double y = 0; y < size.height; y += 32) {
      canvas.drawLine(Offset(0, y), Offset(size.width, y), grid);
    }
    final byId = <String, Map<String, dynamic>>{for (final node in nodes) (node['id'] ?? '').toString(): node};
    for (final edge in edges) {
      final sourceId = (edge['source'] ?? '').toString();
      final targetId = (edge['target'] ?? '').toString();
      final source = byId[sourceId];
      final target = byId[targetId];
      if (source == null || target == null) continue;
      final sp = source['position'] is Map ? (source['position'] as Map) : const <String, dynamic>{};
      final tp = target['position'] is Map ? (target['position'] as Map) : const <String, dynamic>{};
      final a = Offset(_n(sp['x']) + nodeWidth, _n(sp['y']) + nodeHeight / 2);
      final b = Offset(_n(tp['x']), _n(tp['y']) + nodeHeight / 2);
      final c = math.max(70.0, (b.dx - a.dx).abs() * 0.45);
      final path = Path()
        ..moveTo(a.dx, a.dy)
        ..cubicTo(a.dx + c, a.dy, b.dx - c, b.dy, b.dx, b.dy);
      var color = lineColor;
      final targetStatus = (stepRuns[targetId]?['status'] ?? '').toString().toLowerCase();
      if (targetStatus == 'running' || targetStatus == 'retry_wait') color = activeColor;
      if (targetStatus == 'succeeded' || targetStatus == 'completed') color = successColor;
      if (targetStatus == 'failed') color = errorColor;
      canvas.drawPath(path, Paint()..color = color..style = PaintingStyle.stroke..strokeWidth = targetStatus.isEmpty ? 1.5 : 2.4..strokeCap = StrokeCap.round);
      final tangent = Offset(b.dx - (b.dx - c), b.dy - b.dy);
      final angle = math.atan2(tangent.dy, tangent.dx);
      const arrow = 8.0;
      final p1 = Offset(b.dx - arrow * math.cos(angle - math.pi / 6), b.dy - arrow * math.sin(angle - math.pi / 6));
      final p2 = Offset(b.dx - arrow * math.cos(angle + math.pi / 6), b.dy - arrow * math.sin(angle + math.pi / 6));
      canvas.drawPath(Path()..moveTo(b.dx, b.dy)..lineTo(p1.dx, p1.dy)..moveTo(b.dx, b.dy)..lineTo(p2.dx, p2.dy), Paint()..color = color..style = PaintingStyle.stroke..strokeWidth = 1.5);
    }
  }

  @override
  bool shouldRepaint(covariant _WorkflowGraphPainter oldDelegate) => true;
}

class _NodeTraceCard extends StatelessWidget {
  final Map<String, dynamic>? stepRun;
  final List<Map<String, dynamic>> attempts;
  final bool checkpoint;
  const _NodeTraceCard({required this.stepRun, this.attempts = const <Map<String, dynamic>>[], this.checkpoint = false});

  @override
  Widget build(BuildContext context) {
    if (stepRun == null) {
      return Container(
        padding: const EdgeInsets.all(12),
        decoration: BoxDecoration(color: context.surfaceSecondary, borderRadius: AppRadius.brMedium),
        child: const Text('该节点暂无运行 Trace。'),
      );
    }
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(color: context.surfaceSecondary, borderRadius: AppRadius.brMedium),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(children: [const Text('Execution Trace', style: TextStyle(fontWeight: FontWeight.w600)), if (checkpoint) ...[const SizedBox(width: 6), const Chip(label: Text('checkpoint'), visualDensity: VisualDensity.compact)], const Spacer(), _RunStatusBadge(status: (stepRun!['status'] ?? '').toString())]),
          const SizedBox(height: 8),
          Text('最终 Attempt: ${stepRun!['attempt'] ?? 0}', style: AppTypography.caption(context)),
          if ((stepRun!['error'] ?? '').toString().isNotEmpty) Text('Error: ${stepRun!['error']}', style: TextStyle(color: context.error, fontSize: 12)),
          if (attempts.isNotEmpty) ...[
            const SizedBox(height: 8),
            const Text('Retry Attempts', style: TextStyle(fontWeight: FontWeight.w600, fontSize: 12)),
            const SizedBox(height: 4),
            ...attempts.map((item) => Padding(
                  padding: const EdgeInsets.only(bottom: 3),
                  child: Text(
                    'G${item['generation'] ?? 0} / #${item['attempt'] ?? 0} · ${item['status'] ?? ''}${(item['nextBackoffMs'] ?? 0) == 0 ? '' : ' · ${item['nextBackoffMs']}ms 后重试'}',
                    style: TextStyle(fontSize: 11, color: <String>{'failed', 'timed_out'}.contains((item['status'] ?? '').toString()) ? context.error : context.textSecondary),
                  ),
                )),
          ],
          if (stepRun!.containsKey('input')) SelectableText('Input: ${stepRun!['input']}', style: const TextStyle(fontFamily: 'monospace', fontSize: 11)),
          if (stepRun!.containsKey('output')) SelectableText('Output: ${stepRun!['output']}', style: const TextStyle(fontFamily: 'monospace', fontSize: 11)),
        ],
      ),
    );
  }
}

class _WorkflowStatTile extends StatelessWidget {
  final String label;
  final String value;
  const _WorkflowStatTile({required this.label, required this.value});
  @override
  Widget build(BuildContext context) => Container(
        padding: const EdgeInsets.symmetric(vertical: 8, horizontal: 6),
        decoration: BoxDecoration(color: context.surfaceSecondary, borderRadius: AppRadius.brSmall),
        child: Column(children: [Text(value, style: const TextStyle(fontWeight: FontWeight.w700, fontSize: 12)), Text(label, style: AppTypography.caption(context), maxLines: 1)]),
      );
}

class _RiskBadge extends StatelessWidget {
  final String level;
  const _RiskBadge({required this.level});
  @override
  Widget build(BuildContext context) {
    final normalized = level.toLowerCase();
    final color = normalized == 'high' || normalized == 'forbidden' || normalized == 'missing' ? context.error : normalized == 'medium' ? context.warning : context.success;
    return Container(padding: const EdgeInsets.symmetric(horizontal: 7, vertical: 3), decoration: BoxDecoration(color: color.withValues(alpha: 0.12), borderRadius: AppRadius.brSmall), child: Text(level, style: TextStyle(fontSize: 10, color: color, fontWeight: FontWeight.w600)));
  }
}

class _SafetySection extends StatelessWidget {
  final String title;
  final String emptyText;
  final List<String> items;
  const _SafetySection({required this.title, required this.emptyText, required this.items});
  @override
  Widget build(BuildContext context) => Column(crossAxisAlignment: CrossAxisAlignment.start, children: [Text(title, style: Theme.of(context).textTheme.titleSmall), const SizedBox(height: 6), if (items.isEmpty) Text(emptyText, style: AppTypography.caption(context)) else ...items.map((item) => Padding(padding: const EdgeInsets.only(bottom: 4), child: SelectableText(item, style: const TextStyle(fontFamily: 'monospace', fontSize: 11))))]);
}

class _RunStatusIcon extends StatelessWidget {
  final String status;
  const _RunStatusIcon({required this.status});

  @override
  Widget build(BuildContext context) {
    final s = status.toLowerCase();
    final icon = s == 'succeeded' || s == 'completed' ? Icons.check_circle_outline : s == 'failed' ? Icons.error_outline : s == 'cancelled' ? Icons.cancel_outlined : s == 'paused' ? Icons.pause_circle_outline : Icons.timelapse;
    final color = s == 'succeeded' || s == 'completed' ? context.success : s == 'failed' ? context.error : s == 'cancelled' ? context.warning : context.info;
    return Icon(icon, color: color);
  }
}

class _RunStatusBadge extends StatelessWidget {
  final String status;
  const _RunStatusBadge({required this.status});

  @override
  Widget build(BuildContext context) {
    final s = status.isEmpty ? 'unknown' : status;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(color: context.surfaceSecondary, borderRadius: BorderRadius.circular(999)),
      child: Text(s, style: const TextStyle(fontSize: 11)),
    );
  }
}

class _NodeType {
  final String type;
  final String label;
  final String description;
  final IconData icon;
  const _NodeType(this.type, this.label, this.description, this.icon);
}

const _nodeTypes = <_NodeType>[
  _NodeType('tool', 'Tool', '调用 Kernel Tool', Icons.build_outlined),
  _NodeType('mcp', 'MCP', '调用 MCP Server / Tool', Icons.hub_outlined),
  _NodeType('task', 'Task', '调用 Task Runtime', Icons.task_alt_outlined),
  _NodeType('javascript', 'JavaScript', '沙箱 JavaScript Runtime', Icons.code),
  _NodeType('wasm', 'WASM', 'WASM Runtime 节点', Icons.memory_outlined),
  _NodeType('trusted_service', 'Trusted Service', '可信服务处理器', Icons.verified_user_outlined),
  _NodeType('nested_workflow', 'Nested Workflow', '复用另一个工作流', Icons.account_tree_outlined),
  _NodeType('condition', 'Condition', '条件判断与分支', Icons.call_split_outlined),
  _NodeType('transform', 'Transform', '结构化数据转换', Icons.transform_outlined),
  _NodeType('wait', 'Wait', '暂停一段时间再继续', Icons.hourglass_bottom_outlined),
];

const _triggerTypes = <String>[
  'manual',
  'event',
  'cron',
  'interval',
  'one_shot',
];

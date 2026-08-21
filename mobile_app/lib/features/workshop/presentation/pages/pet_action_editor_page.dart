import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class PetActionEditorPage extends ConsumerStatefulWidget {
  final String taskId;
  final String actionKey;

  const PetActionEditorPage({super.key, required this.taskId, required this.actionKey});

  @override
  ConsumerState<PetActionEditorPage> createState() => _PetActionEditorPageState();
}

class _PetActionEditorPageState extends ConsumerState<PetActionEditorPage> {
  bool _loading = true;
  bool _busy = false;
  String? _error;
  Map<String, dynamic> _summary = const {};
  String? _sessionId;
  int _sessionVersion = 0;
  int _fps = 12;
  String _loopType = 'loop';

  @override
  void initState() {
    super.initState();
    _load();
  }

  String _key(String prefix) => '$prefix-${DateTime.now().microsecondsSinceEpoch}';

  Future<void> _load() async {
    setState(() { _loading = true; _error = null; });
    try {
      final api = ref.read(backendServiceProvider);
      final response = await api.get<dynamic>(
        '/api/desktop-pets/processing-tasks/${widget.taskId}/actions/${widget.actionKey}/edit-summary',
      );
      final summary = response is Map ? Map<String, dynamic>.from(response) : const <String, dynamic>{};
      if (summary.isEmpty) throw StateError('未找到动作编辑数据');
      if (!mounted) return;
      setState(() { _summary = summary; _loading = false; });
    } catch (e) {
      if (mounted) setState(() { _error = e.toString(); _loading = false; });
    }
  }

  Future<void> _ensureSession() async {
    if (_sessionId != null) return;
    final revisionId = (_summary['activeRevisionId'] ?? '').toString();
    if (revisionId.isEmpty) throw StateError('当前动作没有可编辑的活动修订');
    final response = await ref.read(backendServiceProvider).post<dynamic>(
      '/api/desktop-pets/processing-tasks/${widget.taskId}/actions/${widget.actionKey}/edit-sessions',
      data: {
        'baseRevisionId': revisionId,
        'clientInstanceId': 'mobile-app',
        'idempotencyKey': _key('session'),
      },
    );
    if (response is! Map) throw StateError('创建编辑会话失败');
    final map = Map<String, dynamic>.from(response);
    _sessionId = (map['sessionId'] ?? '').toString();
    _sessionVersion = (map['sessionVersion'] as num?)?.toInt() ?? 0;
    if (_sessionId!.isEmpty) throw StateError('后端未返回编辑会话 ID');
  }

  Future<void> _apply(String type, Map<String, dynamic> payload) async {
    setState(() => _busy = true);
    try {
      await _ensureSession();
      final response = await ref.read(backendServiceProvider).post<dynamic>(
        '/api/desktop-pets/edit-sessions/$_sessionId/operations',
        data: {
          'baseSessionVersion': _sessionVersion,
          'idempotencyKey': _key('op'),
          'operation': {'type': type, 'schemaVersion': 1, 'payload': payload},
        },
      );
      if (response is Map) _sessionVersion = (response['sessionVersion'] as num?)?.toInt() ?? _sessionVersion;
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('修改已写入编辑会话')));
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('修改失败：$e')));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _undo() => _historyAction('undo');
  Future<void> _redo() => _historyAction('redo');

  Future<void> _historyAction(String action) async {
    setState(() => _busy = true);
    try {
      await _ensureSession();
      final response = await ref.read(backendServiceProvider).post<dynamic>(
        '/api/desktop-pets/edit-sessions/$_sessionId/$action',
        queryParameters: {'baseSessionVersion': _sessionVersion},
      );
      if (response is Map) _sessionVersion = (response['sessionVersion'] as num?)?.toInt() ?? _sessionVersion;
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('${action == 'undo' ? '撤销' : '重做'}失败：$e')));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _commit() async {
    setState(() => _busy = true);
    try {
      await _ensureSession();
      await ref.read(backendServiceProvider).post<dynamic>(
        '/api/desktop-pets/edit-sessions/$_sessionId/commit',
        data: {
          'expectedSessionVersion': _sessionVersion,
          'changeSummary': 'Mobile action editor changes',
          'activationPolicy': 'activate',
          'idempotencyKey': _key('commit'),
        },
      );
      _sessionId = null;
      _sessionVersion = 0;
      await _load();
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('动作修订已提交')));
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('提交失败：$e')));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _abandon() async {
    final sessionId = _sessionId;
    if (sessionId == null) return;
    setState(() => _busy = true);
    try {
      await ref.read(backendServiceProvider).post<dynamic>('/api/desktop-pets/edit-sessions/$sessionId/abandon');
      _sessionId = null;
      _sessionVersion = 0;
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('未提交修改已放弃')));
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('放弃失败：$e')));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '动作编辑 · ${widget.actionKey}',
        showBackButton: true,
        fallbackRoute: AppRoutes.petProcessing(widget.taskId),
        actions: [
          AmitiaIconButton(icon: Icons.undo, onPressed: _busy ? null : _undo, color: context.textSecondary),
          AmitiaIconButton(icon: Icons.redo, onPressed: _busy ? null : _redo, color: context.textSecondary),
          AmitiaIconButton(icon: Icons.refresh, onPressed: _busy ? null : _load, color: context.textSecondary),
        ],
      ),
      body: SafeArea(top: false, child: _buildBody(context)),
    );
  }

  Widget _buildBody(BuildContext context) {
    if (_loading) return const AmitiaLoadingState();
    if (_error != null) return AmitiaErrorState(message: _error!, onRetry: _load);
    final timelineRaw = _summary['timeline'];
    final frames = timelineRaw is List ? timelineRaw.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList() : const <Map<String, dynamic>>[];
    return ListView(
      padding: EdgeInsets.all(AppSpacing.pagePadding),
      children: [
        AmitiaCard(
          child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
            Text(widget.actionKey, style: AppTypography.cardTitle(context)),
            SizedBox(height: AppSpacing.xs),
            Text('活动修订 ${_summary['activeRevisionNum'] ?? '-'} · ${_summary['frameCount'] ?? frames.length} 帧 · 质量 ${_summary['qualityVerdict'] ?? '-'}', style: AppTypography.caption(context)),
            if (_sessionId != null) ...[
              SizedBox(height: AppSpacing.xs),
              Text('编辑会话 $_sessionId · v$_sessionVersion', style: AppTypography.label(context).copyWith(color: context.accentPrimary)),
            ],
          ]),
        ),
        SizedBox(height: AppSpacing.lg),
        const AmitiaSectionHeader(title: '播放参数'),
        SizedBox(height: AppSpacing.sm),
        AmitiaCard(child: Column(children: [
          Row(children: [
            const Expanded(child: Text('默认 FPS')),
            DropdownButton<int>(
              value: _fps,
              items: const [6, 8, 10, 12, 15, 24].map((e) => DropdownMenuItem(value: e, child: Text('$e'))).toList(),
              onChanged: _busy ? null : (value) {
                if (value == null) return;
                setState(() => _fps = value);
                _apply('action.set_default_fps', {'defaultFps': value, 'recalculate': true});
              },
            ),
          ]),
          Row(children: [
            const Expanded(child: Text('循环类型')),
            DropdownButton<String>(
              value: _loopType,
              items: const ['loop', 'once', 'ping_pong'].map((e) => DropdownMenuItem(value: e, child: Text(e))).toList(),
              onChanged: _busy ? null : (value) {
                if (value == null) return;
                setState(() => _loopType = value);
                _apply('action.set_loop_type', {'loopType': value});
              },
            ),
          ]),
        ])),
        SizedBox(height: AppSpacing.lg),
        const AmitiaSectionHeader(title: '帧时间线'),
        SizedBox(height: AppSpacing.sm),
        if (frames.isEmpty)
          const SizedBox(height: 180, child: AmitiaEmptyState(icon: Icons.photo_library_outlined, title: '暂无帧'))
        else
          ...frames.map((frame) => _frameCard(context, frame)),
        SizedBox(height: AppSpacing.lg),
        Row(children: [
          Expanded(child: OutlinedButton(onPressed: _busy || _sessionId == null ? null : _abandon, child: const Text('放弃会话'))),
          SizedBox(width: AppSpacing.sm),
          Expanded(child: FilledButton(onPressed: _busy ? null : _commit, child: Text(_busy ? '处理中…' : '提交修订'))),
        ]),
        SizedBox(height: AppSpacing.xxl),
      ],
    );
  }

  Widget _frameCard(BuildContext context, Map<String, dynamic> frame) {
    final id = (frame['frameId'] ?? '').toString();
    final index = (frame['logicalIndex'] as num?)?.toInt() ?? 0;
    final duration = (frame['durationMs'] as num?)?.toInt() ?? 0;
    final issue = frame['hasQualityIssue'] == true;
    return Padding(
      padding: EdgeInsets.only(bottom: AppSpacing.xs),
      child: AmitiaCard(child: Row(children: [
        Icon(issue ? Icons.warning_amber_rounded : Icons.image_outlined, color: issue ? context.warning : context.textSecondary),
        SizedBox(width: AppSpacing.sm),
        Expanded(child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Text('帧 ${index + 1}', style: AppTypography.bodySmall(context)),
          Text('${frame['width'] ?? 0}×${frame['height'] ?? 0} · ${duration}ms', style: AppTypography.caption(context)),
        ])),
        PopupMenuButton<String>(
          enabled: !_busy,
          onSelected: (value) {
            if (value == 'duration') _editDuration(id, duration);
            if (value == 'delete') _apply('frame.delete', {'frameId': id});
            if (value == 'duplicate') _apply('frame.duplicate', {'frameId': id});
          },
          itemBuilder: (_) => const [
            PopupMenuItem(value: 'duration', child: Text('修改时长')),
            PopupMenuItem(value: 'duplicate', child: Text('复制帧')),
            PopupMenuItem(value: 'delete', child: Text('删除帧')),
          ],
        ),
      ])),
    );
  }

  Future<void> _editDuration(String frameId, int current) async {
    final controller = TextEditingController(text: current.toString());
    final value = await showDialog<int>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('修改帧时长'),
        content: TextField(controller: controller, keyboardType: TextInputType.number, decoration: const InputDecoration(suffixText: 'ms')),
        actions: [
          TextButton(onPressed: () => Navigator.pop(dialogContext), child: const Text('取消')),
          TextButton(onPressed: () => Navigator.pop(dialogContext, int.tryParse(controller.text)), child: const Text('保存')),
        ],
      ),
    );
    controller.dispose();
    if (value != null && value > 0) await _apply('frame.set_duration', {'frameId': frameId, 'durationMs': value});
  }
}

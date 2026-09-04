import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:file_picker/file_picker.dart';
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

  const PetActionEditorPage({
    super.key,
    required this.taskId,
    required this.actionKey,
  });

  @override
  ConsumerState<PetActionEditorPage> createState() =>
      _PetActionEditorPageState();
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

  String _key(String prefix) =>
      '$prefix-${DateTime.now().microsecondsSinceEpoch}';

  List<Map<String, dynamic>> _mapList(dynamic raw) {
    if (raw is! List) return const <Map<String, dynamic>>[];
    return raw
        .whereType<Map>()
        .map((item) => Map<String, dynamic>.from(item))
        .toList();
  }

  String _pretty(dynamic value) {
    try {
      return const JsonEncoder.withIndent('  ').convert(value);
    } catch (_) {
      return value?.toString() ?? '';
    }
  }

  Future<void> _load() async {
    if (mounted) {
      setState(() {
        _loading = true;
        _error = null;
      });
    }
    try {
      final api = ref.read(backendServiceProvider);
      final response = await api.get<dynamic>(
        '/api/desktop-pets/processing-tasks/${widget.taskId}/actions/${widget.actionKey}/edit-summary',
      );
      final summary = response is Map
          ? Map<String, dynamic>.from(response)
          : const <String, dynamic>{};
      if (summary.isEmpty) throw StateError('未找到动作编辑数据');
      final activeRevisionId = (summary['activeRevisionId'] ?? '').toString();
      if (activeRevisionId.isNotEmpty) {
        final detail = await api.get<dynamic>(
          '/api/desktop-pets/processing-tasks/${widget.taskId}/actions/${widget.actionKey}/active-revision',
        );
        if (detail is Map) {
          final revision = detail['revision'];
          if (revision is Map) {
            _fps = (revision['defaultFps'] as num?)?.toInt() ?? _fps;
            _loopType = (revision['loopType'] ?? _loopType).toString();
          }
        }
      }
      if (!mounted) return;
      setState(() {
        _summary = summary;
        _loading = false;
      });
    } catch (e) {
      if (mounted) {
        setState(() {
          _error = e.toString();
          _loading = false;
        });
      }
    }
  }

  Future<void> _ensureSession() async {
    if (_sessionId != null) return;
    final revisionId = (_summary['activeRevisionId'] ?? '').toString();
    if (revisionId.isEmpty) {
      throw StateError('当前动作没有可编辑的活动修订');
    }
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
    if (mounted) setState(() {});
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
          'operation': {
            'type': type,
            'schemaVersion': 1,
            'payload': payload,
          },
        },
      );
      if (response is Map) {
        _sessionVersion =
            (response['sessionVersion'] as num?)?.toInt() ?? _sessionVersion;
      }
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('修改已写入编辑会话')),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('修改失败：$e')),
        );
      }
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
      if (response is Map) {
        _sessionVersion =
            (response['sessionVersion'] as num?)?.toInt() ?? _sessionVersion;
      }
      if (mounted) setState(() {});
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('${action == 'undo' ? '撤销' : '重做'}失败：$e'),
          ),
        );
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _checkpoint() async {
    setState(() => _busy = true);
    try {
      await _ensureSession();
      await ref.read(backendServiceProvider).post<dynamic>(
        '/api/desktop-pets/edit-sessions/$_sessionId/checkpoints',
      );
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('检查点已创建')),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('创建检查点失败：$e')),
        );
      }
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
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('动作修订已提交')),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('提交失败：$e')),
        );
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _abandon() async {
    final sessionId = _sessionId;
    if (sessionId == null) return;
    setState(() => _busy = true);
    try {
      await ref.read(backendServiceProvider).post<dynamic>(
        '/api/desktop-pets/edit-sessions/$sessionId/abandon',
      );
      _sessionId = null;
      _sessionVersion = 0;
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('未提交修改已放弃')),
        );
        setState(() {});
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('放弃失败：$e')),
        );
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _showAdvancedTools() async {
    if (!mounted) return;
    await showModalBottomSheet<void>(
      context: context,
      showDragHandle: true,
      isScrollControlled: true,
      builder: (sheetContext) => SafeArea(
        child: SingleChildScrollView(
          child: Padding(
            padding: EdgeInsets.only(
              left: AppSpacing.md,
              right: AppSpacing.md,
              bottom: AppSpacing.lg,
            ),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                ListTile(
                  leading: const Icon(Icons.history),
                  title: const Text('Revision 历史与切换'),
                  subtitle: const Text('查看所有修订、详情、Preview Manifest，并切换活动修订'),
                  onTap: () {
                    Navigator.pop(sheetContext);
                    _showRevisions();
                  },
                ),
                ListTile(
                  leading: const Icon(Icons.fact_check_outlined),
                  title: const Text('质量评估'),
                  subtitle: const Text('触发并查看当前 Revision 的最新质量评估'),
                  onTap: () {
                    Navigator.pop(sheetContext);
                    _showQuality();
                  },
                ),
                ListTile(
                  leading: const Icon(Icons.bookmark_add_outlined),
                  title: const Text('创建编辑检查点'),
                  subtitle: const Text('保存当前编辑会话 Manifest 状态'),
                  onTap: () {
                    Navigator.pop(sheetContext);
                    _checkpoint();
                  },
                ),
                ListTile(
                  leading: const Icon(Icons.auto_fix_high),
                  title: const Text('再生成任务与候选'),
                  subtitle: const Text('管理 Regeneration Job，接受/拒绝候选帧或上传候选'),
                  onTap: () {
                    Navigator.pop(sheetContext);
                    _showRegenerationManager();
                  },
                ),
                ListTile(
                  leading: const Icon(Icons.event_note_outlined),
                  title: const Text('编辑会话事件'),
                  subtitle: const Text('查看当前 Session 的操作/再生成事件流'),
                  onTap: () {
                    Navigator.pop(sheetContext);
                    _showSessionEvents();
                  },
                ),
                ListTile(
                  leading: const Icon(Icons.control_camera_outlined),
                  title: const Text('批量锚点调整'),
                  subtitle: const Text('对全部帧进行 anchor offset 或重置'),
                  onTap: () {
                    Navigator.pop(sheetContext);
                    _batchAnchorDialog();
                  },
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Future<void> _showRevisions() async {
    try {
      final api = ref.read(backendServiceProvider);
      final raw = await api.get<dynamic>(
        '/api/desktop-pets/processing-tasks/${widget.taskId}/actions/${widget.actionKey}/revisions',
      );
      final revisions = _mapList(raw);
      if (!mounted) return;
      await showDialog<void>(
        context: context,
        builder: (dialogContext) => AlertDialog(
          title: const Text('Revision 历史'),
          content: SizedBox(
            width: 620,
            child: revisions.isEmpty
                ? const Text('暂无 Revision')
                : ListView.separated(
                    shrinkWrap: true,
                    itemCount: revisions.length,
                    separatorBuilder: (_, __) => const Divider(height: 1),
                    itemBuilder: (_, index) {
                      final revision = revisions[index];
                      final id = (revision['id'] ?? '').toString();
                      final active = revision['isActive'] == true ||
                          id == (_summary['activeRevisionId'] ?? '').toString();
                      return ListTile(
                        leading: Icon(
                          active ? Icons.check_circle : Icons.history,
                          color: active ? context.accentPrimary : null,
                        ),
                        title: Text(
                          'Revision #${revision['revisionNumber'] ?? '-'} · ${revision['revisionType'] ?? '-'}',
                        ),
                        subtitle: Text(
                          '${revision['frameCount'] ?? 0} 帧 · ${revision['qualityVerdict'] ?? '-'}\n${revision['changeSummary'] ?? ''}',
                        ),
                        isThreeLine: true,
                        trailing: PopupMenuButton<String>(
                          onSelected: (value) async {
                            if (value == 'detail') {
                              await _showRevisionDetail(id);
                            } else if (value == 'manifest') {
                              await _showPreviewManifest(id);
                            } else if (value == 'activate') {
                              Navigator.pop(dialogContext);
                              await _activateRevision(id);
                            }
                          },
                          itemBuilder: (_) => [
                            const PopupMenuItem(
                              value: 'detail',
                              child: Text('查看详情'),
                            ),
                            const PopupMenuItem(
                              value: 'manifest',
                              child: Text('Preview Manifest'),
                            ),
                            if (!active)
                              const PopupMenuItem(
                                value: 'activate',
                                child: Text('设为活动 Revision'),
                              ),
                          ],
                        ),
                      );
                    },
                  ),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialogContext),
              child: const Text('关闭'),
            ),
          ],
        ),
      );
    } catch (e) {
      _snack('加载 Revision 失败：$e');
    }
  }

  Future<void> _showRevisionDetail(String revisionId) async {
    try {
      final detail = await ref.read(backendServiceProvider).get<dynamic>(
        '/api/desktop-pets/processing-tasks/${widget.taskId}/actions/${widget.actionKey}/revisions/$revisionId',
      );
      await _showJsonDialog('Revision 详情', detail);
    } catch (e) {
      _snack('加载 Revision 详情失败：$e');
    }
  }

  Future<void> _showPreviewManifest(String revisionId) async {
    try {
      final manifest = await ref.read(backendServiceProvider).get<dynamic>(
        '/api/desktop-pets/revisions/$revisionId/preview-manifest',
      );
      await _showJsonDialog('Preview Manifest', manifest);
    } catch (e) {
      _snack('加载 Preview Manifest 失败：$e');
    }
  }

  Future<void> _activateRevision(String revisionId) async {
    setState(() => _busy = true);
    try {
      final expectedBindingVersion =
          (_summary['bindingVersion'] as num?)?.toInt() ?? 0;
      await ref.read(backendServiceProvider).post<dynamic>(
        '/api/desktop-pets/processing-tasks/${widget.taskId}/actions/${widget.actionKey}/active-revision',
        data: {
          'revisionId': revisionId,
          'expectedBindingVersion': expectedBindingVersion,
          'reason': 'mobile_manual_switch',
          'idempotencyKey': _key('activate'),
        },
      );
      _sessionId = null;
      _sessionVersion = 0;
      await _load();
      _snack('活动 Revision 已切换');
    } catch (e) {
      _snack('切换 Revision 失败：$e');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _showQuality() async {
    final revisionId = (_summary['activeRevisionId'] ?? '').toString();
    if (revisionId.isEmpty) {
      _snack('当前没有活动 Revision');
      return;
    }
    try {
      final api = ref.read(backendServiceProvider);
      dynamic latest;
      try {
        latest = await api.get<dynamic>(
          '/api/desktop-pets/revisions/$revisionId/quality-evaluations/latest',
        );
      } catch (_) {
        latest = null;
      }
      if (!mounted) return;
      await showDialog<void>(
        context: context,
        builder: (dialogContext) => AlertDialog(
          title: const Text('质量评估'),
          content: SizedBox(
            width: 560,
            child: SingleChildScrollView(
              child: SelectableText(
                latest == null ? '当前 Revision 暂无质量评估。' : _pretty(latest),
              ),
            ),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialogContext),
              child: const Text('关闭'),
            ),
            FilledButton(
              onPressed: () async {
                Navigator.pop(dialogContext);
                await _triggerQuality(revisionId);
              },
              child: const Text('重新评估'),
            ),
          ],
        ),
      );
    } catch (e) {
      _snack('加载质量评估失败：$e');
    }
  }

  Future<void> _triggerQuality(String revisionId) async {
    setState(() => _busy = true);
    try {
      final response = await ref.read(backendServiceProvider).post<dynamic>(
        '/api/desktop-pets/revisions/$revisionId/quality-evaluations',
      );
      _snack('质量评估已触发：${response is Map ? response['jobId'] ?? '已提交' : '已提交'}');
      await _load();
    } catch (e) {
      _snack('触发质量评估失败：$e');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _showSessionEvents() async {
    try {
      await _ensureSession();
      final response = await ref.read(backendServiceProvider).get<dynamic>(
        '/api/desktop-pets/edit-sessions/$_sessionId/events',
      );
      await _showJsonDialog('Session Events', response);
    } catch (e) {
      _snack('加载 Session Events 失败：$e');
    }
  }

  Future<void> _showRegenerationManager() async {
    try {
      await _ensureSession();
      final api = ref.read(backendServiceProvider);
      final rawJobs = await api.get<dynamic>(
        '/api/desktop-pets/regeneration-jobs',
        queryParameters: {'limit': 100, 'offset': 0},
      );
      final jobs = _mapList(rawJobs)
          .where((job) => (job['sessionId'] ?? '').toString() == _sessionId)
          .toList();
      final candidates = _mapList(
        await api.get<dynamic>(
          '/api/desktop-pets/edit-sessions/$_sessionId/candidates',
        ),
      );
      if (!mounted) return;
      await showDialog<void>(
        context: context,
        builder: (dialogContext) => AlertDialog(
          title: const Text('再生成任务与候选'),
          content: SizedBox(
            width: 680,
            height: 520,
            child: ListView(
              children: [
                Text('Regeneration Jobs', style: AppTypography.cardTitle(context)),
                if (jobs.isEmpty)
                  const Padding(
                    padding: EdgeInsets.symmetric(vertical: 12),
                    child: Text('当前会话暂无再生成任务'),
                  ),
                ...jobs.map(
                  (job) => ListTile(
                    dense: true,
                    title: Text(
                      '${job['jobType'] ?? '-'} · ${job['status'] ?? '-'}',
                    ),
                    subtitle: Text(
                      '目标帧 ${job['targetFrameId'] ?? '-'}\n${job['errorMessage'] ?? ''}',
                    ),
                    trailing: _canCancelJob((job['status'] ?? '').toString())
                        ? TextButton(
                            onPressed: () async {
                              await _cancelRegenerationJob(
                                (job['id'] ?? '').toString(),
                              );
                              if (dialogContext.mounted) {
                                Navigator.pop(dialogContext);
                              }
                              await _showRegenerationManager();
                            },
                            child: const Text('取消'),
                          )
                        : null,
                  ),
                ),
                const Divider(),
                Row(
                  children: [
                    Expanded(
                      child: Text(
                        'Candidates',
                        style: AppTypography.cardTitle(context),
                      ),
                    ),
                    TextButton.icon(
                      onPressed: () async {
                        Navigator.pop(dialogContext);
                        await _uploadCandidate();
                        await _showRegenerationManager();
                      },
                      icon: const Icon(Icons.upload_file),
                      label: const Text('上传候选'),
                    ),
                  ],
                ),
                if (candidates.isEmpty)
                  const Padding(
                    padding: EdgeInsets.symmetric(vertical: 12),
                    child: Text('当前会话暂无候选'),
                  ),
                ...candidates.map(
                  (candidate) => ListTile(
                    dense: true,
                    title: Text(
                      '${candidate['candidateType'] ?? 'candidate'} · ${candidate['status'] ?? '-'}',
                    ),
                    subtitle: Text(
                      '帧 ${candidate['targetFrameId'] ?? '-'} · 质量 ${candidate['qualityStatus'] ?? '-'}',
                    ),
                    trailing: (candidate['status'] ?? '').toString() == 'pending'
                        ? Wrap(
                            spacing: 4,
                            children: [
                              TextButton(
                                onPressed: () async {
                                  await _decideCandidate(
                                    (candidate['id'] ?? '').toString(),
                                    true,
                                  );
                                  if (dialogContext.mounted) {
                                    Navigator.pop(dialogContext);
                                  }
                                  await _showRegenerationManager();
                                },
                                child: const Text('接受'),
                              ),
                              TextButton(
                                onPressed: () async {
                                  await _decideCandidate(
                                    (candidate['id'] ?? '').toString(),
                                    false,
                                  );
                                  if (dialogContext.mounted) {
                                    Navigator.pop(dialogContext);
                                  }
                                  await _showRegenerationManager();
                                },
                                child: const Text('拒绝'),
                              ),
                            ],
                          )
                        : null,
                  ),
                ),
              ],
            ),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialogContext),
              child: const Text('关闭'),
            ),
          ],
        ),
      );
    } catch (e) {
      _snack('加载再生成管理失败：$e');
    }
  }

  bool _canCancelJob(String status) {
    return status != 'completed' &&
        status != 'failed' &&
        status != 'cancelled';
  }

  Future<void> _createRegenerationJob(String frameId) async {
    final intentController = TextEditingController();
    var jobType = 'single_frame';
    var adjacent = true;
    final approved = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (context, setDialogState) => AlertDialog(
          title: const Text('创建再生成任务'),
          content: SizedBox(
            width: 520,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                DropdownButtonFormField<String>(
                  value: jobType,
                  decoration: const InputDecoration(labelText: '任务类型'),
                  items: const [
                    DropdownMenuItem(value: 'single_frame', child: Text('单帧再生成')),
                    DropdownMenuItem(value: 'background_reprocess', child: Text('背景重处理')),
                    DropdownMenuItem(value: 'full_action', child: Text('整动作再生成')),
                  ],
                  onChanged: (value) {
                    if (value != null) setDialogState(() => jobType = value);
                  },
                ),
                TextField(
                  controller: intentController,
                  decoration: const InputDecoration(
                    labelText: '修复意图',
                    hintText: '例如：修复手部、保持上一帧姿态',
                  ),
                ),
                SwitchListTile(
                  value: adjacent,
                  title: const Text('参考相邻帧'),
                  onChanged: (value) =>
                      setDialogState(() => adjacent = value),
                ),
              ],
            ),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialogContext, false),
              child: const Text('取消'),
            ),
            FilledButton(
              onPressed: () => Navigator.pop(dialogContext, true),
              child: const Text('创建'),
            ),
          ],
        ),
      ),
    );
    final intent = intentController.text.trim();
    intentController.dispose();
    if (approved != true) return;
    setState(() => _busy = true);
    try {
      await _ensureSession();
      final response = await ref.read(backendServiceProvider).post<dynamic>(
        '/api/desktop-pets/edit-sessions/$_sessionId/regeneration-jobs',
        data: {
          'targetFrameId': frameId,
          'jobType': jobType,
          'idempotencyKey': _key('regen'),
          'costConfirmationToken': '',
          'fixIntent': intent,
          'useAdjacentFrames': adjacent,
        },
      );
      _snack(
        '再生成任务已创建：${response is Map ? response['jobId'] ?? '已提交' : '已提交'}',
      );
    } catch (e) {
      _snack('创建再生成任务失败：$e');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _cancelRegenerationJob(String jobId) async {
    if (jobId.isEmpty || _sessionId == null) return;
    try {
      await ref.read(backendServiceProvider).post<dynamic>(
        '/api/desktop-pets/edit-sessions/$_sessionId/regeneration-jobs/$jobId/cancel',
      );
      _snack('再生成任务已取消');
    } catch (e) {
      _snack('取消再生成任务失败：$e');
    }
  }

  Future<void> _decideCandidate(String candidateId, bool accept) async {
    if (candidateId.isEmpty || _sessionId == null) return;
    try {
      await ref.read(backendServiceProvider).post<dynamic>(
        '/api/desktop-pets/edit-sessions/$_sessionId/candidates/$candidateId/${accept ? 'accept' : 'reject'}',
        data: {'idempotencyKey': _key(accept ? 'accept' : 'reject')},
      );
      _snack(accept ? '候选已接受并应用' : '候选已拒绝');
    } catch (e) {
      _snack('${accept ? '接受' : '拒绝'}候选失败：$e');
    }
  }

  Future<void> _uploadCandidate() async {
    try {
      await _ensureSession();
      final result = await FilePicker.platform.pickFiles(
        type: FileType.custom,
        allowedExtensions: const ['png', 'jpg', 'jpeg', 'webp'],
        withData: false,
      );
      final file = result?.files.single;
      if (file?.path == null) return;
      final frames = _mapList(_summary['timeline']);
      if (frames.isEmpty) throw StateError('当前动作没有目标帧');
      final target = await _chooseFrame(frames, '选择候选要替换的目标帧');
      if (target == null) return;
      final frameId = (target['frameId'] ?? '').toString();
      final form = FormData.fromMap({
        'targetFrameId': frameId,
        'file': await MultipartFile.fromFile(file!.path!, filename: file.name),
      });
      await ref.read(backendServiceProvider).post<dynamic>(
        '/api/desktop-pets/edit-sessions/$_sessionId/upload-candidates',
        data: form,
      );
      _snack('候选图片已上传');
    } catch (e) {
      _snack('上传候选失败：$e');
    }
  }

  Future<Map<String, dynamic>?> _chooseFrame(
    List<Map<String, dynamic>> frames,
    String title,
  ) async {
    if (!mounted) return null;
    return showDialog<Map<String, dynamic>>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: Text(title),
        content: SizedBox(
          width: 440,
          height: 420,
          child: ListView.builder(
            itemCount: frames.length,
            itemBuilder: (_, index) {
              final frame = frames[index];
              return ListTile(
                leading: const Icon(Icons.image_outlined),
                title: Text('帧 ${((frame['logicalIndex'] as num?)?.toInt() ?? index) + 1}'),
                subtitle: Text((frame['frameId'] ?? '').toString()),
                onTap: () => Navigator.pop(dialogContext, frame),
              );
            },
          ),
        ),
      ),
    );
  }

  Future<void> _editAnchor(Map<String, dynamic> frame) async {
    final frameId = (frame['frameId'] ?? '').toString();
    if (frameId.isEmpty) return;
    final xController = TextEditingController(
      text: ((frame['anchorX'] as num?)?.toDouble() ?? 0.5).toStringAsFixed(3),
    );
    final yController = TextEditingController(
      text: ((frame['anchorY'] as num?)?.toDouble() ?? 0.5).toStringAsFixed(3),
    );
    final save = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('设置帧锚点'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            TextField(
              controller: xController,
              keyboardType: const TextInputType.numberWithOptions(decimal: true),
              decoration: const InputDecoration(labelText: 'Anchor X'),
            ),
            TextField(
              controller: yController,
              keyboardType: const TextInputType.numberWithOptions(decimal: true),
              decoration: const InputDecoration(labelText: 'Anchor Y'),
            ),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext, false),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(dialogContext, true),
            child: const Text('保存'),
          ),
        ],
      ),
    );
    final x = double.tryParse(xController.text);
    final y = double.tryParse(yController.text);
    xController.dispose();
    yController.dispose();
    if (save != true || x == null || y == null) return;
    setState(() => _busy = true);
    try {
      await _ensureSession();
      await ref.read(backendServiceProvider).post<dynamic>(
        '/api/desktop-pets/edit-sessions/$_sessionId/frames/$frameId/anchor',
        data: {
          'frameId': frameId,
          'anchorX': x,
          'anchorY': y,
          'space': 'normalized',
        },
      );
      _snack('锚点已更新');
    } catch (e) {
      _snack('更新锚点失败：$e');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _batchAnchorDialog() async {
    final dxController = TextEditingController(text: '0');
    final dyController = TextEditingController(text: '0');
    final action = await showDialog<String>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('批量锚点调整'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            TextField(
              controller: dxController,
              keyboardType: const TextInputType.numberWithOptions(decimal: true, signed: true),
              decoration: const InputDecoration(labelText: 'Delta X'),
            ),
            TextField(
              controller: dyController,
              keyboardType: const TextInputType.numberWithOptions(decimal: true, signed: true),
              decoration: const InputDecoration(labelText: 'Delta Y'),
            ),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext),
            child: const Text('取消'),
          ),
          TextButton(
            onPressed: () => Navigator.pop(dialogContext, 'reset'),
            child: const Text('全部重置'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(dialogContext, 'offset'),
            child: const Text('应用偏移'),
          ),
        ],
      ),
    );
    final dx = double.tryParse(dxController.text) ?? 0;
    final dy = double.tryParse(dyController.text) ?? 0;
    dxController.dispose();
    dyController.dispose();
    if (action == null) return;
    final frameIds = _mapList(_summary['timeline'])
        .map((frame) => (frame['frameId'] ?? '').toString())
        .where((id) => id.isNotEmpty)
        .toList();
    setState(() => _busy = true);
    try {
      await _ensureSession();
      final api = ref.read(backendServiceProvider);
      if (action == 'reset') {
        await api.post<dynamic>(
          '/api/desktop-pets/edit-sessions/$_sessionId/anchors/reset',
          data: {'frameIds': frameIds},
        );
      } else {
        await api.post<dynamic>(
          '/api/desktop-pets/edit-sessions/$_sessionId/anchors/batch-offset',
          data: {'frameIds': frameIds, 'deltaX': dx, 'deltaY': dy},
        );
      }
      _snack(action == 'reset' ? '锚点已全部重置' : '锚点批量偏移已应用');
    } catch (e) {
      _snack('批量锚点操作失败：$e');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _backgroundPatchDialog(Map<String, dynamic> frame) async {
    final frameId = (frame['frameId'] ?? '').toString();
    if (frameId.isEmpty) return;
    final base64Controller = TextEditingController();
    final sizeController = TextEditingController(text: '24');
    final hardnessController = TextEditingController(text: '0.8');
    final opacityController = TextEditingController(text: '1.0');
    final widthController = TextEditingController(
      text: (frame['width'] ?? 0).toString(),
    );
    final heightController = TextEditingController(
      text: (frame['height'] ?? 0).toString(),
    );
    final action = await showDialog<String>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('背景修补'),
        content: SizedBox(
          width: 560,
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                const Text('可直接提交 brush mask 的 Base64 数据；重置操作不需要填写。'),
                TextField(
                  controller: base64Controller,
                  minLines: 3,
                  maxLines: 6,
                  decoration: const InputDecoration(labelText: 'Brush Data Base64'),
                ),
                TextField(controller: sizeController, decoration: const InputDecoration(labelText: 'Brush Size')),
                TextField(controller: hardnessController, decoration: const InputDecoration(labelText: 'Hardness 0-1')),
                TextField(controller: opacityController, decoration: const InputDecoration(labelText: 'Opacity 0-1')),
                Row(
                  children: [
                    Expanded(child: TextField(controller: widthController, decoration: const InputDecoration(labelText: 'Canvas Width'))),
                    const SizedBox(width: 8),
                    Expanded(child: TextField(controller: heightController, decoration: const InputDecoration(labelText: 'Canvas Height'))),
                  ],
                ),
              ],
            ),
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext),
            child: const Text('取消'),
          ),
          TextButton(
            onPressed: () => Navigator.pop(dialogContext, 'reset'),
            child: const Text('重置补丁'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(dialogContext, 'apply'),
            child: const Text('应用补丁'),
          ),
        ],
      ),
    );
    final brushBase64 = base64Controller.text.trim();
    final brushSize = int.tryParse(sizeController.text) ?? 24;
    final hardness = double.tryParse(hardnessController.text) ?? 0.8;
    final opacity = double.tryParse(opacityController.text) ?? 1.0;
    final width = int.tryParse(widthController.text) ?? 0;
    final height = int.tryParse(heightController.text) ?? 0;
    base64Controller.dispose();
    sizeController.dispose();
    hardnessController.dispose();
    opacityController.dispose();
    widthController.dispose();
    heightController.dispose();
    if (action == null) return;
    setState(() => _busy = true);
    try {
      await _ensureSession();
      final api = ref.read(backendServiceProvider);
      final path =
          '/api/desktop-pets/edit-sessions/$_sessionId/frames/$frameId/background-patches';
      if (action == 'reset') {
        await api.delete(path);
      } else {
        if (brushBase64.isEmpty) throw StateError('Brush Data Base64 不能为空');
        await api.post<dynamic>(
          path,
          data: {
            'frameId': frameId,
            'patchType': 'brush_mask',
            'brushDataBase64': brushBase64,
            'brushSize': brushSize,
            'brushHardness': hardness,
            'brushOpacity': opacity,
            'canvasWidth': width,
            'canvasHeight': height,
          },
        );
      }
      _snack(action == 'reset' ? '背景补丁已重置' : '背景补丁已应用');
    } catch (e) {
      _snack('背景补丁操作失败：$e');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _showJsonDialog(String title, dynamic value) async {
    if (!mounted) return;
    await showDialog<void>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: Text(title),
        content: SizedBox(
          width: 680,
          height: 480,
          child: SingleChildScrollView(
            child: SelectableText(_pretty(value)),
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext),
            child: const Text('关闭'),
          ),
        ],
      ),
    );
  }

  void _snack(String message) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(message)));
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '动作编辑 · ${widget.actionKey}',
        showBackButton: true,
        fallbackRoute: AppRoutes.petProcessing(widget.taskId),
        actions: [
          AmitiaIconButton(
            icon: Icons.undo,
            onPressed: _busy ? null : _undo,
            color: context.textSecondary,
          ),
          AmitiaIconButton(
            icon: Icons.redo,
            onPressed: _busy ? null : _redo,
            color: context.textSecondary,
          ),
          AmitiaIconButton(
            icon: Icons.tune,
            onPressed: _busy ? null : _showAdvancedTools,
            color: context.textSecondary,
          ),
          AmitiaIconButton(
            icon: Icons.refresh,
            onPressed: _busy ? null : _load,
            color: context.textSecondary,
          ),
        ],
      ),
      body: SafeArea(top: false, child: _buildBody(context)),
    );
  }

  Widget _buildBody(BuildContext context) {
    if (_loading) return const AmitiaLoadingState();
    if (_error != null) return AmitiaErrorState(message: _error!, onRetry: _load);
    final frames = _mapList(_summary['timeline']);
    return ListView(
      padding: EdgeInsets.all(AppSpacing.pagePadding),
      children: [
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(widget.actionKey, style: AppTypography.cardTitle(context)),
              SizedBox(height: AppSpacing.xs),
              Text(
                '活动修订 ${_summary['activeRevisionNum'] ?? '-'} · binding v${_summary['bindingVersion'] ?? '-'} · ${_summary['frameCount'] ?? frames.length} 帧 · 质量 ${_summary['qualityVerdict'] ?? '-'}',
                style: AppTypography.caption(context),
              ),
              if (_sessionId != null) ...[
                SizedBox(height: AppSpacing.xs),
                Text(
                  '编辑会话 $_sessionId · v$_sessionVersion',
                  style: AppTypography.label(context).copyWith(
                    color: context.accentPrimary,
                  ),
                ),
              ],
            ],
          ),
        ),
        SizedBox(height: AppSpacing.lg),
        const AmitiaSectionHeader(title: '播放参数'),
        SizedBox(height: AppSpacing.sm),
        AmitiaCard(
          child: Column(
            children: [
              Row(
                children: [
                  const Expanded(child: Text('默认 FPS')),
                  DropdownButton<int>(
                    value: _fps,
                    items: const [6, 8, 10, 12, 15, 24]
                        .map(
                          (e) => DropdownMenuItem(
                            value: e,
                            child: Text('$e'),
                          ),
                        )
                        .toList(),
                    onChanged: _busy
                        ? null
                        : (value) {
                            if (value == null) return;
                            setState(() => _fps = value);
                            _apply('action.set_default_fps', {
                              'defaultFps': value,
                              'recalculate': true,
                            });
                          },
                  ),
                ],
              ),
              Row(
                children: [
                  const Expanded(child: Text('循环类型')),
                  DropdownButton<String>(
                    value: _loopType,
                    items: const ['loop', 'once', 'ping_pong']
                        .map(
                          (e) => DropdownMenuItem(
                            value: e,
                            child: Text(e),
                          ),
                        )
                        .toList(),
                    onChanged: _busy
                        ? null
                        : (value) {
                            if (value == null) return;
                            setState(() => _loopType = value);
                            _apply('action.set_loop_type', {'loopType': value});
                          },
                  ),
                ],
              ),
            ],
          ),
        ),
        SizedBox(height: AppSpacing.lg),
        const AmitiaSectionHeader(title: '帧时间线'),
        SizedBox(height: AppSpacing.sm),
        if (frames.isEmpty)
          const SizedBox(
            height: 180,
            child: AmitiaEmptyState(
              icon: Icons.photo_library_outlined,
              title: '暂无帧',
            ),
          )
        else
          ...frames.map((frame) => _frameCard(context, frame)),
        SizedBox(height: AppSpacing.lg),
        Row(
          children: [
            Expanded(
              child: OutlinedButton(
                onPressed: _busy || _sessionId == null ? null : _abandon,
                child: const Text('放弃会话'),
              ),
            ),
            SizedBox(width: AppSpacing.sm),
            Expanded(
              child: FilledButton(
                onPressed: _busy ? null : _commit,
                child: Text(_busy ? '处理中…' : '提交修订'),
              ),
            ),
          ],
        ),
        SizedBox(height: AppSpacing.xxl),
      ],
    );
  }

  Widget _frameCard(BuildContext context, Map<String, dynamic> frame) {
    final id = (frame['frameId'] ?? '').toString();
    final index = (frame['logicalIndex'] as num?)?.toInt() ?? 0;
    final duration = (frame['durationMs'] as num?)?.toInt() ?? 0;
    final issue = frame['hasQualityIssue'] == true;
    final anchorX = (frame['anchorX'] as num?)?.toDouble() ?? 0.5;
    final anchorY = (frame['anchorY'] as num?)?.toDouble() ?? 0.5;
    return Padding(
      padding: EdgeInsets.only(bottom: AppSpacing.xs),
      child: AmitiaCard(
        child: Row(
          children: [
            Icon(
              issue ? Icons.warning_amber_rounded : Icons.image_outlined,
              color: issue ? context.warning : context.textSecondary,
            ),
            SizedBox(width: AppSpacing.sm),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('帧 ${index + 1}', style: AppTypography.bodySmall(context)),
                  Text(
                    '${frame['width'] ?? 0}×${frame['height'] ?? 0} · ${duration}ms · anchor ${anchorX.toStringAsFixed(2)},${anchorY.toStringAsFixed(2)}',
                    style: AppTypography.caption(context),
                  ),
                ],
              ),
            ),
            PopupMenuButton<String>(
              enabled: !_busy,
              onSelected: (value) {
                if (value == 'duration') _editDuration(id, duration);
                if (value == 'delete') _apply('frame.delete', {'frameId': id});
                if (value == 'duplicate') _apply('frame.duplicate', {'frameId': id});
                if (value == 'anchor') _editAnchor(frame);
                if (value == 'background') _backgroundPatchDialog(frame);
                if (value == 'regenerate') _createRegenerationJob(id);
              },
              itemBuilder: (_) => const [
                PopupMenuItem(value: 'duration', child: Text('修改时长')),
                PopupMenuItem(value: 'duplicate', child: Text('复制帧')),
                PopupMenuItem(value: 'anchor', child: Text('设置锚点')),
                PopupMenuItem(value: 'background', child: Text('背景修补')),
                PopupMenuItem(value: 'regenerate', child: Text('再生成此帧')),
                PopupMenuItem(value: 'delete', child: Text('删除帧')),
              ],
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _editDuration(String frameId, int current) async {
    final controller = TextEditingController(text: current.toString());
    final value = await showDialog<int>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('修改帧时长'),
        content: TextField(
          controller: controller,
          keyboardType: TextInputType.number,
          decoration: const InputDecoration(suffixText: 'ms'),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext),
            child: const Text('取消'),
          ),
          TextButton(
            onPressed: () =>
                Navigator.pop(dialogContext, int.tryParse(controller.text)),
            child: const Text('保存'),
          ),
        ],
      ),
    );
    controller.dispose();
    if (value != null && value > 0) {
      await _apply('frame.set_duration', {
        'frameId': frameId,
        'durationMs': value,
      });
    }
  }
}

import 'dart:async';
import 'dart:developer' as developer;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/services/owned_conversation_reference.dart';

class WorkbenchAgentRunsView extends ConsumerStatefulWidget {
  const WorkbenchAgentRunsView({
    super.key,
    required this.conversationId,
    required this.characterId,
    required this.active,
  });

  final String conversationId;
  final String characterId;
  final bool active;

  @override
  ConsumerState<WorkbenchAgentRunsView> createState() =>
      _WorkbenchAgentRunsViewState();
}

class _WorkbenchAgentRunsViewState
    extends ConsumerState<WorkbenchAgentRunsView> {
  final _draft = TextEditingController();
  final _messages = ScrollController();
  List<Map<String, dynamic>> _runs = const [];
  List<Map<String, dynamic>> _messagesValue = const [];
  String _selectedRunId = '';
  Timer? _pollTimer;
  bool _loading = false;
  bool _sending = false;
  String? _error;

  String get _ownedConversationId {
    try {
      final parsed = parseConversationReference(widget.conversationId);
      return parsed?['id'] ?? widget.conversationId;
    } catch (error, stackTrace) {
      developer.log(
        '[original-error] $error',
        name: 'workbench.agent-runs.conversation',
        error: error,
        stackTrace: stackTrace,
      );
      rethrow;
    }
  }

  Map<String, dynamic>? get _selectedRun {
    for (final run in _runs) {
      if (run['id']?.toString() == _selectedRunId) return run;
    }
    return null;
  }

  @override
  void initState() {
    super.initState();
    _refresh();
    _startPolling();
  }

  @override
  void didUpdateWidget(covariant WorkbenchAgentRunsView oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.conversationId != widget.conversationId) {
      _selectedRunId = '';
      _runs = const [];
      _messagesValue = const [];
      _refresh();
    }
    if (!oldWidget.active && widget.active) _refresh();
  }

  @override
  void dispose() {
    _pollTimer?.cancel();
    _draft.dispose();
    _messages.dispose();
    super.dispose();
  }

  void _startPolling() {
    _pollTimer?.cancel();
    _pollTimer = Timer.periodic(const Duration(seconds: 3), (_) {
      if (mounted && widget.active) _refresh();
    });
  }

  Future<void> _refresh() async {
    if (_loading || widget.conversationId.isEmpty) return;
    final api = ref.read(rawBackendServiceApiProvider);
    if (api == null) {
      if (mounted) setState(() => _error = '执行 Core 尚未就绪');
      return;
    }
    _loading = true;
    try {
      final response = await api.get<Map<String, dynamic>>(
        '/api/roles/runs',
        queryParameters: {'conversationId': _ownedConversationId},
      );
      final values = response?['runs'];
      final runs = values is List
          ? values
                .whereType<Map>()
                .map((item) => Map<String, dynamic>.from(item))
                .toList(growable: false)
          : <Map<String, dynamic>>[];
      runs.sort((left, right) {
        final depth = (left['depth'] as num? ?? 0).compareTo(
          right['depth'] as num? ?? 0,
        );
        if (depth != 0) return depth;
        return (left['createdAt']?.toString() ?? '').compareTo(
          right['createdAt']?.toString() ?? '',
        );
      });
      if (!mounted) return;
      setState(() {
        _runs = runs;
        _error = null;
        if (!runs.any((run) => run['id']?.toString() == _selectedRunId)) {
          _selectedRunId =
              runs
                  .where(
                    (run) => (run['parentRunId']?.toString() ?? '').isEmpty,
                  )
                  .map((run) => run['id']?.toString() ?? '')
                  .firstOrNull ??
              runs.map((run) => run['id']?.toString() ?? '').firstOrNull ??
              '';
        }
      });
      await _loadMessages();
    } catch (error, stackTrace) {
      developer.log(
        '[original-error] $error',
        name: 'workbench.agent-runs.refresh',
        error: error,
        stackTrace: stackTrace,
      );
      if (mounted) setState(() => _error = error.toString());
    } finally {
      _loading = false;
    }
  }

  Future<void> _loadMessages() async {
    if (_selectedRunId.isEmpty) return;
    final api = ref.read(rawBackendServiceApiProvider);
    if (api == null) return;
    final response = await api.get<Map<String, dynamic>>(
      '/api/roles/runs/${Uri.encodeComponent(_selectedRunId)}/messages',
    );
    final values = response?['messages'];
    final messages = values is List
        ? values
              .whereType<Map>()
              .map((item) => Map<String, dynamic>.from(item))
              .toList(growable: false)
        : <Map<String, dynamic>>[];
    if (!mounted) return;
    setState(() => _messagesValue = messages);
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (_messages.hasClients) {
        _messages.animateTo(
          _messages.position.maxScrollExtent,
          duration: const Duration(milliseconds: 180),
          curve: Curves.easeOut,
        );
      }
    });
  }

  Future<void> _selectRun(String id) async {
    if (id == _selectedRunId) return;
    setState(() {
      _selectedRunId = id;
      _messagesValue = const [];
    });
    await _loadMessages();
  }

  Future<void> _send() async {
    final content = _draft.text.trim();
    if (_selectedRunId.isEmpty || content.isEmpty || _sending) return;
    final api = ref.read(rawBackendServiceApiProvider);
    if (api == null) return;
    setState(() {
      _sending = true;
      _error = null;
    });
    try {
      await api.post<Map<String, dynamic>>(
        '/api/roles/runs/${Uri.encodeComponent(_selectedRunId)}/messages',
        data: {'content': content},
      );
      _draft.clear();
      await _loadMessages();
    } catch (error, stackTrace) {
      developer.log(
        '[original-error] $error',
        name: 'workbench.agent-runs.send',
        error: error,
        stackTrace: stackTrace,
      );
      if (mounted) setState(() => _error = error.toString());
    } finally {
      if (mounted) setState(() => _sending = false);
    }
  }

  String _status(String value) =>
      const {
        'planning': '规划中',
        'running': '运行中',
        'waiting': '等待中',
        'succeeded': '已完成',
        'failed': '失败',
        'cancelled': '已取消',
        'paused': '已暂停',
      }[value] ??
      value;

  String _kind(String value) =>
      const {
        'instruction': '指令',
        'progress': '进度',
        'question': '问题',
        'result': '结果',
        'error': '错误',
        'user': '用户',
      }[value] ??
      value;

  String _sender(Map<String, dynamic> message) {
    final sender = message['senderRunId']?.toString() ?? '';
    if (sender.startsWith('user:')) return '你';
    for (final run in _runs) {
      if (run['id']?.toString() != sender) continue;
      return (run['parentRunId']?.toString() ?? '').isEmpty
          ? '主对话智能体'
          : run['workerRole']?.toString() ?? '子智能体';
    }
    return '智能体';
  }

  @override
  Widget build(BuildContext context) {
    final selected = _selectedRun;
    return Column(
      children: [
        Padding(
          padding: EdgeInsets.fromLTRB(
            AppSpacing.md,
            AppSpacing.sm,
            AppSpacing.sm,
            AppSpacing.sm,
          ),
          child: Row(
            children: [
              Expanded(
                child: Text('子智能体', style: AppTypography.sectionTitle(context)),
              ),
              Text(
                '${_runs.length} 个运行',
                style: AppTypography.caption(context),
              ),
              IconButton(
                tooltip: '刷新',
                onPressed: _loading ? null : _refresh,
                icon: const Icon(Icons.refresh),
              ),
            ],
          ),
        ),
        if (_error != null)
          Container(
            width: double.infinity,
            padding: EdgeInsets.all(AppSpacing.sm),
            color: context.error.withValues(alpha: 0.12),
            child: SelectableText(
              _error!,
              style: TextStyle(color: context.error, fontSize: 11),
            ),
          ),
        SizedBox(
          height: 112,
          child: _runs.isEmpty
              ? Center(
                  child: Text(
                    _loading ? '正在加载智能体运行…' : '尚无智能体运行',
                    style: AppTypography.caption(context),
                  ),
                )
              : ListView.separated(
                  scrollDirection: Axis.horizontal,
                  padding: EdgeInsets.symmetric(
                    horizontal: AppSpacing.md,
                    vertical: 4,
                  ),
                  itemCount: _runs.length,
                  separatorBuilder: (_, _) => const SizedBox(width: 8),
                  itemBuilder: (context, index) {
                    final run = _runs[index];
                    final id = run['id']?.toString() ?? '';
                    final active = id == _selectedRunId;
                    final child =
                        (run['parentRunId']?.toString() ?? '').isNotEmpty;
                    return SizedBox(
                      width: 210,
                      child: Material(
                        color: active
                            ? context.accentSoft
                            : context.surfacePrimary,
                        borderRadius: BorderRadius.circular(10),
                        child: InkWell(
                          onTap: () => _selectRun(id),
                          borderRadius: BorderRadius.circular(10),
                          child: Padding(
                            padding: const EdgeInsets.all(10),
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Text(
                                  child
                                      ? run['workerRole']?.toString() ?? '子智能体'
                                      : '主对话',
                                  style: AppTypography.caption(context)
                                      .copyWith(
                                        color: context.accentPrimary,
                                        fontWeight: FontWeight.w600,
                                      ),
                                ),
                                const SizedBox(height: 4),
                                Expanded(
                                  child: Text(
                                    run['objective']?.toString() ??
                                        run['id']?.toString() ??
                                        '',
                                    maxLines: 2,
                                    overflow: TextOverflow.ellipsis,
                                    style: AppTypography.bodySmall(context),
                                  ),
                                ),
                                Text(
                                  _status(run['status']?.toString() ?? ''),
                                  style: AppTypography.caption(context),
                                ),
                              ],
                            ),
                          ),
                        ),
                      ),
                    );
                  },
                ),
        ),
        const Divider(height: 1),
        Expanded(
          child: selected == null
              ? Center(
                  child: Text(
                    '主智能体创建子智能体后，可在这里查看双向消息。',
                    style: AppTypography.caption(context),
                    textAlign: TextAlign.center,
                  ),
                )
              : ListView.builder(
                  controller: _messages,
                  padding: EdgeInsets.all(AppSpacing.md),
                  itemCount: _messagesValue.length,
                  itemBuilder: (context, index) {
                    final message = _messagesValue[index];
                    final mine =
                        message['senderRunId']?.toString().startsWith(
                          'user:',
                        ) ??
                        false;
                    return Align(
                      alignment: mine
                          ? Alignment.centerRight
                          : Alignment.centerLeft,
                      child: Container(
                        constraints: const BoxConstraints(maxWidth: 320),
                        margin: const EdgeInsets.only(bottom: 8),
                        padding: const EdgeInsets.all(9),
                        decoration: BoxDecoration(
                          color: mine
                              ? context.accentSoft
                              : context.surfacePrimary,
                          borderRadius: BorderRadius.circular(10),
                          border: Border.all(
                            color: context.borderPrimary,
                            width: 0.6,
                          ),
                        ),
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              '${_sender(message)} · ${_kind(message['kind']?.toString() ?? '')}',
                              style: AppTypography.caption(context),
                            ),
                            const SizedBox(height: 4),
                            SelectableText(
                              message['content']?.toString() ?? '',
                              style: AppTypography.bodySmall(context),
                            ),
                          ],
                        ),
                      ),
                    );
                  },
                ),
        ),
        if (selected != null)
          SafeArea(
            top: false,
            child: Padding(
              padding: EdgeInsets.fromLTRB(
                AppSpacing.md,
                6,
                AppSpacing.md,
                AppSpacing.sm,
              ),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.end,
                children: [
                  Expanded(
                    child: TextField(
                      controller: _draft,
                      minLines: 1,
                      maxLines: 4,
                      decoration: const InputDecoration(
                        hintText: '向该智能体发送消息',
                        isDense: true,
                      ),
                    ),
                  ),
                  const SizedBox(width: 8),
                  IconButton.filled(
                    onPressed: _sending ? null : _send,
                    icon: _sending
                        ? const SizedBox.square(
                            dimension: 16,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : const Icon(Icons.send),
                  ),
                ],
              ),
            ),
          ),
      ],
    );
  }
}

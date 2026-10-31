import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:dio/dio.dart';

import '../../../../app/amitia_page_route.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/models/character.dart';
import '../../../../core/models/conversation.dart';
import '../../../../core/models/project.dart';
import '../../../../core/errors/original_error.dart';
import '../../../../core/backend_transport/backend_service_api.dart';
import '../../../../core/backend_transport/errors/backend_transport_error.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_message.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../shared/models/models.dart';
import '../../../conversation/rendering/assistant_identity.dart';
import '../../../conversation/rendering/role_switch_divider.dart';

class ChatLogsPage extends ConsumerStatefulWidget {
  const ChatLogsPage({super.key});

  @override
  ConsumerState<ChatLogsPage> createState() => _ChatLogsPageState();
}

class _ChatLogsPageState extends ConsumerState<ChatLogsPage> {
  List<ConversationDto> _conversations = const [];
  List<ProjectDto> _projects = const [];
  bool _loading = true;
  String _error = '';
  String _restoringId = '';
  bool _selecting = false;
  bool _batchBusy = false;
  final Set<String> _selected = {};
  String _projectFilter = '';
  final _searchController = TextEditingController();
  Timer? _searchTimer;
  int _loadRequest = 0;

  @override
  void initState() {
    super.initState();
    _searchController.addListener(_scheduleSearch);
    unawaited(_loadFilters());
    unawaited(_load());
  }

  @override
  void dispose() {
    _searchTimer?.cancel();
    _searchController.removeListener(_scheduleSearch);
    _searchController.dispose();
    super.dispose();
  }

  Future<void> _loadFilters() async {
    try {
      final sidebar = await ref.read(chatServiceProvider).conversationSidebar();
      if (!mounted) return;
      setState(() => _projects = sidebar.projects);
    } catch (error, stackTrace) {
  print("[original-error] $error\n$stackTrace");
}
  }

  void _scheduleSearch() {
    if (mounted) setState(() {});
    _searchTimer?.cancel();
    _searchTimer = Timer(const Duration(milliseconds: 280), _load);
  }

  Widget _buildFilters(BuildContext context) {
    return Padding(
      padding: EdgeInsets.fromLTRB(
        AppSpacing.pagePadding,
        AppSpacing.sm,
        AppSpacing.pagePadding,
        0,
      ),
      child: Column(
        children: [
          DropdownButtonFormField<String?>(
            initialValue: _projectFilter.isEmpty ? null : _projectFilter,
            decoration: const InputDecoration(
              labelText: '项目',
              prefixIcon: Icon(Icons.folder_outlined),
            ),
            items: [
              const DropdownMenuItem<String?>(value: null, child: Text('全部项目')),
              for (final project in _projects)
                DropdownMenuItem<String?>(
                  value: project.id,
                  child: Text(
                    project.name,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
            ],
            onChanged: _batchBusy ? null : (value) {
              setState(() => _projectFilter = value ?? '');
              unawaited(_load());
            },
          ),
          SizedBox(height: AppSpacing.sm),
          TextField(
            controller: _searchController,
            enabled: !_batchBusy,
            decoration: InputDecoration(
              hintText: '搜索标题或消息内容',
              prefixIcon: const Icon(Icons.search),
              suffixIcon: _searchController.text.isEmpty
                  ? null
                  : IconButton(
                      tooltip: '清除搜索',
                      onPressed: _searchController.clear,
                      icon: const Icon(Icons.close),
                    ),
            ),
          ),
        ],
      ),
    );
  }

  Future<void> _load() async {
    if (_batchBusy) return;
    final ticket = ++_loadRequest;
    final projectId = _projectFilter;
    final keyword = _searchController.text;
    setState(() {
      _loading = true;
      _error = '';
    });
    try {
      final conversations = await ref
          .read(chatServiceProvider)
          .archivedConversations(projectId: projectId, keyword: keyword);
      if (!mounted || ticket != _loadRequest) return;
      setState(() {
        _conversations = conversations;
        _selected.removeWhere((id) => !conversations.any((row) => row.id == id));
      });
    } catch (error) {
      if (!mounted || ticket != _loadRequest) return;
      setState(() => _error = error.toString());
    } finally {
      if (mounted && ticket == _loadRequest) setState(() => _loading = false);
    }
  }

  Future<void> _restore(ConversationDto conversation) async {
    if (_restoringId.isNotEmpty) return;
    setState(() => _restoringId = conversation.id);
    try {
      await ref
          .read(chatServiceProvider)
          .restoreArchivedConversation(
            conversation.id,
            characterId: conversation.characterId,
          );
      if (!mounted) return;
      ref.read(conversationCollectionRevisionProvider.notifier).state++;
      ref.invalidate(conversationListProvider);
      ref.invalidate(conversationSidebarProvider);
      setState(() {
        _conversations = _conversations
            .where((item) => item.id != conversation.id)
            .toList(growable: false);
      });
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(const SnackBar(content: Text('对话已恢复')));
    } catch (error) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('恢复失败：$error'), backgroundColor: context.error),
      );
    } finally {
      if (mounted) setState(() => _restoringId = '');
    }
  }

  Future<void> _batchAction({required bool delete}) async {
    if (_batchBusy || _selected.isEmpty) return;
    if (delete) {
      final confirmed = await showAmitiaConfirmDialog(context, title: '永久删除', message: '确定永久删除选中的 ${_selected.length} 个对话及消息吗？记忆会保留，此操作无法撤销。', confirmLabel: '永久删除', isDestructive: true);
      if (confirmed != true || !mounted) return;
    }
    _loadRequest++;
    setState(() { _batchBusy = true; _loading = false; });
    final failures = <String>[];
    final succeeded = <String>[];
    final service = ref.read(chatServiceProvider);
    for (final conversation in _conversations.where((row) => _selected.contains(row.id)).toList()) {
      try {
        if (delete) {
          await service.deleteConversation(conversation.id, characterId: conversation.characterId);
        } else {
          await service.restoreArchivedConversation(conversation.id, characterId: conversation.characterId);
        }
        succeeded.add(conversation.id);
      } catch (error, stackTrace) {
        reportOriginalError(error, stackTrace, source: 'archive-batch');
        failures.add('${_conversationTitle(conversation)}: ${_batchErrorText(error, stackTrace)}');
      }
    }
    if (!mounted) return;
    setState(() {
      _batchBusy = false;
      _selected.removeAll(succeeded);
      _conversations = _conversations.where((row) => !succeeded.contains(row.id)).toList();
    });
    ref.invalidate(conversationListProvider);
    ref.invalidate(conversationSidebarProvider);
    ref.read(conversationCollectionRevisionProvider.notifier).state++;
    if (failures.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('已完成 ${succeeded.length} 项')));
    } else {
      await showDialog<void>(context: context, builder: (context) => AlertDialog(title: Text('已完成 ${succeeded.length} 项，失败 ${failures.length} 项'), content: SingleChildScrollView(child: SelectableText(failures.join('\n\n'))), actions: [TextButton(onPressed: () => Navigator.pop(context), child: const Text('关闭'))]));
    }
  }

  String _batchErrorText(Object error, StackTrace stackTrace) {
    final parts = <String>[];
    final seen = <Object>{};
    Object? current = error;
    while (current != null && seen.add(current)) {
      parts.add('$current');
      parts.add(originalErrorText(current));
      if (current is ServiceApiException) parts.add('code: ${current.code}\ndetail: ${current.detail}');
      if (current is DioException) {
        parts.add('response: ${current.response?.data}\nstack: ${current.stackTrace}');
        current = current.error;
      } else if (current is BackendTransportError) {
        current = current.cause;
      } else {
        current = null;
      }
    }
    parts.add('$stackTrace');
    return parts.join('\n');
  }

  void _toggleSelected(String id) {
    if (_batchBusy) return;
    setState(() { if (!_selected.remove(id)) _selected.add(id); });
  }

  @override
  Widget build(BuildContext context) {
    ref.listen<int>(conversationCollectionRevisionProvider, (previous, next) {
      if (previous != next) unawaited(_load());
    });
    return PopScope(
      canPop: !_selecting,
      onPopInvokedWithResult: (didPop, result) { if (!didPop && _selecting && !_batchBusy) setState(() { _selecting = false; _selected.clear(); }); },
      child: AmitiaScaffold(
      appBar: AmitiaAppBar(title: '归档对话', showBackButton: true),
      body: Column(
        children: [
          _buildFilters(context),
          Padding(
            padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
            child: Wrap(spacing: 4, children: [
              TextButton(onPressed: _batchBusy ? null : () => setState(() { _selecting = !_selecting; _selected.clear(); }), child: Text(_selecting ? '取消多选' : '多选')),
              if (_selecting) ...[
                TextButton(onPressed: _batchBusy || _conversations.isEmpty ? null : () => setState(() { if (_conversations.every((row) => _selected.contains(row.id))) { _selected.clear(); } else { _selected.addAll(_conversations.map((row) => row.id)); } }), child: Text(_conversations.isNotEmpty && _conversations.every((row) => _selected.contains(row.id)) ? '取消全选' : '全选')),
                TextButton(onPressed: _batchBusy || _selected.isEmpty ? null : () => _batchAction(delete: false), child: const Text('撤销')),
                TextButton(onPressed: _batchBusy || _selected.isEmpty ? null : () => _batchAction(delete: true), child: const Text('删除（永久）')),
              ],
            ]),
          ),
          Expanded(
            child: RefreshIndicator(
              onRefresh: _load,
              child: _loading
                  ? const Center(child: CircularProgressIndicator())
                  : _error.isNotEmpty
                  ? Center(child: Text(_error))
                  : _conversations.isEmpty
                  ? Center(
                      child: Text(
                        _projectFilter.isNotEmpty ||
                                _searchController.text.trim().isNotEmpty
                            ? '没有匹配的归档对话'
                            : '暂无归档对话',
                      ),
                    )
                  : ListView.separated(
                      padding: EdgeInsets.all(AppSpacing.pagePadding),
                      itemCount: _conversations.length,
                      separatorBuilder: (_, _) =>
                          SizedBox(height: AppSpacing.sm),
                      itemBuilder: (context, index) {
                        final conversation = _conversations[index];
                        return Material(
                          color: context.surfacePrimary,
                          borderRadius: BorderRadius.circular(12),
                          child: InkWell(
                            borderRadius: BorderRadius.circular(12),
                            onTap: _batchBusy ? null : () => _selecting ? _toggleSelected(conversation.id) : Navigator.of(context).push(
                              amitiaSlideRoute<void>(
                                builder: (_) => ArchivedConversationDetailPage(
                                  conversation: conversation,
                                  projectName: _projectName(
                                    conversation.projectId,
                                  ),
                                ),
                              ),
                            ),
                            child: Padding(
                              padding: const EdgeInsets.fromLTRB(14, 10, 6, 10),
                              child: Row(
                                children: [
                                  if (_selecting) IgnorePointer(child: Checkbox(value: _selected.contains(conversation.id), onChanged: (_) {})),
                                  Icon(
                                    Icons.inventory_2_outlined,
                                    color: context.textTertiary,
                                  ),
                                  const SizedBox(width: 12),
                                  Expanded(
                                    child: Column(
                                      crossAxisAlignment:
                                          CrossAxisAlignment.start,
                                      mainAxisSize: MainAxisSize.min,
                                      children: [
                                        Text(
                                          _conversationTitle(conversation),
                                          maxLines: 1,
                                          overflow: TextOverflow.ellipsis,
                                          style: AppTypography.cardTitle(
                                            context,
                                          ),
                                        ),
                                        const SizedBox(height: 3),
                                        Text(
                                          '${_formatTime(conversation.archivedAt)} · ${conversation.messageCount} 条消息',
                                          maxLines: 1,
                                          overflow: TextOverflow.ellipsis,
                                          style: AppTypography.label(context),
                                        ),
                                      ],
                                    ),
                                  ),
                                  if (!_selecting) TextButton(
                                    onPressed: _restoringId.isEmpty
                                        ? () => _restore(conversation)
                                        : null,
                                    child: _restoringId == conversation.id
                                        ? const SizedBox(
                                            width: 16,
                                            height: 16,
                                            child: CircularProgressIndicator(
                                              strokeWidth: 2,
                                            ),
                                          )
                                        : const Text('撤销'),
                                  ),
                                ],
                              ),
                            ),
                          ),
                        );
                      },
                    ),
            ),
          ),
        ],
      ),
      ),
    );
  }

  String _projectName(String projectId) {
    final id = projectId.trim();
    if (id.isEmpty) return '';
    for (final project in _projects) {
      if (project.id == id) return project.name.trim();
    }
    return '';
  }

  String _conversationTitle(ConversationDto conversation) {
    final title = conversation.title.trim().isEmpty
        ? '新对话'
        : conversation.title.trim();
    final projectName = _projectName(conversation.projectId);
    return projectName.isEmpty ? title : '$projectName - $title';
  }
}

class ArchivedConversationDetailPage extends ConsumerStatefulWidget {
  final ConversationDto conversation;
  final String projectName;

  const ArchivedConversationDetailPage({
    super.key,
    required this.conversation,
    this.projectName = '',
  });

  @override
  ConsumerState<ArchivedConversationDetailPage> createState() =>
      _ArchivedConversationDetailPageState();
}

class _ArchivedConversationDetailPageState
    extends ConsumerState<ArchivedConversationDetailPage> {
  List<MessageDto> _messages = const [];
  bool _loading = true;
  String _error = '';
  bool _processing = false;

  Future<void> _processAction(String action) async {
    if (_processing || _loading) return;
    final service = ref.read(chatServiceProvider);
    final revision = service.owned.revision;
    setState(() => _processing = true);
    try {
      if (action == 'summary') {
        final result = await service.generateConversationSummary(
          widget.conversation.id,
          characterId: widget.conversation.characterId,
        );
        if (!mounted || revision != service.owned.revision) return;
        final text = result?['summaryText']?.toString() ?? '';
        if (text.trim().isEmpty) throw StateError('摘要结果为空');
        await showDialog<void>(
          context: context,
          builder: (dialogContext) => AlertDialog(
            title: const Text('对话摘要'),
            content: SingleChildScrollView(child: SelectableText(text)),
            actions: [
              TextButton(
                onPressed: () => Navigator.of(dialogContext).pop(),
                child: const Text('关闭'),
              ),
            ],
          ),
        );
      } else {
        final text = await service.exportConversation(
          widget.conversation.id,
          format: action,
          characterId: widget.conversation.characterId,
        );
        if (!mounted || revision != service.owned.revision) return;
        await Clipboard.setData(ClipboardData(text: text));
        if (!mounted || revision != service.owned.revision) return;
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('对话已复制到剪贴板')),
        );
      }
    } catch (error) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('操作失败：$error'), backgroundColor: context.error),
      );
    } finally {
      if (mounted) setState(() => _processing = false);
    }
  }

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = '';
    });
    try {
      final messages = await ref
          .read(chatServiceProvider)
          .getMessages(
            widget.conversation.id,
            characterId: widget.conversation.characterId,
          );
      if (!mounted) return;
      setState(() => _messages = messages);
    } catch (error) {
      if (!mounted) return;
      setState(() => _error = error.toString());
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final characters =
        ref.watch(characterListProvider).valueOrNull ?? const <CharacterDto>[];
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: widget.projectName.trim().isEmpty
            ? (widget.conversation.title.trim().isEmpty
                  ? '归档对话'
                  : widget.conversation.title)
            : '${widget.projectName} - ${widget.conversation.title.trim().isEmpty ? '新对话' : widget.conversation.title}',
        showBackButton: true,
        actions: [
          if (_processing)
            const Padding(
              padding: EdgeInsets.all(16),
              child: SizedBox(
                width: 20,
                height: 20,
                child: CircularProgressIndicator(strokeWidth: 2),
              ),
            )
          else
            PopupMenuButton<String>(
              tooltip: '对话操作',
              enabled: !_loading && _error.isEmpty,
              onSelected: _processAction,
              itemBuilder: (_) => const [
                PopupMenuItem(value: 'summary', child: Text('生成摘要')),
                PopupMenuItem(value: 'json', child: Text('复制 JSON')),
                PopupMenuItem(value: 'markdown', child: Text('复制 Markdown')),
              ],
            ),
        ],
      ),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _error.isNotEmpty
          ? Center(child: Text(_error))
          : _messages.isEmpty
          ? const Center(child: Text('暂无消息'))
          : ListView.builder(
              padding: EdgeInsets.all(AppSpacing.pagePadding),
              itemCount: _messages.length,
              itemBuilder: (context, index) {
                final message = _messages[index];
                final current = _toChatMessage(message);
                final previous = index > 0
                    ? _toChatMessage(_messages[index - 1])
                    : null;
                final next = index + 1 < _messages.length
                    ? _toChatMessage(_messages[index + 1])
                    : null;
                final character = characters
                    .where((item) => item.id == message.characterId)
                    .firstOrNull;
                final characterName = character?.name.trim().isNotEmpty == true
                    ? character!.name.trim()
                    : message.characterId.trim().isEmpty
                    ? 'AI'
                    : '未知角色';
                return Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    if (shouldShowRoleSwitch(previous, current))
                      AmitiaRoleSwitchDivider(characterName: characterName),
                    AmitiaMessageBubble(
                      message: current,
                      avatarInitial: characterName.isNotEmpty
                          ? characterName.characters.first
                          : 'AI',
                      characterName: characterName,
                      showAvatar: shouldShowAssistantIdentity(
                        current,
                        previous,
                      ),
                      showHeader: shouldShowAssistantIdentity(
                        current,
                        previous,
                      ),
                      compactBottom: shouldCompactAfterAssistantMessage(
                        current,
                        next,
                      ),
                    ),
                  ],
                );
              },
            ),
    );
  }
}

ChatMessage _toChatMessage(MessageDto message) {
  final type = message.imageUrl.isNotEmpty
      ? MessageType.image
      : message.audioUrl.isNotEmpty
      ? MessageType.audio
      : message.videoUrl.isNotEmpty
      ? MessageType.video
      : MessageType.text;
  return ChatMessage(
    id: message.id,
    characterId: message.characterId,
    role: message.role == 'user'
        ? MessageRole.user
        : message.role == 'assistant'
        ? MessageRole.assistant
        : MessageRole.system,
    type: type,
    content: message.content,
    reasoningContent: message.reasoningContent,
    reasoningDurationMs: message.reasoningDurationMs,
    time: DateTime.tryParse(message.createdAt) ?? DateTime.now(),
    status: MessageStatus.sent,
    mediaUrl: message.imageUrl.isNotEmpty
        ? message.imageUrl
        : message.audioUrl.isNotEmpty
        ? message.audioUrl
        : message.videoUrl.isNotEmpty
        ? message.videoUrl
        : null,
    durationMs: (message.audioDuration * 1000).round(),
    replyToMessageId: message.replyToMessageId,
    replyToExcerpt: message.replyToExcerpt,
    citationSnapshot: message.citationSnapshot,
  );
}

String _formatTime(String value) {
  final time = DateTime.tryParse(value);
  if (time == null) return '未知时间';
  final local = time.toLocal();
  return '${local.year}/${local.month}/${local.day} ${local.hour.toString().padLeft(2, '0')}:${local.minute.toString().padLeft(2, '0')}';
}

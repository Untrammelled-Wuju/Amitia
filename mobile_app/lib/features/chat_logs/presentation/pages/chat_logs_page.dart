import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/models/conversation.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class ChatLogsPage extends ConsumerStatefulWidget {
  const ChatLogsPage({super.key});

  @override
  ConsumerState<ChatLogsPage> createState() => _ChatLogsPageState();
}

class _ChatLogsPageState extends ConsumerState<ChatLogsPage> {
  List<ConversationDto> _conversations = const [];
  bool _loading = true;
  String _error = '';
  String _restoringId = '';

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
      final conversations = await ref
          .read(chatServiceProvider)
          .archivedConversations();
      if (!mounted) return;
      setState(() => _conversations = conversations);
    } catch (error) {
      if (!mounted) return;
      setState(() => _error = error.toString());
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  Future<void> _restore(ConversationDto conversation) async {
    if (_restoringId.isNotEmpty) return;
    setState(() => _restoringId = conversation.id);
    try {
      await ref
          .read(chatServiceProvider)
          .restoreArchivedConversation(conversation.id);
      if (!mounted) return;
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

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '归档对话', showBackButton: true),
      body: RefreshIndicator(
        onRefresh: _load,
        child: _loading
            ? const Center(child: CircularProgressIndicator())
            : _error.isNotEmpty
            ? Center(child: Text(_error))
            : _conversations.isEmpty
            ? const Center(child: Text('暂无归档对话'))
            : ListView.separated(
                padding: EdgeInsets.all(AppSpacing.pagePadding),
                itemCount: _conversations.length,
                separatorBuilder: (_, _) => SizedBox(height: AppSpacing.sm),
                itemBuilder: (context, index) {
                  final conversation = _conversations[index];
                  return Material(
                    color: context.surfacePrimary,
                    borderRadius: BorderRadius.circular(12),
                    child: InkWell(
                      borderRadius: BorderRadius.circular(12),
                      onTap: () => Navigator.of(context).push(
                        MaterialPageRoute<void>(
                          builder: (_) => ArchivedConversationDetailPage(
                            conversation: conversation,
                          ),
                        ),
                      ),
                      child: Padding(
                        padding: const EdgeInsets.fromLTRB(14, 10, 6, 10),
                        child: Row(
                          children: [
                            Icon(
                              Icons.archive_outlined,
                              color: context.textTertiary,
                            ),
                            const SizedBox(width: 12),
                            Expanded(
                              child: Column(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                mainAxisSize: MainAxisSize.min,
                                children: [
                                  Text(
                                    conversation.title.trim().isEmpty
                                        ? '新对话'
                                        : conversation.title,
                                    maxLines: 1,
                                    overflow: TextOverflow.ellipsis,
                                    style: AppTypography.cardTitle(context),
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
                            TextButton(
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
    );
  }
}

class ArchivedConversationDetailPage extends ConsumerStatefulWidget {
  final ConversationDto conversation;

  const ArchivedConversationDetailPage({super.key, required this.conversation});

  @override
  ConsumerState<ArchivedConversationDetailPage> createState() =>
      _ArchivedConversationDetailPageState();
}

class _ArchivedConversationDetailPageState
    extends ConsumerState<ArchivedConversationDetailPage> {
  List<MessageDto> _messages = const [];
  bool _loading = true;
  String _error = '';

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
          .getMessages(widget.conversation.id);
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
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: widget.conversation.title.trim().isEmpty
            ? '归档对话'
            : widget.conversation.title,
        showBackButton: true,
      ),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _error.isNotEmpty
          ? Center(child: Text(_error))
          : _messages.isEmpty
          ? const Center(child: Text('暂无消息'))
          : ListView.separated(
              padding: EdgeInsets.all(AppSpacing.pagePadding),
              itemCount: _messages.length,
              separatorBuilder: (_, _) => SizedBox(height: AppSpacing.sm),
              itemBuilder: (context, index) {
                final message = _messages[index];
                return Align(
                  alignment: message.role == 'user'
                      ? Alignment.centerRight
                      : Alignment.centerLeft,
                  child: Container(
                    constraints: const BoxConstraints(maxWidth: 320),
                    padding: const EdgeInsets.symmetric(
                      horizontal: 12,
                      vertical: 9,
                    ),
                    decoration: BoxDecoration(
                      color: message.role == 'user'
                          ? context.accentSoft
                          : context.surfacePrimary,
                      borderRadius: BorderRadius.circular(12),
                    ),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          message.role == 'user' ? '用户' : 'AI',
                          style: AppTypography.label(context),
                        ),
                        const SizedBox(height: 4),
                        Text(
                          message.content,
                          style: AppTypography.body(context),
                        ),
                      ],
                    ),
                  ),
                );
              },
            ),
    );
  }
}

String _formatTime(String value) {
  final time = DateTime.tryParse(value);
  if (time == null) return '未知时间';
  final local = time.toLocal();
  return '${local.year}/${local.month}/${local.day} ${local.hour.toString().padLeft(2, '0')}:${local.minute.toString().padLeft(2, '0')}';
}

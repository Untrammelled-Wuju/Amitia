import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_message.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_drawer.dart';
import '../../../../shared/models/models.dart';
import '../../../../shared/mock_data/mock_data.dart';

class ChatPage extends ConsumerStatefulWidget {
  const ChatPage({super.key});

  @override
  ConsumerState<ChatPage> createState() => _ChatPageState();
}

class _ChatPageState extends ConsumerState<ChatPage> {
  late List<ChatMessage> _messages;
  final _scrollController = ScrollController();
  final Map<String, String> _agentTaskStatus = {};

  @override
  void initState() {
    super.initState();
    _messages = List.from(MockData.chatMessages);
  }

  @override
  void dispose() {
    _scrollController.dispose();
    super.dispose();
  }

  void _scrollToBottom() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (_scrollController.hasClients) {
        _scrollController.animateTo(
          _scrollController.position.maxScrollExtent,
          duration: const Duration(milliseconds: 300),
          curve: Curves.easeOut,
        );
      }
    });
  }

  void _jumpToMessage(int index) {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!_scrollController.hasClients) return;
      final target = (index * 76.0).clamp(0.0, _scrollController.position.maxScrollExtent);
      _scrollController.animateTo(
        target,
        duration: const Duration(milliseconds: 300),
        curve: Curves.easeOut,
      );
    });
  }

  ChatMessage _cloneWithStatus(ChatMessage m, MessageStatus s) {
    return ChatMessage(
      id: m.id,
      role: m.role,
      type: m.type,
      content: m.content,
      time: m.time,
      status: s,
      agentTaskTitle: m.agentTaskTitle,
      agentTaskSteps: m.agentTaskSteps,
      agentTaskProgress: m.agentTaskProgress,
      agentTaskElapsed: m.agentTaskElapsed,
      fileName: m.fileName,
      fileSizeKB: m.fileSizeKB,
      toolName: m.toolName,
      toolResult: m.toolResult,
    );
  }

  void _retryMessage(int index) {
    if (index < 0 || index >= _messages.length) return;
    final msg = _messages[index];
    setState(() {
      _messages[index] = _cloneWithStatus(msg, MessageStatus.sending);
    });
    Future.delayed(const Duration(milliseconds: 600), () {
      if (!mounted) return;
      setState(() {
        if (index < _messages.length && _messages[index].id == msg.id) {
          _messages[index] = _cloneWithStatus(_messages[index], MessageStatus.sent);
        }
      });
    });
  }

  void _pauseAgentTask(int index) {
    if (index < 0 || index >= _messages.length) return;
    final id = _messages[index].id;
    setState(() => _agentTaskStatus[id] = '已暂停');
    amitiaSnackBar(context, '任务已暂停');
  }

  void _resumeAgentTask(int index) {
    if (index < 0 || index >= _messages.length) return;
    final id = _messages[index].id;
    setState(() => _agentTaskStatus[id] = '运行中');
    amitiaSnackBar(context, '任务已继续执行');
  }

  void _addUserMessage(ChatMessage message) {
    setState(() => _messages.add(message));
    _scrollToBottom();
    _replyAfter(message);
  }

  void _replyAfter(ChatMessage userMessage) {
    final String reply;
    switch (userMessage.type) {
      case MessageType.file:
        reply = '收到你的文件「${userMessage.fileName ?? ''}」，需要我帮你分析或处理吗？';
      case MessageType.image:
        reply = '收到图片，我看了一下，内容看起来很清晰。需要我帮你做点什么吗？';
      default:
        if (userMessage.content.startsWith('__mock:audio|')) {
          reply = '收到你的语音消息，我已听取。';
        } else if (userMessage.content.startsWith('__mock:emote|')) {
          reply = '😊';
        } else if (userMessage.content.startsWith('__mock:code|')) {
          reply = '收到代码，我帮你检查了一下，语法没有问题。需要解释或优化建议吗？';
        } else {
          reply = '收到你的消息了，让我想想怎么回复你……';
        }
    }
    Future.delayed(const Duration(milliseconds: 600), () {
      if (!mounted) return;
      final replyTime = DateTime.now();
      setState(() {
        _messages.add(ChatMessage(
          id: 'a${replyTime.millisecondsSinceEpoch}',
          role: MessageRole.assistant,
          type: MessageType.text,
          content: reply,
          time: replyTime,
        ));
      });
      _scrollToBottom();
    });
  }

  void _onSend(String text) {
    final now = DateTime.now();
    _addUserMessage(ChatMessage(
      id: 'u${now.millisecondsSinceEpoch}',
      role: MessageRole.user,
      type: MessageType.text,
      content: text,
      time: now,
    ));
  }

  void _onSendFile(String fileName, int sizeKB) {
    final now = DateTime.now();
    _addUserMessage(ChatMessage(
      id: 'u${now.millisecondsSinceEpoch}',
      role: MessageRole.user,
      type: MessageType.file,
      content: fileName,
      time: now,
      fileName: fileName,
      fileSizeKB: sizeKB,
    ));
  }

  void _onSendImage(String name) {
    final now = DateTime.now();
    _addUserMessage(ChatMessage(
      id: 'u${now.millisecondsSinceEpoch}',
      role: MessageRole.user,
      type: MessageType.text,
      content: mockImagePayload(name),
      time: now,
    ));
  }

  void _onSendCode(String lang, String code) {
    final now = DateTime.now();
    _addUserMessage(ChatMessage(
      id: 'u${now.millisecondsSinceEpoch}',
      role: MessageRole.user,
      type: MessageType.text,
      content: mockCodePayload(lang, code),
      time: now,
    ));
  }

  void _onSendVoice(String duration) {
    final now = DateTime.now();
    _addUserMessage(ChatMessage(
      id: 'u${now.millisecondsSinceEpoch}',
      role: MessageRole.user,
      type: MessageType.text,
      content: mockAudioPayload(duration),
      time: now,
    ));
  }

  void _onSendEmote(String emoji, String name) {
    final now = DateTime.now();
    _addUserMessage(ChatMessage(
      id: 'u${now.millisecondsSinceEpoch}',
      role: MessageRole.user,
      type: MessageType.text,
      content: mockEmotePayload(emoji, name),
      time: now,
    ));
  }

  bool _shouldShowAvatar(int index) {
    final current = _messages[index];
    if (current.role != MessageRole.assistant) return false;
    if (current.type == MessageType.agentTask || current.type == MessageType.toolCall) return false;
    if (index == 0) return true;
    final previous = _messages[index - 1];
    if (previous.role != MessageRole.assistant) return true;
    return false;
  }

  void _showMessageSearch(BuildContext context) {
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(20))),
      builder: (sheetCtx) {
        return _MessageSearchSheet(
          messages: _messages,
          onJump: (index) {
            Navigator.pop(sheetCtx);
            _jumpToMessage(index);
          },
        );
      },
    );
  }

  void _showChatActionsSheet(BuildContext context) {
    showAmitiaActionSheet<int>(
      context,
      title: '聊天操作',
      actions: const [
        AmitiaActionSheetItem(icon: Icons.person_outline, label: '查看角色详情', value: 0),
        AmitiaActionSheetItem(icon: Icons.cleaning_services_outlined, label: '清空聊天记录', value: 1, isDestructive: true),
        AmitiaActionSheetItem(icon: Icons.file_download_outlined, label: '导出聊天记录', value: 2),
        AmitiaActionSheetItem(icon: Icons.share_outlined, label: '分享对话', value: 3),
      ],
    ).then((result) {
      if (result == null || !mounted) return;
      switch (result) {
        case 0:
          context.push(AppRoutes.character(ref.read(currentCharacterIdProvider)));
        case 1:
          showAmitiaConfirmDialog(
            context,
            title: '清空聊天记录',
            message: '确定要清空当前聊天记录吗？此操作不可撤销。',
            confirmLabel: '清空',
            isDestructive: true,
          ).then((confirmed) {
            if (confirmed == true && mounted) {
              setState(() {
                _messages.clear();
                _agentTaskStatus.clear();
                _messages.add(ChatMessage(
                  id: 'sys${DateTime.now().millisecondsSinceEpoch}',
                  role: MessageRole.system,
                  type: MessageType.systemNotice,
                  content: '聊天记录已清空',
                  time: DateTime.now(),
                ));
              });
              amitiaSnackBar(context, '聊天记录已清空');
            }
          });
        case 2:
          amitiaSnackBar(context, '聊天记录已导出到本地');
        case 3:
          amitiaSnackBar(context, '对话链接已复制');
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final isAgentMode = ref.watch(isAgentModeProvider);
    final characterId = ref.watch(currentCharacterIdProvider);
    final character = MockData.characters.firstWhere(
      (c) => c.id == characterId,
      orElse: () => MockData.characters.first,
    );

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        centerTitle: true,
        navigation: AmitiaAppBarNavigation.drawer,
        titleWidget: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(character.name, style: AppTypography.cardTitle(context)),
            const SizedBox(height: 2),
            Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Container(
                  width: 6,
                  height: 6,
                  decoration: BoxDecoration(
                    color: context.success,
                    shape: BoxShape.circle,
                  ),
                ),
                const SizedBox(width: 4),
                Text('在线 · 空闲中', style: AppTypography.label(context)),
              ],
            ),
          ],
        ),
        actions: [
          AmitiaIconButton(
            icon: Icons.search,
            onPressed: () => _showMessageSearch(context),
          ),
          AmitiaIconButton(
            icon: Icons.list_alt_outlined,
            onPressed: () => context.push(AppRoutes.conversations),
          ),
          AmitiaIconButton(
            icon: Icons.more_horiz,
            onPressed: () => _showChatActionsSheet(context),
          ),
        ],
      ),
      body: Column(
        children: [
          Expanded(
            child: ListView.builder(
              controller: _scrollController,
              padding: const EdgeInsets.symmetric(vertical: AppSpacing.sm),
              itemCount: _messages.length,
              itemBuilder: (context, index) {
                final message = _messages[index];
                final isAgentTask = message.type == MessageType.agentTask;
                return AmitiaMessageBubble(
                  message: message,
                  showAvatar: _shouldShowAvatar(index),
                  avatarInitial: character.avatarInitial,
                  avatarColor: character.avatarColor,
                  characterName: character.name,
                  onRetry: message.status == MessageStatus.error ? () => _retryMessage(index) : null,
                  onAgentTaskTap: isAgentTask ? () => context.push(AppRoutes.agentTask('t1')) : null,
                  onPauseAgentTask: isAgentTask ? () => _pauseAgentTask(index) : null,
                  onResumeAgentTask: isAgentTask ? () => _resumeAgentTask(index) : null,
                  agentTaskStatusLabel: isAgentTask ? (_agentTaskStatus[message.id] ?? '运行中') : null,
                );
              },
            ),
          ),
          AmitiaChatInput(
            onSend: _onSend,
            isAgentMode: isAgentMode,
            onAgentModeChanged: (value) {
              ref.read(isAgentModeProvider.notifier).state = value;
            },
            onSendFile: _onSendFile,
            onSendImage: _onSendImage,
            onSendCode: _onSendCode,
            onSendVoice: _onSendVoice,
            onSendEmote: _onSendEmote,
          ),
        ],
      ),
    );
  }
}

class _MessageSearchSheet extends StatefulWidget {
  final List<ChatMessage> messages;
  final ValueChanged<int> onJump;

  const _MessageSearchSheet({required this.messages, required this.onJump});

  @override
  State<_MessageSearchSheet> createState() => _MessageSearchSheetState();
}

class _MessageSearchSheetState extends State<_MessageSearchSheet> {
  final _controller = TextEditingController();
  String _query = '';

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  List<(int, ChatMessage)> get _results {
    if (_query.trim().isEmpty) return const [];
    final q = _query.trim().toLowerCase();
    final list = <(int, ChatMessage)>[];
    for (var i = 0; i < widget.messages.length; i++) {
      final m = widget.messages[i];
      if (m.type == MessageType.systemNotice) continue;
      if (m.content.toLowerCase().contains(q)) {
        list.add((i, m));
      }
    }
    return list;
  }

  String _preview(ChatMessage m) {
    if (m.type == MessageType.file) return '[文件] ${m.fileName ?? m.content}';
    if (m.content.startsWith('__mock:image|')) return '[图片] ${m.content.split('|').last}';
    if (m.content.startsWith('__mock:video|')) return '[视频] ${m.content.split('|').last}';
    if (m.content.startsWith('__mock:audio|')) return '[语音] ${m.content.split('|').last}';
    if (m.content.startsWith('__mock:emote|')) return '[表情]';
    if (m.content.startsWith('__mock:code|')) return '[代码] ${m.content.split('|').last}';
    return m.content;
  }

  @override
  Widget build(BuildContext context) {
    final results = _results;
    return SafeArea(
      child: SizedBox(
        height: MediaQuery.sizeOf(context).height * 0.65,
        child: Padding(
          padding: const EdgeInsets.fromLTRB(20, 0, 20, 20),
          child: Column(
            children: [
              const SizedBox(height: 8),
              Center(
                child: Container(
                  width: 40,
                  height: 4,
                  decoration: BoxDecoration(
                    color: context.borderPrimary,
                    borderRadius: BorderRadius.circular(2),
                  ),
                ),
              ),
              const SizedBox(height: 16),
              Text('搜索聊天消息', style: AppTypography.pageTitle(context)),
              const SizedBox(height: 12),
              AmitiaSearchField(
                hintText: '输入关键词搜索消息',
                controller: _controller,
                onChanged: (v) => setState(() => _query = v),
              ),
              const SizedBox(height: 8),
              Expanded(
                child: _query.trim().isEmpty
                    ? Center(
                        child: Text('输入关键词后在当前会话中搜索消息', style: AppTypography.caption(context), textAlign: TextAlign.center),
                      )
                    : results.isEmpty
                        ? Center(
                            child: Text('没有找到匹配「$_query」的消息', style: AppTypography.caption(context), textAlign: TextAlign.center),
                          )
                        : ListView.separated(
                            itemCount: results.length,
                            separatorBuilder: (_, _) => Divider(height: 1, color: context.borderSecondary),
                            itemBuilder: (ctx, i) {
                              final item = results[i];
                              final msg = item.$2;
                              return ListTile(
                                leading: Icon(
                                  msg.role == MessageRole.user ? Icons.person_outline : Icons.smart_toy_outlined,
                                  size: 20,
                                  color: context.textTertiary,
                                ),
                                title: Text(
                                  _preview(msg),
                                  style: AppTypography.bodySmall(context),
                                  maxLines: 2,
                                  overflow: TextOverflow.ellipsis,
                                ),
                                subtitle: Text(
                                  '${msg.time.hour.toString().padLeft(2, '0')}:${msg.time.minute.toString().padLeft(2, '0')}',
                                  style: AppTypography.label(context),
                                ),
                                onTap: () => widget.onJump(item.$1),
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
}

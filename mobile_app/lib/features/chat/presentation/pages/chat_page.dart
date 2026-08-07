import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_motion.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_message.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_drawer.dart';
import '../../../../core/services/providers.dart';
import '../../../../shared/models/models.dart';

class ChatPage extends ConsumerStatefulWidget {
  const ChatPage({super.key});

  @override
  ConsumerState<ChatPage> createState() => _ChatPageState();
}

class _ChatPageState extends ConsumerState<ChatPage> {
  final List<ChatMessage> _messages = [];
  final _scrollController = ScrollController();
  final Map<String, String> _agentTaskStatus = {};

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
          duration: AppMotion.extended,
          curve: AppMotion.standardCurve,
        );
      }
    });
  }

  void _jumpToMessage(int index) {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!_scrollController.hasClients) return;
      final target = (index * 76.0).clamp(
        0.0,
        _scrollController.position.maxScrollExtent,
      );
      _scrollController.animateTo(
        target,
        duration: AppMotion.extended,
        curve: AppMotion.standardCurve,
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
          _messages[index] = _cloneWithStatus(
            _messages[index],
            MessageStatus.sent,
          );
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
    final chatApi = ref.read(chatServiceProvider);
    final content = userMessage.content;

    String strippedContent = content;
    if (content.startsWith('__mock:audio|')) {
      strippedContent = '[语音消息]';
    } else if (content.startsWith('__mock:emote|')) {
      strippedContent = '[表情]';
    } else if (content.startsWith('__mock:code|')) {
      strippedContent = content.substring('__mock:code|'.length);
    }

    chatApi.chat(strippedContent).then((response) {
      if (!mounted) return;
      final replyTime = DateTime.now();
      String reply = '已收到你的消息';
      if (response != null) {
        reply = response['reply'] as String? ??
            response['content'] as String? ??
            response['message'] as String? ??
            '已收到你的消息';
      }
      setState(() {
        _messages.add(
          ChatMessage(
            id: 'a${replyTime.millisecondsSinceEpoch}',
            role: MessageRole.assistant,
            type: MessageType.text,
            content: reply,
            time: replyTime,
          ),
        );
      });
      _scrollToBottom();
    }).catchError((err) {
      if (!mounted) return;
      final replyTime = DateTime.now();
      setState(() {
        _messages.add(
          ChatMessage(
            id: 'a${replyTime.millisecondsSinceEpoch}',
            role: MessageRole.assistant,
            type: MessageType.text,
            content: '连接失败: ${err.toString().replaceFirst('Exception: ', '')}',
            time: replyTime,
            status: MessageStatus.error,
          ),
        );
      });
      _scrollToBottom();
    });
  }

  void _onSend(String text) {
    final now = DateTime.now();
    _addUserMessage(
      ChatMessage(
        id: 'u${now.millisecondsSinceEpoch}',
        role: MessageRole.user,
        type: MessageType.text,
        content: text,
        time: now,
      ),
    );
  }

  void _onSendFile(String fileName, int sizeKB) {
    final now = DateTime.now();
    _addUserMessage(
      ChatMessage(
        id: 'u${now.millisecondsSinceEpoch}',
        role: MessageRole.user,
        type: MessageType.file,
        content: fileName,
        time: now,
        fileName: fileName,
        fileSizeKB: sizeKB,
      ),
    );
  }

  void _onSendImage(String name) {
    final now = DateTime.now();
    _addUserMessage(
      ChatMessage(
        id: 'u${now.millisecondsSinceEpoch}',
        role: MessageRole.user,
        type: MessageType.text,
        content: mockImagePayload(name),
        time: now,
      ),
    );
  }

  void _onSendCode(String lang, String code) {
    final now = DateTime.now();
    _addUserMessage(
      ChatMessage(
        id: 'u${now.millisecondsSinceEpoch}',
        role: MessageRole.user,
        type: MessageType.text,
        content: mockCodePayload(lang, code),
        time: now,
      ),
    );
  }

  void _onSendVoice(String duration) {
    final now = DateTime.now();
    _addUserMessage(
      ChatMessage(
        id: 'u${now.millisecondsSinceEpoch}',
        role: MessageRole.user,
        type: MessageType.text,
        content: mockAudioPayload(duration),
        time: now,
      ),
    );
  }

  void _onSendEmote(String emoji, String name) {
    final now = DateTime.now();
    _addUserMessage(
      ChatMessage(
        id: 'u${now.millisecondsSinceEpoch}',
        role: MessageRole.user,
        type: MessageType.text,
        content: mockEmotePayload(emoji, name),
        time: now,
      ),
    );
  }

  bool _shouldShowAvatar(int index) {
    final current = _messages[index];
    if (current.role != MessageRole.assistant) return false;
    if (current.type == MessageType.agentTask ||
        current.type == MessageType.toolCall)
      return false;
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
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
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
        AmitiaActionSheetItem(
          icon: Icons.person_outline,
          label: '查看角色详情',
          value: 0,
        ),
        AmitiaActionSheetItem(
          icon: Icons.cleaning_services_outlined,
          label: '清空聊天记录',
          value: 1,
          isDestructive: true,
        ),
        AmitiaActionSheetItem(
          icon: Icons.file_download_outlined,
          label: '导出聊天记录',
          value: 2,
        ),
        AmitiaActionSheetItem(
          icon: Icons.share_outlined,
          label: '分享对话',
          value: 3,
        ),
        AmitiaActionSheetItem(icon: Icons.search, label: '搜索当前会话', value: 4),
      ],
    ).then((result) {
      if (result == null || !mounted) return;
      switch (result) {
        case 0:
          context.push(
            AppRoutes.character(ref.read(currentCharacterIdProvider)),
          );
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
                _messages.add(
                  ChatMessage(
                    id: 'sys${DateTime.now().millisecondsSinceEpoch}',
                    role: MessageRole.system,
                    type: MessageType.systemNotice,
                    content: '聊天记录已清空',
                    time: DateTime.now(),
                  ),
                );
              });
              amitiaSnackBar(context, '聊天记录已清空');
            }
          });
        case 2:
          amitiaSnackBar(context, '聊天记录已导出到本地');
        case 3:
          amitiaSnackBar(context, '对话链接已复制');
        case 4:
          _showMessageSearch(context);
      }
    });
  }

  void _openDrawer(BuildContext context) {
    final scope = ShellDrawerScope.of(context);
    if (scope != null) {
      scope.openDrawer();
      return;
    }
    Scaffold.of(context).openDrawer();
  }

  @override
  Widget build(BuildContext context) {
    final isAgentMode = ref.watch(isAgentModeProvider);
    final characterId = ref.watch(currentCharacterIdProvider);
    final charactersAsync = ref.watch(characterListProvider);

    final character = charactersAsync.when(
      loading: () => null,
      error: (_, __) => null,
      data: (characters) {
        return characters.where((c) => c.id == characterId).firstOrNull;
      },
    );

    final avatarInitial = character?.name.isNotEmpty == true ? character!.name[0] : '?';
    final avatarColor = Color((character!.name.hashCode & 0xFFFFFF) | 0xFF000000);
    final characterName = character?.name ?? '';

    return AmitiaScaffold(
      resizeToAvoidBottomInset: false,
      body: Stack(
        key: const ValueKey('ime-single-scaffold-20260805-0325'),
        children: [
          Positioned.fill(
            child: SafeArea(
              bottom: false,
              child: Column(
                children: [
                  Expanded(
                    child: Stack(
                      children: [
                        ListView.builder(
                          controller: _scrollController,
                          padding: const EdgeInsets.symmetric(vertical: 32),
                          itemCount: _messages.length,
                          itemBuilder: (context, index) {
                            final message = _messages[index];
                            final isAgentTask =
                                message.type == MessageType.agentTask;
                            return AmitiaMessageBubble(
                              message: message,
                              showAvatar: _shouldShowAvatar(index),
                              avatarInitial: avatarInitial,
                              avatarColor: avatarColor,
                              characterName: characterName,
                              onRetry: message.status == MessageStatus.error
                                  ? () => _retryMessage(index)
                                  : null,
                              onAgentTaskTap: isAgentTask
                                  ? () =>
                                        context.push(AppRoutes.agentTask('t1'))
                                  : null,
                              onPauseAgentTask: isAgentTask
                                  ? () => _pauseAgentTask(index)
                                  : null,
                              onResumeAgentTask: isAgentTask
                                  ? () => _resumeAgentTask(index)
                                  : null,
                              agentTaskStatusLabel: isAgentTask
                                  ? (_agentTaskStatus[message.id] ?? '运行中')
                                  : null,
                            );
                          },
                        ),
                        _ChatScrollFade(
                          alignment: Alignment.topCenter,
                          begin: Alignment.topCenter,
                          end: Alignment.bottomCenter,
                          color: context.backgroundPrimary,
                          height: _chatTopBarHeight,
                        ),
                        _ChatScrollFade(
                          alignment: Alignment.bottomCenter,
                          begin: Alignment.bottomCenter,
                          end: Alignment.topCenter,
                          color: context.backgroundPrimary,
                        ),
                      ],
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
            ),
          ),
          Positioned(
            top: 0,
            left: 0,
            right: 0,
            child: _ChatTopBar(
              onOpenDrawer: () => _openDrawer(context),
              onNewConversation: () {
                final chatApi = ref.read(chatServiceProvider);
                chatApi.createConversation(characterId).then((conv) {
                  if (conv != null && mounted) {
                    setState(() {
                      _messages.clear();
                      _agentTaskStatus.clear();
                    });
                  }
                });
              },
              onMore: () => _showChatActionsSheet(context),
            ),
          ),
        ],
      ),
    );
  }
}

const double _chatTopBarHeight = 68;

class _ChatScrollFade extends StatelessWidget {
  final Alignment alignment;
  final Alignment begin;
  final Alignment end;
  final Color color;
  final double height;

  const _ChatScrollFade({
    required this.alignment,
    required this.begin,
    required this.end,
    required this.color,
    this.height = 32,
  });

  @override
  Widget build(BuildContext context) {
    return IgnorePointer(
      child: Align(
        alignment: alignment,
        child: Container(
          width: double.infinity,
          height: height,
          decoration: BoxDecoration(
            gradient: LinearGradient(
              begin: begin,
              end: end,
              colors: [color, color.withValues(alpha: 0)],
            ),
          ),
        ),
      ),
    );
  }
}

class _ChatTopBar extends StatelessWidget implements PreferredSizeWidget {
  final VoidCallback onOpenDrawer;
  final VoidCallback onNewConversation;
  final VoidCallback onMore;

  const _ChatTopBar({
    required this.onOpenDrawer,
    required this.onNewConversation,
    required this.onMore,
  });

  @override
  Size get preferredSize => const Size.fromHeight(_chatTopBarHeight);

  @override
  Widget build(BuildContext context) {
    final platform = Theme.of(context).platform;
    final isApplePlatform =
        platform == TargetPlatform.iOS ||
        platform == TargetPlatform.macOS;

    return SafeArea(
      bottom: false,
      child: Padding(
        padding: const EdgeInsets.fromLTRB(12, 8, 12, 8),
        child: Row(
          children: [
            ConduitStyleToolbarButton(
              icon: isApplePlatform
                  ? CupertinoIcons.line_horizontal_3
                  : Icons.menu,
              iconSize: 20,
              tooltip: '打开侧边栏',
              onPressed: onOpenDrawer,
            ),
            const Spacer(),
            Material(
              color: context.surfaceSecondary,
              shape: RoundedRectangleBorder(
                side: BorderSide(color: context.borderPrimary),
                borderRadius: BorderRadius.circular(20),
              ),
              child: SizedBox(
                height: 36,
                child: Row(
                  children: [
                    Tooltip(
                      message: '新建聊天',
                      child: InkWell(
                        borderRadius: const BorderRadius.horizontal(
                          left: Radius.circular(20),
                        ),
                        onTap: onNewConversation,
                        child: Padding(
                          padding: const EdgeInsets.fromLTRB(10, 0, 6, 0),
                          child: Icon(
                            Icons.edit_square,
                            size: 20,
                            color: context.textPrimary,
                          ),
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ),
            const SizedBox(width: 8),
            ConduitStyleToolbarButton(
              icon: isApplePlatform
                  ? CupertinoIcons.ellipsis
                  : Icons.more_vert,
              iconSize: 22,
              tooltip: '更多',
              onPressed: onMore,
            ),
          ],
        ),
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
    if (m.content.startsWith('__mock:image|'))
      return '[图片] ${m.content.split('|').last}';
    if (m.content.startsWith('__mock:video|'))
      return '[视频] ${m.content.split('|').last}';
    if (m.content.startsWith('__mock:audio|'))
      return '[语音] ${m.content.split('|').last}';
    if (m.content.startsWith('__mock:emote|')) return '[表情]';
    if (m.content.startsWith('__mock:code|'))
      return '[代码] ${m.content.split('|').last}';
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
                        child: Text(
                          '输入关键词后在当前会话中搜索消息',
                          style: AppTypography.caption(context),
                          textAlign: TextAlign.center,
                        ),
                      )
                    : results.isEmpty
                    ? Center(
                        child: Text(
                          '没有找到匹配「$_query」的消息',
                          style: AppTypography.caption(context),
                          textAlign: TextAlign.center,
                        ),
                      )
                    : ListView.separated(
                        itemCount: results.length,
                        separatorBuilder: (_, _) =>
                            Divider(height: 1, color: context.borderSecondary),
                        itemBuilder: (ctx, i) {
                          final item = results[i];
                          final msg = item.$2;
                          return ListTile(
                            leading: Icon(
                              msg.role == MessageRole.user
                                  ? Icons.person_outline
                                  : Icons.smart_toy_outlined,
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

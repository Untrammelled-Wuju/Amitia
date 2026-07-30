import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_message.dart';
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

  void _onSend(String text) {
    final now = DateTime.now();
    setState(() {
      _messages.add(ChatMessage(
        id: 'u${now.millisecondsSinceEpoch}',
        role: MessageRole.user,
        type: MessageType.text,
        content: text,
        time: now,
      ));
    });
    _scrollToBottom();

    Future.delayed(const Duration(milliseconds: 600), () {
      final replyTime = DateTime.now();
      setState(() {
        _messages.add(ChatMessage(
          id: 'a${replyTime.millisecondsSinceEpoch}',
          role: MessageRole.assistant,
          type: MessageType.text,
          content: '收到你的消息了，让我想想怎么回复你……',
          time: replyTime,
        ));
      });
      _scrollToBottom();
    });
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
        leading: AmitiaIconButton(
          icon: Icons.menu,
          onPressed: () => Scaffold.of(context).openDrawer(),
        ),
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
            onPressed: () => context.push('/conversations'),
          ),
          AmitiaIconButton(
            icon: Icons.more_horiz,
            onPressed: () {},
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
                return AmitiaMessageBubble(
                  message: message,
                  showAvatar: _shouldShowAvatar(index),
                  avatarInitial: character.avatarInitial,
                  avatarColor: character.avatarColor,
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
          ),
        ],
      ),
    );
  }
}

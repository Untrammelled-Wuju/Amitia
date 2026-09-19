import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/models/conversation.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/ui_runtime/channel_presentation.dart';
import '../../../../core/ui_runtime/ui_message_renderer_registry.dart';
import '../../../../core/ui_runtime/ui_provider_host.dart';
import '../../../../core/ui_runtime/ui_runtime_controller.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class ChannelMessagesPage extends ConsumerStatefulWidget {
  const ChannelMessagesPage({super.key});

  @override
  ConsumerState<ChannelMessagesPage> createState() =>
      _ChannelMessagesPageState();
}

class _ChannelMessagesPageState extends ConsumerState<ChannelMessagesPage> {
  List<ConversationDto> _conversations = const [];
  bool _loading = true;
  String _error = '';

  @override
  void initState() {
    super.initState();
    _load();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) _ensureChannelEntryAvailable();
    });
  }

  Future<void> _ensureChannelEntryAvailable() async {
    await ref.read(uiRuntimeProvider.notifier).ensureLoaded();
    if (!mounted) return;
    final snapshot = ref.read(uiRuntimeProvider).valueOrNull;
    if (ChannelPresentationRegistry.resolve(snapshot).isEmpty) {
      context.go(AppRoutes.chat);
    }
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = '';
    });
    try {
      final conversations = await ref
          .read(chatServiceProvider)
          .channelConversations();
      if (!mounted) return;
      setState(() => _conversations = conversations);
    } catch (error) {
      if (!mounted) return;
      setState(() => _error = error.toString());
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final snapshot = ref.watch(uiRuntimeProvider).valueOrNull;
    ref.listen(uiRuntimeProvider, (_, next) {
      final nextSnapshot = next.valueOrNull;
      if (mounted &&
          nextSnapshot != null &&
          ChannelPresentationRegistry.resolve(nextSnapshot).isEmpty) {
        context.go(AppRoutes.chat);
      }
    });
    final presentations = ChannelPresentationRegistry.resolve(snapshot);
    final presentationsByChannel = <String, ChannelPresentationDescriptor>{
      for (final item in presentations) item.channelId: item,
    };
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '渠道消息', showBackButton: true),
      body: RefreshIndicator(
        onRefresh: _load,
        child: _loading
            ? const Center(child: CircularProgressIndicator())
            : _error.isNotEmpty
            ? Center(child: Text(_error))
            : _conversations.isEmpty
            ? const Center(child: Text('暂无渠道消息'))
            : ListView.separated(
                padding: EdgeInsets.all(AppSpacing.pagePadding),
                itemCount: _conversations.length,
                separatorBuilder: (_, _) => SizedBox(height: AppSpacing.sm),
                itemBuilder: (context, index) {
                  final conversation = _conversations[index];
                  return ListTile(
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(12),
                    ),
                    tileColor: context.surfacePrimary,
                    leading: const Icon(Icons.forum_outlined),
                    title: Text(
                      conversation.title.trim().isEmpty
                          ? _channelLabel(
                              conversation.channel,
                              presentationsByChannel,
                            )
                          : conversation.title,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                    subtitle: Text(
                      '${_channelLabel(conversation.channel, presentationsByChannel)} · ${conversation.messageCount} 条',
                    ),
                    trailing: const Icon(Icons.chevron_right),
                    onTap: () => Navigator.of(context).push(
                      MaterialPageRoute<void>(
                        builder: (_) => ChannelMessageDetailPage(
                          conversation: conversation,
                          channelName: _channelLabel(
                            conversation.channel,
                            presentationsByChannel,
                          ),
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

class ChannelMessageDetailPage extends ConsumerStatefulWidget {
  final ConversationDto conversation;
  final String channelName;

  const ChannelMessageDetailPage({
    super.key,
    required this.conversation,
    required this.channelName,
  });

  @override
  ConsumerState<ChannelMessageDetailPage> createState() =>
      _ChannelMessageDetailPageState();
}

class _ChannelMessageDetailPageState
    extends ConsumerState<ChannelMessageDetailPage> {
  List<MessageDto> _messages = const [];
  bool _loading = true;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final messages = await ref
          .read(chatServiceProvider)
          .getMessages(widget.conversation.id);
      if (!mounted) return;
      setState(() => _messages = messages);
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final snapshot = ref.watch(uiRuntimeProvider).valueOrNull;
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: widget.conversation.title.trim().isEmpty
            ? widget.channelName
            : widget.conversation.title,
        showBackButton: true,
      ),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _messages.isEmpty
          ? const Center(child: Text('暂无消息'))
          : ListView.separated(
              padding: EdgeInsets.all(AppSpacing.pagePadding),
              itemCount: _messages.length,
              separatorBuilder: (_, _) => SizedBox(height: AppSpacing.sm),
              itemBuilder: (context, index) {
                final message = _messages[index];
                final provider = UIMessageRendererRegistry.resolve(
                  snapshot,
                  messageType: message.msgType,
                  role: message.role,
                  channelId: widget.conversation.channel,
                );
                final payload = _channelMessage(message);
                return Align(
                  alignment: message.role == 'user'
                      ? Alignment.centerRight
                      : Alignment.centerLeft,
                  child: UIProviderHost(
                    capability: 'conversation.message_renderer',
                    providerId: provider?.providerId,
                    fallback: _ChannelMessageBubble(message: message),
                    context: <String, dynamic>{
                      'channelId': widget.conversation.channel,
                      'conversationId': widget.conversation.id,
                      'readOnly': true,
                      'message': payload,
                    },
                  ),
                );
              },
            ),
    );
  }

  Map<String, dynamic> _channelMessage(MessageDto message) => <String, dynamic>{
    'id': message.id,
    'conversationId': message.conversationId,
    'channelId': widget.conversation.channel,
    'role': message.role,
    'content': message.content,
    'msgType': message.msgType,
    'extensionType': message.extensionType,
    'imageUrl': message.imageUrl,
    'audioUrl': message.audioUrl,
    'audioDuration': message.audioDuration,
    'videoUrl': message.videoUrl,
    'createdAt': message.createdAt,
  };
}

String _channelLabel(
  String channel, [
  Map<String, ChannelPresentationDescriptor> presentations = const {},
]) {
  final presentation = presentations[channel];
  if (presentation != null && presentation.displayName.isNotEmpty) {
    return presentation.displayName;
  }
  return channel.isEmpty ? '渠道' : channel;
}

class _ChannelMessageBubble extends StatelessWidget {
  final MessageDto message;

  const _ChannelMessageBubble({required this.message});

  @override
  Widget build(BuildContext context) {
    final imageUrl = message.imageUrl.trim();
    final videoUrl = message.videoUrl.trim();
    final audioUrl = message.audioUrl.trim();
    return Container(
      constraints: const BoxConstraints(maxWidth: 320),
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 9),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: BorderRadius.circular(12),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(message.role, style: AppTypography.label(context)),
          if (imageUrl.isNotEmpty) ...[
            const SizedBox(height: 6),
            ClipRRect(
              borderRadius: BorderRadius.circular(8),
              child: Image.network(
                imageUrl,
                fit: BoxFit.contain,
                errorBuilder: (_, _, _) => const SizedBox.shrink(),
              ),
            ),
          ],
          if (videoUrl.isNotEmpty) ...[
            const SizedBox(height: 6),
            Text(videoUrl, style: AppTypography.body(context)),
          ],
          if (audioUrl.isNotEmpty) ...[
            const SizedBox(height: 6),
            Text(audioUrl, style: AppTypography.body(context)),
          ],
          if (message.content.trim().isNotEmpty) ...[
            const SizedBox(height: 4),
            Text(message.content, style: AppTypography.body(context)),
          ],
        ],
      ),
    );
  }
}

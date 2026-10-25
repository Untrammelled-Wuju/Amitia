import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../app/amitia_page_route.dart';
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
import '../../../../core/widgets/compact_empty_state.dart';

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
            ? _error.contains('没有可用角色')
                  ? ListView(
                      padding: EdgeInsets.all(AppSpacing.pagePadding),
                      children: [
                        CompactEmptyState(
                          icon: Icons.person_add_alt_1_outlined,
                          title: '暂无渠道消息',
                          message: '创建角色并接入渠道后，渠道消息会显示在这里。',
                          actionLabel: '创建角色',
                          onAction: () =>
                              context.push(AppRoutes.charactersCreate),
                        ),
                      ],
                    )
                  : Center(child: Text(_error))
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
                      amitiaSlideRoute<void>(
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
    final content = message.content.trim();
    final hasMedia =
        imageUrl.isNotEmpty || videoUrl.isNotEmpty || audioUrl.isNotEmpty;
    final placeholder = RegExp(r'^\[(图片|视频|语音|音频|文件)\]$').hasMatch(content);
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
              child: _channelImage(context, imageUrl),
            ),
          ],
          if (videoUrl.isNotEmpty) ...[
            const SizedBox(height: 6),
            Row(
              children: [
                const Icon(Icons.play_circle_outline, size: 18),
                const SizedBox(width: 6),
                Expanded(
                  child: Text('视频消息', style: AppTypography.body(context)),
                ),
              ],
            ),
          ],
          if (audioUrl.isNotEmpty) ...[
            const SizedBox(height: 6),
            Row(
              children: [
                const Icon(Icons.graphic_eq, size: 18),
                const SizedBox(width: 6),
                Expanded(
                  child: Text(
                    message.audioDuration > 0
                        ? '语音消息 ${message.audioDuration.toStringAsFixed(1)} 秒'
                        : '语音消息',
                    style: AppTypography.body(context),
                  ),
                ),
              ],
            ),
          ],
          if (content.isNotEmpty && !(hasMedia && placeholder)) ...[
            const SizedBox(height: 4),
            Text(content, style: AppTypography.body(context)),
          ],
        ],
      ),
    );
  }

  Widget _channelImage(BuildContext context, String url) {
    if (!url.startsWith('data:')) {
      return Image.network(
        url,
        fit: BoxFit.contain,
        errorBuilder: (_, _, _) =>
            Text('图片加载失败', style: AppTypography.label(context)),
      );
    }
    final separator = url.indexOf(',');
    if (separator < 0 || !url.substring(0, separator).endsWith(';base64')) {
      return Text('图片格式不受支持', style: AppTypography.label(context));
    }
    try {
      final bytes = base64Decode(url.substring(separator + 1));
      return Image.memory(
        bytes,
        fit: BoxFit.contain,
        errorBuilder: (_, _, _) =>
            Text('图片解码失败', style: AppTypography.label(context)),
      );
    } catch (_) {
      return Text('图片解码失败', style: AppTypography.label(context));
    }
  }
}

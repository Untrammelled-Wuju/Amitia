import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/models/conversation.dart';
import '../../../../core/services/providers.dart';
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
                          ? _channelLabel(conversation.channel)
                          : conversation.title,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                    subtitle: Text(
                      '${_channelLabel(conversation.channel)} · ${conversation.messageCount} 条',
                    ),
                    trailing: const Icon(Icons.chevron_right),
                    onTap: () => Navigator.of(context).push(
                      MaterialPageRoute<void>(
                        builder: (_) => ChannelMessageDetailPage(
                          conversation: conversation,
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

  const ChannelMessageDetailPage({super.key, required this.conversation});

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
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: widget.conversation.title.trim().isEmpty
            ? _channelLabel(widget.conversation.channel)
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
                      color: context.surfacePrimary,
                      borderRadius: BorderRadius.circular(12),
                    ),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(message.role, style: AppTypography.label(context)),
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

String _channelLabel(String channel) {
  const labels = <String, String>{
    'qq': 'QQ',
    'wechat': '微信',
    'wechat_personal': '个人微信',
  };
  return labels[channel] ?? (channel.isEmpty ? '渠道' : channel);
}

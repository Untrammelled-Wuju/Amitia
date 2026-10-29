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
import '../../../../core/ui_runtime/channel_plugin_navigation.dart';
import '../../../../core/ui_runtime/channel_presentation.dart';
import '../../../../core/ui_runtime/ui_message_renderer_registry.dart';
import '../../../../core/ui_runtime/ui_provider_host.dart';
import '../../../../core/ui_runtime/ui_runtime_controller.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/compact_empty_state.dart';

class ChannelMessagesPage extends ConsumerWidget {
  const ChannelMessagesPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final snapshot = ref.watch(uiRuntimeProvider).valueOrNull;
    final channels = ChannelPluginNavigationRegistry.resolve(snapshot);
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '渠道消息', showBackButton: true),
      body: channels.isEmpty
          ? Padding(
              padding: EdgeInsets.all(AppSpacing.pagePadding),
              child: CompactEmptyState(
                icon: Icons.extension_off_outlined,
                title: '暂无渠道插件',
                message: '安装并启用渠道插件后，插件注册的页面会显示在这里。',
              ),
            )
          : Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Padding(
                  padding: EdgeInsets.fromLTRB(
                    AppSpacing.pagePadding,
                    AppSpacing.md,
                    AppSpacing.pagePadding,
                    AppSpacing.sm,
                  ),
                  child: Text(
                    '已接入渠道',
                    style: AppTypography.sectionTitle(context),
                  ),
                ),
                SizedBox(
                  height: 82,
                  child: ListView.separated(
                    scrollDirection: Axis.horizontal,
                    padding: EdgeInsets.symmetric(
                      horizontal: AppSpacing.pagePadding,
                    ),
                    itemCount: channels.length,
                    separatorBuilder: (_, _) => SizedBox(width: AppSpacing.sm),
                    itemBuilder: (context, index) {
                      final channel = channels[index];
                      return Material(
                        color: context.surfacePrimary,
                        borderRadius: BorderRadius.circular(14),
                        child: InkWell(
                          key: ValueKey('channel-plugin-${channel.channelId}'),
                          borderRadius: BorderRadius.circular(14),
                          onTap: () => context.push(channel.route),
                          child: Container(
                            width: 172,
                            padding: EdgeInsets.all(AppSpacing.md),
                            decoration: BoxDecoration(
                              borderRadius: BorderRadius.circular(14),
                              border: Border.all(
                                color: context.borderPrimary,
                                width: 0.6,
                              ),
                            ),
                            child: Row(
                              children: [
                                Icon(
                                  Icons.forum_outlined,
                                  color: context.accentPrimary,
                                ),
                                SizedBox(width: AppSpacing.sm),
                                Expanded(
                                  child: Column(
                                    mainAxisAlignment: MainAxisAlignment.center,
                                    crossAxisAlignment:
                                        CrossAxisAlignment.start,
                                    children: [
                                      Text(
                                        channel.label,
                                        maxLines: 1,
                                        overflow: TextOverflow.ellipsis,
                                        style: AppTypography.body(
                                          context,
                                        ).copyWith(fontWeight: FontWeight.w600),
                                      ),
                                      const SizedBox(height: 4),
                                      Text(
                                        '打开插件页面',
                                        style: AppTypography.caption(context),
                                      ),
                                    ],
                                  ),
                                ),
                                const Icon(Icons.chevron_right, size: 18),
                              ],
                            ),
                          ),
                        ),
                      );
                    },
                  ),
                ),
                Padding(
                  padding: EdgeInsets.fromLTRB(
                    AppSpacing.pagePadding,
                    AppSpacing.lg,
                    AppSpacing.pagePadding,
                    AppSpacing.sm,
                  ),
                  child: Text(
                    '渠道会话',
                    style: AppTypography.sectionTitle(context),
                  ),
                ),
                const Expanded(child: ChannelHostMessagesView()),
              ],
            ),
    );
  }
}

class ChannelHostMessagesView extends ConsumerStatefulWidget {
  final String channelId;
  final bool showSummary;

  const ChannelHostMessagesView({
    super.key,
    this.channelId = '',
    this.showSummary = false,
  });

  @override
  ConsumerState<ChannelHostMessagesView> createState() =>
      _ChannelHostMessagesViewState();
}

class _ChannelHostMessagesViewState
    extends ConsumerState<ChannelHostMessagesView> {
  List<ConversationDto> _conversations = const [];
  bool _loading = true;
  String _error = '';

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void didUpdateWidget(covariant ChannelHostMessagesView oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.channelId != widget.channelId) {
      _load();
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
      setState(() {
        _conversations = _filterChannelConversations(
          conversations,
          widget.channelId,
        );
      });
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
    final presentation = presentationsByChannel[widget.channelId];
    final conversations = _filterChannelConversations(
      _conversations,
      widget.channelId,
      singleConversation: presentation?.singleConversation ?? false,
    );
    if (_loading) return const Center(child: CircularProgressIndicator());
    late Widget content;
    if (_error.isNotEmpty) {
      if (!_error.contains('没有可用角色')) {
        content = Center(child: Text(_error));
      } else {
        content = ListView(
          padding: EdgeInsets.all(AppSpacing.pagePadding),
          children: [
            CompactEmptyState(
              icon: Icons.person_add_alt_1_outlined,
              title: '暂无渠道消息',
              message: '创建角色并接入渠道后，渠道消息会显示在这里。',
              actionLabel: '创建角色',
              onAction: () => context.push(AppRoutes.charactersCreate),
            ),
          ],
        );
      }
    } else {
      content = RefreshIndicator(
        onRefresh: _load,
        child: conversations.isEmpty
            ? ListView(
                physics: const AlwaysScrollableScrollPhysics(),
                children: [
                  SizedBox(height: AppSpacing.xxl),
                  const Center(child: Text('暂无渠道消息')),
                ],
              )
            : ListView.separated(
                padding: EdgeInsets.fromLTRB(
                  AppSpacing.pagePadding,
                  0,
                  AppSpacing.pagePadding,
                  AppSpacing.xl,
                ),
                itemCount: conversations.length,
                separatorBuilder: (_, _) => SizedBox(height: AppSpacing.sm),
                itemBuilder: (context, index) {
                  final conversation = conversations[index];
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
      );
    }
    if (widget.showSummary &&
        presentation?.singleConversation == true &&
        conversations.isNotEmpty) {
      content = ChannelMessageDetailPage(
        key: ValueKey(conversations.first.id),
        conversation: conversations.first,
        channelName: presentation!.displayName,
        embedded: true,
      );
    }
    if (!widget.showSummary) return content;
    final totalMessages = conversations.fold<int>(
      0,
      (total, conversation) => total + conversation.messageCount,
    );
    final groupCount = conversations
        .where((conversation) => _conversationPeerKind(conversation) == 'group')
        .length;
    final privateCount = conversations.length - groupCount;
    final channelTitle =
        presentationsByChannel[widget.channelId]?.displayName ??
        widget.channelId;
    final lastActivity = conversations
        .map((conversation) => conversation.updatedAt)
        .where((value) => value.trim().isNotEmpty)
        .fold<String>(
          '',
          (latest, value) => value.compareTo(latest) > 0 ? value : latest,
        );
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Padding(
          padding: EdgeInsets.fromLTRB(
            AppSpacing.pagePadding,
            AppSpacing.md,
            AppSpacing.pagePadding,
            AppSpacing.sm,
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                channelTitle.trim().isEmpty ? '消息统计' : channelTitle,
                style: AppTypography.sectionTitle(context),
              ),
              const SizedBox(height: 4),
              Text('展示宿主已保存的渠道会话和消息。', style: AppTypography.caption(context)),
            ],
          ),
        ),
        Padding(
          padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
          child: GridView.count(
            crossAxisCount: 2,
            shrinkWrap: true,
            physics: const NeverScrollableScrollPhysics(),
            crossAxisSpacing: AppSpacing.sm,
            mainAxisSpacing: AppSpacing.sm,
            childAspectRatio: 2.35,
            children: [
              _ChannelMetric(
                label: '会话数',
                value: conversations.length.toString(),
              ),
              _ChannelMetric(
                label: '群聊 / 私聊',
                value: '$groupCount / $privateCount',
              ),
              _ChannelMetric(label: '宿主消息数', value: totalMessages.toString()),
              _ChannelMetric(
                label: '最近活动',
                value: _formatActivity(lastActivity),
                compact: true,
              ),
            ],
          ),
        ),
        Padding(
          padding: EdgeInsets.fromLTRB(
            AppSpacing.pagePadding,
            AppSpacing.md,
            AppSpacing.pagePadding,
            AppSpacing.sm,
          ),
          child: Text(
            presentation?.singleConversation == true ? '对话消息' : '当前渠道的对话',
            style: AppTypography.sectionTitle(context),
          ),
        ),
        Expanded(child: content),
      ],
    );
  }
}

String _conversationPeerKind(ConversationDto conversation) {
  final origin = conversation.conversationOrigin;
  final explicit = (origin?['peerKind'] ?? origin?['peer_kind'] ?? '')
      .toString()
      .trim()
      .toLowerCase();
  if (explicit == 'group' || explicit == 'private') return explicit;
  final resource = [
    conversation.id,
    conversation.sourceResourceId,
    conversation.title,
  ].join(' ').toLowerCase();
  return resource.contains('group') || resource.contains('群')
      ? 'group'
      : 'private';
}

String _formatActivity(String raw) {
  final parsed = DateTime.tryParse(raw);
  if (parsed == null) return raw.trim().isEmpty ? '—' : raw;
  final local = parsed.toLocal();
  String two(int value) => value.toString().padLeft(2, '0');
  return '${two(local.month)}-${two(local.day)} ${two(local.hour)}:${two(local.minute)}';
}

class _ChannelMetric extends StatelessWidget {
  final String label;
  final String value;
  final bool compact;

  const _ChannelMetric({
    required this.label,
    required this.value,
    this.compact = false,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: EdgeInsets.all(AppSpacing.md),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: context.borderPrimary, width: 0.6),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Text(label, style: AppTypography.caption(context)),
          const SizedBox(height: 4),
          Text(
            value,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: compact
                ? AppTypography.bodySmall(
                    context,
                  ).copyWith(fontWeight: FontWeight.w600)
                : AppTypography.sectionTitle(context),
          ),
        ],
      ),
    );
  }
}

List<ConversationDto> _filterChannelConversations(
  List<ConversationDto> conversations,
  String channelId, {
  bool singleConversation = false,
}) {
  final filtered = channelId.trim().isEmpty
      ? conversations
      : conversations
            .where((conversation) => conversation.channel == channelId)
            .toList(growable: false);
  return singleConversation
      ? filtered.take(1).toList(growable: false)
      : filtered;
}

class ChannelMessageDetailPage extends ConsumerStatefulWidget {
  final ConversationDto conversation;
  final String channelName;
  final bool embedded;

  const ChannelMessageDetailPage({
    super.key,
    required this.conversation,
    required this.channelName,
    this.embedded = false,
  });

  @override
  ConsumerState<ChannelMessageDetailPage> createState() =>
      _ChannelMessageDetailPageState();
}

class _ChannelMessageDetailPageState
    extends ConsumerState<ChannelMessageDetailPage> {
  List<MessageDto> _messages = const [];
  bool _loading = true;
  String _messageError = '';

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
    } catch (error) {
      if (mounted) setState(() => _messageError = error.toString());
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final snapshot = ref.watch(uiRuntimeProvider).valueOrNull;
    final body = _messageError.isNotEmpty
        ? Center(child: Text(_messageError))
        : _loading
        ? const Center(child: CircularProgressIndicator())
        : _messages.isEmpty
        ? const Center(child: Text('暂无消息'))
        : _buildMessages(context, snapshot);
    if (widget.embedded) return body;
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: widget.conversation.title.trim().isEmpty
            ? widget.channelName
            : widget.conversation.title,
        showBackButton: true,
      ),
      body: body,
    );
  }

  Widget _buildMessages(BuildContext context, dynamic snapshot) =>
      ListView.separated(
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
      );

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

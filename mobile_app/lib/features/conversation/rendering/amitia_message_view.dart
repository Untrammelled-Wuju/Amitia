import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../../../shared/models/models.dart';
import 'amitia_message_theme.dart';
import 'amrp.dart';
import 'markdown/amitia_markdown.dart';
import 'rich_blocks/amitia_citation_list.dart';
import 'rich_blocks/amitia_rich_blocks.dart';
import 'rich_blocks/renderer_registry.dart';

class AmitiaMessageView extends StatefulWidget {
  final ChatMessage message;
  final String characterId;
  final String characterName;
  final String avatarInitial;
  final String avatarColor;
  final bool showAvatar;
  final String roleLabel;
  final bool showThinking;
  final List<AmrpToolBlock> toolBlocks;
  final VoidCallback? onRetry;
  final VoidCallback? onReply;
  final VoidCallback? onCopy;

  const AmitiaMessageView({
    super.key,
    required this.message,
    this.characterId = '',
    this.characterName = 'Amitia',
    this.avatarInitial = 'A',
    this.avatarColor = '#7060E8',
    this.showAvatar = true,
    this.roleLabel = '默认角色',
    this.showThinking = false,
    this.toolBlocks = const <AmrpToolBlock>[],
    this.onRetry,
    this.onReply,
    this.onCopy,
  });

  @override
  State<AmitiaMessageView> createState() => _AmitiaMessageViewState();
}

class _AmitiaMessageViewState extends State<AmitiaMessageView> {
  String _highlightCitation = '';

  @override
  void initState() {
    super.initState();
    AmrpRendererRegistry.registerExtension(
      'minecraft.server',
      (context, block) => _MinecraftServerBlock(block: block),
    );
  }

  @override
  Widget build(BuildContext context) {
    final tokens = AmitiaMessageTheme.of(context);
    final message = AmrpMessage.fromChatMessage(
      widget.message,
      character: AmrpCharacter(
        id: widget.characterId,
        name: widget.characterName,
        avatar: widget.avatarColor,
      ),
    );
    if (message.role == AmrpMessageRole.system) {
      return _SystemNotice(message: message, tokens: tokens);
    }
    if (message.role == AmrpMessageRole.user) {
      return _UserMessage(message: message, tokens: tokens);
    }
    return Center(
      child: ConstrainedBox(
        constraints: BoxConstraints(maxWidth: tokens.messageMaxWidth),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            if (widget.showAvatar)
              _Avatar(
                size: tokens.avatarSize,
                initial: widget.avatarInitial,
                colorHex: widget.avatarColor,
              )
            else
              SizedBox(width: tokens.avatarSize, height: tokens.avatarSize),
            SizedBox(width: tokens.messageGap),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  _MessageHead(
                    name: widget.characterName,
                    role: widget.roleLabel,
                    time: _formatTime(message.createdAt),
                    tokens: tokens,
                  ),
                  if (message.thinking != null)
                    AmitiaThinkingBlock(block: message.thinking!),
                  if (widget.showThinking && message.thinking == null)
                    AmitiaThinkingBlock(
                      block: AmrpThinkingBlock(
                        content: '正在思考并组织回复',
                        state: AmrpMessageState.streaming,
                      ),
                    ),
                  if (_stateNotice(message.state) != null)
                    _StateNotice(
                      data: _stateNotice(message.state)!,
                      state: message.state,
                      tokens: tokens,
                    ),
                  if (message.markdown.isNotEmpty)
                    AmitiaMarkdownView(
                      source: message.markdown,
                      streaming: message.state == AmrpMessageState.streaming,
                      onCitation: (id) =>
                          setState(() => _highlightCitation = id),
                    ),
                  ..._renderBlocks(message.blocks, message.id),
                  for (final tool in widget.toolBlocks)
                    AmitiaToolBlock(block: tool),
                  AmitiaCitationList(
                    sources: message.sources,
                    highlightId: _highlightCitation,
                  ),
                  if (!(widget.showThinking && message.markdown.trim().isEmpty))
                    _MessageActions(
                      streaming: message.state == AmrpMessageState.streaming,
                      onCopy: () => _copy(context, message.plainText),
                      onCopyMarkdown: () => _copy(context, message.markdown),
                      onReply: widget.onReply,
                      onRetry: widget.onRetry,
                    ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  List<Widget> _renderBlocks(List<AmrpRichBlock> blocks, String messageId) {
    final widgets = <Widget>[];
    final images = <AmrpImageBlock>[];
    void flushImages() {
      if (images.isEmpty) return;
      widgets.add(AmitiaImageBlock(images: List<AmrpImageBlock>.from(images)));
      images.clear();
    }

    for (final block in blocks) {
      if (block is AmrpImageBlock) {
        images.add(block);
        continue;
      }
      flushImages();
      widgets.add(KeyedSubtree(
        key: ValueKey<String>('$messageId:${block.id}'),
        child: AmitiaRichBlockRenderer(block: block),
      ));
    }
    flushImages();
    return widgets;
  }

  Map<String, String>? _stateNotice(AmrpMessageState state) {
    return switch (state) {
      AmrpMessageState.interrupted => <String, String>{
        'title': '已中断',
        'detail': '保留已生成内容 · 可继续生成',
      },
      AmrpMessageState.failed => <String, String>{
        'title': '生成失败',
        'detail': '网络错误 · 可重试',
      },
      AmrpMessageState.cancelled => <String, String>{
        'title': '已取消',
        'detail': '用户主动停止生成',
      },
      AmrpMessageState.queued => <String, String>{
        'title': '等待开始生成',
        'detail': '任务已进入队列',
      },
      _ => null,
    };
  }

  String _formatTime(DateTime value) {
    final hour = value.hour.toString().padLeft(2, '0');
    final minute = value.minute.toString().padLeft(2, '0');
    return '$hour:$minute';
  }

  Future<void> _copy(BuildContext context, String value) async {
    await Clipboard.setData(ClipboardData(text: value));
    if (!context.mounted) return;
    ScaffoldMessenger.of(
      context,
    ).showSnackBar(const SnackBar(content: Text('已复制')));
  }
}

class _MessageHead extends StatelessWidget {
  final String name;
  final String role;
  final String time;
  final AmitiaMessageTheme tokens;

  const _MessageHead({
    required this.name,
    required this.role,
    required this.time,
    required this.tokens,
  });

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      height: tokens.avatarSize,
      child: Row(
        children: [
          Flexible(
            child: Text(
              name,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(
                color: tokens.text,
                fontSize: 14,
                fontWeight: FontWeight.w700,
              ),
            ),
          ),
          const SizedBox(width: 7),
          Text(
            role,
            style: TextStyle(color: tokens.muted, fontSize: 11),
          ),
          const SizedBox(width: 7),
          Text(
            time,
            style: TextStyle(
              color: tokens.muted.withValues(alpha: 0.75),
              fontSize: 11,
            ),
          ),
        ],
      ),
    );
  }
}

class _Avatar extends StatelessWidget {
  final double size;
  final String initial;
  final String colorHex;

  const _Avatar({
    required this.size,
    required this.initial,
    required this.colorHex,
  });

  @override
  Widget build(BuildContext context) {
    final color = _parseColor(colorHex);
    return Container(
      width: size,
      height: size,
      alignment: Alignment.center,
      decoration: BoxDecoration(
        color: color,
        borderRadius: BorderRadius.circular(size * 0.32),
      ),
      child: Text(
        initial.trim().isEmpty ? 'A' : initial.trim().characters.first,
        style: const TextStyle(
          color: Colors.white,
          fontSize: 12,
          fontWeight: FontWeight.w800,
        ),
      ),
    );
  }

  Color _parseColor(String value) {
    final normalized = value.replaceFirst('#', '');
    final parsed = int.tryParse(normalized, radix: 16);
    if (parsed == null) return const Color(0xFF7060E8);
    return Color(0xFF000000 | parsed);
  }
}

class _StateNotice extends StatelessWidget {
  final Map<String, String> data;
  final AmrpMessageState state;
  final AmitiaMessageTheme tokens;

  const _StateNotice({
    required this.data,
    required this.state,
    required this.tokens,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      constraints: const BoxConstraints(maxWidth: 700),
      margin: const EdgeInsets.only(bottom: 10),
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
      decoration: BoxDecoration(
        color: tokens.surface,
        border: Border.all(
          color: state == AmrpMessageState.failed
              ? tokens.danger.withValues(alpha: 0.45)
              : tokens.line,
        ),
        borderRadius: BorderRadius.circular(tokens.toolRadius),
      ),
      child: Row(
        children: [
          Text(
            data['title']!,
            style: TextStyle(
              color: tokens.text,
              fontSize: 11,
              fontWeight: FontWeight.w700,
            ),
          ),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              data['detail']!,
              style: TextStyle(color: tokens.muted, fontSize: 11),
            ),
          ),
        ],
      ),
    );
  }
}

class _MessageActions extends StatelessWidget {
  final bool streaming;
  final VoidCallback onCopy;
  final VoidCallback onCopyMarkdown;
  final VoidCallback? onReply;
  final VoidCallback? onRetry;

  const _MessageActions({
    required this.streaming,
    required this.onCopy,
    required this.onCopyMarkdown,
    this.onReply,
    this.onRetry,
  });

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(top: 9),
      child: Wrap(
        spacing: 2,
        children: [
          TextButton(onPressed: onCopy, child: const Text('复制')),
          TextButton(
            onPressed: onCopyMarkdown,
            child: const Text('复制 Markdown'),
          ),
          if (!streaming && onReply != null)
            TextButton(onPressed: onReply, child: const Text('回复')),
          if (!streaming && onRetry != null)
            TextButton(onPressed: onRetry, child: const Text('重新生成')),
        ],
      ),
    );
  }
}

class _SystemNotice extends StatelessWidget {
  final AmrpMessage message;
  final AmitiaMessageTheme tokens;

  const _SystemNotice({
    required this.message,
    required this.tokens,
  });

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Container(
        constraints: const BoxConstraints(maxWidth: 700),
        margin: const EdgeInsets.symmetric(vertical: 13),
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 7),
        decoration: BoxDecoration(
          color: tokens.soft,
          borderRadius: BorderRadius.circular(8),
        ),
        child: Text(
          message.markdown,
          textAlign: TextAlign.center,
          style: TextStyle(color: tokens.muted, fontSize: 11),
        ),
      ),
    );
  }
}

class _UserMessage extends StatelessWidget {
  final AmrpMessage message;
  final AmitiaMessageTheme tokens;

  const _UserMessage({
    required this.message,
    required this.tokens,
  });

  @override
  Widget build(BuildContext context) {
    return Align(
      alignment: Alignment.centerRight,
      child: Container(
        constraints: const BoxConstraints(maxWidth: 620),
        margin: const EdgeInsets.only(bottom: 34),
        padding: const EdgeInsets.symmetric(horizontal: 15, vertical: 11),
        decoration: BoxDecoration(
          color: tokens.userBubble,
          borderRadius: const BorderRadius.only(
            topLeft: Radius.circular(18),
            topRight: Radius.circular(6),
            bottomLeft: Radius.circular(18),
            bottomRight: Radius.circular(18),
          ),
        ),
        child: SelectableText(
          message.markdown,
          style: TextStyle(
            color: tokens.text,
            fontSize: 14,
            height: 1.6,
          ),
        ),
      ),
    );
  }
}

class _MinecraftServerBlock extends StatelessWidget {
  final AmrpExtensionBlock block;

  const _MinecraftServerBlock({required this.block});

  @override
  Widget build(BuildContext context) {
    final tokens = AmitiaMessageTheme.of(context);
    final payload = block.payload is Map
        ? Map<String, dynamic>.from(block.payload! as Map)
        : <String, dynamic>{};
    final detail = [
      payload['player']?.toString() ?? '',
      payload['address']?.toString() ?? '',
      payload['tps'] == null ? '' : '${payload['tps']} TPS',
    ].where((value) => value.isNotEmpty).join(' · ');
    return Container(
      constraints: const BoxConstraints(maxWidth: 700),
      margin: const EdgeInsets.only(bottom: 16),
      clipBehavior: Clip.antiAlias,
      decoration: BoxDecoration(
        color: tokens.surface,
        border: Border.all(color: tokens.line),
        borderRadius: BorderRadius.circular(tokens.codeRadius),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Container(
            height: tokens.codeToolbarHeight,
            padding: const EdgeInsets.symmetric(horizontal: 10),
            decoration: BoxDecoration(
              color: tokens.soft,
              border: Border(bottom: BorderSide(color: tokens.line)),
            ),
            child: Row(
              children: [
                Expanded(
                  child: Text(
                    'conversation.renderer · minecraft.server',
                    style: TextStyle(
                      color: tokens.text,
                      fontSize: 11,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                ),
                Text(
                  'Extension Block',
                  style: TextStyle(color: tokens.muted, fontSize: 10),
                ),
              ],
            ),
          ),
          Padding(
            padding: const EdgeInsets.all(12),
            child: Container(
              padding: const EdgeInsets.symmetric(horizontal: 14),
              decoration: BoxDecoration(
                gradient: LinearGradient(
                  colors: [tokens.accentSoft, tokens.soft],
                ),
                borderRadius: BorderRadius.circular(8),
              ),
              child: Row(
                children: [
                  Container(
                    width: 36,
                    height: 36,
                    alignment: Alignment.center,
                    decoration: BoxDecoration(
                      color: tokens.accent,
                      borderRadius: BorderRadius.circular(10),
                    ),
                    child: const Text(
                      'M',
                      style: TextStyle(
                        color: Colors.white,
                        fontSize: 16,
                      ),
                    ),
                  ),
                  const SizedBox(width: 10),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Padding(
                          padding: const EdgeInsets.only(top: 14),
                          child: Text(
                            payload['title']?.toString() ??
                                'Minecraft 服务器已连接',
                            style: TextStyle(
                              color: tokens.text,
                              fontWeight: FontWeight.w700,
                            ),
                          ),
                        ),
                        Padding(
                          padding: const EdgeInsets.only(bottom: 14),
                          child: Text(
                            detail,
                            style: TextStyle(
                              color: tokens.muted,
                              fontSize: 10.5,
                            ),
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}

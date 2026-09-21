import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../../../shared/models/models.dart';
import '../../../core/widgets/amitia_popup_menu.dart';
import '../../../core/models/conversation.dart';
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
  final bool showHeader;
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
    this.showHeader = true,
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
    final streaming =
        message.state == AmrpMessageState.streaming ||
        message.state == AmrpMessageState.queued;
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
                  if (widget.showHeader)
                    _MessageHead(
                      name: widget.characterName,
                      role: widget.roleLabel,
                      time: _formatTime(message.createdAt),
                      tokens: tokens,
                    ),
                  if (_stateNotice(message.state) != null)
                    _StateNotice(
                      data: _stateNotice(message.state)!,
                      state: message.state,
                      tokens: tokens,
                    ),
                  if (widget.message.assistantTurn != null)
                    ..._renderAssistantTurn(
                      widget.message.assistantTurn!,
                    )
                  else ...[
                    if (message.thinking != null)
                      AmitiaThinkingBlock(block: message.thinking!),
                    if (widget.showThinking && message.thinking == null)
                      AmitiaThinkingBlock(
                        block: AmrpThinkingBlock(
                          content: '',
                          state: AmrpMessageState.streaming,
                        ),
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
                  ],
                  AmitiaCitationList(
                    sources: message.sources,
                    highlightId: _highlightCitation,
                  ),
                  if (!streaming &&
                      !(widget.showThinking && message.markdown.trim().isEmpty))
                    _MessageActions(
                      streaming: streaming,
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
      widgets.add(
        KeyedSubtree(
          key: ValueKey<String>('$messageId:${block.id}'),
          child: AmitiaRichBlockRenderer(block: block),
        ),
      );
    }
    flushImages();
    return widgets;
  }

  List<Widget> _renderAssistantTurn(AssistantTurnDto turn) {
    final items = [...turn.items]
      ..sort((left, right) => left.sequence.compareTo(right.sequence));
    final entries = <_TurnTimelineEntry>[];
    _TurnTimelineEntry? toolGroup;
    for (final item in items) {
      if (item.type == 'tool_call' || item.type == 'tool_result') {
        if (toolGroup == null) {
          toolGroup = _TurnTimelineEntry.tools();
          entries.add(toolGroup);
        }
        toolGroup.items.add(item);
        continue;
      }
      toolGroup = null;
      entries.add(_TurnTimelineEntry.item(item));
    }
    return [
      for (final entry in entries)
        if (entry.items.isNotEmpty)
          _TurnToolStream(items: entry.items)
        else
          switch (entry.item!.type) {
            'reasoning' => AmitiaThinkingBlock(
              block: AmrpThinkingBlock(
                content: entry.item!.content,
                state: _isTurnStreaming(entry.item!.status)
                    ? AmrpMessageState.streaming
                    : AmrpMessageState.completed,
                duration: entry.item!.durationMs > 0
                    ? Duration(milliseconds: entry.item!.durationMs)
                    : null,
              ),
            ),
            'text' => AmitiaMarkdownView(
              source: entry.item!.content,
              streaming: _isTurnStreaming(entry.item!.status),
              onCitation: (id) => setState(() => _highlightCitation = id),
            ),
            _ => const SizedBox.shrink(),
          },
    ];
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
          Text(role, style: TextStyle(color: tokens.muted, fontSize: 11)),
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

enum _CopyMode { plain, markdown }

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
    final tokens = AmitiaMessageTheme.of(context);
    return Padding(
      padding: const EdgeInsets.only(top: 2),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          AmitiaPopupMenuButton<_CopyMode>(
            tooltip: '复制',
            menuWidth: 200,
            itemHorizontalMargin: 0,
            itemHorizontalPadding: 14,
            itemVerticalPadding: 0,
            itemFontSize: 14.5,
            itemIconSize: 18,
            itemMinHeight: 46,
            onSelected: (mode) {
              switch (mode) {
                case _CopyMode.plain:
                  onCopy();
                  return;
                case _CopyMode.markdown:
                  onCopyMarkdown();
                  return;
              }
            },
            itemBuilder: (_) => const [
              PopupMenuItem(value: _CopyMode.plain, child: Text('复制纯文本')),
              PopupMenuItem(
                value: _CopyMode.markdown,
                child: Text('复制 Markdown'),
              ),
            ],
            child: SizedBox(
              width: 32,
              height: 32,
              child: Center(
                child: Icon(Icons.copy_outlined, size: 15, color: tokens.muted),
              ),
            ),
          ),
          if (!streaming && onReply != null) ...[
            _CompactMessageAction(
              tooltip: '引用',
              onPressed: onReply!,
              icon: Icons.format_quote_rounded,
              color: tokens.muted,
            ),
          ],
          if (!streaming && onRetry != null)
            _CompactMessageAction(
              tooltip: '重试',
              onPressed: onRetry!,
              icon: Icons.refresh_rounded,
              color: tokens.muted,
            ),
        ],
      ),
    );
  }
}

class _CompactMessageAction extends StatelessWidget {
  final String tooltip;
  final VoidCallback? onPressed;
  final IconData icon;
  final Color color;

  const _CompactMessageAction({
    required this.tooltip,
    required this.onPressed,
    required this.icon,
    required this.color,
  });

  @override
  Widget build(BuildContext context) {
    return IconButton(
      tooltip: tooltip,
      onPressed: onPressed,
      padding: EdgeInsets.zero,
      style: IconButton.styleFrom(
        tapTargetSize: MaterialTapTargetSize.shrinkWrap,
      ),
      constraints: const BoxConstraints.tightFor(width: 32, height: 32),
      icon: Icon(icon, size: 15, color: color),
    );
  }
}

class _SystemNotice extends StatelessWidget {
  final AmrpMessage message;
  final AmitiaMessageTheme tokens;

  const _SystemNotice({required this.message, required this.tokens});

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

  const _UserMessage({required this.message, required this.tokens});

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    return Align(
      alignment: Alignment.centerRight,
      child: Container(
        constraints: const BoxConstraints(maxWidth: 620),
        margin: const EdgeInsets.only(bottom: 34),
        padding: const EdgeInsets.symmetric(horizontal: 15, vertical: 11),
        decoration: BoxDecoration(
          color: colorScheme.primaryContainer,
          borderRadius: const BorderRadius.only(
            topLeft: Radius.circular(18),
            topRight: Radius.circular(18),
            bottomLeft: Radius.circular(18),
            bottomRight: Radius.circular(18),
          ),
        ),
        child: SelectableText(
          message.markdown,
          style: TextStyle(
            color: colorScheme.onPrimaryContainer,
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
                      style: TextStyle(color: Colors.white, fontSize: 16),
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
                            payload['title']?.toString() ?? 'Minecraft 服务器已连接',
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

bool _isTurnStreaming(String status) {
  final value = status.trim().toLowerCase();
  return const <String>{
    'running',
    'streaming',
    'sending',
    'pending',
    'queued',
  }.contains(value);
}

Color _turnStatusColor(String status) {
  final value = status.trim().toLowerCase();
  if (const <String>{'failed', 'error', 'unknown'}.contains(value)) {
    return const Color(0xFFD46B6B);
  }
  if (value == 'interrupted') {
    return const Color(0xFFA0A1A6);
  }
  if (const <String>{
    'completed',
    'success',
    'succeeded',
    'sent',
    'delivered',
  }.contains(value)) {
    return const Color(0xFF77A982);
  }
  return const Color(0xFFD1A24D);
}

String _turnStatusLabel(String status) {
  final value = status.trim().toLowerCase();
  if (const <String>{'failed', 'error', 'unknown'}.contains(value)) {
    return '失败';
  }
  if (value == 'interrupted') {
    return '已中断';
  }
  if (const <String>{
    'completed',
    'success',
    'succeeded',
    'sent',
    'delivered',
  }.contains(value)) {
    return '完成';
  }
  return '运行中';
}

dynamic _decodeTurnJSON(String value) {
  final source = value.trim();
  if (source.isEmpty) return null;
  try {
    return jsonDecode(source);
  } catch (_) {
    return source;
  }
}

String _turnJSONText(dynamic value) {
  if (value == null) return '';
  if (value is String) return value;
  try {
    return const JsonEncoder.withIndent('  ').convert(value);
  } catch (_) {
    return value.toString();
  }
}

String _turnToolSubject(AssistantTurnItemDto item) {
  final decoded = _decodeTurnJSON(item.argumentsJson);
  if (decoded is Map) {
    for (final key in const <String>[
      'path',
      'file',
      'filePath',
      'query',
      'command',
      'url',
      'cwd',
    ]) {
      final value = decoded[key]?.toString().trim() ?? '';
      if (value.isNotEmpty) return value;
    }
  }
  final text = _turnJSONText(decoded).replaceAll(RegExp(r'\s+'), ' ').trim();
  return text.length > 72 ? '${text.substring(0, 72)}…' : text;
}

String _turnResultText(AssistantTurnItemDto item) {
  final text = _turnJSONText(_decodeTurnJSON(item.resultJson));
  if (text.trim().isNotEmpty) return text;
  return item.errorCode.trim().isNotEmpty ? item.errorCode : '无返回内容';
}

class _TurnTimelineEntry {
  final AssistantTurnItemDto? item;
  final List<AssistantTurnItemDto> items;

  const _TurnTimelineEntry.item(this.item) : items = const [];
  _TurnTimelineEntry.tools() : item = null, items = <AssistantTurnItemDto>[];
}

class _TurnToolStream extends StatefulWidget {
  final List<AssistantTurnItemDto> items;

  const _TurnToolStream({required this.items});

  @override
  State<_TurnToolStream> createState() => _TurnToolStreamState();
}

class _TurnToolStreamState extends State<_TurnToolStream> {
  late bool _expanded;
  bool _touched = false;

  @override
  void initState() {
    super.initState();
    _expanded = _hasRunning(widget.items);
  }

  @override
  void didUpdateWidget(covariant _TurnToolStream oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (_touched) return;
    final next = _hasRunning(widget.items);
    if (next != _expanded) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) setState(() => _expanded = next);
      });
    }
  }

  bool _hasRunning(List<AssistantTurnItemDto> items) =>
      items.any((item) => _isTurnStreaming(item.status));

  String _summary() {
    final calls = widget.items
        .where((item) => item.type == 'tool_call')
        .toList(growable: false);
    final failed = widget.items
        .where((item) => item.status.toLowerCase().contains('fail'))
        .length;
    final running = widget.items.any((item) => _isTurnStreaming(item.status));
    final total = calls.isEmpty ? widget.items.length : calls.length;
    if (failed > 0) return '$total 个工具 · $failed 个失败';
    if (running) {
      final completed = widget.items
          .where((item) => item.status.toLowerCase().contains('complete'))
          .length;
      return '执行中 · $completed/$total 完成';
    }
    return '$total 个工具 · 已完成';
  }

  @override
  Widget build(BuildContext context) {
    final tokens = AmitiaMessageTheme.of(context);
    return Container(
      constraints: const BoxConstraints(maxWidth: 700),
      margin: const EdgeInsets.symmetric(vertical: 8),
      decoration: BoxDecoration(
        border: Border.all(color: tokens.line),
        borderRadius: BorderRadius.circular(10),
      ),
      clipBehavior: Clip.antiAlias,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Semantics(
            button: true,
            expanded: _expanded,
            label: '工具执行流，${_summary()}',
            child: InkWell(
              key: const ValueKey('tool-stream-toggle'),
              onTap: () => setState(() {
                _touched = true;
                _expanded = !_expanded;
              }),
              child: Padding(
                padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 9),
                child: Row(
                  children: [
                    AnimatedRotation(
                      turns: _expanded ? 0.25 : 0,
                      duration: const Duration(milliseconds: 180),
                      child: Icon(
                        Icons.chevron_right_rounded,
                        size: 17,
                        color: tokens.muted,
                      ),
                    ),
                    const SizedBox(width: 6),
                    Text(
                      '工具执行流',
                      style: TextStyle(
                        color: tokens.text,
                        fontSize: 12,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Text(
                        _summary(),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: TextStyle(color: tokens.muted, fontSize: 10.5),
                      ),
                    ),
                    Text(
                      _expanded ? '收起' : '展开',
                      style: TextStyle(color: tokens.accent, fontSize: 10.5),
                    ),
                  ],
                ),
              ),
            ),
          ),
          AnimatedSize(
            duration: const Duration(milliseconds: 180),
            curve: Curves.easeOut,
            child: _expanded
                ? Container(
                    padding: const EdgeInsets.fromLTRB(8, 2, 8, 8),
                    decoration: BoxDecoration(
                      border: Border(top: BorderSide(color: tokens.line)),
                    ),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        for (final item in widget.items)
                          item.type == 'tool_call'
                              ? _TurnToolCallRow(item: item)
                              : _TurnToolResultBlock(item: item),
                      ],
                    ),
                  )
                : const SizedBox.shrink(),
          ),
        ],
      ),
    );
  }
}

class _TurnToolCallRow extends StatelessWidget {
  final AssistantTurnItemDto item;

  const _TurnToolCallRow({required this.item});

  @override
  Widget build(BuildContext context) {
    final tokens = AmitiaMessageTheme.of(context);
    final subject = _turnToolSubject(item);
    final duration = item.durationMs > 0 ? ' · ${item.durationMs} ms' : '';
    return Container(
      constraints: const BoxConstraints(maxWidth: 700),
      margin: const EdgeInsets.symmetric(vertical: 8),
      padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 7),
      decoration: BoxDecoration(
        color: tokens.soft,
        borderRadius: BorderRadius.circular(9),
      ),
      child: Row(
        children: [
          Container(
            width: 7,
            height: 7,
            decoration: BoxDecoration(
              color: _turnStatusColor(item.status),
              shape: BoxShape.circle,
            ),
          ),
          const SizedBox(width: 9),
          Text(
            item.toolName.trim().isEmpty ? '工具调用' : item.toolName,
            style: TextStyle(
              color: tokens.text,
              fontSize: 12,
              fontWeight: FontWeight.w700,
            ),
          ),
          if (subject.isNotEmpty) ...[
            const SizedBox(width: 9),
            Expanded(
              child: Text(
                subject,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(color: tokens.muted, fontSize: 11.5),
              ),
            ),
          ] else
            const Spacer(),
          const SizedBox(width: 8),
          Text(
            '${_turnStatusLabel(item.status)}$duration',
            style: TextStyle(
              color: item.status.toLowerCase().contains('fail')
                  ? tokens.danger
                  : tokens.muted,
              fontSize: 11,
            ),
          ),
        ],
      ),
    );
  }
}

class _TurnToolResultBlock extends StatefulWidget {
  final AssistantTurnItemDto item;

  const _TurnToolResultBlock({required this.item});

  @override
  State<_TurnToolResultBlock> createState() => _TurnToolResultBlockState();
}

class _TurnToolResultBlockState extends State<_TurnToolResultBlock> {
  bool _expanded = false;

  @override
  Widget build(BuildContext context) {
    final tokens = AmitiaMessageTheme.of(context);
    final result = _turnResultText(widget.item);
    final summary = result.replaceAll(RegExp(r'\s+'), ' ').trim();
    return Container(
      constraints: const BoxConstraints(maxWidth: 700),
      margin: const EdgeInsets.only(bottom: 15),
      decoration: BoxDecoration(
        border: Border.all(color: tokens.line),
        borderRadius: BorderRadius.circular(9),
      ),
      clipBehavior: Clip.antiAlias,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          InkWell(
            onTap: () => setState(() => _expanded = !_expanded),
            child: Container(
              padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 7),
              color: tokens.soft,
              child: Row(
                children: [
                  Container(
                    width: 7,
                    height: 7,
                    decoration: BoxDecoration(
                      color: _turnStatusColor(widget.item.status),
                      shape: BoxShape.circle,
                    ),
                  ),
                  const SizedBox(width: 8),
                  Text(
                    widget.item.toolName.trim().isEmpty
                        ? '工具结果'
                        : widget.item.toolName,
                    style: TextStyle(
                      color: tokens.text,
                      fontSize: 11.5,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                  if (summary.isNotEmpty) ...[
                    const SizedBox(width: 8),
                    Expanded(
                      child: Text(
                        summary.length > 80
                            ? '${summary.substring(0, 80)}…'
                            : summary,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: TextStyle(color: tokens.muted, fontSize: 10.5),
                      ),
                    ),
                  ] else
                    const Spacer(),
                  Text(
                    _expanded ? '收起' : '展开',
                    style: TextStyle(color: tokens.accent, fontSize: 10.5),
                  ),
                ],
              ),
            ),
          ),
          if (_expanded)
            Container(
              constraints: const BoxConstraints(maxHeight: 240),
              padding: const EdgeInsets.fromLTRB(11, 9, 11, 10),
              child: SingleChildScrollView(
                child: SelectableText(
                  result,
                  style: TextStyle(
                    color: tokens.muted,
                    fontFamily: 'monospace',
                    fontSize: 10.5,
                    height: 1.55,
                  ),
                ),
              ),
            ),
        ],
      ),
    );
  }
}

import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../../../shared/models/models.dart';
import '../../../core/widgets/amitia_popup_menu.dart';
import '../../../core/widgets/character_avatar.dart';
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
  final String avatarUrl;
  final bool showAvatar;
  final bool showHeader;
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
    this.avatarColor = '',
    this.avatarUrl = '',
    this.showAvatar = true,
    this.showHeader = true,
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
    final citationSources = _mergeCitationSources(
      message.sources,
      widget.message.assistantTurn,
    );
    final citationIds = citationSources.map((source) => source.id).toSet();
    final hasAssistantTurnContent =
        widget.message.assistantTurn?.items.isNotEmpty == true;
    final hasVisibleContent =
        message.markdown.trim().isNotEmpty ||
        message.blocks.isNotEmpty ||
        message.thinking != null ||
        widget.toolBlocks.isNotEmpty ||
        hasAssistantTurnContent;
    final isActiveAssistant =
        message.role == AmrpMessageRole.assistant &&
        (message.state == AmrpMessageState.queued ||
            message.state == AmrpMessageState.streaming);
    final showThinkingPlaceholder =
        message.role == AmrpMessageRole.assistant &&
        message.thinking == null &&
        ((isActiveAssistant && !hasVisibleContent) || widget.showThinking);
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
              CharacterAvatar(
                characterId: widget.characterId,
                avatar: widget.avatarUrl,
                size: tokens.avatarSize,
                initial: widget.avatarInitial,
                color: _parseAvatarColor(widget.avatarColor, tokens.accent),
                borderRadius: BorderRadius.circular(tokens.avatarSize * 0.32),
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
                      time: _formatTime(message.createdAt),
                      tokens: tokens,
                    ),
                  if (_stateNotice(message.state) != null)
                    _StateNotice(
                      data: _stateNotice(message.state)!,
                      state: message.state,
                      tokens: tokens,
                    ),
                  if (showThinkingPlaceholder)
                    AmitiaThinkingBlock(
                      block: const AmrpThinkingBlock(
                        content: '',
                        state: AmrpMessageState.streaming,
                      ),
                    )
                  else if (widget.message.assistantTurn?.items.isNotEmpty ==
                      true)
                    ..._renderAssistantTurn(
                      widget.message.assistantTurn!,
                      citationIds,
                    )
                  else ...[
                    if (message.thinking != null)
                      AmitiaThinkingBlock(block: message.thinking!),
                    if (message.markdown.isNotEmpty)
                      AmitiaMarkdownView(
                        source: message.markdown,
                        streaming: message.state == AmrpMessageState.streaming,
                        citationIds: citationIds,
                        onCitation: (id) =>
                            setState(() => _highlightCitation = id),
                      ),
                    ..._renderBlocks(message.blocks, message.id),
                    for (final tool in widget.toolBlocks)
                      AmitiaToolBlock(block: tool),
                  ],
                  AmitiaCitationList(
                    sources: citationSources,
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

  List<Widget> _renderAssistantTurn(
    AssistantTurnDto turn,
    Set<String> citationIds,
  ) {
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
        if (!(entry.item?.type == 'text' && entry.item!.content.trim().isEmpty))
          Padding(
            padding: const EdgeInsets.symmetric(vertical: 4),
            child: entry.items.isNotEmpty
                ? _TurnToolStream(items: entry.items)
                : switch (entry.item!.type) {
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
                      padding: EdgeInsets.zero,
                    ),
                    'text' => AmitiaMarkdownView(
                      source: entry.item!.content,
                      streaming: _isTurnStreaming(entry.item!.status),
                      citationIds: citationIds,
                      onCitation: (id) =>
                          setState(() => _highlightCitation = id),
                    ),
                    _ => const SizedBox.shrink(),
                  },
          ),
    ];
  }

  List<AmrpCitationSource> _mergeCitationSources(
    List<AmrpCitationSource> messageSources,
    AssistantTurnDto? turn,
  ) {
    final byId = <String, AmrpCitationSource>{
      for (final source in messageSources) source.id: source,
    };
    if (turn != null) {
      for (final item in turn.items) {
        if (item.type != 'tool_result' ||
            (item.toolName != 'web_run' && item.toolName != 'web.run') ||
            item.resultJson.trim().isEmpty) {
          continue;
        }
        try {
          final decoded = jsonDecode(item.resultJson);
          if (decoded is! Map) continue;
          final citations = decoded['citations'];
          if (citations is! List) continue;
          for (final raw in citations) {
            if (raw is! Map) continue;
            final citation = Map<String, dynamic>.from(raw);
            final index = (citation['index'] as num?)?.toInt() ?? 0;
            final fallbackId =
                (citation['ref_id'] ?? citation['evidence_id'] ?? '')
                    .toString()
                    .trim();
            final id = index > 0 ? index.toString() : fallbackId;
            if (id.isEmpty || byId.containsKey(id)) continue;
            final url = (citation['url'] ?? '').toString();
            final title = (citation['title'] ?? '').toString().trim();
            byId[id] = AmrpCitationSource(
              id: id,
              title: title.isNotEmpty
                  ? title
                  : (url.isNotEmpty ? url : 'Source $id'),
              url: url,
              snippet: (citation['text'] ?? '').toString(),
            );
          }
        } catch (_) {
          // Ignore legacy/non-JSON tool results.
        }
      }
    }
    final sources = byId.values.toList();
    sources.sort((left, right) {
      final leftNumber = int.tryParse(left.id);
      final rightNumber = int.tryParse(right.id);
      if (leftNumber != null && rightNumber != null) {
        return leftNumber.compareTo(rightNumber);
      }
      return left.id.compareTo(right.id);
    });
    return sources;
  }

  Map<String, String>? _stateNotice(AmrpMessageState state) {
    return switch (state) {
      AmrpMessageState.interrupted => <String, String>{
        'title': '已中断',
        'detail': '保留已生成内容 · 可继续生成',
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
  final String time;
  final AmitiaMessageTheme tokens;

  const _MessageHead({
    required this.name,
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

Color _parseAvatarColor(String value, Color fallback) {
  final parsed = int.tryParse(value.replaceFirst('#', ''), radix: 16);
  return parsed == null ? fallback : Color(0xFF000000 | parsed);
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
    if (state == AmrpMessageState.failed) {
      return Container(
        constraints: const BoxConstraints(maxWidth: 700),
        margin: const EdgeInsets.only(bottom: 10),
        child: Row(
          children: [
            Expanded(child: Container(height: 1, color: tokens.line)),
            const SizedBox(width: 10),
            Flexible(
              flex: 4,
              child: Text.rich(
                TextSpan(
                  children: [
                    TextSpan(
                      text: data['title']!,
                      style: TextStyle(
                        color: tokens.danger,
                        fontSize: 11,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                    const TextSpan(text: '  '),
                    TextSpan(
                      text: data['detail']!,
                      style: TextStyle(color: tokens.muted, fontSize: 11),
                    ),
                  ],
                ),
                textAlign: TextAlign.center,
              ),
            ),
            const SizedBox(width: 10),
            Expanded(child: Container(height: 1, color: tokens.line)),
          ],
        ),
      );
    }
    return Container(
      constraints: const BoxConstraints(maxWidth: 700),
      margin: const EdgeInsets.only(bottom: 10),
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
      decoration: BoxDecoration(
        color: tokens.surface,
        border: Border.all(color: tokens.line),
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
              width: 48,
              height: 48,
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
      constraints: const BoxConstraints.tightFor(width: 48, height: 48),
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

String _turnToolDisplayName(String raw, {String fallback = '工具调用'}) {
  final value = raw.trim();
  if (value.isEmpty) return fallback;
  if (value == 'web_run' || value == 'web.run') return '联网研究';
  return value;
}

String _turnToolSubject(AssistantTurnItemDto item) {
  final status = item.status.toLowerCase();
  final progress = item.content.replaceAll(RegExp(r'\s+'), ' ').trim();
  final isRunning =
      status == 'queued' ||
      status == 'starting' ||
      status == 'running' ||
      status == 'waiting_tool' ||
      status == 'pending' ||
      status == 'streaming';
  if (progress.isNotEmpty && isRunning) {
    return progress.length > 96 ? '${progress.substring(0, 96)}…' : progress;
  }
  final decoded = _decodeTurnJSON(item.argumentsJson);
  if (decoded is Map) {
    final searchQueries = decoded['search_query'];
    if (searchQueries is List && searchQueries.isNotEmpty) {
      final first = searchQueries.first;
      if (first is Map) {
        final query = first['q']?.toString().trim() ?? '';
        if (query.isNotEmpty) {
          return searchQueries.length > 1
              ? '$query · ${searchQueries.length} 个查询'
              : query;
        }
      }
    }
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
    for (final key in const <String>['open', 'find', 'click', 'screenshot']) {
      final commands = decoded[key];
      if (commands is List && commands.isNotEmpty) {
        return '$key · ${commands.length}';
      }
    }
  }
  final text = _turnJSONText(decoded).replaceAll(RegExp(r'\s+'), ' ').trim();
  return text.length > 72 ? '${text.substring(0, 72)}…' : text;
}

bool _isWebResearchTool(String name) => name == 'web_run' || name == 'web.run';

String _webResearchStopReasonText(String reason) {
  const labels = <String, String>{
    'coverage_satisfied': '关键问题已覆盖',
    'max_rounds': '达到研究轮次上限',
    'max_search_calls': '达到搜索调用上限',
    'max_provider_cost': '达到 Provider 成本上限',
    'max_provider_credits': '达到 Provider credits 上限',
    'no_new_queries': '没有新的有效查询',
    'no_new_sources': '没有发现新的有效来源',
    'low_information_gain': '新增信息已低于阈值',
    'budget_satisfied': '研究预算已满足',
  };
  return labels[reason] ?? reason;
}

String _formatWebResearchDetails(Map<dynamic, dynamic> record) {
  final lines = <String>[];
  final operation = record['operation']?.toString().trim() ?? 'research';
  const operationLabels = <String, String>{
    'search': '搜索',
    'open': '网页读取',
    'find': '页面查找',
    'click': '链接读取',
    'screenshot': '视觉证据',
  };
  lines.add('联网研究 · ${operationLabels[operation] ?? '研究'}');

  final research = record['research'];
  if (research is Map) {
    final unresolved = <String>{
      if (research['unresolved_questions'] is List)
        for (final value in research['unresolved_questions'] as List)
          if (value.toString().trim().isNotEmpty) value.toString().trim(),
    };
    final plan = research['plan'];
    if (plan is Map) {
      final goal = plan['goal']?.toString().trim() ?? '';
      if (goal.isNotEmpty) {
        lines
          ..add('')
          ..add('研究目标：$goal');
      }
      final questions = plan['questions'];
      if (questions is List && questions.isNotEmpty) {
        lines
          ..add('')
          ..add('研究计划');
        for (final question in questions) {
          if (question is! Map) continue;
          final text = question['question']?.toString().trim() ?? '';
          if (text.isEmpty) continue;
          lines.add('${unresolved.contains(text) ? '…' : '✓'} $text');
        }
      }
    }
    final findings = research['findings'];
    if (findings is List && findings.isNotEmpty) {
      lines
        ..add('')
        ..add('研究结论');
      for (final finding in findings.take(12)) {
        if (finding is! Map) continue;
        final question = finding['question']?.toString().trim() ?? '';
        final status = finding['status']?.toString().trim() ?? 'insufficient';
        final icon = status == 'corroborated'
            ? '✓✓'
            : (status == 'supported' ? '✓' : '…');
        if (question.isNotEmpty) lines.add('$icon $question');
        final summary = finding['summary']?.toString().trim() ?? '';
        if (summary.isNotEmpty) {
          final compact = summary.replaceAll(RegExp(r'\s+'), ' ');
          lines.add(
            '  ${compact.length > 600 ? compact.substring(0, 600) : compact}',
          );
        }
      }
    }
    final stopReason = research['stop_reason']?.toString().trim() ?? '';
    if (stopReason.isNotEmpty) {
      lines
        ..add('')
        ..add('停止条件：${_webResearchStopReasonText(stopReason)}');
    }
  }

  final search = record['search'];
  if (search is List && search.isNotEmpty) {
    lines
      ..add('')
      ..add('来源（${search.length}）');
    for (final source in search.take(20)) {
      if (source is! Map) continue;
      final url = source['url']?.toString().trim() ?? '';
      final title = source['title']?.toString().trim();
      final label = title == null || title.isEmpty
          ? (url.isEmpty ? '来源' : url)
          : title;
      lines.add('- $label${url.isNotEmpty ? '\n  $url' : ''}');
    }
  }

  final citations = record['citations'];
  if (citations is List && citations.isNotEmpty) {
    lines
      ..add('')
      ..add('证据与引用（${citations.length}）');
    for (final citation in citations.take(24)) {
      if (citation is! Map) continue;
      final index = int.tryParse(citation['index']?.toString() ?? '') ?? 0;
      final url = citation['url']?.toString().trim() ?? '';
      final title = citation['title']?.toString().trim();
      final label = title == null || title.isEmpty
          ? (url.isEmpty ? '证据' : url)
          : title;
      var locatorText = '';
      final locator = citation['locator'];
      if (locator is Map && locator['kind']?.toString() == 'pdf_page') {
        final page = int.tryParse(locator['page']?.toString() ?? '');
        if (page != null && page >= 0) locatorText = ' · PDF 第 ${page + 1} 页';
      }
      lines.add('- ${index > 0 ? '[$index] ' : ''}$label$locatorText');
    }
  }

  final graph = record['evidence_graph'];
  if (graph is Map) {
    final conflicts =
        int.tryParse(graph['conflict_count']?.toString() ?? '') ?? 0;
    if (conflicts > 0) {
      lines
        ..add('')
        ..add('证据冲突：$conflicts 组（已保留供最终回答审计）');
    }
  }

  final security = record['security'];
  if (security is Map && security['potential_prompt_injection'] == true) {
    lines
      ..add('')
      ..add('安全：检测到网页中的指令型文本，已按不可信外部内容处理。');
  }

  final stats = record['stats'];
  if (stats is Map) {
    final calls = int.tryParse(stats['search_calls']?.toString() ?? '') ?? 0;
    final fetches = int.tryParse(stats['fetch_calls']?.toString() ?? '') ?? 0;
    final duration = int.tryParse(stats['duration_ms']?.toString() ?? '') ?? 0;
    final summary = <String>[];
    if (calls > 0) summary.add('$calls 次搜索');
    if (fetches > 0) summary.add('$fetches 次抓取');
    if (duration > 0) summary.add('$duration ms');
    if (summary.isNotEmpty) {
      lines
        ..add('')
        ..add('运行统计：${summary.join(' · ')}');
    }
  }

  if (research is Map && research['cost'] is Map) {
    final cost = research['cost'] as Map;
    final usd =
        double.tryParse(cost['provider_cost_usd']?.toString() ?? '') ?? 0;
    final credits =
        double.tryParse(cost['provider_credits']?.toString() ?? '') ?? 0;
    final values = <String>[];
    if (usd > 0) values.add(r'$' + usd.toStringAsFixed(4));
    if (credits > 0) {
      final creditText = credits == credits.roundToDouble()
          ? credits.toInt().toString()
          : credits.toStringAsFixed(2);
      values.add('$creditText credits');
    }
    if (values.isNotEmpty) lines.add('Provider 成本：${values.join(' · ')}');
  }

  return lines.join('\n').trim();
}

String _turnResultText(AssistantTurnItemDto item) {
  final decoded = _decodeTurnJSON(item.resultJson);
  if (_isWebResearchTool(item.toolName) && decoded is Map) {
    final formatted = _formatWebResearchDetails(decoded);
    if (formatted.isNotEmpty) return formatted;
  }
  final text = _turnJSONText(decoded);
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

class _TurnToolStreamState extends State<_TurnToolStream>
    with SingleTickerProviderStateMixin {
  late bool _expanded;
  late final AnimationController _expansionController;
  late final Animation<double> _expansionAnimation;
  bool _touched = false;

  @override
  void initState() {
    super.initState();
    _expanded = _hasRunning(widget.items);
    _expansionController = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 180),
      value: _expanded ? 1 : 0,
    );
    _expansionAnimation = CurvedAnimation(
      parent: _expansionController,
      curve: Curves.easeOut,
    );
  }

  @override
  void didUpdateWidget(covariant _TurnToolStream oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (_touched) return;
    final next = _hasRunning(widget.items);
    if (next != _expanded) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) _setExpanded(next);
      });
    }
  }

  @override
  void dispose() {
    _expansionController.dispose();
    super.dispose();
  }

  void _setExpanded(bool expanded) {
    setState(() => _expanded = expanded);
    if (expanded) {
      _expansionController.forward();
    } else {
      _expansionController.reverse();
    }
  }

  bool _hasRunning(List<AssistantTurnItemDto> items) =>
      items.any((item) => _isTurnStreaming(item.status));

  String _summary() {
    final calls = _uniqueToolRecords(
      widget.items.where((item) => item.type == 'tool_call'),
    );
    final results = _uniqueToolRecords(
      widget.items.where((item) => item.type == 'tool_result'),
    );
    final counted = calls.isNotEmpty ? calls : results;
    final failed = counted
        .where((item) => _isFailedToolStatus(item.status))
        .length;
    final running = counted.any((item) => _isTurnStreaming(item.status));
    final total = counted.length;
    if (failed > 0) return '$total 个工具 · $failed 个失败';
    if (running) {
      final completed = counted
          .where((item) => item.status.trim().toLowerCase() == 'completed')
          .length;
      return '执行中 · $completed/$total 完成';
    }
    return '$total 个工具 · 已完成';
  }

  List<AssistantTurnItemDto> _uniqueToolRecords(
    Iterable<AssistantTurnItemDto> items,
  ) {
    final records = <String, AssistantTurnItemDto>{};
    for (final item in items) {
      final rawKey = item.callId.trim().isNotEmpty ? item.callId : item.id;
      final key = rawKey.trim();
      if (key.isEmpty) continue;
      final current = records[key];
      if (current == null || item.revision >= current.revision) {
        records[key] = item;
      }
    }
    return records.values.toList(growable: false);
  }

  bool _isFailedToolStatus(String status) {
    final value = status.trim().toLowerCase();
    return value == 'failed' || value == 'error' || value == 'unknown';
  }

  List<_TurnToolEntry> _toolEntries() {
    final resultsByCall = <String, AssistantTurnItemDto>{};
    for (final item in _uniqueToolRecords(
      widget.items.where((item) => item.type == 'tool_result'),
    )) {
      final key = _turnToolKey(item);
      if (key.isNotEmpty) resultsByCall[key] = item;
    }
    final entries = <_TurnToolEntry>[];
    final matched = <String>{};
    for (final item in _uniqueToolRecords(
      widget.items.where((item) => item.type == 'tool_call'),
    )) {
      final key = _turnToolKey(item);
      final result = key.isEmpty ? null : resultsByCall[key];
      if (key.isNotEmpty) matched.add(key);
      entries.add(_TurnToolEntry(item: item, result: result));
    }
    for (final item in _uniqueToolRecords(
      widget.items.where((item) => item.type == 'tool_result'),
    )) {
      final key = _turnToolKey(item);
      if (key.isNotEmpty && matched.contains(key)) continue;
      entries.add(_TurnToolEntry(item: item, result: item));
    }
    return entries;
  }

  @override
  Widget build(BuildContext context) {
    final tokens = AmitiaMessageTheme.of(context);
    return Container(
      key: const ValueKey('tool-stream-container'),
      constraints: const BoxConstraints(maxWidth: 700),
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
              onTap: () {
                _touched = true;
                _setExpanded(!_expanded);
              },
              child: Padding(
                padding: const EdgeInsets.symmetric(
                  horizontal: 10,
                  vertical: 9,
                ),
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
          SizeTransition(
            sizeFactor: _expansionAnimation,
            axisAlignment: -1,
            child: IgnorePointer(
              ignoring: !_expanded,
              child: ExcludeSemantics(
                excluding: !_expanded,
                child: Container(
                  padding: const EdgeInsets.fromLTRB(8, 2, 8, 8),
                  decoration: BoxDecoration(
                    border: Border(top: BorderSide(color: tokens.line)),
                  ),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      for (final entry in _toolEntries())
                        _TurnToolCallRow(
                          item: entry.item,
                          result: entry.result,
                        ),
                    ],
                  ),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _TurnToolEntry {
  final AssistantTurnItemDto item;
  final AssistantTurnItemDto? result;

  const _TurnToolEntry({required this.item, required this.result});
}

class _TurnToolCallRow extends StatefulWidget {
  final AssistantTurnItemDto item;
  final AssistantTurnItemDto? result;

  const _TurnToolCallRow({required this.item, required this.result});

  @override
  State<_TurnToolCallRow> createState() => _TurnToolCallRowState();
}

class _TurnToolCallRowState extends State<_TurnToolCallRow> {
  bool _expanded = false;

  @override
  Widget build(BuildContext context) {
    final tokens = AmitiaMessageTheme.of(context);
    final subject = _turnToolSubject(widget.item);
    final duration = widget.item.durationMs > 0
        ? ' · ${widget.item.durationMs} ms'
        : '';
    final arguments = _turnJSONText(
      _decodeTurnJSON(widget.item.argumentsJson),
    ).trim();
    final result = widget.result == null ? '' : _turnResultText(widget.result!);
    return Container(
      constraints: const BoxConstraints(maxWidth: 700),
      margin: const EdgeInsets.symmetric(vertical: 8),
      decoration: BoxDecoration(
        color: tokens.soft,
        borderRadius: BorderRadius.circular(9),
      ),
      clipBehavior: Clip.antiAlias,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 7),
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
                const SizedBox(width: 9),
                Expanded(
                  flex: subject.isEmpty ? 1 : 3,
                  child: Text(
                    _turnToolDisplayName(widget.item.toolName),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(
                      color: tokens.text,
                      fontSize: 12,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                ),
                if (subject.isNotEmpty) ...[
                  const SizedBox(width: 9),
                  Expanded(
                    flex: 2,
                    child: Text(
                      subject,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(color: tokens.muted, fontSize: 11.5),
                    ),
                  ),
                ],
                const SizedBox(width: 8),
                Text(
                  '${_turnStatusLabel(widget.item.status)}$duration',
                  style: TextStyle(
                    color: widget.item.status.toLowerCase().contains('fail')
                        ? tokens.danger
                        : tokens.muted,
                    fontSize: 11,
                  ),
                ),
                const SizedBox(width: 4),
                TextButton(
                  key: ValueKey(
                    'tool-call-expand-${_turnToolKey(widget.item)}',
                  ),
                  onPressed: () => setState(() => _expanded = !_expanded),
                  style: TextButton.styleFrom(
                    minimumSize: Size.zero,
                    padding: const EdgeInsets.symmetric(
                      horizontal: 6,
                      vertical: 4,
                    ),
                    tapTargetSize: MaterialTapTargetSize.shrinkWrap,
                    foregroundColor: tokens.accent,
                  ),
                  child: Text(
                    _expanded ? '收起' : '展开',
                    maxLines: 1,
                    style: const TextStyle(fontSize: 10.5),
                  ),
                ),
              ],
            ),
          ),
          if (_expanded)
            Container(
              key: ValueKey('tool-call-details-${_turnToolKey(widget.item)}'),
              padding: const EdgeInsets.fromLTRB(11, 9, 11, 10),
              decoration: BoxDecoration(
                border: Border(top: BorderSide(color: tokens.line)),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  _TurnToolDetailSection(
                    title: '调用工具',
                    content: _turnToolDisplayName(widget.item.toolName),
                  ),
                  const SizedBox(height: 9),
                  _TurnToolDetailSection(
                    title: '调用参数',
                    content: arguments.isEmpty ? '无参数' : arguments,
                  ),
                  const SizedBox(height: 9),
                  _TurnToolDetailSection(
                    title: '调用结果',
                    content: result.trim().isEmpty ? '无返回内容' : result,
                  ),
                ],
              ),
            ),
        ],
      ),
    );
  }
}

class _TurnToolDetailSection extends StatelessWidget {
  final String title;
  final String content;

  const _TurnToolDetailSection({required this.title, required this.content});

  @override
  Widget build(BuildContext context) {
    final tokens = AmitiaMessageTheme.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(
          title,
          style: TextStyle(
            color: tokens.text,
            fontSize: 10.5,
            fontWeight: FontWeight.w600,
          ),
        ),
        const SizedBox(height: 5),
        ConstrainedBox(
          constraints: const BoxConstraints(maxHeight: 240),
          child: SingleChildScrollView(
            child: SelectableText(
              content,
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
    );
  }
}

String _turnToolKey(AssistantTurnItemDto item) {
  final value = item.callId.trim().isNotEmpty ? item.callId : item.id;
  return value.trim();
}

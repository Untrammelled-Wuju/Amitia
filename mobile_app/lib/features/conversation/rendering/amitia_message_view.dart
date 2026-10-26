import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter/services.dart';

import '../../../shared/models/models.dart';
import '../../../core/models/feedback.dart';
import '../../../core/services/providers.dart';
import '../../../core/widgets/amitia_popup_menu.dart';
import '../../../core/widgets/user_message_surface.dart';
import '../../../core/widgets/character_avatar.dart';
import '../../../core/settings/chat_appearance_preferences.dart';
import '../../../app/theme/app_colors.dart';
import '../../../core/models/conversation.dart';
import '../../../core/ui_runtime/mobile_style_runtime.dart';
import 'amitia_message_theme.dart';
import 'amrp.dart';
import 'bubble_message_projection.dart';
import 'markdown/amitia_markdown.dart';
import 'rich_blocks/amitia_citation_list.dart';
import 'rich_blocks/amitia_rich_blocks.dart';
import 'rich_blocks/renderer_registry.dart';

class AmitiaMessageView extends ConsumerStatefulWidget {
  final ChatMessage message;
  final ChatMessageStyle messageStyle;
  final UserMessageMaterial userMessageMaterial;
  final String characterId;
  final String characterName;
  final String avatarInitial;
  final String avatarColor;
  final String avatarUrl;
  final bool showAvatar;
  final bool aiAvatarEnabled;
  final AiAvatarShapePreference aiAvatarShape;
  final bool aiNameEnabled;
  final bool showHeader;
  final bool showThinking;
  final List<AmrpToolBlock> toolBlocks;
  final VoidCallback? onRetry;
  final VoidCallback? onReply;
  final VoidCallback? onCopy;

  const AmitiaMessageView({
    super.key,
    required this.message,
    this.messageStyle = ChatMessageStyle.flow,
    this.userMessageMaterial = UserMessageMaterial.solid,
    this.characterId = '',
    this.characterName = 'Amitia',
    this.avatarInitial = 'A',
    this.avatarColor = '',
    this.avatarUrl = '',
    this.showAvatar = true,
    this.aiAvatarEnabled = true,
    this.aiAvatarShape = const AiAvatarShapePreference(),
    this.aiNameEnabled = true,
    this.showHeader = true,
    this.showThinking = false,
    this.toolBlocks = const <AmrpToolBlock>[],
    this.onRetry,
    this.onReply,
    this.onCopy,
  });

  @override
  ConsumerState<AmitiaMessageView> createState() => _AmitiaMessageViewState();
}

class _AmitiaMessageViewState extends ConsumerState<AmitiaMessageView> {
  String _highlightCitation = '';
  FeedbackDto? _feedback;
  bool _feedbackBusy = false;
  int _feedbackGeneration = 0;

  bool get _feedbackReady =>
      widget.message.role == MessageRole.assistant &&
      widget.message.id.trim().isNotEmpty &&
      widget.characterId.trim().isNotEmpty &&
      (widget.message.sourceRevision ?? 0) > 0 &&
      ![
        MessageStatus.queued,
        MessageStatus.sending,
        MessageStatus.streaming,
        MessageStatus.error,
      ].contains(widget.message.status);

  String _feedbackIdentity(AmitiaMessageView value) => jsonEncode([
    value.message.id,
    value.characterId,
    value.message.sourceOwnerId,
    value.message.sourceScope,
    value.message.sourceRevision,
    value.message.status.name,
  ]);

  @override
  void initState() {
    super.initState();
    AmrpRendererRegistry.registerExtension(
      'minecraft.server',
      (context, block) => _MinecraftServerBlock(block: block),
    );
    Future.microtask(_loadFeedback);
  }

  @override
  void didUpdateWidget(covariant AmitiaMessageView oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (_feedbackIdentity(oldWidget) != _feedbackIdentity(widget)) {
      _feedbackGeneration++;
      _feedbackBusy = false;
      _feedback = null;
      Future.microtask(_loadFeedback);
    }
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
        !widget.message.outputComplete &&
        (message.state == AmrpMessageState.streaming ||
            message.state == AmrpMessageState.queued);
    final citationSources = _mergeCitationSources(message.sources);
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
        !widget.message.outputComplete &&
        ((isActiveAssistant && !hasVisibleContent) || widget.showThinking);
    if (message.role == AmrpMessageRole.system) {
      return _SystemNotice(message: message, tokens: tokens);
    }
    if (message.role == AmrpMessageRole.user) {
      if (widget.messageStyle == ChatMessageStyle.bubble) {
        return Align(
          alignment: Alignment.topRight,
          child: ConstrainedBox(
            constraints: BoxConstraints(
              maxWidth: MediaQuery.sizeOf(context).width * 0.78,
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.end,
              children: [
                if ((widget.message.replyToMessageId ?? '').isNotEmpty)
                  Padding(
                    padding: const EdgeInsets.only(bottom: 6),
                    child: Text(
                      '引用：${widget.message.replyToExcerpt ?? '原消息'}',
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                        color: context.textSecondary,
                        fontSize: 12,
                      ),
                    ),
                  ),
                ..._renderBubbleContent(message, const {}, pending: false),
              ],
            ),
          ),
        );
      }
      return _UserMessage(
        message: message,
        tokens: tokens,
        material: widget.userMessageMaterial,
      );
    }
    final bubble = widget.messageStyle == ChatMessageStyle.bubble;
    final assistant = Align(
      alignment: bubble ? Alignment.topLeft : Alignment.topCenter,
      child: ConstrainedBox(
        constraints: BoxConstraints(
          maxWidth: bubble
              ? MediaQuery.sizeOf(context).width * 0.86
              : tokens.messageMaxWidth,
        ),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            if (!bubble && widget.aiAvatarEnabled && widget.showAvatar)
              CharacterAvatar(
                characterId: widget.characterId,
                avatar: widget.avatarUrl,
                size: tokens.avatarSize,
                initial: widget.avatarInitial,
                color: _parseAvatarColor(widget.avatarColor, tokens.accent),
                borderRadius: BorderRadius.circular(
                  widget.aiAvatarShape.radius(tokens.avatarSize),
                ),
              )
            else if (!bubble && widget.aiAvatarEnabled)
              SizedBox(width: tokens.avatarSize, height: tokens.avatarSize),
            if (!bubble && widget.aiAvatarEnabled)
              SizedBox(width: tokens.messageGap),
            Expanded(
              child: DefaultTextStyle.merge(
                style: context
                    .amitiaStyleFor(
                      'message.assistant',
                      states: <String>[streaming ? 'loading' : 'default'],
                    )
                    .applyTextStyle(DefaultTextStyle.of(context).style),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    if (widget.showHeader)
                      _MessageHead(
                        name: widget.characterName,
                        showName: !bubble && widget.aiNameEnabled,
                        time: _formatTime(message.createdAt),
                        tokens: tokens,
                      ),
                    if (_stateNotice(message.state) != null)
                      _StateNotice(
                        data: _stateNotice(message.state)!,
                        state: message.state,
                        tokens: tokens,
                      ),
                    if (bubble)
                      ..._renderBubbleContent(
                        message,
                        citationIds,
                        pending: streaming,
                      )
                    else if (showThinkingPlaceholder)
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
                      if (widget.message.outputComplete &&
                          widget.message.sourceRevision == null)
                        const Text("正在确认保存…"),
                      if (message.thinking != null)
                        AmitiaThinkingBlock(block: message.thinking!),
                      if (message.markdown.isNotEmpty)
                        AmitiaMarkdownView(
                          source: flowBubbleText(message.markdown),
                          streaming:
                              !widget.message.outputComplete &&
                              message.state == AmrpMessageState.streaming,
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
                        !(widget.showThinking &&
                            message.markdown.trim().isEmpty))
                      _MessageActions(
                        streaming: streaming,
                        onCopy: () => _copy(context, message.plainText),
                        onCopyMarkdown: () => _copy(context, message.markdown),
                        onReply: widget.onReply,
                        onRetry: widget.onRetry,
                        feedbackType: _feedback?.feedbackType,
                        feedbackBusy: _feedbackBusy || !_feedbackReady,
                        onHelpful: () => _submitFeedback('good'),
                        onImprove: _submitImprovement,
                        onUnsafe: _submitUnsafe,
                        onDeleteFeedback: _feedback == null
                            ? null
                            : _deleteFeedback,
                      ),
                  ],
                ),
              ),
            ),
          ],
        ),
      ),
    );
    return bubble
        ? assistant
        : _applyMessageContainerStyle(
            context,
            'message.assistant',
            states: <String>[streaming ? 'loading' : 'default'],
            child: assistant,
          );
  }

  Future<void> _loadFeedback() async {
    final messageId = widget.message.id.trim();
    final identity = _feedbackIdentity(widget);
    final generation = ++_feedbackGeneration;
    if (!mounted || !_feedbackReady) {
      return;
    }
    try {
      final feedback = await ref
          .read(feedbackServiceProvider)
          .getByMessage(
            messageId,
            characterId: widget.characterId,
            expectedExecutionScope: widget.message.sourceScope,
          );
      if (mounted &&
          generation == _feedbackGeneration &&
          identity == _feedbackIdentity(widget)) {
        setState(() => _feedback = feedback);
      }
    } catch (_) {}
  }

  Future<void> _submitFeedback(String feedbackType, {String? reason}) async {
    if (_feedbackBusy || !_feedbackReady) return;
    final identity = _feedbackIdentity(widget);
    final generation = ++_feedbackGeneration;
    setState(() => _feedbackBusy = true);
    try {
      final feedback = await ref
          .read(feedbackServiceProvider)
          .submit(
            widget.message.id,
            feedbackType,
            comment: reason,
            characterId: widget.characterId,
            expectedExecutionScope: widget.message.sourceScope,
          );
      if (!mounted ||
          generation != _feedbackGeneration ||
          identity != _feedbackIdentity(widget)) {
        return;
      }
      setState(() => _feedback = feedback);
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(feedbackType == 'unsafe' ? '安全报告已提交' : '消息反馈已记录'),
        ),
      );
    } catch (error) {
      if (!mounted ||
          generation != _feedbackGeneration ||
          identity != _feedbackIdentity(widget)) {
        return;
      }
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text('反馈提交失败：$error')));
    } finally {
      if (mounted && generation == _feedbackGeneration) {
        setState(() => _feedbackBusy = false);
      }
    }
  }

  Future<void> _submitImprovement() async {
    final identity = _feedbackIdentity(widget);
    final reason = await _promptFeedbackReason(
      title: '回复质量反馈',
      hint: '请说明这条回复需要改进的地方',
    );
    if (mounted && identity == _feedbackIdentity(widget) && reason != null) {
      await _submitFeedback('other', reason: reason);
    }
  }

  Future<void> _submitUnsafe() async {
    final identity = _feedbackIdentity(widget);
    final reason = await _promptFeedbackReason(
      title: '报告不安全内容',
      hint: '请描述风险、违规或不适内容',
    );
    if (mounted && identity == _feedbackIdentity(widget) && reason != null) {
      await _submitFeedback('unsafe', reason: reason);
    }
  }

  Future<String?> _promptFeedbackReason({
    required String title,
    required String hint,
  }) async {
    final controller = TextEditingController();
    final result = await showDialog<String>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: Text(title),
        content: TextField(
          controller: controller,
          autofocus: true,
          minLines: 3,
          maxLines: 6,
          decoration: InputDecoration(hintText: hint),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(dialogContext).pop(),
            child: const Text('取消'),
          ),
          TextButton(
            onPressed: () {
              final value = controller.text.trim();
              if (value.isNotEmpty) Navigator.of(dialogContext).pop(value);
            },
            child: const Text('提交'),
          ),
        ],
      ),
    );
    controller.dispose();
    return result;
  }

  Future<void> _deleteFeedback() async {
    final feedback = _feedback;
    if (_feedbackBusy || !_feedbackReady || feedback == null) return;
    final identity = _feedbackIdentity(widget);
    final generation = ++_feedbackGeneration;
    setState(() => _feedbackBusy = true);
    try {
      await ref
          .read(feedbackServiceProvider)
          .delete(
            feedback.id,
            characterId: widget.characterId,
            expectedExecutionScope: widget.message.sourceScope,
          );
      if (!mounted ||
          generation != _feedbackGeneration ||
          identity != _feedbackIdentity(widget)) {
        return;
      }
      setState(() => _feedback = null);
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(const SnackBar(content: Text('反馈已撤销')));
    } catch (error) {
      if (!mounted ||
          generation != _feedbackGeneration ||
          identity != _feedbackIdentity(widget)) {
        return;
      }
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text('撤销反馈失败：$error')));
    } finally {
      if (mounted && generation == _feedbackGeneration) {
        setState(() => _feedbackBusy = false);
      }
    }
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

  List<Widget> _renderBubbleContent(
    AmrpMessage message,
    Set<String> citationIds, {
    required bool pending,
  }) {
    final user = message.role == AmrpMessageRole.user;
    final items = projectBubbleContent(
      message: message,
      turn: widget.message.assistantTurn,
      toolBlocks: widget.toolBlocks,
    );
    Widget frame(String key, Widget child) {
      final target = user ? 'message.user' : 'message.assistant';
      final resolved = context.amitiaStyleFor(
        target,
        states: <String>[user || !pending ? 'default' : 'loading'],
      );
      final hasExplicitSurface =
          resolved.has('backgroundColor') ||
          resolved.has('background') ||
          resolved.has('gradient');
      final surface = UserMessageSurface(
        material: user && !hasExplicitSurface
            ? widget.userMessageMaterial
            : UserMessageMaterial.solid,
        padding:
            resolved.edgeInsets('padding') ??
            const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
        decoration: resolved.applyBoxDecoration(
          BoxDecoration(
            color: user ? context.accentSoft : context.surfacePrimary,
            border: user
                ? null
                : Border.all(color: context.borderPrimary, width: 0.5),
            borderRadius: BorderRadius.only(
              topLeft: Radius.circular(user ? 16 : 4),
              topRight: Radius.circular(user ? 4 : 16),
              bottomLeft: const Radius.circular(16),
              bottomRight: const Radius.circular(16),
            ),
          ),
        ),
        child: child,
      );
      final body = Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: user
            ? CrossAxisAlignment.end
            : CrossAxisAlignment.start,
        children: [
          if (!user && widget.aiNameEnabled)
            Padding(
              padding: const EdgeInsets.only(bottom: 6),
              child: Text(
                widget.characterName,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(
                  color: context.textPrimary,
                  fontSize: 14,
                  fontWeight: FontWeight.w700,
                ),
              ),
            ),
          surface,
        ],
      );
      Widget result = Padding(
        key: ValueKey<String>('bubble:$key'),
        padding:
            resolved.edgeInsets('margin') ?? const EdgeInsets.only(bottom: 10),
        child: user || !widget.aiAvatarEnabled
            ? body
            : Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize: MainAxisSize.min,
                children: [
                  CharacterAvatar(
                    characterId: widget.characterId,
                    avatar: widget.avatarUrl,
                    size: 28,
                    initial: widget.avatarInitial,
                    color: _parseAvatarColor(
                      widget.avatarColor,
                      Theme.of(context).colorScheme.primary,
                    ),
                    borderRadius: BorderRadius.circular(
                      widget.aiAvatarShape.radius(28),
                    ),
                  ),
                  const SizedBox(width: 8),
                  Flexible(child: body),
                ],
              ),
      );
      final opacity = resolved.number('opacity');
      if (opacity != null) {
        result = Opacity(
          opacity: opacity.toDouble().clamp(0.0, 1.0).toDouble(),
          child: result,
        );
      }
      return result;
    }

    return [
      for (final item in items)
        frame(item.key, switch (item) {
          BubbleText() =>
            user
                ? Text(
                    item.content,
                    style: TextStyle(
                      color: context.textPrimary,
                      fontSize: 14,
                      height: 1.52,
                    ),
                  )
                : AmitiaMarkdownView(
                    source: item.content,
                    streaming: false,
                    citationIds: citationIds,
                    onCitation: (id) => setState(() => _highlightCitation = id),
                  ),
          BubbleThinking() => AmitiaThinkingBlock(
            block: item.block,
            padding: EdgeInsets.zero,
          ),
          BubbleTools() => _TurnToolStream(items: item.items),
          BubbleRichContent() => AmitiaRichBlockRenderer(block: item.block),
        }),
      if (pending &&
          !items.any(
            (item) =>
                item is BubbleThinking &&
                item.block.state == AmrpMessageState.streaming,
          ))
        frame(
          '${message.id}:pending',
          Text(
            '正在回复…',
            style: TextStyle(color: context.textSecondary, fontSize: 13),
          ),
        ),
    ];
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
                      source: flowBubbleText(entry.item!.content),
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
  ) {
    final byId = <String, AmrpCitationSource>{
      for (final source in messageSources) source.id: source,
    };
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
  final bool showName;
  final String name;
  final String time;
  final AmitiaMessageTheme tokens;

  const _MessageHead({
    this.showName = true,
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
          if (showName)
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
          if (showName) const SizedBox(width: 7),
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
  final String? feedbackType;
  final bool feedbackBusy;
  final VoidCallback onHelpful;
  final VoidCallback onImprove;
  final VoidCallback onUnsafe;
  final VoidCallback? onDeleteFeedback;

  const _MessageActions({
    required this.streaming,
    required this.onCopy,
    required this.onCopyMarkdown,
    this.onReply,
    this.onRetry,
    required this.feedbackType,
    required this.feedbackBusy,
    required this.onHelpful,
    required this.onImprove,
    required this.onUnsafe,
    this.onDeleteFeedback,
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
          if (!streaming) ...[
            _CompactMessageAction(
              tooltip: '有帮助',
              onPressed: feedbackBusy ? null : onHelpful,
              icon: Icons.thumb_up_alt_outlined,
              color: feedbackType == 'good' ? tokens.accent : tokens.muted,
            ),
            _CompactMessageAction(
              tooltip: '需要改进',
              onPressed: feedbackBusy ? null : onImprove,
              icon: Icons.thumb_down_alt_outlined,
              color:
                  feedbackType != null &&
                      feedbackType != 'good' &&
                      feedbackType != 'unsafe'
                  ? tokens.accent
                  : tokens.muted,
            ),
            _CompactMessageAction(
              tooltip: '报告不安全内容',
              onPressed: feedbackBusy ? null : onUnsafe,
              icon: Icons.flag_outlined,
              color: feedbackType == 'unsafe' ? tokens.accent : tokens.muted,
            ),
            if (onDeleteFeedback != null)
              _CompactMessageAction(
                tooltip: '撤销反馈',
                onPressed: feedbackBusy ? null : onDeleteFeedback,
                icon: Icons.delete_outline,
                color: tokens.muted,
              ),
          ],
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
  final UserMessageMaterial material;
  final AmrpMessage message;
  final AmitiaMessageTheme tokens;

  const _UserMessage({
    required this.message,
    required this.tokens,
    this.material = UserMessageMaterial.solid,
  });

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    final style = context.amitiaStyleFor('message.user');
    final effectiveMaterial =
        style.has('backgroundColor') ||
            style.has('background') ||
            style.has('gradient')
        ? UserMessageMaterial.solid
        : material;
    final textStyle = style.applyTextStyle(
      TextStyle(
        color: colorScheme.onPrimaryContainer,
        fontSize: 14,
        height: 1.6,
      ),
    );
    final result = Align(
      alignment: Alignment.centerRight,
      child: UserMessageSurface(
        material: effectiveMaterial,
        constraints: const BoxConstraints(maxWidth: 620),
        margin: style.edgeInsets('margin') ?? const EdgeInsets.only(bottom: 34),
        padding:
            style.edgeInsets('padding') ??
            const EdgeInsets.symmetric(horizontal: 15, vertical: 11),
        decoration: style.applyBoxDecoration(
          BoxDecoration(
            color: colorScheme.primaryContainer,
            borderRadius: const BorderRadius.only(
              topLeft: Radius.circular(18),
              topRight: Radius.circular(18),
              bottomLeft: Radius.circular(18),
              bottomRight: Radius.circular(18),
            ),
          ),
        ),
        child: SelectableText(message.markdown, style: textStyle),
      ),
    );
    final opacity = style.number('opacity');
    return opacity == null
        ? result
        : Opacity(
            opacity: opacity.toDouble().clamp(0.0, 1.0).toDouble(),
            child: result,
          );
  }
}

Widget _applyMessageContainerStyle(
  BuildContext context,
  String target, {
  required Widget child,
  List<String> states = const <String>['default'],
}) {
  final style = context.amitiaStyleFor(target, states: states);
  final decoration = style.applyBoxDecoration(BoxDecoration());
  final padding = style.edgeInsets('padding');
  final margin = style.edgeInsets('margin');
  final hasDecoration =
      decoration.color != null ||
      decoration.gradient != null ||
      decoration.border != null ||
      decoration.boxShadow != null;
  Widget result = hasDecoration || padding != null
      ? Container(
          padding: padding,
          decoration: hasDecoration ? decoration : null,
          child: child,
        )
      : child;
  if (margin != null) result = Padding(padding: margin, child: result);
  final opacity = style.number('opacity');
  if (opacity != null) {
    result = Opacity(
      opacity: opacity.toDouble().clamp(0.0, 1.0).toDouble(),
      child: result,
    );
  }
  return result;
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
  final decoded = _decodeTurnJSON(item.resultJson);
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
            alignment: Alignment.topCenter,
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

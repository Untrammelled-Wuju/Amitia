import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';

class CharacterGenerationChat extends ConsumerStatefulWidget {
  final Map<String, dynamic> Function() currentDraft;
  final ValueChanged<Map<String, dynamic>> onApply;
  const CharacterGenerationChat({
    super.key,
    required this.currentDraft,
    required this.onApply,
  });

  @override
  ConsumerState<CharacterGenerationChat> createState() =>
      _CharacterGenerationChatState();
}

class _CharacterGenerationChatState
    extends ConsumerState<CharacterGenerationChat> {
  final _input = TextEditingController();
  final _scroll = ScrollController();
  final _messages = <Map<String, String>>[];
  Map<String, dynamic>? _proposal;
  String? _pendingMessage;
  bool _busy = false;
  String? _error;

  @override
  void dispose() {
    _input.dispose();
    _scroll.dispose();
    super.dispose();
  }

  Future<void> _send() async {
    final text = _input.text.trim();
    if (_busy || text.isEmpty) return;
    setState(() {
      _busy = true;
      _error = null;
      _pendingMessage = text;
    });
    _scrollToLatest();
    try {
      final history = [
        ..._messages.skip(_messages.length > 30 ? _messages.length - 30 : 0),
        {'role': 'user', 'content': text},
      ];
      final current = widget.currentDraft();
      final result = await ref
          .read(backendServiceProvider)
          .post<Map<String, dynamic>>(
            '/api/characters/generate-card',
            data: {
              'messages': history,
              'draft': {
                ...current,
                ...?_proposal,
                'personalityConfig': {
                  ...?current['personalityConfig'] as Map?,
                  ...?_proposal?['personalityConfig'] as Map?,
                },
              },
            },
            fromJson: (value) => Map<String, dynamic>.from(value as Map),
          );
      if (!mounted) return;
      if (result?['reply'] is! String || result?['draft'] is! Map) {
        throw StateError('生成结果格式无效');
      }
      final patch = Map<String, dynamic>.from(result!['draft'] as Map);
      setState(() {
        _proposal = {
          ...?_proposal,
          ...patch,
          'personalityConfig': {
            ...?_proposal?['personalityConfig'] as Map?,
            ...?patch['personalityConfig'] as Map?,
          },
        };
        _messages
          ..clear()
          ..addAll(history)
          ..add({'role': 'assistant', 'content': result['reply'] as String});
        _input.clear();
        _pendingMessage = null;
      });
    } catch (_) {
      if (mounted) setState(() => _error = '生成失败，请检查默认文本模型后重试；现有编辑内容未修改。');
    } finally {
      if (mounted) setState(() => _busy = false);
      _scrollToLatest();
    }
  }

  void _scrollToLatest() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted && _scroll.hasClients) {
        _scroll.jumpTo(_scroll.position.maxScrollExtent);
      }
    });
  }

  void _next() {
    FocusManager.instance.primaryFocus?.unfocus();
    widget.onApply(_proposal ?? widget.currentDraft());
    setState(() => _proposal = null);
  }

  @override
  Widget build(BuildContext context) => Column(
    crossAxisAlignment: CrossAxisAlignment.stretch,
    children: [
      Expanded(
        child: ListView(
          controller: _scroll,
          padding: const EdgeInsets.fromLTRB(16, 8, 16, 16),
          children: [
            _bubble(
              context,
              '你想创建怎样的角色？告诉我角色的身份、性格或故事，我们可以一起完善。也可以直接点击下一步，从空白角色卡开始编辑。',
              false,
            ),
            for (final message in _messages)
              _bubble(context, message['content']!, message['role'] == 'user'),
            if (_pendingMessage != null)
              _bubble(context, _pendingMessage!, true),
            if (_busy) _bubble(context, '正在整理角色草稿…', false),
            if (_error != null) _bubble(context, _error!, false),
            if (_proposal != null)
              Padding(
                padding: const EdgeInsets.only(top: 8),
                child: Text(
                  '角色草稿已更新，下一步可手动调整。',
                  style: Theme.of(context).textTheme.bodySmall,
                ),
              ),
          ],
        ),
      ),
      Material(
        color: Theme.of(context).colorScheme.surface,
        child: Padding(
          padding: const EdgeInsets.all(12),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            mainAxisSize: MainAxisSize.min,
            children: [
              Row(
                crossAxisAlignment: CrossAxisAlignment.end,
                children: [
                  Expanded(
                    child: TextField(
                      controller: _input,
                      enabled: !_busy,
                      minLines: 1,
                      maxLines: 4,
                      maxLength: 4000,
                      decoration: const InputDecoration(
                        labelText: '角色需求',
                        hintText: '描述你想创建的角色…',
                        counterText: '',
                      ),
                    ),
                  ),
                  const SizedBox(width: 8),
                  IconButton.filled(
                    tooltip: '发送',
                    onPressed: _busy ? null : _send,
                    icon: const Icon(Icons.arrow_upward),
                  ),
                ],
              ),
              const SizedBox(height: 12),
              FilledButton(onPressed: _next, child: const Text('下一步：编辑角色卡')),
            ],
          ),
        ),
      ),
    ],
  );

  Widget _bubble(BuildContext context, String content, bool user) {
    final colors = Theme.of(context).colorScheme;
    return Align(
      alignment: user ? Alignment.centerRight : Alignment.centerLeft,
      child: Padding(
        padding: const EdgeInsets.only(bottom: 16),
        child: ConstrainedBox(
          constraints: BoxConstraints(
            maxWidth: MediaQuery.sizeOf(context).width * .84,
          ),
          child: Column(
            crossAxisAlignment: user
                ? CrossAxisAlignment.end
                : CrossAxisAlignment.start,
            children: [
              Padding(
                padding: const EdgeInsets.only(bottom: 6),
                child: Text(
                  user ? '你' : '角色设计助手',
                  style: Theme.of(context).textTheme.labelMedium,
                ),
              ),
              Container(
                padding: const EdgeInsets.symmetric(
                  horizontal: 14,
                  vertical: 12,
                ),
                decoration: BoxDecoration(
                  color: user
                      ? colors.primaryContainer
                      : colors.surfaceContainerHigh,
                  borderRadius: BorderRadius.circular(18),
                ),
                child: SelectableText(
                  content,
                  style: TextStyle(
                    color: user ? colors.onPrimaryContainer : colors.onSurface,
                    height: 1.5,
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

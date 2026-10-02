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
  final _messages = <Map<String, String>>[];
  Map<String, dynamic>? _proposal;
  bool _hasAppliedDraft = false;
  bool _busy = false;
  String? _error;

  @override
  void dispose() {
    _input.dispose();
    super.dispose();
  }

  Future<void> _send() async {
    final text = _input.text.trim();
    if (_busy || text.isEmpty) return;
    setState(() {
      _busy = true;
      _error = null;
    });
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
      });
    } catch (_) {
      if (mounted) setState(() => _error = '生成失败，请检查默认文本模型后重试；现有编辑内容未修改。');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) => Column(
    crossAxisAlignment: CrossAxisAlignment.stretch,
    children: [
      const Text('描述角色设想，或通过多轮对话调整。生成草稿同步后仍可修改，保存后才生效。'),
      const SizedBox(height: 16),
      ConstrainedBox(
        constraints: const BoxConstraints(maxHeight: 320),
        child: SingleChildScrollView(
          child: Column(
            children: [
              for (final message in _messages)
                Align(
                  alignment: message['role'] == 'user'
                      ? Alignment.centerRight
                      : Alignment.centerLeft,
                  child: Card(
                    child: Padding(
                      padding: const EdgeInsets.all(12),
                      child: SelectableText(message['content']!),
                    ),
                  ),
                ),
              if (_busy)
                const Padding(
                  padding: EdgeInsets.all(16),
                  child: Text('正在整理角色草稿…'),
                ),
            ],
          ),
        ),
      ),
      TextField(
        controller: _input,
        enabled: !_busy,
        maxLines: 3,
        maxLength: 4000,
        decoration: const InputDecoration(
          labelText: '角色需求',
          hintText: '例如：设计一位喜欢天文、说话简洁的图书管理员',
        ),
      ),
      Wrap(
        spacing: 12,
        runSpacing: 8,
        children: [
          FilledButton(
            onPressed: _busy ? null : _send,
            child: const Text('发送'),
          ),
          OutlinedButton(
            onPressed: _busy || (_proposal == null && !_hasAppliedDraft)
                ? null
                : () {
                    widget.onApply(_proposal ?? widget.currentDraft());
                    setState(() {
                      _proposal = null;
                      _hasAppliedDraft = true;
                    });
                  },
            child: const Text('下一步：编辑角色卡'),
          ),
        ],
      ),
      if (_error != null)
        Padding(padding: const EdgeInsets.only(top: 12), child: Text(_error!)),
      if (_proposal != null)
        const Padding(
          padding: EdgeInsets.only(top: 12),
          child: Text('草稿已更新，可以继续对话或进入下一步手动调整。'),
        ),
    ],
  );
}

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../core/settings/chat_input_preferences.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/errors/original_error.dart';

class ChatInputSettings extends ConsumerStatefulWidget {
  const ChatInputSettings({super.key});
  @override
  ConsumerState<ChatInputSettings> createState() => _ChatInputSettingsState();
}

class _ChatInputSettingsState extends ConsumerState<ChatInputSettings> {
  bool _busy = true;
  String? _error;
  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    if (mounted) {
      setState(() {
        _busy = true;
        _error = null;
      });
    }
    try {
      await ref.read(chatInputPreferencesProvider.notifier).init();
    } catch (error, stackTrace) {
      if (mounted) _error = '$error\n${originalErrorText(error)}\n$stackTrace';
    }
    if (mounted) setState(() => _busy = false);
  }

  Future<void> _update(bool value) async {
    setState(() => _busy = true);
    try {
      await ref
          .read(chatInputPreferencesProvider.notifier)
          .setSendOnEnter(value);
    } catch (error, stackTrace) {
      if (mounted) {
        _showDetails(
          '保存发送方式失败。\n\n原始错误：\n$error\n${originalErrorText(error)}\n$stackTrace',
        );
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  void _showDetails(String message) {
    showDialog<void>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('发送方式'),
        content: SingleChildScrollView(child: SelectableText(message)),
        actions: [
          if (_error != null)
            TextButton(
              onPressed: () {
                Navigator.pop(context);
                _load();
              },
              child: const Text('重试'),
            ),
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('知道了'),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) => GestureDetector(
    behavior: HitTestBehavior.opaque,
    onTap: _busy || _error != null
        ? () => _showDetails(
            _busy ? '正在处理，请稍后重试。' : '无法读取发送方式。\n\n原始错误：\n$_error',
          )
        : null,
    child: AmitiaSwitchTile(
      title: '回车发送',
      subtitle: _error != null
          ? '无法读取发送方式，点击查看原因并重试。'
          : '开启时回车发送；关闭时回车换行。发送按钮始终可用。',
      value: ref.watch(chatInputPreferencesProvider),
      onChanged: _busy || _error != null ? null : _update,
    ),
  );
}

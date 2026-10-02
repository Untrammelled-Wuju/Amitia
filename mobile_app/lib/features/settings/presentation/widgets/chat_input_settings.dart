import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../core/settings/chat_input_preferences.dart';
import '../../../../core/widgets/amitia_misc.dart';

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
    try {
      await ref.read(chatInputPreferencesProvider.notifier).init();
    } catch (_) {
      _error = '无法读取发送方式';
    }
    if (mounted) setState(() => _busy = false);
  }

  Future<void> _update(bool value) async {
    setState(() => _busy = true);
    try {
      await ref
          .read(chatInputPreferencesProvider.notifier)
          .setSendOnEnter(value);
    } catch (_) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(const SnackBar(content: Text('保存发送方式失败，请重试')));
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) => AmitiaSwitchTile(
    title: '回车发送',
    subtitle: _error ?? '开启时回车发送；关闭时回车换行。发送按钮始终可用。',
    value: ref.watch(chatInputPreferencesProvider),
    onChanged: _busy || _error != null ? null : _update,
  );
}

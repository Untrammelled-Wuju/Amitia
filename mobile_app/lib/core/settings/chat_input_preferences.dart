import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

const chatSendOnEnterStorageKey = 'amitia.chat.send-on-enter.mobile.v1';
final chatInputPreferencesProvider =
    StateNotifierProvider<ChatInputPreferencesNotifier, bool>(
      (ref) => ChatInputPreferencesNotifier(),
    );

class ChatInputPreferencesNotifier extends StateNotifier<bool> {
  ChatInputPreferencesNotifier() : super(false);
  Future<void>? _initialization;

  Future<void> init() => _initialization ??= _load();

  Future<void> _load() async {
    final preferences = await SharedPreferences.getInstance();
    if (mounted) {
      state = preferences.getBool(chatSendOnEnterStorageKey) ?? false;
    }
  }

  Future<void> setSendOnEnter(bool value) async {
    await init();
    final preferences = await SharedPreferences.getInstance();
    if (!await preferences.setBool(chatSendOnEnterStorageKey, value)) {
      throw StateError('保存发送方式失败');
    }
    if (mounted) state = value;
  }
}

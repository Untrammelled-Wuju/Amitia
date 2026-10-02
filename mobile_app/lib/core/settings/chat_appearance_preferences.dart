import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

enum ChatMessageStyle { flow, bubble }

const chatMessageStyleStorageKey = 'amitia.chat.message-style.mobile.v1';
final chatAppearancePreferencesProvider =
    StateNotifierProvider<ChatAppearancePreferencesNotifier, ChatMessageStyle>(
      (ref) => ChatAppearancePreferencesNotifier(),
    );

class ChatAppearancePreferencesNotifier
    extends StateNotifier<ChatMessageStyle> {
  ChatAppearancePreferencesNotifier() : super(ChatMessageStyle.flow);
  Future<void>? _initialization;

  Future<void> init() => _initialization ??= _load();

  Future<void> _load() async {
    final preferences = await SharedPreferences.getInstance();
    if (mounted) {
      state = preferences.getString(chatMessageStyleStorageKey) == 'bubble'
          ? ChatMessageStyle.bubble
          : ChatMessageStyle.flow;
    }
  }

  Future<void> setMessageStyle(ChatMessageStyle value) async {
    await init();
    final preferences = await SharedPreferences.getInstance();
    if (!await preferences.setString(chatMessageStyleStorageKey, value.name)) {
      throw StateError('保存聊天界面风格失败');
    }
    if (mounted) state = value;
  }
}

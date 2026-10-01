import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

const chatPermissionRequestApproval = 'request_approval';
const chatPermissionFullAccess = 'full_access';
const chatPermissionModeStorageKey = 'amitia.chat.permission-mode.v1';

String normalizeChatPermissionMode(String? value) {
  return value == chatPermissionFullAccess
      ? chatPermissionFullAccess
      : chatPermissionRequestApproval;
}

final chatPermissionPreferencesProvider =
    StateNotifierProvider<ChatPermissionPreferencesNotifier, String>(
      (ref) => ChatPermissionPreferencesNotifier(),
    );

class ChatPermissionPreferencesNotifier extends StateNotifier<String> {
  ChatPermissionPreferencesNotifier() : super(chatPermissionRequestApproval);

  bool _initialized = false;

  String get mode => state;

  Future<void> init() async {
    if (_initialized) return;
    _initialized = true;
    final preferences = await SharedPreferences.getInstance();
    state = normalizeChatPermissionMode(
      preferences.getString(chatPermissionModeStorageKey),
    );
  }

  Future<void> setMode(String value) async {
    final next = normalizeChatPermissionMode(value);
    if (state != next) {
      state = next;
    }
    final preferences = await SharedPreferences.getInstance();
    await preferences.setString(chatPermissionModeStorageKey, next);
  }
}

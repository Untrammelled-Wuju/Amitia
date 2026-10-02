import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

final replyNotificationPreferencesProvider =
    StateNotifierProvider<
      ReplyNotificationPreferences,
      ReplyNotificationOptions
    >((ref) => ReplyNotificationPreferences());

class ReplyNotificationOptions {
  const ReplyNotificationOptions({
    this.enabled = false,
    this.soundEnabled = false,
    this.vibrationEnabled = false,
  });
  final bool enabled;
  final bool soundEnabled;
  final bool vibrationEnabled;

  ReplyNotificationOptions copyWith({
    bool? enabled,
    bool? soundEnabled,
    bool? vibrationEnabled,
  }) => ReplyNotificationOptions(
    enabled: enabled ?? this.enabled,
    soundEnabled: soundEnabled ?? this.soundEnabled,
    vibrationEnabled: vibrationEnabled ?? this.vibrationEnabled,
  );
}

class ReplyNotificationPreferences
    extends StateNotifier<ReplyNotificationOptions> {
  ReplyNotificationPreferences() : super(const ReplyNotificationOptions());
  bool get enabled => state.enabled;
  bool get soundEnabled => state.soundEnabled;
  bool get vibrationEnabled => state.vibrationEnabled;
  Future<void>? _initialization;
  Future<void> init() => _initialization ??= _load().catchError((Object error) {
    _initialization = null;
    throw error;
  });
  Future<void> _load() async {
    final prefs = await SharedPreferences.getInstance();
    if (mounted) {
      state = ReplyNotificationOptions(
        enabled: prefs.getBool('amitia.reply-notifications.mobile.v1') ?? false,
        soundEnabled: prefs.getBool('amitia.reply-sound.mobile.v1') ?? false,
        vibrationEnabled:
            prefs.getBool('amitia.reply-vibration.mobile.v1') ?? false,
      );
    }
  }

  Future<void> setEnabled(bool value) async {
    await init();
    final prefs = await SharedPreferences.getInstance();
    if (!await prefs.setBool('amitia.reply-notifications.mobile.v1', value)) {
      throw StateError('保存通知设置失败');
    }
    if (mounted) {
      state = state.copyWith(enabled: value);
    }
  }

  Future<void> setSoundEnabled(bool value) async {
    await init();
    final prefs = await SharedPreferences.getInstance();
    if (!await prefs.setBool('amitia.reply-sound.mobile.v1', value)) {
      throw StateError('保存提示音设置失败');
    }
    if (mounted) {
      state = state.copyWith(soundEnabled: value);
    }
  }

  Future<void> setVibrationEnabled(bool value) async {
    await init();
    final prefs = await SharedPreferences.getInstance();
    if (!await prefs.setBool('amitia.reply-vibration.mobile.v1', value)) {
      throw StateError('保存震动设置失败');
    }
    if (mounted) {
      state = state.copyWith(vibrationEnabled: value);
    }
  }
}

import 'dart:convert';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

enum ChatMessageStyle { flow, bubble }

enum AiAvatarShape { circle, rounded, custom }

class AiAvatarShapePreference {
  final AiAvatarShape shape;
  final double roundness;
  const AiAvatarShapePreference({
    this.shape = AiAvatarShape.rounded,
    this.roundness = 64,
  });
  double radius(double size) =>
      size *
      (shape == AiAvatarShape.circle
          ? 0.5
          : shape == AiAvatarShape.custom
          ? roundness / 200
          : 0.32);
}

const aiAvatarShapeStorageKey = 'amitia.chat.ai-avatar-shape.mobile.v1';
final aiAvatarShapeProvider =
    StateNotifierProvider<AiAvatarShapeNotifier, AiAvatarShapePreference>(
      (ref) => AiAvatarShapeNotifier(),
    );

class AiAvatarShapeNotifier extends StateNotifier<AiAvatarShapePreference> {
  AiAvatarShapeNotifier() : super(const AiAvatarShapePreference());
  Future<void>? _initialization;
  Future<void> init() => _initialization ??= _load();
  Future<void> _load() async {
    final preferences = await SharedPreferences.getInstance();
    AiAvatarShapePreference loaded = const AiAvatarShapePreference();
    try {
      final saved = jsonDecode(
        preferences.getString(aiAvatarShapeStorageKey) ?? 'null',
      );
      if (saved is Map &&
          saved['roundness'] is num &&
          (saved['roundness'] as num).isFinite) {
        final shape = AiAvatarShape.values
            .where((value) => value.name == saved['shape'])
            .firstOrNull;
        if (shape != null)
          loaded = AiAvatarShapePreference(
            shape: shape,
            roundness: (saved['roundness'] as num).clamp(0, 100).toDouble(),
          );
      }
    } catch (_) {}
    if (mounted) state = loaded;
  }

  Future<void> setShape(AiAvatarShape shape, [double? roundness]) async {
    await init();
    final value = roundness ?? state.roundness;
    if (!value.isFinite) throw ArgumentError.value(value);
    final next = AiAvatarShapePreference(
      shape: shape,
      roundness: value.clamp(0, 100).toDouble(),
    );
    final preferences = await SharedPreferences.getInstance();
    if (!await preferences.setString(
      aiAvatarShapeStorageKey,
      jsonEncode({'shape': shape.name, 'roundness': next.roundness}),
    ))
      throw StateError('保存 AI 头像形状失败');
    if (mounted) state = next;
  }
}

const aiAvatarStorageKey = 'amitia.chat.ai-avatar.mobile.v1';
final aiAvatarPreferencesProvider =
    StateNotifierProvider<AiAvatarPreferencesNotifier, bool>(
      (ref) => AiAvatarPreferencesNotifier(),
    );

class AiAvatarPreferencesNotifier extends StateNotifier<bool> {
  AiAvatarPreferencesNotifier() : super(true);
  Future<void>? _initialization;
  Future<void> init() => _initialization ??= _load();

  Future<void> _load() async {
    final preferences = await SharedPreferences.getInstance();
    if (mounted) {
      state = preferences.getBool(aiAvatarStorageKey) ?? true;
    }
  }

  Future<void> setEnabled(bool value) async {
    await init();
    final preferences = await SharedPreferences.getInstance();
    if (!await preferences.setBool(aiAvatarStorageKey, value)) {
      throw StateError('保存 AI 头像设置失败');
    }
    if (mounted) state = value;
  }
}

const userMessageGlassStorageKey = 'amitia.chat.user-message-glass.mobile.v1';
const aiNameStorageKey = 'amitia.chat.ai-name.mobile.v1';
final aiNamePreferencesProvider =
    StateNotifierProvider<AiNamePreferencesNotifier, bool>(
      (ref) => AiNamePreferencesNotifier(),
    );

class AiNamePreferencesNotifier extends StateNotifier<bool> {
  AiNamePreferencesNotifier() : super(true);
  Future<void>? _initialization;
  Future<void> init() => _initialization ??= _load();

  Future<void> _load() async {
    final preferences = await SharedPreferences.getInstance();
    if (mounted) {
      state = preferences.getBool(aiNameStorageKey) ?? true;
    }
  }

  Future<void> setEnabled(bool value) async {
    await init();
    final preferences = await SharedPreferences.getInstance();
    if (!await preferences.setBool(aiNameStorageKey, value)) {
      throw StateError('保存 AI 名称设置失败');
    }
    if (mounted) state = value;
  }
}

enum UserMessageMaterial { solid, frosted, water }

const userMessageMaterialStorageKey =
    'amitia.chat.user-message-material.mobile.v1';
final userMessageMaterialProvider =
    StateNotifierProvider<UserMessageMaterialNotifier, UserMessageMaterial>(
      (ref) => UserMessageMaterialNotifier(),
    );

class UserMessageMaterialNotifier extends StateNotifier<UserMessageMaterial> {
  UserMessageMaterialNotifier() : super(UserMessageMaterial.solid);
  Future<void>? _initialization;
  Future<void> init() => _initialization ??= _load();

  Future<void> _load() async {
    final preferences = await SharedPreferences.getInstance();
    if (mounted) {
      final saved = preferences.getString(userMessageMaterialStorageKey);
      state =
          UserMessageMaterial.values
              .where((value) => value.name == saved)
              .firstOrNull ??
          (preferences.getBool(userMessageGlassStorageKey) == true
              ? UserMessageMaterial.frosted
              : UserMessageMaterial.solid);
    }
  }

  Future<void> setMaterial(UserMessageMaterial value) async {
    await init();
    final preferences = await SharedPreferences.getInstance();
    if (!await preferences.setString(
      userMessageMaterialStorageKey,
      value.name,
    )) {
      throw StateError('保存用户消息背景材质失败');
    }
    if (mounted) state = value;
  }
}

const chatMessageStyleStorageKey = 'amitia.chat.message-style.mobile.v1';
final chatAppearancePreferencesProvider =
    StateNotifierProvider<ChatAppearancePreferencesNotifier, ChatMessageStyle>(
      (ref) => ChatAppearancePreferencesNotifier(),
    );

class ChatAppearancePreferencesNotifier
    extends StateNotifier<ChatMessageStyle> {
  ChatAppearancePreferencesNotifier() : super(ChatMessageStyle.flow);
  ChatMessageStyle get messageStyle => state;
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

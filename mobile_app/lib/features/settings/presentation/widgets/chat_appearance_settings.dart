import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/settings/chat_appearance_preferences.dart';
import '../../../../core/widgets/amitia_misc.dart';

class ChatAppearanceSettings extends ConsumerStatefulWidget {
  const ChatAppearanceSettings({super.key});

  @override
  ConsumerState<ChatAppearanceSettings> createState() =>
      _ChatAppearanceSettingsState();
}

class _ChatAppearanceSettingsState
    extends ConsumerState<ChatAppearanceSettings> {
  bool _busy = true;
  bool _failed = false;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      await ref.read(chatAppearancePreferencesProvider.notifier).init();
      await ref.read(userMessageMaterialProvider.notifier).init();
      await ref.read(aiAvatarPreferencesProvider.notifier).init();
      await ref.read(aiNamePreferencesProvider.notifier).init();
    } catch (_) {
      _failed = true;
    }
    if (mounted) setState(() => _busy = false);
  }

  Future<void> _update(int index) async {
    setState(() => _busy = true);
    try {
      await ref
          .read(chatAppearancePreferencesProvider.notifier)
          .setMessageStyle(ChatMessageStyle.values[index]);
    } catch (_) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(const SnackBar(content: Text('保存聊天界面风格失败，请重试')));
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final style = ref.watch(chatAppearancePreferencesProvider);
    final material = ref.watch(userMessageMaterialProvider);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        IgnorePointer(
          ignoring: _busy || _failed,
          child: AmitiaSegmentedControl(
            segments: const ['流式消息', '气泡消息'],
            selectedIndex: style.index,
            onChanged: _update,
          ),
        ),
        const SizedBox(height: 10),
        Text(
          _failed
              ? '无法读取聊天界面风格，请重新打开设置。'
              : style == ChatMessageStyle.flow
              ? '保留当前的连续阅读与实时生成。'
              : '用户消息靠右，AI 消息靠左；按完整消息推送，思考、工具和多媒体各自独立显示。',
          style: AppTypography.caption(
            context,
          ).copyWith(color: context.textSecondary),
        ),
        const SizedBox(height: 12),
        AmitiaSwitchTile(
          title: '显示 AI 头像',
          subtitle: '在 AI 消息旁显示角色头像，两种聊天风格均适用。',
          value: ref.watch(aiAvatarPreferencesProvider),
          onChanged: _busy || _failed ? null : _updateAvatar,
        ),
        AmitiaSwitchTile(
          title: '显示 AI 名称',
          subtitle: '在 AI 消息中显示角色名称，与头像独立控制。',
          value: ref.watch(aiNamePreferencesProvider),
          onChanged: _busy || _failed ? null : _updateName,
        ),
        AmitiaSwitchTile(
          title: '用户消息磨砂玻璃',
          subtitle: '以半透明背景和局部模糊显示用户消息，两种聊天风格均适用。',
          value: material == UserMessageMaterial.frosted,
          onChanged: _busy || _failed
              ? null
              : (value) => _updateMaterial(
                  value
                      ? UserMessageMaterial.frosted
                      : UserMessageMaterial.solid,
                ),
        ),
        AmitiaSwitchTile(
          title: '用户消息水玻璃',
          subtitle: '更通透的轻模糊与边缘高光，开启后自动关闭磨砂玻璃。',
          value: material == UserMessageMaterial.water,
          onChanged: _busy || _failed
              ? null
              : (value) => _updateMaterial(
                  value ? UserMessageMaterial.water : UserMessageMaterial.solid,
                ),
        ),
      ],
    );
  }

  Future<void> _updateMaterial(UserMessageMaterial value) async {
    setState(() => _busy = true);
    try {
      await ref.read(userMessageMaterialProvider.notifier).setMaterial(value);
    } catch (_) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(const SnackBar(content: Text('保存用户消息背景材质失败，请重试')));
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _updateAvatar(bool value) async {
    setState(() => _busy = true);
    try {
      await ref.read(aiAvatarPreferencesProvider.notifier).setEnabled(value);
    } catch (_) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(const SnackBar(content: Text('保存 AI 头像设置失败，请重试')));
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _updateName(bool value) async {
    setState(() => _busy = true);
    try {
      await ref.read(aiNamePreferencesProvider.notifier).setEnabled(value);
    } catch (_) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(const SnackBar(content: Text('保存 AI 名称设置失败，请重试')));
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }
}

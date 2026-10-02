import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../core/services/reply_notification_service.dart';
import '../../../../core/settings/reply_notification_preferences.dart';
import '../../../../core/widgets/amitia_misc.dart';

enum _ReplyFeedback { notification, sound, vibration }

class ReplyNotificationSettings extends ConsumerStatefulWidget {
  const ReplyNotificationSettings({super.key});
  @override
  ConsumerState<ReplyNotificationSettings> createState() =>
      _ReplyNotificationSettingsState();
}

class _ReplyNotificationSettingsState
    extends ConsumerState<ReplyNotificationSettings> {
  bool _busy = true;
  bool _loaded = false;
  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      await ref.read(replyNotificationPreferencesProvider.notifier).init();
      _loaded = true;
    } catch (_) {
      _loaded = false;
    }
    if (mounted) setState(() => _busy = false);
  }

  Future<void> _update(
    bool value, {
    _ReplyFeedback kind = _ReplyFeedback.notification,
  }) async {
    setState(() => _busy = true);
    try {
      if (value &&
          kind != _ReplyFeedback.vibration &&
          (kind == _ReplyFeedback.notification ||
              ref.read(replyNotificationServiceProvider).platform == 'ios')) {
        await ref.read(replyNotificationServiceProvider).requestPermission();
      }
      final preferences = ref.read(
        replyNotificationPreferencesProvider.notifier,
      );
      await switch (kind) {
        _ReplyFeedback.notification => preferences.setEnabled(value),
        _ReplyFeedback.sound => preferences.setSoundEnabled(value),
        _ReplyFeedback.vibration => preferences.setVibrationEnabled(value),
      };
    } catch (_) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(const SnackBar(content: Text('无法保存提醒设置，请检查系统权限后重试')));
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) => Column(
    children: [
      AmitiaSwitchTile(
        title: '回复完成通知',
        subtitle: '仅后台正常完成回复时通知；前台、失败和取消不通知',
        value: ref.watch(replyNotificationPreferencesProvider).enabled,
        onChanged: _busy || !_loaded ? null : _update,
      ),
      AmitiaSwitchTile(
        title: '回复提示音',
        subtitle: '仅后台正常完成回复时播放，可独立开启；遵循系统静音和勿扰设置',
        value: ref.watch(replyNotificationPreferencesProvider).soundEnabled,
        onChanged: _busy || !_loaded
            ? null
            : (value) => _update(value, kind: _ReplyFeedback.sound),
      ),
      if (ref.watch(replyNotificationServiceProvider).platform == 'android')
        AmitiaSwitchTile(
          title: '回复震动',
          subtitle: '仅后台正常完成回复时短震动一次，可独立开启；遵循系统静音和勿扰设置',
          value: ref
              .watch(replyNotificationPreferencesProvider)
              .vibrationEnabled,
          onChanged: _busy || !_loaded
              ? null
              : (value) => _update(value, kind: _ReplyFeedback.vibration),
        ),
      if (!_loaded && !_busy)
        TextButton(onPressed: _load, child: const Text('重试读取通知设置')),
    ],
  );
}

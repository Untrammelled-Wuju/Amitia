import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/native_bridge/providers/native_bridge_relay_provider.dart';
import '../../../../core/ui_runtime/ui_device_identity.dart';
import '../widgets/reply_notification_settings.dart';
import '../widgets/settings_section.dart';

class NotificationSettingsPage extends ConsumerStatefulWidget {
  const NotificationSettingsPage({super.key});
  @override
  ConsumerState<NotificationSettingsPage> createState() =>
      _NotificationSettingsPageState();
}

class _NotificationSettingsPageState
    extends ConsumerState<NotificationSettingsPage> {
  bool _notifications = false;
  bool _notificationsUpdating = false;
  bool _loading = true;
  String? _error;
  String? _notificationDeviceIdValue;

  @override
  void initState() {
    super.initState();
    _loadSettings();
  }

  Future<void> _loadSettings() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final deviceId = await _notificationDeviceId();
      final settings = await ref
          .read(systemServiceProvider)
          .notificationSettings(deviceId: deviceId);
      if (!mounted) return;
      setState(() {
        _notifications =
            settings?['enabled'] == true && settings?['subscribed'] == true;
        _loading = false;
      });
    } catch (error) {
      if (mounted) {
        setState(() {
          _error = error.toString();
          _loading = false;
        });
      }
    }
  }

  Future<String> _notificationDeviceId() async {
    final cached = _notificationDeviceIdValue;
    if (cached != null && cached.isNotEmpty) return cached;
    final resolved = await UIDeviceIdentity().getOrCreate();
    _notificationDeviceIdValue = resolved;
    return resolved;
  }

  Future<void> _setNotifications(bool enabled) async {
    if (_notificationsUpdating) return;
    setState(() => _notificationsUpdating = true);
    try {
      final svc = ref.read(systemServiceProvider);
      final deviceId = await _notificationDeviceId();
      if (enabled) {
        await _ensureNativeNotificationPermission();
      }
      final result = enabled
          ? await svc.subscribeNotifications(deviceId: deviceId)
          : await svc.unsubscribeNotifications(deviceId: deviceId);
      final settings = await svc.notificationSettings(deviceId: deviceId);
      if (!mounted) return;
      setState(
        () => _notifications =
            settings?['enabled'] == true && settings?['subscribed'] == true,
      );
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(_notifications ? '通知已开启' : '通知已关闭'),
          duration: const Duration(seconds: 1),
        ),
      );
      if (result == null) return;
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('更新通知设置失败: $e')));
      }
    } finally {
      if (mounted) setState(() => _notificationsUpdating = false);
    }
  }

  Future<void> _ensureNativeNotificationPermission() async {
    if (kIsWeb) return;
    final platform = switch (defaultTargetPlatform) {
      TargetPlatform.android => 'android',
      TargetPlatform.iOS => 'ios',
      _ => null,
    };
    if (platform == null) return;
    final dispatcher = ref.read(nativeBridgePlatformDispatcherProvider);
    final response = await dispatcher.execute(<String, dynamic>{
      'protocolVersion': 1,
      'requestId':
          'settings-notification-permission-${DateTime.now().microsecondsSinceEpoch}',
      'platform': platform,
      'operation': 'notification.request_permission',
      'payload': const <String, dynamic>{},
    });
    if (!const {
      'success',
      'ok',
    }.contains((response['status'] ?? '').toString())) {
      final error = response['error'];
      final message = error is Map
          ? (error['message'] ?? error['code'])?.toString()
          : null;
      throw StateError(message?.isNotEmpty == true ? message! : '系统通知权限未授予');
    }
  }

  Future<void> _testNotification() async {
    try {
      final deviceId = await _notificationDeviceId();
      final backendResult = await ref
          .read(systemServiceProvider)
          .testNotification(deviceId: deviceId);
      final accepted = backendResult?['accepted'] == true;
      final reason = backendResult?['reason']?.toString();
      if (!accepted) {
        throw StateError(
          reason != null && reason.isNotEmpty ? reason : '后端通知配置未就绪',
        );
      }
      final platform = switch (defaultTargetPlatform) {
        TargetPlatform.android => 'android',
        TargetPlatform.iOS => 'ios',
        TargetPlatform.windows => 'windows',
        _ => null,
      };
      if (kIsWeb || platform == null) {
        throw UnsupportedError('当前平台尚未接入可验证的本地系统通知投递桥');
      }
      final dispatcher = ref.read(nativeBridgePlatformDispatcherProvider);
      final nativeResult = await dispatcher.execute({
        'protocolVersion': 1,
        'requestId':
            'settings-notification-test-${DateTime.now().microsecondsSinceEpoch}',
        'platform': platform,
        'operation': 'notification.post',
        'payload': const {
          'title': 'Amitia 测试通知',
          'body': '如果你看到这条通知，说明系统通知投递链路可用。',
          'channel': 'amitia_agent',
          'silent': false,
        },
      });
      if (!const {
        'success',
        'ok',
      }.contains((nativeResult['status'] ?? '').toString())) {
        final error = nativeResult['error'];
        final message = error is Map
            ? (error['message'] ?? error['code'])?.toString()
            : null;
        throw StateError(message?.isNotEmpty == true ? message! : '系统通知投递失败');
      }
      if (!mounted) return;
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(const SnackBar(content: Text('测试通知已真实投递到系统通知中心')));
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(
              '发送测试通知失败: ${e.toString().replaceFirst('Bad state: ', '').replaceFirst('Unsupported operation: ', '')}',
            ),
          ),
        );
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '通知',
        showBackButton: true,
        fallbackRoute: AppRoutes.settings,
      ),
      body: ListView(
        padding: EdgeInsets.symmetric(vertical: AppSpacing.lg),
        children: [
          const SettingsSection(
            title: '对话回复',
            children: [ReplyNotificationSettings()],
          ),
          SettingsSection(
            title: '设备通知',
            children: [
              if (_loading)
                const Padding(
                  padding: EdgeInsets.all(20),
                  child: Center(child: CircularProgressIndicator()),
                )
              else if (_error != null)
                Padding(
                  padding: const EdgeInsets.all(16),
                  child: Column(
                    children: [
                      Text('读取通知设置失败：$_error'),
                      TextButton(
                        onPressed: _loadSettings,
                        child: const Text('重试'),
                      ),
                    ],
                  ),
                )
              else ...[
                AmitiaSwitchTile(
                  title: '消息与提醒',
                  subtitle: '接收服务和提醒通知',
                  value: _notifications,
                  onChanged: _notificationsUpdating ? null : _setNotifications,
                ),
                ListTile(
                  leading: const Icon(Icons.notifications_active_outlined),
                  title: const Text('发送测试通知'),
                  onTap: _notificationsUpdating ? null : _testNotification,
                ),
              ],
            ],
          ),
        ],
      ),
    );
  }
}

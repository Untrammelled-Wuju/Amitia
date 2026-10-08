import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/notification_runtime_bootstrap.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../core/notifications/notification_platform_bridge.dart';
import '../../../../core/notifications/native_data_capabilities.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../widgets/settings_section.dart';

class NotificationSettingsPage extends ConsumerStatefulWidget {
  const NotificationSettingsPage({super.key});

  @override
  ConsumerState<NotificationSettingsPage> createState() =>
      _NotificationSettingsPageState();
}

class _NotificationSettingsPageState
    extends ConsumerState<NotificationSettingsPage> {
  bool _loading = true;
  bool _updating = false;
  String? _error;
  String? _deviceId;
  String? _provider;
  NotificationPlatformState? _platformState;
  Map<String, dynamic> _serverCapabilities = const <String, dynamic>{};

  bool _pushEnabled = true;
  bool _messagePushEnabled = true;
  bool _executionActivityEnabled = true;
  bool _callPushEnabled = true;
  bool _reminderPushEnabled = true;
  bool _soundEnabled = true;
  String _previewMode = 'full';

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    if (!mounted) return;
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final coordinator = ref.read(notificationCoordinatorProvider);
      await coordinator.refreshRegistration();
      final device = await coordinator.currentDevice();
      final platformState = coordinator.platformState;
      final serverCapabilities = await coordinator.serverCapabilities();
      if (!mounted) return;
      setState(() {
        _platformState = platformState;
        _serverCapabilities = serverCapabilities;
        final rawDeviceId = (device?['deviceId'] ?? '').toString().trim();
        _deviceId = rawDeviceId.isEmpty ? null : rawDeviceId;
        _provider = (device?['preferredProvider'] ?? platformState?.provider)
            ?.toString()
            .trim();
        _pushEnabled = device?['pushEnabled'] != false;
        _messagePushEnabled = device?['messagePushEnabled'] != false;
        _executionActivityEnabled =
            device?['executionActivityEnabled'] != false;
        _callPushEnabled = device?['callPushEnabled'] != false;
        _reminderPushEnabled = device?['reminderPushEnabled'] != false;
        _soundEnabled = device?['soundEnabled'] != false;
        final preview = (device?['previewMode'] ?? 'full').toString();
        _previewMode = const {'full', 'sender_only', 'hidden'}.contains(preview)
            ? preview
            : 'full';
        _loading = false;
      });
    } catch (error) {
      if (!mounted) return;
      setState(() {
        _error = error.toString();
        _loading = false;
      });
    }
  }

  Future<void> _update({
    bool? pushEnabled,
    bool? messagePushEnabled,
    bool? executionActivityEnabled,
    bool? callPushEnabled,
    bool? reminderPushEnabled,
    bool? soundEnabled,
    String? previewMode,
  }) async {
    if (_updating) return;
    setState(() => _updating = true);
    try {
      final coordinator = ref.read(notificationCoordinatorProvider);
      if (pushEnabled == true) {
        final granted = await coordinator.requestPermission();
        if (!granted) {
          throw StateError('系统通知权限未授予');
        }
      }
      await coordinator.updatePreferences(
        pushEnabled: pushEnabled,
        messagePushEnabled: messagePushEnabled,
        executionActivityEnabled: executionActivityEnabled,
        callPushEnabled: callPushEnabled,
        reminderPushEnabled: reminderPushEnabled,
        soundEnabled: soundEnabled,
        previewMode: previewMode,
      );
      await _load();
    } catch (error) {
      if (!mounted) return;
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text('更新通知设置失败：$error')));
    } finally {
      if (mounted) setState(() => _updating = false);
    }
  }

  Future<void> _testNotification() async {
    if (_updating) return;
    setState(() => _updating = true);
    try {
      final result = await ref
          .read(notificationCoordinatorProvider)
          .testNotification();
      final accepted = result?['accepted'] == true;
      final provider = (result?['provider'] ?? _provider ?? 'push').toString();
      if (!accepted) {
        final code = (result?['errorCode'] ?? '').toString();
        final message = (result?['errorMessage'] ?? '').toString();
        final detail = [
          code,
          message,
        ].where((value) => value.isNotEmpty).join(' · ');
        throw StateError(detail.isEmpty ? 'Provider 未接受通知' : detail);
      }
      if (!mounted) return;
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text('测试通知已提交到 $provider')));
    } catch (error) {
      if (!mounted) return;
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text('测试通知失败：$error')));
    } finally {
      if (mounted) setState(() => _updating = false);
    }
  }

  Future<void> _openSystemSettings() async {
    try {
      await ref.read(notificationCoordinatorProvider).openSystemSettings();
    } catch (error) {
      if (!mounted) return;
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text('无法打开系统通知设置：$error')));
    }
  }

  String get _previewLabel {
    return switch (_previewMode) {
      'sender_only' => '仅显示角色名',
      'hidden' => '隐藏内容',
      _ => '显示角色名与消息预览',
    };
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '通知',
        showBackButton: true,
        fallbackRoute: AppRoutes.settings,
      ),
      body: RefreshIndicator(
        onRefresh: _load,
        child: ListView(
          physics: const AlwaysScrollableScrollPhysics(),
          padding: EdgeInsets.symmetric(vertical: AppSpacing.lg),
          children: [
            SettingsSection(
              title: '移动端 Push',
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
                        Text('读取真实通知状态失败：$_error'),
                        const SizedBox(height: 8),
                        TextButton(onPressed: _load, child: const Text('重试')),
                      ],
                    ),
                  )
                else ...[
                  AmitiaSwitchTile(
                    title: '允许 Push',
                    subtitle: '控制此设备的远程消息、任务、通话与提醒通知',
                    value: _pushEnabled,
                    onChanged: _updating
                        ? null
                        : (value) => _update(pushEnabled: value),
                  ),
                  AmitiaSwitchTile(
                    title: '聊天消息',
                    subtitle: 'AI 回复完成后发送系统消息通知',
                    value: _messagePushEnabled,
                    onChanged: !_pushEnabled || _updating
                        ? null
                        : (value) => _update(messagePushEnabled: value),
                  ),
                  AmitiaSwitchTile(
                    title: 'Agent 执行状态',
                    subtitle: _executionSubtitle,
                    value: _executionActivityEnabled,
                    onChanged: !_pushEnabled || _updating
                        ? null
                        : (value) => _update(executionActivityEnabled: value),
                  ),
                  AmitiaSwitchTile(
                    title: '通话',
                    subtitle: '接收实时语音/视频通话相关通知',
                    value: _callPushEnabled,
                    onChanged: !_pushEnabled || _updating
                        ? null
                        : (value) => _update(callPushEnabled: value),
                  ),
                  AmitiaSwitchTile(
                    title: '提醒与主动消息',
                    subtitle: '接收日程、学习、角色主动消息等提醒',
                    value: _reminderPushEnabled,
                    onChanged: !_pushEnabled || _updating
                        ? null
                        : (value) => _update(reminderPushEnabled: value),
                  ),
                  AmitiaSwitchTile(
                    title: '声音',
                    subtitle: '允许此设备的消息 Push 使用通知声音',
                    value: _soundEnabled,
                    onChanged: !_pushEnabled || _updating
                        ? null
                        : (value) => _update(soundEnabled: value),
                  ),
                  ListTile(
                    leading: const Icon(Icons.visibility_outlined),
                    title: const Text('锁屏预览'),
                    subtitle: Text(_previewLabel),
                    trailing: DropdownButtonHideUnderline(
                      child: DropdownButton<String>(
                        value: _previewMode,
                        onChanged: !_pushEnabled || _updating
                            ? null
                            : (value) {
                                if (value == null) return;
                                _update(previewMode: value);
                              },
                        items: const [
                          DropdownMenuItem(value: 'full', child: Text('完整')),
                          DropdownMenuItem(
                            value: 'sender_only',
                            child: Text('仅角色'),
                          ),
                          DropdownMenuItem(value: 'hidden', child: Text('隐藏')),
                        ],
                      ),
                    ),
                  ),
                  ListTile(
                    leading: const Icon(Icons.notifications_active_outlined),
                    title: const Text('发送真实测试通知'),
                    subtitle: Text(_testDeliveryLabel),
                    onTap: !_pushEnabled || _updating
                        ? null
                        : _testNotification,
                  ),
                  ListTile(
                    leading: const Icon(Icons.settings_outlined),
                    title: const Text('系统通知设置'),
                    subtitle: const Text('打开 Android / iOS 系统通知权限页面'),
                    onTap: _openSystemSettings,
                  ),
                ],
              ],
            ),
            if (!_loading && _error == null)
              SettingsSection(
                title: '设备能力',
                children: [
                  _CapabilityTile(title: '设备', value: _deviceId ?? '等待注册'),
                  _CapabilityTile(title: '投递模式', value: _deliveryModeLabel),
                  _CapabilityTile(
                    title: '设备首选 Provider',
                    value: _provider?.isNotEmpty == true ? _provider! : '未配置',
                  ),
                  _CapabilityTile(
                    title: '系统通知权限',
                    value: _platformState?.notificationsEnabled == true
                        ? '已允许'
                        : '未允许',
                  ),
                  _CapabilityTile(
                    title: 'Live Activity',
                    value: _platformState?.liveActivitySupported == true
                        ? '支持'
                        : '不支持',
                  ),
                  _CapabilityTile(
                    title: 'Dynamic Island',
                    value: _platformState?.dynamicIslandSupported == true
                        ? '支持'
                        : '不支持',
                  ),
                  _CapabilityTile(
                    title: 'Android ProgressStyle',
                    value: _platformState?.progressStyleSupported == true
                        ? '支持'
                        : '当前系统回落普通进度通知',
                  ),
                  _CapabilityTile(
                    title: 'Core Provider',
                    value: _providerReadinessLabel,
                  ),
                  _CapabilityTile(
                    title: 'Native Data',
                    value: _nativeDataReadinessLabel,
                  ),
                ],
              ),
          ],
        ),
      ),
    );
  }

  String get _deliveryModeLabel {
    return switch ((_serverCapabilities['deliveryMode'] ?? '').toString()) {
      'native' => '本地 Core · Native Bridge',
      'remote' => 'Cloud Core · Remote Push',
      _ => '等待 Core 能力',
    };
  }

  String get _testDeliveryLabel {
    final provider = _provider?.isNotEmpty == true
        ? _provider!
        : '当前设备 Provider';
    return _serverCapabilities['deliveryMode'] == 'native'
        ? '通过本地 Core → Native Bridge 投递到本机'
        : '通过 Cloud Core → $provider 投递到本机';
  }

  String get _providerReadinessLabel {
    final providers = _serverCapabilities['pushProviders'];
    if (providers is! Map) return '未返回能力';
    final ready = <String>[];
    for (final entry in providers.entries) {
      if (entry.value == true) ready.add(entry.key.toString());
    }
    if (ready.isNotEmpty) return ready.join(' / ');
    return _serverCapabilities['deliveryMode'] == 'native'
        ? 'Native Bridge 暂不可用'
        : '未配置远程 Provider';
  }

  String get _nativeDataReadinessLabel {
    if (_serverCapabilities['deliveryMode'] == 'native') {
      return '本地 Native Bridge';
    }
    final providers = _serverCapabilities['nativeDataPushProviders'];
    if (providers is! Map) return 'Cloud Core 未返回透传能力';
    final deviceProviders =
        _platformState?.nativeDataProviders ?? const <String>[];
    final ready = effectiveNativeDataProviders(deviceProviders, providers);
    if (ready.isNotEmpty) return ready.join(' / ');
    if (deviceProviders.isEmpty) return '当前设备未启用原生透传接收器';
    return '设备与 Cloud Core 暂无共同透传通道（使用系统通知降级）';
  }

  String get _executionSubtitle {
    if (_platformState?.liveActivitySupported == true) {
      return 'iOS 使用 Live Activity / 灵动岛显示长任务进度';
    }
    if (_platformState?.progressStyleSupported == true) {
      return 'Android 使用系统 ProgressStyle 显示长任务进度';
    }
    return '当前系统使用持续进度通知显示长任务状态';
  }
}

class _CapabilityTile extends StatelessWidget {
  const _CapabilityTile({required this.title, required this.value});

  final String title;
  final String value;

  @override
  Widget build(BuildContext context) {
    return ListTile(
      dense: true,
      title: Text(title),
      trailing: SizedBox(
        width: 190,
        child: Text(
          value,
          textAlign: TextAlign.end,
          maxLines: 2,
          overflow: TextOverflow.ellipsis,
        ),
      ),
    );
  }
}

import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../app/app_routes.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/native_bridge/providers/native_bridge_relay_provider.dart';

class SystemSettingsPage extends ConsumerStatefulWidget {
  const SystemSettingsPage({super.key});

  @override
  ConsumerState<SystemSettingsPage> createState() => _SystemSettingsPageState();
}

class _SystemSettingsPageState extends ConsumerState<SystemSettingsPage> {
  String _language = '简体中文';
  bool _notifications = true;
  bool _notificationsUpdating = false;
  Map<String, dynamic>? _healthData;
  bool _loadingHealth = true;
  String? _healthError;

  static const _languages = ['简体中文', 'English', '日本語'];
  static const _languageCodes = <String, String>{
    '简体中文': 'zh-CN',
    'English': 'en-US',
    '日本語': 'ja-JP',
  };

  @override
  void initState() {
    super.initState();
    _loadSettings();
  }

  Future<void> _loadSettings() async {
    setState(() {
      _loadingHealth = true;
      _healthError = null;
    });
    try {
      final svc = ref.read(systemServiceProvider);
      final results = await Future.wait([
        svc.health(),
        svc.notificationSettings(),
        svc.config(),
      ]);
      final health = results[0];
      final notifications = results[1];
      final config = results[2];
      if (!mounted) return;
      final languageCode = (config?['language'] ?? 'zh-CN').toString();
      var language = '简体中文';
      for (final entry in _languageCodes.entries) {
        if (entry.value == languageCode) {
          language = entry.key;
          break;
        }
      }
      setState(() {
        _healthData = health;
        _notifications = notifications?['enabled'] != false;
        _language = language;
        _loadingHealth = false;
      });
    } catch (e) {
      if (mounted) {
        setState(() {
          _healthError = e.toString();
          _loadingHealth = false;
        });
      }
    }
  }

  Future<void> _setNotifications(bool enabled) async {
    if (_notificationsUpdating) return;
    setState(() => _notificationsUpdating = true);
    try {
      final svc = ref.read(systemServiceProvider);
      final result = enabled
          ? await svc.subscribeNotifications()
          : await svc.unsubscribeNotifications();
      final settings = await svc.notificationSettings();
      if (!mounted) return;
      setState(() => _notifications = settings?['enabled'] == true);
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(_notifications ? '通知已开启' : '通知已关闭'),
          duration: const Duration(seconds: 1),
        ),
      );
      if (result == null) return;
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('更新通知设置失败: $e')),
        );
      }
    } finally {
      if (mounted) setState(() => _notificationsUpdating = false);
    }
  }

  Future<void> _setLanguage(String language) async {
    final code = _languageCodes[language];
    if (code == null) return;
    try {
      await ref.read(systemServiceProvider).updateConfig({'language': code});
      if (mounted) setState(() => _language = language);
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('保存语言设置失败: $e')),
        );
      }
    }
  }

  Future<void> _testNotification() async {
    try {
      final backendResult = await ref.read(systemServiceProvider).testNotification();
      final accepted = backendResult?['accepted'] == true;
      final reason = backendResult?['reason']?.toString();
      if (!accepted) {
        throw StateError(reason?.isNotEmpty == true ? reason : '后端通知配置未就绪');
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
        'requestId': 'settings-notification-test-${DateTime.now().microsecondsSinceEpoch}',
        'platform': platform,
        'operation': 'notification.post',
        'payload': const {
          'title': 'Amitia 测试通知',
          'body': '如果你看到这条通知，说明系统通知投递链路可用。',
          'channel': 'amitia_agent',
          'silent': false,
        },
      });
      if (!const {'success', 'ok'}.contains((nativeResult['status'] ?? '').toString())) {
        final error = nativeResult['error'];
        final message = error is Map ? (error['message'] ?? error['code'])?.toString() : null;
        throw StateError(message?.isNotEmpty == true ? message! : '系统通知投递失败');
      }
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('测试通知已真实投递到系统通知中心')),
      );
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('发送测试通知失败: ${e.toString().replaceFirst('Bad state: ', '').replaceFirst('Unsupported operation: ', '')}')),
        );
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '系统设置', showBackButton: true, fallbackRoute: AppRoutes.settings),
      body: ListView(
        padding: EdgeInsets.symmetric(vertical: AppSpacing.md),
        children: [
          _SectionLabel(text: '系统状态'),
          SizedBox(height: AppSpacing.sm),
          _buildHealthCard(),
          SizedBox(height: AppSpacing.sectionGap),
          _SectionLabel(text: '通用'),
          SizedBox(height: AppSpacing.sm),
          _buildCard([
            _buildDropdownTile(
              icon: Icons.language,
              title: '语言选择',
              value: _language,
              options: _languages,
              onChanged: _setLanguage,
            ),
            _divider(),
            AmitiaSwitchTile(
              title: '通知设置',
              subtitle: '接收消息和提醒通知',
              value: _notifications,
              onChanged: _notificationsUpdating ? null : _setNotifications,
            ),
            _divider(),
            _buildNavTile(icon: Icons.notifications_active_outlined, title: '发送测试通知', onTap: _testNotification),
          ]),
          SizedBox(height: AppSpacing.sectionGap),
          _SectionLabel(text: '功能入口'),
          SizedBox(height: AppSpacing.sm),
          _buildCard([
            _buildNavTile(icon: Icons.record_voice_over_outlined, title: '语音识别', onTap: () => context.push(AppRoutes.settingsAsr)),
            _divider(),
            _buildNavTile(icon: Icons.palette_outlined, title: '外观设置', onTap: () => context.push(AppRoutes.settingsAppearance)),
            _divider(),
            _buildNavTile(icon: Icons.schedule_outlined, title: '时间设置', onTap: () => context.push(AppRoutes.settingsTemporal)),
            _divider(),
            _buildNavTile(icon: Icons.color_lens_outlined, title: '主题设置', onTap: () => context.push(AppRoutes.settingsTheme)),
            _divider(),
            _buildNavTile(icon: Icons.cleaning_services_outlined, title: '存储清理', onTap: () => context.push(AppRoutes.settingsStorage)),
          ]),
          SizedBox(height: AppSpacing.sectionGap),
          _SectionLabel(text: '高级'),
          SizedBox(height: AppSpacing.sm),
          _buildCard([
            _buildNavTile(
              icon: Icons.developer_mode_outlined,
              title: '开发者模式',
              onTap: () => context.push(AppRoutes.kernelPage('dev-mode')),
            ),
          ]),
          SizedBox(height: AppSpacing.xl),
        ],
      ),
    );
  }

  Widget _buildCard(List<Widget> children) {
    return Container(
      margin: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Column(children: children),
    );
  }

  Widget _buildHealthCard() {
    return Container(
      margin: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: _loadingHealth
          ? Padding(
              padding: EdgeInsets.all(AppSpacing.lg),
              child: Center(child: CircularProgressIndicator()),
            )
          : _healthError != null
              ? Padding(
                  padding: EdgeInsets.all(AppSpacing.md),
                  child: Text('获取系统状态失败: $_healthError', style: TextStyle(color: context.error)),
                )
              : _buildHealthContent(),
    );
  }

  Widget _buildHealthContent() {
    final health = _healthData ?? const <String, dynamic>{};
    final checks = health['checks'] is Map
        ? Map<String, dynamic>.from(health['checks'] as Map)
        : <String, dynamic>{};
    final orchestrator = checks['orchestrator'] is Map
        ? Map<String, dynamic>.from(checks['orchestrator'] as Map)
        : <String, dynamic>{};
    final unifiedEntry = checks['unifiedEntry'] is Map
        ? Map<String, dynamic>.from(checks['unifiedEntry'] as Map)
        : <String, dynamic>{};
    final version = (health['version'] ?? '').toString();
    final ready = health['ready'] == true || health['health'] == true;
    final databaseHealthy = health['database'] == 'ok' || checks['database'] == 'ok';
    final modelConfigured = health['model'] == 'configured';

    final components = <({String name, bool ready})>[
      (name: '数据库', ready: databaseHealthy),
      (name: 'Agent Runtime', ready: orchestrator['ready'] == true),
      (name: '统一入口', ready: unifiedEntry['ready'] == true),
      (name: '模型', ready: modelConfigured),
    ];

    return Padding(
      padding: EdgeInsets.all(AppSpacing.cardPadding),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                width: 10,
                height: 10,
                decoration: BoxDecoration(
                  color: ready ? context.success : context.warning,
                  shape: BoxShape.circle,
                ),
              ),
              SizedBox(width: AppSpacing.sm),
              Text(ready ? '系统就绪' : '系统未完全就绪', style: AppTypography.body(context)),
              const Spacer(),
              if (version.isNotEmpty) Text('v$version', style: AppTypography.label(context)),
              SizedBox(width: AppSpacing.sm),
              GestureDetector(
                onTap: _loadSettings,
                child: Icon(Icons.refresh, size: 18, color: context.textTertiary),
              ),
            ],
          ),
          SizedBox(height: AppSpacing.md),
          Wrap(
            spacing: AppSpacing.md,
            runSpacing: AppSpacing.sm,
            children: components.map((component) {
              return Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(
                    component.ready ? Icons.check_circle : Icons.error_outline,
                    size: 14,
                    color: component.ready ? context.success : context.warning,
                  ),
                  const SizedBox(width: 4),
                  Text(component.name, style: AppTypography.label(context)),
                ],
              );
            }).toList(),
          ),
          if ((health['deployMode'] ?? '').toString().isNotEmpty) ...[
            SizedBox(height: AppSpacing.sm),
            Text('部署模式：${health['deployMode']}', style: AppTypography.label(context)),
          ],
        ],
      ),
    );
  }

  Widget _divider() {
    return Padding(
      padding: const EdgeInsets.only(left: 56),
      child: Divider(height: 1, thickness: 0.5, color: context.borderSecondary),
    );
  }

  Widget _buildNavTile({required IconData icon, required String title, required VoidCallback onTap}) {
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: onTap,
      child: Padding(
        padding: EdgeInsets.symmetric(horizontal: AppSpacing.lg, vertical: 13),
        child: Row(
          children: [
            Container(
              width: 32,
              height: 32,
              decoration: BoxDecoration(color: context.accentSoft, shape: BoxShape.circle),
              child: Icon(icon, size: 17, color: context.accentPrimary),
            ),
            const SizedBox(width: 12),
            Expanded(child: Text(title, style: AppTypography.body(context))),
            Icon(Icons.chevron_right, size: 20, color: context.textTertiary),
          ],
        ),
      ),
    );
  }

  Widget _buildDropdownTile({
    required IconData icon,
    required String title,
    required String value,
    required List<String> options,
    required ValueChanged<String> onChanged,
  }) {
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: () => _showOptionSheet(title, options, value, onChanged),
      child: Padding(
        padding: EdgeInsets.symmetric(horizontal: AppSpacing.lg, vertical: 13),
        child: Row(
          children: [
            Container(
              width: 32,
              height: 32,
              decoration: BoxDecoration(color: context.accentSoft, shape: BoxShape.circle),
              child: Icon(icon, size: 17, color: context.accentPrimary),
            ),
            const SizedBox(width: 12),
            Expanded(child: Text(title, style: AppTypography.body(context))),
            Text(value, style: AppTypography.caption(context)),
            const SizedBox(width: 4),
            Icon(Icons.chevron_right, size: 20, color: context.textTertiary),
          ],
        ),
      ),
    );
  }

  void _showOptionSheet(String title, List<String> options, String current, ValueChanged<String> onChanged) {
    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(20))),
      builder: (ctx) {
        return SafeArea(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Padding(
                padding: EdgeInsets.all(AppSpacing.lg),
                child: Text(title, style: AppTypography.sectionTitle(context)),
              ),
              ...options.map((opt) {
                final isSelected = opt == current;
                return ListTile(
                  leading: Icon(
                    isSelected ? Icons.radio_button_checked : Icons.radio_button_off,
                    size: 20,
                    color: isSelected ? context.accentPrimary : context.textTertiary,
                  ),
                  title: Text(opt, style: AppTypography.body(context)),
                  onTap: () {
                    onChanged(opt);
                    Navigator.pop(ctx);
                  },
                );
              }),
              SizedBox(height: AppSpacing.sm),
            ],
          ),
        );
      },
    );
  }

}

class _SectionLabel extends StatelessWidget {
  final String text;
  const _SectionLabel({required this.text});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.sm, AppSpacing.pagePadding, AppSpacing.sm),
      child: Text(text, style: AppTypography.caption(context)),
    );
  }
}

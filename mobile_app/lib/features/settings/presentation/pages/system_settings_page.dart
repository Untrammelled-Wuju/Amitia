import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/services/core_configuration_guard.dart';
import '../../../../core/services/core_configuration_session.dart';
import '../../../../core/backend_transport/core_configuration_intent.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/runtime/backend/mobile_backend_providers.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/ui_runtime/mobile_extension_slot.dart';
import '../widgets/timeout_settings.dart';
import '../widgets/chat_input_settings.dart';
import '../widgets/screen_awake_settings.dart';
import '../widgets/background_keep_alive_settings.dart';
import '../widgets/settings_section.dart';

class SystemSettingsPage extends ConsumerStatefulWidget {
  const SystemSettingsPage({super.key});
  @override
  ConsumerState<SystemSettingsPage> createState() => _SystemSettingsPageState();
}

class _SystemSettingsPageState extends ConsumerState<SystemSettingsPage> {
  String _language = '简体中文';
  Map<String, dynamic> _healthData = const {};
  String? _error;
  bool _loading = true;
  late final CoreConfigurationSession _configuration;
  int _loadEpoch = 0;
  static const _languages = ['简体中文', 'English', '日本語'];
  static const _languageCodes = {
    '简体中文': 'zh-CN',
    'English': 'en-US',
    '日本語': 'ja-JP',
  };

  @override
  void initState() {
    super.initState();
    _configuration = CoreConfigurationSession(
      coreConfigurationGuardFor(ref),
      onInvalidated: (reason) {
        if (!mounted) return;
        _loadEpoch++;
        setState(() {
          _error = reason.toString();
          _loading = false;
          _healthData = const {};
        });
      },
    );
    _loadSettings();
    _loadHealth();
  }

  Future<void> _loadSettings() async {
    final epoch = ++_loadEpoch;
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final config = await _configuration.load(
        () => ref.read(systemServiceProvider).config(),
      );
      if (!mounted || epoch != _loadEpoch) return;
      final code = (config?['language'] ?? 'zh-CN').toString();
      setState(() {
        _language =
            _languageCodes.entries
                .where((entry) => entry.value == code)
                .firstOrNull
                ?.key ??
            '简体中文';
        _loading = false;
      });
    } catch (error) {
      if (mounted && epoch == _loadEpoch) {
        setState(() {
          _error = error.toString();
          _loading = false;
        });
      }
    }
  }

  Future<void> _loadHealth() async {
    final api = ref.read(rawBackendServiceApiProvider);
    final deployment = ref.read(mobileDeploymentConfigProvider);
    try {
      final health = await ref.read(systemServiceProvider).health();
      if (mounted &&
          identical(api, ref.read(rawBackendServiceApiProvider)) &&
          deployment == ref.read(mobileDeploymentConfigProvider))
        setState(() => _healthData = health ?? const {});
    } catch (_) {}
  }

  Future<void> _setLanguage(
    String language,
    CoreConfigurationIntent? intent,
  ) async {
    final code = _languageCodes[language];
    if (code == null) return;
    try {
      await _configuration.write(
        intent,
        () => ref.read(systemServiceProvider).updateConfig({'language': code}),
      );
      if (mounted) setState(() => _language = language);
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('保存语言设置失败: $e')));
      }
    }
  }

  @override
  void dispose() {
    _loadEpoch++;
    _configuration.close();
    super.dispose();
  }

  ValueChanged<String> _languageChange() {
    final intent = _configuration.intent;
    return (language) => _setLanguage(language, intent);
  }

  @override
  Widget build(BuildContext context) {
    ref.listen(
      rawBackendServiceApiProvider,
      (_, __) => _configuration.invalidate(StateError('Core 连接已变化，请重新加载系统配置')),
    );
    ref.listen(
      mobileDeploymentConfigProvider,
      (_, __) => _configuration.invalidate(StateError('设备模式已变化，请重新加载系统配置')),
    );
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '通用',
        showBackButton: true,
        fallbackRoute: AppRoutes.settings,
      ),
      body: ListView(
        padding: EdgeInsets.symmetric(vertical: AppSpacing.lg),
        children: [
          SettingsSection(
            title: '语言',
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
                      Text('读取语言设置失败：$_error'),
                      TextButton(
                        onPressed: _loadSettings,
                        child: const Text('重试'),
                      ),
                    ],
                  ),
                )
              else
                _buildDropdownTile(
                  icon: Icons.language,
                  title: '语言选择',
                  value: _language,
                  options: _languages,
                  onChanged: _languageChange(),
                ),
            ],
          ),
          const SettingsSection(title: '对话输入', children: [ChatInputSettings()]),
          SettingsSection(
            title: '设备行为',
            children: [
              const ScreenAwakeSettings(),
              if (!kIsWeb && defaultTargetPlatform == TargetPlatform.android)
                const BackgroundKeepAliveSettings(),
            ],
          ),
          const SettingsSection(title: '执行时限', children: [TimeoutSettings()]),
          for (final slot in [
            'system.status.item',
            'system.settings.section',
            'extension.settings.page',
            'extension.settings.section',
          ])
            Padding(
              padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
              child: MobileExtensionSlot(
                slotId: slot,
                context: {
                  'route': '/settings/system',
                  'health': _healthData,
                  'ready':
                      _healthData['ready'] == true ||
                      _healthData['health'] == true,
                },
              ),
            ),
          SizedBox(height: AppSpacing.xl),
        ],
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
              decoration: BoxDecoration(
                color: context.accentSoft,
                shape: BoxShape.circle,
              ),
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

  void _showOptionSheet(
    String title,
    List<String> options,
    String current,
    ValueChanged<String> onChanged,
  ) {
    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (ctx) {
        return SafeArea(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Padding(
                padding: EdgeInsets.fromLTRB(
                  AppSpacing.lg,
                  0,
                  AppSpacing.lg,
                  AppSpacing.lg,
                ),
                child: Text(title, style: AppTypography.sectionTitle(context)),
              ),
              ...options.map((opt) {
                final isSelected = opt == current;
                return ListTile(
                  leading: Icon(
                    isSelected
                        ? Icons.radio_button_checked
                        : Icons.radio_button_off,
                    size: 20,
                    color: isSelected
                        ? context.accentPrimary
                        : context.textTertiary,
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

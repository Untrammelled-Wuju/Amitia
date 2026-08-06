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

class SystemSettingsPage extends ConsumerStatefulWidget {
  const SystemSettingsPage({super.key});

  @override
  ConsumerState<SystemSettingsPage> createState() => _SystemSettingsPageState();
}

class _SystemSettingsPageState extends ConsumerState<SystemSettingsPage> {
  String _language = '简体中文';
  bool _autoStart = false;
  bool _notifications = true;
  bool _developerMode = false;
  Map<String, dynamic>? _healthData;
  bool _loadingHealth = true;
  String? _healthError;

  static const _languages = ['简体中文', 'English', '日本語'];

  @override
  void initState() {
    super.initState();
    _loadHealth();
  }

  Future<void> _loadHealth() async {
    setState(() { _loadingHealth = true; _healthError = null; });
    try {
      final svc = ref.read(systemServiceProvider);
      final result = await svc.health();
      if (mounted) setState(() { _healthData = result; _loadingHealth = false; });
    } catch (e) {
      if (mounted) setState(() { _healthError = e.toString(); _loadingHealth = false; });
    }
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '系统设置', showBackButton: true, fallbackRoute: AppRoutes.settings),
      body: ListView(
        padding: const EdgeInsets.symmetric(vertical: AppSpacing.md),
        children: [
          _SectionLabel(text: '系统状态'),
          const SizedBox(height: AppSpacing.sm),
          _buildHealthCard(),
          const SizedBox(height: AppSpacing.sectionGap),
          _SectionLabel(text: '通用'),
          const SizedBox(height: AppSpacing.sm),
          _buildCard([
            _buildDropdownTile(
              icon: Icons.language,
              title: '语言选择',
              value: _language,
              options: _languages,
              onChanged: (v) => setState(() => _language = v),
            ),
            _divider(),
            AmitiaSwitchTile(
              title: '开机启动',
              subtitle: '系统启动时自动运行 Amitia',
              value: _autoStart,
              onChanged: (v) => setState(() => _autoStart = v),
            ),
            _divider(),
            AmitiaSwitchTile(
              title: '通知设置',
              subtitle: '接收消息和提醒通知',
              value: _notifications,
              onChanged: (v) => setState(() => _notifications = v),
            ),
          ]),
          const SizedBox(height: AppSpacing.sectionGap),
          _SectionLabel(text: '功能入口'),
          const SizedBox(height: AppSpacing.sm),
          _buildCard([
            _buildNavTile(icon: Icons.record_voice_over_outlined, title: '语音设置', onTap: () => _showTip('语音设置')),
            _divider(),
            _buildNavTile(icon: Icons.palette_outlined, title: '外观设置', onTap: () => context.push(AppRoutes.settingsAppearance)),
            _divider(),
            _buildNavTile(icon: Icons.schedule_outlined, title: '时间设置', onTap: () => context.push(AppRoutes.settingsTemporal)),
            _divider(),
            _buildNavTile(icon: Icons.color_lens_outlined, title: '主题设置', onTap: () => context.push(AppRoutes.settingsTheme)),
            _divider(),
            _buildNavTile(icon: Icons.cleaning_services_outlined, title: '存储清理', onTap: () => context.push(AppRoutes.settingsStorage)),
          ]),
          const SizedBox(height: AppSpacing.sectionGap),
          _SectionLabel(text: '高级'),
          const SizedBox(height: AppSpacing.sm),
          _buildCard([
            AmitiaSwitchTile(
              title: '开发者模式',
              subtitle: '启用高级调试和开发选项',
              value: _developerMode,
              onChanged: (v) {
                setState(() => _developerMode = v);
                ScaffoldMessenger.of(context).showSnackBar(
                  SnackBar(content: Text(v ? '开发者模式已开启' : '开发者模式已关闭'), duration: const Duration(seconds: 1)),
                );
              },
            ),
          ]),
          const SizedBox(height: AppSpacing.xl),
        ],
      ),
    );
  }

  Widget _buildCard(List<Widget> children) {
    return Container(
      margin: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
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
      margin: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: _loadingHealth
          ? const Padding(
              padding: EdgeInsets.all(AppSpacing.lg),
              child: Center(child: CircularProgressIndicator()),
            )
          : _healthError != null
              ? Padding(
                  padding: const EdgeInsets.all(AppSpacing.md),
                  child: Text('获取系统状态失败: $_healthError', style: TextStyle(color: context.error)),
                )
              : _buildHealthContent(),
    );
  }

  Widget _buildHealthContent() {
    final components = (_healthData?['components'] as List?)
            ?.map((e) => Map<String, dynamic>.from(e as Map))
            .toList() ??
        [];
    final version = (_healthData?['version'] ?? '').toString();
    final status = (_healthData?['status'] ?? 'unknown').toString();

    return Padding(
      padding: const EdgeInsets.all(AppSpacing.cardPadding),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                width: 10,
                height: 10,
                decoration: BoxDecoration(
                  color: status == 'ok' ? context.success : context.warning,
                  shape: BoxShape.circle,
                ),
              ),
              const SizedBox(width: AppSpacing.sm),
              Text('系统 $status', style: AppTypography.body(context)),
              const Spacer(),
              if (version.isNotEmpty) Text('v$version', style: AppTypography.label(context)),
              const SizedBox(width: AppSpacing.sm),
              Icon(Icons.refresh, size: 18, color: context.textTertiary),
            ],
          ),
          if (components.isNotEmpty) ...[
            const SizedBox(height: AppSpacing.md),
            Wrap(
              spacing: AppSpacing.sm,
              runSpacing: AppSpacing.xs,
              children: components.map((c) {
                final name = (c['name'] ?? '').toString();
                final compStatus = (c['status'] ?? 'unknown').toString();
                final isOk = compStatus == 'ok';
                return Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Icon(
                      isOk ? Icons.check_circle : Icons.error_outline,
                      size: 14,
                      color: isOk ? context.success : context.warning,
                    ),
                    const SizedBox(width: 4),
                    Text(name, style: AppTypography.label(context)),
                  ],
                );
              }).toList(),
            ),
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
        padding: const EdgeInsets.symmetric(horizontal: AppSpacing.lg, vertical: 13),
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
        padding: const EdgeInsets.symmetric(horizontal: AppSpacing.lg, vertical: 13),
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
                padding: const EdgeInsets.all(AppSpacing.lg),
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
              const SizedBox(height: AppSpacing.sm),
            ],
          ),
        );
      },
    );
  }

  void _showTip(String title) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text('$title · 即将开放'), duration: const Duration(seconds: 1)),
    );
  }
}

class _SectionLabel extends StatelessWidget {
  final String text;
  const _SectionLabel({required this.text});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.sm, AppSpacing.pagePadding, AppSpacing.sm),
      child: Text(text, style: AppTypography.caption(context)),
    );
  }
}

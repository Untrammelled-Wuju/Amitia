import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/models/safety.dart';

final _safetyConfigProvider = FutureProvider<SafetyConfigDto?>((ref) async {
  final svc = ref.read(safetyServiceProvider);
  return svc.config();
});

class SafetyPage extends ConsumerWidget {
  const SafetyPage({super.key});

  static const _modes = ['手动审批', '自动审批', '免审批'];

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final configAsync = ref.watch(_safetyConfigProvider);

    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '安全设置', showBackButton: true, fallbackRoute: AppRoutes.settings),
      body: configAsync.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (err, _) => Center(
          child: Padding(
            padding: const EdgeInsets.all(32),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(Icons.error_outline, size: 48, color: context.textSecondary),
                const SizedBox(height: 16),
                Text(
                  '加载失败: ${err.toString().replaceFirst('Exception: ', '')}',
                  style: AppTypography.body(context).copyWith(color: context.error),
                  textAlign: TextAlign.center,
                ),
                const SizedBox(height: 16),
                AmitiaButton(
                  label: '重试',
                  onPressed: () => ref.invalidate(_safetyConfigProvider),
                ),
              ],
            ),
          ),
        ),
        data: (config) {
          return _SafetyContent(config: config);
        },
      ),
    );
  }
}

class _SafetyContent extends ConsumerStatefulWidget {
  final SafetyConfigDto? config;

  const _SafetyContent({this.config});

  @override
  ConsumerState<_SafetyContent> createState() => _SafetyContentState();
}

class _SafetyContentState extends ConsumerState<_SafetyContent> {
  late String _permissionMode;
  late bool _sensitiveApproval;
  late bool _dataAccessApproval;
  late bool _outputBoundary;
  bool _saving = false;

  static const _modes = ['手动审批', '自动审批', '免审批'];

  @override
  void initState() {
    super.initState();
    _permissionMode = _modes[widget.config?.blockLevel ?? 1];
    _sensitiveApproval = widget.config?.enabled == 1;
    _dataAccessApproval = true;
    _outputBoundary = true;
  }

  Future<void> _saveConfig() async {
    setState(() => _saving = true);
    final svc = ref.read(safetyServiceProvider);
    await svc.updateConfig({
      'enabled': _sensitiveApproval ? 1 : 0,
      'blockLevel': _modes.indexOf(_permissionMode),
      'sensitiveWords': widget.config?.sensitiveWords ?? [],
    });
    if (mounted) {
      setState(() => _saving = false);
      ref.invalidate(_safetyConfigProvider);
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('安全设置已保存'), duration: Duration(seconds: 1)),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    return ListView(
      padding: const EdgeInsets.symmetric(vertical: AppSpacing.md),
      children: [
        _SectionLabel(text: '权限审批'),
        const SizedBox(height: AppSpacing.sm),
        _buildCard([
          _buildDropdownTile(
            icon: Icons.shield_outlined,
            title: '审批模式',
            value: _permissionMode,
            options: _modes,
            onChanged: (v) => setState(() => _permissionMode = v),
          ),
          _divider(),
          AmitiaSwitchTile(
            title: '敏感操作审批',
            subtitle: '执行敏感操作前需要确认',
            value: _sensitiveApproval,
            onChanged: (v) => setState(() => _sensitiveApproval = v),
          ),
          _divider(),
          AmitiaSwitchTile(
            title: '数据访问审批',
            subtitle: '访问用户数据前需要授权',
            value: _dataAccessApproval,
            onChanged: (v) => setState(() => _dataAccessApproval = v),
          ),
          _divider(),
          AmitiaSwitchTile(
            title: '模型输出边界',
            subtitle: '拦截超出安全范围的输出内容',
            value: _outputBoundary,
            onChanged: (v) => setState(() => _outputBoundary = v),
          ),
        ]),
        const SizedBox(height: AppSpacing.sectionGap),
        _SectionLabel(text: '隐私与边界'),
        const SizedBox(height: AppSpacing.sm),
        _buildCard([
          _buildNavTile(icon: Icons.privacy_tip_outlined, title: '隐私说明', onTap: () => _showTip('隐私说明')),
          _divider(),
          _buildNavTile(icon: Icons.gpp_good_outlined, title: '使用边界', onTap: () => _showTip('使用边界')),
          _divider(),
          _buildNavTile(icon: Icons.security, title: '隐私扫描', onTap: null),
        ]),
        const SizedBox(height: AppSpacing.sectionGap),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
          child: AmitiaButton(
            label: _saving ? '保存中...' : '保存配置',
            icon: Icons.check,
            isFullWidth: true,
            onPressed: _saving ? null : _saveConfig,
          ),
        ),
        const SizedBox(height: AppSpacing.xl),
      ],
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

  Widget _divider() {
    return Padding(
      padding: const EdgeInsets.only(left: 56),
      child: Divider(height: 1, thickness: 0.5, color: context.borderSecondary),
    );
  }

  Widget _buildNavTile(
      {required IconData icon, required String title, VoidCallback? onTap}) {
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
      builder: (ctx) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Padding(padding: const EdgeInsets.all(AppSpacing.lg), child: Text(title, style: AppTypography.sectionTitle(context))),
            ...options.map((opt) => ListTile(
                  leading: Icon(opt == current ? Icons.radio_button_checked : Icons.radio_button_off,
                      size: 20, color: opt == current ? context.accentPrimary : context.textTertiary),
                  title: Text(opt, style: AppTypography.body(context)),
                  onTap: () { onChanged(opt); Navigator.pop(ctx); },
                )),
            const SizedBox(height: AppSpacing.sm),
          ],
        ),
      ),
    );
  }

  void _showTip(String title) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text('$title · 即将打开'), duration: const Duration(seconds: 1)),
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

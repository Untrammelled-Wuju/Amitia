import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../shared/mock_data/mock_data.dart';
import '../../../../app/app_routes.dart';

class SafetyPage extends ConsumerStatefulWidget {
  const SafetyPage({super.key});

  @override
  ConsumerState<SafetyPage> createState() => _SafetyPageState();
}

class _SafetyPageState extends ConsumerState<SafetyPage> {
  late String _permissionMode;
  late bool _sensitiveApproval;
  late bool _dataAccessApproval;
  late bool _outputBoundary;

  static const _modes = ['手动审批', '自动审批', '免审批'];
  static const _events = [
    ('敏感文件访问', '2分钟前', 'warning'),
    ('模型输出被拦截', '15分钟前', 'error'),
    ('权限请求已批准', '1小时前', 'success'),
  ];

  @override
  void initState() {
    super.initState();
    final s = MockSettings.safetySettings;
    _permissionMode = s.permissionMode;
    _sensitiveApproval = s.sensitiveOperationApproval;
    _dataAccessApproval = s.dataAccessApproval;
    _outputBoundary = s.modelOutputBoundary;
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '安全设置', showBackButton: true),
      body: ListView(
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
          _SectionLabel(text: '安全事件 (${_events.length})'),
          const SizedBox(height: AppSpacing.sm),
          ..._events.map((e) => _buildEventTile(e.$1, e.$2, e.$3)),
          const SizedBox(height: AppSpacing.sectionGap),
          _SectionLabel(text: '隐私与边界'),
          const SizedBox(height: AppSpacing.sm),
          _buildCard([
            _buildNavTile(icon: Icons.privacy_tip_outlined, title: '隐私说明', onTap: () => _showTip('隐私说明')),
            _divider(),
            _buildNavTile(icon: Icons.gpp_good_outlined, title: '使用边界', onTap: () => _showTip('使用边界')),
            _divider(),
            _buildNavTile(icon: Icons.security, title: '隐私扫描', onTap: () => context.push(AppRoutes.settingsPrivacyScan)),
          ]),
          const SizedBox(height: AppSpacing.sectionGap),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
            child: AmitiaButton(
              label: '清除安全日志',
              icon: Icons.delete_sweep_outlined,
              isDestructive: true,
              isFullWidth: true,
              onPressed: _confirmClearLog,
            ),
          ),
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

  Widget _divider() {
    return Padding(
      padding: const EdgeInsets.only(left: 56),
      child: Divider(height: 1, thickness: 0.5, color: context.borderSecondary),
    );
  }

  Widget _buildEventTile(String title, String time, String level) {
    final (color, icon) = switch (level) {
      'warning' => (context.warning, Icons.warning_amber_outlined),
      'error' => (context.error, Icons.error_outline),
      _ => (context.success, Icons.check_circle_outline),
    };
    return Container(
      margin: const EdgeInsets.only(left: AppSpacing.pagePadding, right: AppSpacing.pagePadding, bottom: AppSpacing.sm),
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.lg, vertical: 12),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Row(
        children: [
          Icon(icon, size: 20, color: color),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(title, style: AppTypography.body(context)),
                Text(time, style: AppTypography.label(context)),
              ],
            ),
          ),
        ],
      ),
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

  void _confirmClearLog() {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
        title: Text('清除安全日志', style: AppTypography.cardTitle(context)),
        content: Text('确定要清除所有安全日志吗？此操作不可恢复。', style: AppTypography.body(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(content: Text('安全日志已清除'), duration: Duration(seconds: 1)),
              );
            },
            child: Text('清除', style: TextStyle(color: context.error)),
          ),
        ],
      ),
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

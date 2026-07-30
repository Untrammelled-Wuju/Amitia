import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../shared/mock_data/mock_data.dart';

class DeploymentPage extends ConsumerStatefulWidget {
  const DeploymentPage({super.key});

  @override
  ConsumerState<DeploymentPage> createState() => _DeploymentPageState();
}

class _DeploymentPageState extends ConsumerState<DeploymentPage> {
  late String _currentMode;
  late String _address;
  int _testState = 0;

  static const _modes = <(String, IconData, String)>[
    ('本地', Icons.dns_outlined, '完整功能本地运行，数据不离开设备'),
    ('远程', Icons.cloud_outlined, '连接远程服务器，适合低性能设备'),
    ('混合', Icons.sync_alt, '核心本地运行，AI 推理使用远程服务'),
  ];

  @override
  void initState() {
    super.initState();
    _currentMode = MockSettings.deploymentConfig.mode;
    _address = MockSettings.deploymentConfig.address;
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '部署模式', showBackButton: true),
      body: ListView(
        padding: const EdgeInsets.symmetric(vertical: AppSpacing.md),
        children: [
          _SectionLabel(text: '选择部署模式'),
          const SizedBox(height: AppSpacing.sm),
          ..._modes.map((m) => Padding(
                padding: const EdgeInsets.only(left: AppSpacing.pagePadding, right: AppSpacing.pagePadding, bottom: AppSpacing.md),
                child: _ModeCard(
                  mode: m.$1,
                  icon: m.$2,
                  description: m.$3,
                  isSelected: m.$1 == _currentMode,
                  onTap: () => _confirmSwitch(m.$1),
                ),
              )),
          const SizedBox(height: AppSpacing.sm),
          _SectionLabel(text: '当前配置'),
          const SizedBox(height: AppSpacing.sm),
          _buildConfigCard(),
          const SizedBox(height: AppSpacing.sectionGap),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
            child: AmitiaButton(
              label: _testState == 1 ? '测试中...' : '测试连接',
              icon: Icons.wifi_protected_setup,
              isFullWidth: true,
              onPressed: _testState == 1 ? null : _testConnection,
            ),
          ),
          if (_testState == 2) ...[
            const SizedBox(height: AppSpacing.md),
            Center(child: _buildTestResult()),
          ],
          const SizedBox(height: AppSpacing.xl),
        ],
      ),
    );
  }

  Widget _buildConfigCard() {
    return Container(
      margin: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      padding: const EdgeInsets.all(AppSpacing.cardPadding),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Column(
        children: [
          _InfoRow(label: '当前模式', value: _currentMode),
          _InfoRow(label: '服务地址', value: _address),
          _InfoRow(label: '运行状态', value: '已连接'),
        ],
      ),
    );
  }

  Widget _buildTestResult() {
    return Row(
      mainAxisAlignment: MainAxisAlignment.center,
      children: [
        Icon(Icons.check_circle, size: 16, color: context.success),
        const SizedBox(width: 6),
        Text('连接成功 · 延迟 12ms', style: AppTypography.caption(context).copyWith(color: context.success)),
      ],
    );
  }

  void _confirmSwitch(String newMode) {
    if (newMode == _currentMode) return;
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
        title: Text('切换部署模式', style: AppTypography.cardTitle(context)),
        content: Text('确定要将部署模式切换为「$newMode」吗？切换后服务将重新连接。', style: AppTypography.body(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              setState(() {
                _currentMode = newMode;
                _address = newMode == '本地' ? 'localhost:18899' : (newMode == '远程' ? 'remote.amitia.top:18899' : 'hybrid.amitia.top:18899');
                _testState = 0;
              });
              ScaffoldMessenger.of(context).showSnackBar(
                SnackBar(content: Text('已切换为$newMode模式'), duration: const Duration(seconds: 1)),
              );
            },
            child: Text('确定', style: TextStyle(color: context.accentPrimary)),
          ),
        ],
      ),
    );
  }

  Future<void> _testConnection() async {
    setState(() => _testState = 1);
    await Future.delayed(const Duration(milliseconds: 1500));
    if (mounted) setState(() => _testState = 2);
  }
}

class _ModeCard extends StatelessWidget {
  final String mode;
  final IconData icon;
  final String description;
  final bool isSelected;
  final VoidCallback onTap;

  const _ModeCard({
    required this.mode,
    required this.icon,
    required this.description,
    required this.isSelected,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.all(AppSpacing.cardPadding),
        decoration: BoxDecoration(
          color: context.surfacePrimary,
          borderRadius: AppRadius.brMedium,
          border: Border.all(
            color: isSelected ? context.accentPrimary : context.borderPrimary,
            width: isSelected ? 2 : 0.5,
          ),
        ),
        child: Row(
          children: [
            Container(
              width: 48,
              height: 48,
              decoration: BoxDecoration(
                color: isSelected ? context.accentSoft : context.surfaceSecondary,
                borderRadius: AppRadius.brSmall,
              ),
              child: Icon(icon, size: 24, color: isSelected ? context.accentPrimary : context.textSecondary),
            ),
            const SizedBox(width: AppSpacing.md),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(mode, style: AppTypography.cardTitle(context)),
                  const SizedBox(height: 2),
                  Text(description, style: AppTypography.caption(context)),
                ],
              ),
            ),
            Icon(
              isSelected ? Icons.check_circle : Icons.radio_button_off,
              size: 22,
              color: isSelected ? context.accentPrimary : context.textTertiary,
            ),
          ],
        ),
      ),
    );
  }
}

class _InfoRow extends StatelessWidget {
  final String label;
  final String value;
  const _InfoRow({required this.label, required this.value});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 7),
      child: Row(
        children: [
          SizedBox(width: 72, child: Text(label, style: AppTypography.label(context))),
          const SizedBox(width: AppSpacing.md),
          Expanded(child: Text(value, style: AppTypography.bodySmall(context), textAlign: TextAlign.end)),
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

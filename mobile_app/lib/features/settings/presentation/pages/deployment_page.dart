import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/backend_connection/backend_connection_availability.dart';
import '../../../../core/backend_connection/backend_uri_builder.dart';
import '../../../../core/backend_connection/providers/backend_connection_providers.dart';

class DeploymentPage extends ConsumerStatefulWidget {
  const DeploymentPage({super.key});

  @override
  ConsumerState<DeploymentPage> createState() => _DeploymentPageState();
}

class _DeploymentPageState extends ConsumerState<DeploymentPage> {
  String _currentMode = '本地';
  bool _loading = true;
  bool _testState = false;

  static const _modes = <(String, IconData, String)>[
    ('本地', Icons.dns_outlined, '完整功能本地运行，数据不离开设备'),
    ('远程', Icons.cloud_outlined, '暂不可用'),
    ('混合', Icons.sync_alt, '暂不可用'),
  ];

  @override
  void initState() {
    super.initState();
    _loadConfig();
  }

  Future<void> _loadConfig() async {
    await Future.delayed(Duration.zero);
    if (mounted) {
      setState(() => _loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final connectionAsync = ref.watch(backendConnectionProvider);
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '部署模式', showBackButton: true, fallbackRoute: AppRoutes.settings),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : ListView(
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
                        onTap: m.$1 == '本地' ? () => _confirmSwitch(m.$1) : null,
                      ),
                    )),
                const SizedBox(height: AppSpacing.sm),
                _SectionLabel(text: '当前配置'),
                const SizedBox(height: AppSpacing.sm),
                _buildConfigCard(connectionAsync),
                const SizedBox(height: AppSpacing.sectionGap),
                Padding(
                  padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
                  child: AmitiaButton(
                    label: _testState ? '检查中...' : '检查运行时状态',
                    icon: Icons.wifi_protected_setup,
                    isFullWidth: true,
                    onPressed: _testState ? null : _checkRuntimeStatus,
                  ),
                ),
                const SizedBox(height: AppSpacing.xl),
              ],
            ),
    );
  }

  Widget _buildConfigCard(AsyncValue<BackendConnectionAvailability> connectionAsync) {
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
          connectionAsync.when(
            data: (avail) {
              if (avail is BackendConnectionAvailable) {
                final uri = BackendUriBuilder().httpBase(avail.config);
                return _InfoRow(label: '后端地址', value: uri.toString());
              }
              return const _InfoRow(label: '后端地址', value: '运行环境未就绪');
            },
            loading: () => const _InfoRow(label: '后端地址', value: '检查中...'),
            error: (_, __) => const _InfoRow(label: '后端地址', value: '运行环境未就绪'),
          ),
          connectionAsync.when(
            data: (avail) {
              final ready = avail is BackendConnectionAvailable;
              return _InfoRow(label: '运行状态', value: ready ? '后端就绪' : '运行环境未就绪');
            },
            loading: () => const _InfoRow(label: '运行状态', value: '检查中...'),
            error: (_, __) => const _InfoRow(label: '运行状态', value: '运行环境未就绪'),
          ),
        ],
      ),
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
                _testState = false;
              });
              ScaffoldMessenger.of(context).showSnackBar(
                SnackBar(content: Text('已切换为${newMode}模式'), duration: const Duration(seconds: 1)),
              );
            },
            child: Text('确定', style: TextStyle(color: context.accentPrimary)),
          ),
        ],
      ),
    );
  }

  Future<void> _checkRuntimeStatus() async {
    setState(() => _testState = true);
    await ref.read(backendConnectionProvider.future);
    await Future.delayed(const Duration(milliseconds: 300));
    if (mounted) setState(() => _testState = false);
  }
}

class _ModeCard extends StatelessWidget {
  final String mode;
  final IconData icon;
  final String description;
  final bool isSelected;
  final VoidCallback? onTap;

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

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
import '../../../../shared/mock_data/mock_data.dart';

class DesktopPetPage extends ConsumerStatefulWidget {
  const DesktopPetPage({super.key});

  @override
  ConsumerState<DesktopPetPage> createState() => _DesktopPetPageState();
}

class _DesktopPetPageState extends ConsumerState<DesktopPetPage> {
  bool _floatingWindow = true;
  double _transparency = 0.85;

  final _petColors = ['#7668EE', '#52B788', '#E9A23B'];

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '桌宠中心',
        showBackButton: true,
      ),
      body: SafeArea(
        top: false,
        child: ListView(
          padding: const EdgeInsets.only(bottom: AppSpacing.xxl),
          children: [
            const SizedBox(height: AppSpacing.sm),
            _buildCurrentPetCard(context),
            const SizedBox(height: AppSpacing.sm),
            _buildActionEntry(context),
            const SizedBox(height: AppSpacing.sectionGap),
            const AmitiaSectionHeader(title: '已安装桌宠插件'),
            const SizedBox(height: AppSpacing.sm),
            _buildPluginsCard(context),
            const SizedBox(height: AppSpacing.sectionGap),
            const AmitiaSectionHeader(title: '显示设置'),
            const SizedBox(height: AppSpacing.sm),
            _buildSettingsCard(context),
            const SizedBox(height: AppSpacing.sectionGap),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
              child: AmitiaButton(
                label: '生成新桌宠',
                icon: Icons.auto_awesome_outlined,
                isFullWidth: true,
                onPressed: () => context.push(AppRoutes.workshopPet),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildCurrentPetCard(BuildContext context) {
    final petName = MockData.desktopPetPlugins.first;
    final color = _parseColor(_petColors.first);
    final initial = _getInitial(petName);

    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      child: AmitiaCard(
        child: Row(
          children: [
            Container(
              width: 56,
              height: 56,
              decoration: BoxDecoration(
                color: color,
                shape: BoxShape.circle,
              ),
              child: Center(
                child: Text(
                  initial,
                  style: const TextStyle(
                    color: Colors.white,
                    fontSize: 22,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
            ),
            const SizedBox(width: AppSpacing.md),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(petName, style: AppTypography.cardTitle(context)),
                  const SizedBox(height: 2),
                  Text('运行中 · 心情很好', style: AppTypography.caption(context)),
                ],
              ),
            ),
            AmitiaStatusBadge(label: '运行中', type: BadgeType.success),
          ],
        ),
      ),
    );
  }

  Widget _buildActionEntry(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      child: AmitiaCard(
        child: GestureDetector(
          onTap: () => amitiaComingSoon(context, '动作管理'),
          behavior: HitTestBehavior.opaque,
          child: Row(
            children: [
              Container(
                width: 36,
                height: 36,
                decoration: BoxDecoration(
                  color: context.accentSoft,
                  borderRadius: AppRadius.brExtraSmall,
                ),
                child: Icon(Icons.touch_app_outlined, size: 18, color: context.accentPrimary),
              ),
              const SizedBox(width: AppSpacing.md),
              Expanded(
                child: Text('动作管理', style: AppTypography.body(context)),
              ),
              Icon(Icons.chevron_right, size: 18, color: context.textTertiary),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildPluginsCard(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      child: AmitiaCard(
        padding: const EdgeInsets.symmetric(vertical: AppSpacing.xs),
        child: Column(
          children: [
            for (int i = 0; i < MockData.desktopPetPlugins.length; i++) ...[
              _buildPluginItem(context, MockData.desktopPetPlugins[i], i),
              if (i < MockData.desktopPetPlugins.length - 1)
                Divider(height: 1, color: context.borderSecondary),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildPluginItem(BuildContext context, String name, int index) {
    final colorHex = _petColors[index % _petColors.length];
    final color = _parseColor(colorHex);
    final initial = _getInitial(name);
    final isCurrent = index == 0;

    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.cardPadding, vertical: 10),
      child: Row(
        children: [
          Container(
            width: 36,
            height: 36,
            decoration: BoxDecoration(
              color: color,
              shape: BoxShape.circle,
            ),
            child: Center(
              child: Text(
                initial,
                style: const TextStyle(
                  color: Colors.white,
                  fontSize: 14,
                  fontWeight: FontWeight.w600,
                ),
              ),
            ),
          ),
          const SizedBox(width: AppSpacing.md),
          Expanded(
            child: Text(name, style: AppTypography.body(context)),
          ),
          if (isCurrent)
            AmitiaStatusBadge(label: '当前', type: BadgeType.accent)
          else
            Icon(Icons.chevron_right, size: 18, color: context.textTertiary),
        ],
      ),
    );
  }

  Widget _buildSettingsCard(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      child: AmitiaCard(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text('悬浮窗显示', style: AppTypography.body(context)),
                      const SizedBox(height: 2),
                      Text('在其他应用上方显示桌宠', style: AppTypography.label(context)),
                    ],
                  ),
                ),
                Switch(
                  value: _floatingWindow,
                  onChanged: (value) {
                    setState(() {
                      _floatingWindow = value;
                    });
                  },
                ),
              ],
            ),
            const SizedBox(height: AppSpacing.sm),
            Row(
              children: [
                Expanded(
                  child: Text('透明度', style: AppTypography.body(context)),
                ),
                Text(
                  '${(_transparency * 100).round()}%',
                  style: AppTypography.caption(context),
                ),
              ],
            ),
            Slider(
              value: _transparency,
              min: 0.2,
              max: 1.0,
              divisions: 8,
              activeColor: context.accentPrimary,
              onChanged: (value) {
                setState(() {
                  _transparency = value;
                });
              },
            ),
          ],
        ),
      ),
    );
  }

  Color _parseColor(String hex) {
    final cleaned = hex.replaceAll('#', '');
    return Color(int.parse('FF$cleaned', radix: 16));
  }

  String _getInitial(String name) {
    return name.isNotEmpty ? name.substring(0, 1) : '?';
  }
}

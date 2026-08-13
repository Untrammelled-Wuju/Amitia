import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/center_navigation.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/services/providers.dart';
import '../sections/desktop_pet_plugin_section.dart';

class DesktopPetPage extends ConsumerStatefulWidget {
  const DesktopPetPage({super.key});

  @override
  ConsumerState<DesktopPetPage> createState() => _DesktopPetPageState();
}

class _DesktopPetPageState extends ConsumerState<DesktopPetPage> {
  bool _floatingWindow = true;
  double _transparency = 0.85;

  static const _defaultPetName = '桌宠';
  static const _defaultPetColor = '#7668EE';

  @override
  Widget build(BuildContext context) {
    final companionAsync = ref.watch(companionStateProvider);

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '桌宠中心',
        navigation: AmitiaAppBarNavigation.back,
      ),
      body: SafeArea(
        top: false,
        child: companionAsync.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (_, __) => _buildStaticContent(context),
          data: (state) {
            final rawName = state?['currentActivity']?.toString();
            final petName = (rawName != null && rawName.isNotEmpty) ? rawName : _defaultPetName;
            return _buildContentWithState(context, petName);
          },
        ),
      ),
    );
  }

  Widget _buildStaticContent(BuildContext context) {
    return ListView(
      padding: const EdgeInsets.only(bottom: AppSpacing.xxl),
      children: [
        const SizedBox(height: AppSpacing.sm),
        _buildCurrentPetCard(context, _defaultPetName),
        const SizedBox(height: AppSpacing.sectionGap),
        const DesktopPetPluginSection(),
        const SizedBox(height: AppSpacing.sectionGap),
        const AmitiaSectionHeader(title: '显示设置'),
        const SizedBox(height: AppSpacing.sm),
        _buildSettingsCard(context),
        const SizedBox(height: AppSpacing.sectionGap),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
          child: AmitiaCard(
            onTap: () => CenterNavigation.openExtensionCenter(context),
            child: Row(
              children: [
                Container(
                  width: 32,
                  height: 32,
                  decoration: BoxDecoration(
                    color: context.accentSoft,
                    borderRadius: AppRadius.brSmall,
                  ),
                  child: Icon(Icons.extension_outlined, size: 18, color: context.accentPrimary),
                ),
                const SizedBox(width: AppSpacing.sm),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text('扩展中心', style: AppTypography.label(context)),
                      Text('管理所有扩展', style: AppTypography.caption(context)),
                    ],
                  ),
                ),
                Icon(Icons.chevron_right, size: 16, color: context.textSecondary),
              ],
            ),
          ),
        ),
        const SizedBox(height: AppSpacing.sectionGap),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
          child: AmitiaButton(
            label: '生成新桌宠',
            icon: Icons.auto_awesome_outlined,
            isFullWidth: true,
            onPressed: () => context.go(AppRoutes.workshopPetCreate),
          ),
        ),
      ],
    );
  }

  Widget _buildContentWithState(BuildContext context, String petName) {
    return ListView(
      padding: const EdgeInsets.only(bottom: AppSpacing.xxl),
      children: [
        const SizedBox(height: AppSpacing.sm),
        _buildCurrentPetCard(context, petName),
        const SizedBox(height: AppSpacing.sectionGap),
        const DesktopPetPluginSection(),
        const SizedBox(height: AppSpacing.sectionGap),
        const AmitiaSectionHeader(title: '显示设置'),
        const SizedBox(height: AppSpacing.sm),
        _buildSettingsCard(context),
        const SizedBox(height: AppSpacing.sectionGap),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
          child: AmitiaCard(
            onTap: () => CenterNavigation.openExtensionCenter(context),
            child: Row(
              children: [
                Container(
                  width: 32,
                  height: 32,
                  decoration: BoxDecoration(
                    color: context.accentSoft,
                    borderRadius: AppRadius.brSmall,
                  ),
                  child: Icon(Icons.extension_outlined, size: 18, color: context.accentPrimary),
                ),
                const SizedBox(width: AppSpacing.sm),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text('扩展中心', style: AppTypography.label(context)),
                      Text('管理所有扩展', style: AppTypography.caption(context)),
                    ],
                  ),
                ),
                Icon(Icons.chevron_right, size: 16, color: context.textSecondary),
              ],
            ),
          ),
        ),
        const SizedBox(height: AppSpacing.sectionGap),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
          child: AmitiaButton(
            label: '生成新桌宠',
            icon: Icons.auto_awesome_outlined,
            isFullWidth: true,
            onPressed: () => context.go(AppRoutes.workshopPetCreate),
          ),
        ),
      ],
    );
  }

  Widget _buildCurrentPetCard(BuildContext context, String petName) {
    final color = _parseColor(_defaultPetColor);
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

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/settings/appearance_preferences.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';

class ThemeSettingsPage extends ConsumerWidget {
  const ThemeSettingsPage({super.key});

  static const _accentNames = ['Amitia 暖棕', '清晰蓝', '自然绿', '琥珀'];
  static const _accentHexLight = ['#8A5728', '#6C8FEA', '#52B788', '#E9A23B'];
  static const _accentHexDark = ['#9C8068', '#8CA8F0', '#78C99A', '#F0B65D'];

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final appearance = ref.watch(appearancePreferencesProvider);
    final notifier = ref.read(appearancePreferencesProvider.notifier);
    final themeIndex = appearance.themeMode == ThemeMode.dark
        ? 1
        : (appearance.themeMode == ThemeMode.system ? 2 : 0);
    final accentIndex = appearance.accentColorIndex.clamp(0, _accentNames.length - 1).toInt();

    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '主题设置', showBackButton: true, fallbackRoute: AppRoutes.settings),
      body: ListView(
        padding: EdgeInsets.symmetric(vertical: AppSpacing.md),
        children: [
          const _SectionLabel(text: '主题模式'),
          SizedBox(height: AppSpacing.sm),
          Padding(
            padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
            child: AmitiaSegmentedControl(
              segments: const ['亮色', '暗色', '跟随系统'],
              selectedIndex: themeIndex,
              onChanged: (i) => notifier.setThemeMode(
                i == 0 ? ThemeMode.light : (i == 1 ? ThemeMode.dark : ThemeMode.system),
              ),
            ),
          ),
          SizedBox(height: AppSpacing.sectionGap),
          const _SectionLabel(text: '强调色'),
          SizedBox(height: AppSpacing.sm),
          Container(
            margin: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
            padding: EdgeInsets.all(AppSpacing.cardPadding),
            decoration: BoxDecoration(
              color: context.surfacePrimary,
              borderRadius: AppRadius.brMedium,
              border: Border.all(color: context.borderPrimary, width: 0.5),
            ),
            child: Row(
              children: [
                Container(
                  width: 40,
                  height: 40,
                  decoration: BoxDecoration(color: context.accentPrimary, shape: BoxShape.circle),
                  child: const Icon(Icons.check, size: 20, color: Colors.white),
                ),
                SizedBox(width: AppSpacing.md),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(_accentNames[accentIndex], style: AppTypography.body(context)),
                      Text(
                        Theme.of(context).brightness == Brightness.dark
                            ? _accentHexDark[accentIndex]
                            : _accentHexLight[accentIndex],
                        style: AppTypography.label(context),
                      ),
                    ],
                  ),
                ),
                const AmitiaStatusBadge(label: '当前', type: BadgeType.accent),
              ],
            ),
          ),
          SizedBox(height: AppSpacing.sectionGap),
          _SectionLabel(text: '字体缩放 (${(appearance.fontScale * 100).round()}%)'),
          SizedBox(height: AppSpacing.sm),
          Padding(
            padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
            child: Slider(
              value: appearance.fontScale,
              min: 0.8,
              max: 1.4,
              divisions: 6,
              activeColor: context.accentPrimary,
              onChanged: notifier.setFontScale,
            ),
          ),
          SizedBox(height: AppSpacing.sectionGap),
          const _SectionLabel(text: '动效'),
          SizedBox(height: AppSpacing.sm),
          Container(
            margin: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
            decoration: BoxDecoration(
              color: context.surfacePrimary,
              borderRadius: AppRadius.brMedium,
              border: Border.all(color: context.borderPrimary, width: 0.5),
            ),
            child: Column(
              children: [
                AmitiaSwitchTile(
                  title: '启用动画',
                  subtitle: '页面过渡和微交互动画',
                  value: appearance.dynamicEffect,
                  onChanged: notifier.setDynamicEffect,
                ),
                Divider(height: 1, indent: AppSpacing.lg, color: context.borderSecondary),
                AmitiaSwitchTile(
                  title: '减少动画',
                  subtitle: '关闭主题切换和系统可感知动效',
                  value: appearance.reduceAnimation,
                  onChanged: notifier.setReduceAnimation,
                ),
              ],
            ),
          ),
          SizedBox(height: AppSpacing.sectionGap),
          const _SectionLabel(text: '预览'),
          SizedBox(height: AppSpacing.sm),
          Container(
            margin: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
            padding: EdgeInsets.all(AppSpacing.cardPadding),
            decoration: BoxDecoration(
              color: context.surfacePrimary,
              borderRadius: AppRadius.brMedium,
              border: Border.all(color: context.borderPrimary, width: 0.5),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('预览效果', style: AppTypography.cardTitle(context)),
                SizedBox(height: AppSpacing.sm),
                Text('这是一段正文文字，用于展示当前全局字体缩放效果。', style: AppTypography.body(context)),
                SizedBox(height: AppSpacing.md),
                Row(
                  children: [
                    AmitiaButton(label: '主按钮', onPressed: () => amitiaSnackBar(context, '预览按钮已点击')),
                    SizedBox(width: AppSpacing.sm),
                    AmitiaButton(label: '次按钮', isSecondary: true, onPressed: () => amitiaSnackBar(context, '预览按钮已点击')),
                  ],
                ),
                SizedBox(height: AppSpacing.md),
                AmitiaProgressBar(progress: 0.65),
              ],
            ),
          ),
          SizedBox(height: AppSpacing.xl),
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
      padding: EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.sm, AppSpacing.pagePadding, AppSpacing.sm),
      child: Text(text, style: AppTypography.caption(context)),
    );
  }
}

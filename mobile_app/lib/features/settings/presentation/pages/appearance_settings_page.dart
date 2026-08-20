import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_drawer.dart';

class AppearanceSettingsPage extends ConsumerStatefulWidget {
  const AppearanceSettingsPage({super.key});

  @override
  ConsumerState<AppearanceSettingsPage> createState() =>
      _AppearanceSettingsPageState();
}

class _AppearanceSettingsPageState extends ConsumerState<AppearanceSettingsPage> {
  int _fontSizeIndex = 1;
  int _accentColorIndex = 0;
  int _cornerStyleIndex = 1;
  bool _dynamicEffect = true;
  bool _reduceAnimation = false;

  static const _accentColors = <Color>[
    Color(0xFF7668EE),
    Color(0xFF6C8FEA),
    Color(0xFF52B788),
    Color(0xFFE9A23B),
  ];

  @override
  Widget build(BuildContext context) {
    final themeMode = ref.watch(themeModeProvider);
    final themeIndex = themeMode == ThemeMode.dark
        ? 1
        : (themeMode == ThemeMode.system ? 2 : 0);

    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '外观设置', showBackButton: true, fallbackRoute: AppRoutes.settings),
      body: ListView(
        padding: EdgeInsets.symmetric(vertical: AppSpacing.md),
        children: [
          const _SectionLabel(text: '主题模式'),
          Padding(
            padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
            child: AmitiaSegmentedControl(
              segments: const ['亮色', '暗色', '跟随系统'],
              selectedIndex: themeIndex,
              onChanged: (i) {
                ref.read(themeModeProvider.notifier).state = i == 0
                    ? ThemeMode.light
                    : (i == 1 ? ThemeMode.dark : ThemeMode.system);
              },
            ),
          ),
          SizedBox(height: AppSpacing.sectionGap),
          const _SectionLabel(text: '字体大小'),
          Padding(
            padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
            child: _OptionChips(
              options: const ['小', '标准', '大', '超大'],
              selectedIndex: _fontSizeIndex,
              onChanged: (i) => setState(() => _fontSizeIndex = i),
            ),
          ),
          SizedBox(height: AppSpacing.sectionGap),
          const _SectionLabel(text: '主题色调'),
          Padding(
            padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
            child: Row(
              children: [
                for (int i = 0; i < _accentColors.length; i++) ...[
                  _ColorDot(
                    color: _accentColors[i],
                    isSelected: _accentColorIndex == i,
                    onTap: () => setState(() => _accentColorIndex = i),
                  ),
                  if (i < _accentColors.length - 1)
                    SizedBox(width: AppSpacing.lg),
                ],
              ],
            ),
          ),
          SizedBox(height: AppSpacing.sectionGap),
          const _SectionLabel(text: '圆角风格'),
          Padding(
            padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
            child: _OptionChips(
              options: const ['克制', '标准', '圆润'],
              selectedIndex: _cornerStyleIndex,
              onChanged: (i) => setState(() => _cornerStyleIndex = i),
            ),
          ),
          SizedBox(height: AppSpacing.sectionGap),
          const _SectionLabel(text: '其他'),
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
                  title: '动态效果',
                  subtitle: '启用页面过渡和微交互动画',
                  value: _dynamicEffect,
                  onChanged: (v) => setState(() => _dynamicEffect = v),
                ),
                Divider(
                  height: 1,
                  indent: AppSpacing.lg,
                  color: context.borderSecondary,
                ),
                AmitiaSwitchTile(
                  title: '减少动画',
                  subtitle: '降低界面动画强度以提升性能',
                  value: _reduceAnimation,
                  onChanged: (v) => setState(() => _reduceAnimation = v),
                ),
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
      padding: EdgeInsets.fromLTRB(
        AppSpacing.pagePadding,
        AppSpacing.sm,
        AppSpacing.pagePadding,
        AppSpacing.sm,
      ),
      child: Text(text, style: AppTypography.caption(context)),
    );
  }
}

class _OptionChips extends StatelessWidget {
  final List<String> options;
  final int selectedIndex;
  final ValueChanged<int> onChanged;

  const _OptionChips({
    required this.options,
    required this.selectedIndex,
    required this.onChanged,
  });

  @override
  Widget build(BuildContext context) {
    return Wrap(
      spacing: AppSpacing.sm,
      runSpacing: AppSpacing.sm,
      children: [
        for (int i = 0; i < options.length; i++)
          GestureDetector(
            onTap: () => onChanged(i),
            child: Container(
              padding: EdgeInsets.symmetric(
                horizontal: AppSpacing.lg,
                vertical: 9,
              ),
              decoration: BoxDecoration(
                color: i == selectedIndex
                    ? context.accentSoft
                    : context.surfaceSecondary,
                borderRadius: AppRadius.brTag,
                border: Border.all(
                  color: i == selectedIndex
                      ? context.accentPrimary
                      : Colors.transparent,
                  width: 1,
                ),
              ),
              child: Text(
                options[i],
                style: TextStyle(
                  fontSize: 13,
                  fontWeight: i == selectedIndex
                      ? FontWeight.w500
                      : FontWeight.w400,
                  color: i == selectedIndex
                      ? context.accentPrimary
                      : context.textSecondary,
                ),
              ),
            ),
          ),
      ],
    );
  }
}

class _ColorDot extends StatelessWidget {
  final Color color;
  final bool isSelected;
  final VoidCallback onTap;

  const _ColorDot({
    required this.color,
    required this.isSelected,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        width: 40,
        height: 40,
        decoration: BoxDecoration(
          shape: BoxShape.circle,
          border: Border.all(
            color: isSelected ? context.accentPrimary : context.borderPrimary,
            width: isSelected ? 2.5 : 1,
          ),
        ),
        child: Center(
          child: Container(
            width: 28,
            height: 28,
            decoration: BoxDecoration(
              color: color,
              shape: BoxShape.circle,
            ),
            child: isSelected
                ? const Icon(Icons.check, size: 16, color: Colors.white)
                : null,
          ),
        ),
      ),
    );
  }
}

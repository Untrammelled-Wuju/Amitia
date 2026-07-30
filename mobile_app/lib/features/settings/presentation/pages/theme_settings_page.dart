import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_drawer.dart';

class ThemeSettingsPage extends ConsumerStatefulWidget {
  const ThemeSettingsPage({super.key});

  @override
  ConsumerState<ThemeSettingsPage> createState() => _ThemeSettingsPageState();
}

class _ThemeSettingsPageState extends ConsumerState<ThemeSettingsPage> {
  double _fontScale = 1.0;
  bool _animationEnabled = true;

  @override
  Widget build(BuildContext context) {
    final themeMode = ref.watch(themeModeProvider);
    final themeIndex = themeMode == ThemeMode.dark ? 1 : (themeMode == ThemeMode.system ? 2 : 0);

    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '主题设置', showBackButton: true),
      body: ListView(
        padding: const EdgeInsets.symmetric(vertical: AppSpacing.md),
        children: [
          _SectionLabel(text: '主题模式'),
          const SizedBox(height: AppSpacing.sm),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
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
          const SizedBox(height: AppSpacing.sectionGap),
          _SectionLabel(text: '强调色'),
          const SizedBox(height: AppSpacing.sm),
          Container(
            margin: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
            padding: const EdgeInsets.all(AppSpacing.cardPadding),
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
                  decoration: BoxDecoration(
                    color: const Color(0xFF7668EE),
                    shape: BoxShape.circle,
                  ),
                  child: const Icon(Icons.check, size: 20, color: Colors.white),
                ),
                const SizedBox(width: AppSpacing.md),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text('浅紫', style: AppTypography.body(context)),
                      Text('#7668EE', style: AppTypography.label(context)),
                    ],
                  ),
                ),
                AmitiaStatusBadge(label: '当前', type: BadgeType.accent),
              ],
            ),
          ),
          const SizedBox(height: AppSpacing.sectionGap),
          _SectionLabel(text: '字体缩放 (${(_fontScale * 100).round()}%)'),
          const SizedBox(height: AppSpacing.sm),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
            child: Slider(
              value: _fontScale,
              min: 0.8,
              max: 1.4,
              divisions: 6,
              activeColor: context.accentPrimary,
              onChanged: (v) => setState(() => _fontScale = v),
            ),
          ),
          const SizedBox(height: AppSpacing.sectionGap),
          _SectionLabel(text: '动效'),
          const SizedBox(height: AppSpacing.sm),
          Container(
            margin: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
            decoration: BoxDecoration(
              color: context.surfacePrimary,
              borderRadius: AppRadius.brMedium,
              border: Border.all(color: context.borderPrimary, width: 0.5),
            ),
            child: AmitiaSwitchTile(
              title: '启用动画',
              subtitle: '页面过渡和微交互动画',
              value: _animationEnabled,
              onChanged: (v) => setState(() => _animationEnabled = v),
            ),
          ),
          const SizedBox(height: AppSpacing.sectionGap),
          _SectionLabel(text: '预览'),
          const SizedBox(height: AppSpacing.sm),
          Container(
            margin: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
            padding: const EdgeInsets.all(AppSpacing.cardPadding),
            decoration: BoxDecoration(
              color: context.surfacePrimary,
              borderRadius: AppRadius.brMedium,
              border: Border.all(color: context.borderPrimary, width: 0.5),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('预览效果', style: AppTypography.cardTitle(context).copyWith(fontSize: 16 * _fontScale)),
                const SizedBox(height: AppSpacing.sm),
                Text('这是一段正文文字，用于展示当前字体缩放效果。', style: AppTypography.body(context).copyWith(fontSize: 15 * _fontScale)),
                const SizedBox(height: AppSpacing.md),
                Row(
                  children: [
                    AmitiaButton(
                      label: '主按钮',
                      onPressed: () => ScaffoldMessenger.of(context).showSnackBar(
                        const SnackBar(content: Text('预览按钮点击'), duration: Duration(seconds: 1)),
                      ),
                    ),
                    const SizedBox(width: AppSpacing.sm),
                    AmitiaButton(
                      label: '次按钮',
                      isSecondary: true,
                      onPressed: () {},
                    ),
                  ],
                ),
                const SizedBox(height: AppSpacing.md),
                AmitiaProgressBar(progress: 0.65),
                const SizedBox(height: AppSpacing.sm),
                Row(
                  children: [
                    AmitiaStatusBadge(label: '正常', type: BadgeType.success),
                    const SizedBox(width: AppSpacing.sm),
                    AmitiaStatusBadge(label: '警告', type: BadgeType.warning),
                    const SizedBox(width: AppSpacing.sm),
                    AmitiaStatusBadge(label: '错误', type: BadgeType.error),
                  ],
                ),
              ],
            ),
          ),
          const SizedBox(height: AppSpacing.xl),
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

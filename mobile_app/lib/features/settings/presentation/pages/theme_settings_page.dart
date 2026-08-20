import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/app_routes.dart';
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
      appBar: AmitiaAppBar(title: '主题设置', showBackButton: true, fallbackRoute: AppRoutes.settings),
      body: ListView(
        padding: EdgeInsets.symmetric(vertical: AppSpacing.md),
        children: [
          _SectionLabel(text: '主题模式'),
          SizedBox(height: AppSpacing.sm),
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
          _SectionLabel(text: '强调色'),
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
                  decoration: BoxDecoration(
                    color: const Color(0xFF7668EE),
                    shape: BoxShape.circle,
                  ),
                  child: const Icon(Icons.check, size: 20, color: Colors.white),
                ),
                SizedBox(width: AppSpacing.md),
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
          SizedBox(height: AppSpacing.sectionGap),
          _SectionLabel(text: '字体缩放 (${(_fontScale * 100).round()}%)'),
          SizedBox(height: AppSpacing.sm),
          Padding(
            padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
            child: Slider(
              value: _fontScale,
              min: 0.8,
              max: 1.4,
              divisions: 6,
              activeColor: context.accentPrimary,
              onChanged: (v) => setState(() => _fontScale = v),
            ),
          ),
          SizedBox(height: AppSpacing.sectionGap),
          _SectionLabel(text: '动效'),
          SizedBox(height: AppSpacing.sm),
          Container(
            margin: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
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
          SizedBox(height: AppSpacing.sectionGap),
          _SectionLabel(text: '预览'),
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
                Text('预览效果', style: AppTypography.cardTitle(context).copyWith(fontSize: 16 * _fontScale)),
                SizedBox(height: AppSpacing.sm),
                Text('这是一段正文文字，用于展示当前字体缩放效果。', style: AppTypography.body(context).copyWith(fontSize: 15 * _fontScale)),
                SizedBox(height: AppSpacing.md),
                Row(
                  children: [
                    AmitiaButton(
                      label: '主按钮',
                      onPressed: () => ScaffoldMessenger.of(context).showSnackBar(
                        const SnackBar(content: Text('预览按钮点击'), duration: Duration(seconds: 1)),
                      ),
                    ),
                    SizedBox(width: AppSpacing.sm),
                    AmitiaButton(
                      label: '次按钮',
                      isSecondary: true,
                      onPressed: () => amitiaSnackBar(context, '预览按钮已点击'),
                    ),
                  ],
                ),
                SizedBox(height: AppSpacing.md),
                AmitiaProgressBar(progress: 0.65),
                SizedBox(height: AppSpacing.sm),
                Row(
                  children: [
                    AmitiaStatusBadge(label: '正常', type: BadgeType.success),
                    SizedBox(width: AppSpacing.sm),
                    AmitiaStatusBadge(label: '警告', type: BadgeType.warning),
                    SizedBox(width: AppSpacing.sm),
                    AmitiaStatusBadge(label: '错误', type: BadgeType.error),
                  ],
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
      padding: EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.sm, AppSpacing.pagePadding, AppSpacing.sm),
      child: Text(text, style: AppTypography.caption(context)),
    );
  }
}

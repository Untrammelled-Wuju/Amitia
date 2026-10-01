import 'package:flutter/material.dart';
import '../../../app/theme/app_colors.dart';
import '../../../app/theme/design_tokens.dart';

class AmitiaMessageTheme extends ThemeExtension<AmitiaMessageTheme> {
  final Color background;
  final Color text;
  final Color muted;
  final Color line;
  final Color userBubble;
  final Color codeBackground;
  final Color codeHeader;
  final Color soft;
  final Color surface;
  final Color accent;
  final Color accentSoft;
  final Color good;
  final Color danger;
  final double messageMaxWidth;
  final double textMaxWidth;
  final double avatarSize;
  final double messageGap;
  final double codeRadius;
  final double codeToolbarHeight;
  final double codeFontSize;
  final double codeMaxHeight;
  final double paragraphLineHeight;
  final double headingSpacing;
  final EdgeInsets tableCellPadding;
  final double toolRadius;
  final double toolFontSize;

  const AmitiaMessageTheme({
    required this.background,
    required this.text,
    required this.muted,
    required this.line,
    required this.userBubble,
    required this.codeBackground,
    required this.codeHeader,
    required this.soft,
    required this.surface,
    required this.accent,
    required this.accentSoft,
    required this.good,
    required this.danger,
    this.messageMaxWidth = 820,
    this.textMaxWidth = 700,
    this.avatarSize = 34,
    this.messageGap = 11,
    this.codeRadius = 10,
    this.codeToolbarHeight = 36,
    this.codeFontSize = 12.5,
    this.codeMaxHeight = 420,
    this.paragraphLineHeight = 1.72,
    this.headingSpacing = 22,
    this.tableCellPadding = const EdgeInsets.symmetric(
      horizontal: 11,
      vertical: 9,
    ),
    this.toolRadius = 9,
    this.toolFontSize = 12,
  });

  static final light = AmitiaMessageTheme(
    background: const Color(0xFFF7F7F8),
    text: const Color(0xFF19191C),
    muted: const Color(0xFF8C8E95),
    line: const Color(0xFFE8E8EB),
    userBubble: AppColors.light.accentSoft,
    codeBackground: const Color(0xFF1B1C20),
    codeHeader: const Color(0xFF232429),
    soft: const Color(0xFFF1F1F3),
    surface: const Color(0xFFFFFFFF),
    accent: AppColors.light.accentPrimary,
    accentSoft: AppColors.light.accentSoft,
    good: const Color(0xFF3E8D5D),
    danger: const Color(0xFFC85353),
  );

  static final dark = AmitiaMessageTheme(
    background: const Color(0xFF17181B),
    text: const Color(0xFFECECEF),
    muted: const Color(0xFF9B9DA5),
    line: const Color(0xFF2A2B30),
    userBubble: AppColors.dark.accentSoft,
    codeBackground: const Color(0xFF111216),
    codeHeader: const Color(0xFF1C1D22),
    soft: const Color(0xFF24252A),
    surface: const Color(0xFF1D1E22),
    accent: AppColors.dark.accentPrimary,
    accentSoft: AppColors.dark.accentSoft,
    good: const Color(0xFF6FBD8B),
    danger: const Color(0xFFE07878),
  );

  static AmitiaMessageTheme of(BuildContext context) {
    final theme = Theme.of(context);
    final base = theme.extension<AmitiaMessageTheme>() ??
        (theme.brightness == Brightness.dark ? dark : light);
    final colors = theme.extension<AmitiaColorTokens>();
    if (colors == null) return base;
    return base.copyWith(
      accent: colors.accentPrimary,
      accentSoft: colors.accentSoft,
      userBubble: colors.accentSoft,
    );
  }

  AmitiaMessageTheme mobile() {
    return copyWith(
      avatarSize: 30,
      messageGap: 9,
      codeRadius: 9,
      paragraphLineHeight: 1.68,
    );
  }

  @override
  AmitiaMessageTheme copyWith({
    Color? background,
    Color? text,
    Color? muted,
    Color? line,
    Color? userBubble,
    Color? codeBackground,
    Color? codeHeader,
    Color? soft,
    Color? surface,
    Color? accent,
    Color? accentSoft,
    Color? good,
    Color? danger,
    double? messageMaxWidth,
    double? textMaxWidth,
    double? avatarSize,
    double? messageGap,
    double? codeRadius,
    double? codeToolbarHeight,
    double? codeFontSize,
    double? codeMaxHeight,
    double? paragraphLineHeight,
    double? headingSpacing,
    EdgeInsets? tableCellPadding,
    double? toolRadius,
    double? toolFontSize,
  }) {
    return AmitiaMessageTheme(
      background: background ?? this.background,
      text: text ?? this.text,
      muted: muted ?? this.muted,
      line: line ?? this.line,
      userBubble: userBubble ?? this.userBubble,
      codeBackground: codeBackground ?? this.codeBackground,
      codeHeader: codeHeader ?? this.codeHeader,
      soft: soft ?? this.soft,
      surface: surface ?? this.surface,
      accent: accent ?? this.accent,
      accentSoft: accentSoft ?? this.accentSoft,
      good: good ?? this.good,
      danger: danger ?? this.danger,
      messageMaxWidth: messageMaxWidth ?? this.messageMaxWidth,
      textMaxWidth: textMaxWidth ?? this.textMaxWidth,
      avatarSize: avatarSize ?? this.avatarSize,
      messageGap: messageGap ?? this.messageGap,
      codeRadius: codeRadius ?? this.codeRadius,
      codeToolbarHeight: codeToolbarHeight ?? this.codeToolbarHeight,
      codeFontSize: codeFontSize ?? this.codeFontSize,
      codeMaxHeight: codeMaxHeight ?? this.codeMaxHeight,
      paragraphLineHeight: paragraphLineHeight ?? this.paragraphLineHeight,
      headingSpacing: headingSpacing ?? this.headingSpacing,
      tableCellPadding: tableCellPadding ?? this.tableCellPadding,
      toolRadius: toolRadius ?? this.toolRadius,
      toolFontSize: toolFontSize ?? this.toolFontSize,
    );
  }

  @override
  AmitiaMessageTheme lerp(
    covariant ThemeExtension<AmitiaMessageTheme>? other,
    double t,
  ) {
    if (other is! AmitiaMessageTheme) return this;
    return t < 0.5 ? this : other;
  }
}

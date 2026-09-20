import 'package:flutter/material.dart';

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

  static const light = AmitiaMessageTheme(
    background: Color(0xFFF7F7F8),
    text: Color(0xFF19191C),
    muted: Color(0xFF8C8E95),
    line: Color(0xFFE8E8EB),
    userBubble: Color(0xFFECE9FF),
    codeBackground: Color(0xFF1B1C20),
    codeHeader: Color(0xFF232429),
    soft: Color(0xFFF1F1F3),
    surface: Color(0xFFFFFFFF),
    accent: Color(0xFF7060E8),
    accentSoft: Color(0xFFEFEDFF),
    good: Color(0xFF3E8D5D),
    danger: Color(0xFFC85353),
  );

  static const dark = AmitiaMessageTheme(
    background: Color(0xFF17181B),
    text: Color(0xFFECECEF),
    muted: Color(0xFF9B9DA5),
    line: Color(0xFF2A2B30),
    userBubble: Color(0xFF302D4F),
    codeBackground: Color(0xFF111216),
    codeHeader: Color(0xFF1C1D22),
    soft: Color(0xFF24252A),
    surface: Color(0xFF1D1E22),
    accent: Color(0xFF9A8CFF),
    accentSoft: Color(0xFF2E294D),
    good: Color(0xFF6FBD8B),
    danger: Color(0xFFE07878),
  );

  static AmitiaMessageTheme of(BuildContext context) {
    return Theme.of(context).extension<AmitiaMessageTheme>() ??
        (Theme.of(context).brightness == Brightness.dark ? dark : light);
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

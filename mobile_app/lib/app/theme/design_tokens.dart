import 'package:flutter/material.dart';

@immutable
class AmitiaColorTokens extends ThemeExtension<AmitiaColorTokens> {
  const AmitiaColorTokens({
    required this.backgroundPrimary,
    required this.backgroundSecondary,
    required this.surfacePrimary,
    required this.surfaceSecondary,
    required this.accentPrimary,
    required this.accentSecondary,
    required this.accentSoft,
    required this.accentPressed,
    required this.textPrimary,
    required this.textSecondary,
    required this.textTertiary,
    required this.textDisabled,
    required this.borderPrimary,
    required this.borderSecondary,
    required this.success,
    required this.warning,
    required this.error,
    required this.info,
    required this.scrim,
    required this.overlay,
  });

  final Color backgroundPrimary;
  final Color backgroundSecondary;
  final Color surfacePrimary;
  final Color surfaceSecondary;
  final Color accentPrimary;
  final Color accentSecondary;
  final Color accentSoft;
  final Color accentPressed;
  final Color textPrimary;
  final Color textSecondary;
  final Color textTertiary;
  final Color textDisabled;
  final Color borderPrimary;
  final Color borderSecondary;
  final Color success;
  final Color warning;
  final Color error;
  final Color info;
  final Color scrim;
  final Color overlay;

  @override
  AmitiaColorTokens copyWith({
    Color? backgroundPrimary,
    Color? backgroundSecondary,
    Color? surfacePrimary,
    Color? surfaceSecondary,
    Color? accentPrimary,
    Color? accentSecondary,
    Color? accentSoft,
    Color? accentPressed,
    Color? textPrimary,
    Color? textSecondary,
    Color? textTertiary,
    Color? textDisabled,
    Color? borderPrimary,
    Color? borderSecondary,
    Color? success,
    Color? warning,
    Color? error,
    Color? info,
    Color? scrim,
    Color? overlay,
  }) {
    return AmitiaColorTokens(
      backgroundPrimary: backgroundPrimary ?? this.backgroundPrimary,
      backgroundSecondary: backgroundSecondary ?? this.backgroundSecondary,
      surfacePrimary: surfacePrimary ?? this.surfacePrimary,
      surfaceSecondary: surfaceSecondary ?? this.surfaceSecondary,
      accentPrimary: accentPrimary ?? this.accentPrimary,
      accentSecondary: accentSecondary ?? this.accentSecondary,
      accentSoft: accentSoft ?? this.accentSoft,
      accentPressed: accentPressed ?? this.accentPressed,
      textPrimary: textPrimary ?? this.textPrimary,
      textSecondary: textSecondary ?? this.textSecondary,
      textTertiary: textTertiary ?? this.textTertiary,
      textDisabled: textDisabled ?? this.textDisabled,
      borderPrimary: borderPrimary ?? this.borderPrimary,
      borderSecondary: borderSecondary ?? this.borderSecondary,
      success: success ?? this.success,
      warning: warning ?? this.warning,
      error: error ?? this.error,
      info: info ?? this.info,
      scrim: scrim ?? this.scrim,
      overlay: overlay ?? this.overlay,
    );
  }

  @override
  AmitiaColorTokens lerp(covariant AmitiaColorTokens? other, double t) {
    if (other == null) return this;
    Color l(Color a, Color b) => Color.lerp(a, b, t)!;
    return AmitiaColorTokens(
      backgroundPrimary: l(backgroundPrimary, other.backgroundPrimary),
      backgroundSecondary: l(backgroundSecondary, other.backgroundSecondary),
      surfacePrimary: l(surfacePrimary, other.surfacePrimary),
      surfaceSecondary: l(surfaceSecondary, other.surfaceSecondary),
      accentPrimary: l(accentPrimary, other.accentPrimary),
      accentSecondary: l(accentSecondary, other.accentSecondary),
      accentSoft: l(accentSoft, other.accentSoft),
      accentPressed: l(accentPressed, other.accentPressed),
      textPrimary: l(textPrimary, other.textPrimary),
      textSecondary: l(textSecondary, other.textSecondary),
      textTertiary: l(textTertiary, other.textTertiary),
      textDisabled: l(textDisabled, other.textDisabled),
      borderPrimary: l(borderPrimary, other.borderPrimary),
      borderSecondary: l(borderSecondary, other.borderSecondary),
      success: l(success, other.success),
      warning: l(warning, other.warning),
      error: l(error, other.error),
      info: l(info, other.info),
      scrim: l(scrim, other.scrim),
      overlay: l(overlay, other.overlay),
    );
  }
}

/// Layout values are intentionally brightness-independent. Legacy
/// AppSpacing/AppRadius names are compatibility facades backed by
/// DesignTokenRuntime, so existing pages consume runtime provider values too.
@immutable
class AmitiaLayoutTokens extends ThemeExtension<AmitiaLayoutTokens> {
  const AmitiaLayoutTokens({
    this.xs = 4,
    this.sm = 8,
    this.md = 12,
    this.lg = 16,
    this.xl = 20,
    this.xxl = 24,
    this.xxxl = 32,
    this.pagePadding = 16,
    this.cardPadding = 16,
    this.sectionGap = 24,
    this.componentGap = 12,
    this.tightGap = 8,
    this.listItemMinHeight = 52,
    this.listItemMaxHeight = 64,
    this.buttonHeight = 44,
    this.inputMinHeight = 46,
    this.radiusExtraSmall = 8,
    this.radiusSmall = 12,
    this.radiusMedium = 18,
    this.radiusLarge = 22,
    this.radiusTag = 10,
    this.radiusPill = 999,
    this.density = 1,
  });

  final double xs;
  final double sm;
  final double md;
  final double lg;
  final double xl;
  final double xxl;
  final double xxxl;
  final double pagePadding;
  final double cardPadding;
  final double sectionGap;
  final double componentGap;
  final double tightGap;
  final double listItemMinHeight;
  final double listItemMaxHeight;
  final double buttonHeight;
  final double inputMinHeight;
  final double radiusExtraSmall;
  final double radiusSmall;
  final double radiusMedium;
  final double radiusLarge;
  final double radiusTag;
  final double radiusPill;
  final double density;

  @override
  AmitiaLayoutTokens copyWith({
    double? xs,
    double? sm,
    double? md,
    double? lg,
    double? xl,
    double? xxl,
    double? xxxl,
    double? pagePadding,
    double? cardPadding,
    double? sectionGap,
    double? componentGap,
    double? tightGap,
    double? listItemMinHeight,
    double? listItemMaxHeight,
    double? buttonHeight,
    double? inputMinHeight,
    double? radiusExtraSmall,
    double? radiusSmall,
    double? radiusMedium,
    double? radiusLarge,
    double? radiusTag,
    double? radiusPill,
    double? density,
  }) {
    return AmitiaLayoutTokens(
      xs: xs ?? this.xs,
      sm: sm ?? this.sm,
      md: md ?? this.md,
      lg: lg ?? this.lg,
      xl: xl ?? this.xl,
      xxl: xxl ?? this.xxl,
      xxxl: xxxl ?? this.xxxl,
      pagePadding: pagePadding ?? this.pagePadding,
      cardPadding: cardPadding ?? this.cardPadding,
      sectionGap: sectionGap ?? this.sectionGap,
      componentGap: componentGap ?? this.componentGap,
      tightGap: tightGap ?? this.tightGap,
      listItemMinHeight: listItemMinHeight ?? this.listItemMinHeight,
      listItemMaxHeight: listItemMaxHeight ?? this.listItemMaxHeight,
      buttonHeight: buttonHeight ?? this.buttonHeight,
      inputMinHeight: inputMinHeight ?? this.inputMinHeight,
      radiusExtraSmall: radiusExtraSmall ?? this.radiusExtraSmall,
      radiusSmall: radiusSmall ?? this.radiusSmall,
      radiusMedium: radiusMedium ?? this.radiusMedium,
      radiusLarge: radiusLarge ?? this.radiusLarge,
      radiusTag: radiusTag ?? this.radiusTag,
      radiusPill: radiusPill ?? this.radiusPill,
      density: density ?? this.density,
    );
  }

  @override
  AmitiaLayoutTokens lerp(covariant AmitiaLayoutTokens? other, double t) {
    if (other == null) return this;
    double l(double a, double b) => a + (b - a) * t;
    return AmitiaLayoutTokens(
      xs: l(xs, other.xs),
      sm: l(sm, other.sm),
      md: l(md, other.md),
      lg: l(lg, other.lg),
      xl: l(xl, other.xl),
      xxl: l(xxl, other.xxl),
      xxxl: l(xxxl, other.xxxl),
      pagePadding: l(pagePadding, other.pagePadding),
      cardPadding: l(cardPadding, other.cardPadding),
      sectionGap: l(sectionGap, other.sectionGap),
      componentGap: l(componentGap, other.componentGap),
      tightGap: l(tightGap, other.tightGap),
      listItemMinHeight: l(listItemMinHeight, other.listItemMinHeight),
      listItemMaxHeight: l(listItemMaxHeight, other.listItemMaxHeight),
      buttonHeight: l(buttonHeight, other.buttonHeight),
      inputMinHeight: l(inputMinHeight, other.inputMinHeight),
      radiusExtraSmall: l(radiusExtraSmall, other.radiusExtraSmall),
      radiusSmall: l(radiusSmall, other.radiusSmall),
      radiusMedium: l(radiusMedium, other.radiusMedium),
      radiusLarge: l(radiusLarge, other.radiusLarge),
      radiusTag: l(radiusTag, other.radiusTag),
      radiusPill: l(radiusPill, other.radiusPill),
      density: l(density, other.density),
    );
  }
}

@immutable
class AmitiaTypographyTokens extends ThemeExtension<AmitiaTypographyTokens> {
  const AmitiaTypographyTokens({
    this.fontFamily,
    this.pageTitleSize = 17,
    this.pageLargeTitleSize = 22,
    this.sectionTitleSize = 12,
    this.cardTitleSize = 14,
    this.bodySize = 14,
    this.bodySmallSize = 13,
    this.captionSize = 11,
    this.labelSize = 10,
    this.statusLabelSize = 10,
    this.buttonSize = 14,
    this.pageTitleWeight = 600,
    this.sectionTitleWeight = 600,
    this.cardTitleWeight = 500,
    this.bodyWeight = 400,
    this.labelWeight = 400,
    this.buttonWeight = 500,
  });

  final String? fontFamily;
  final double pageTitleSize;
  final double pageLargeTitleSize;
  final double sectionTitleSize;
  final double cardTitleSize;
  final double bodySize;
  final double bodySmallSize;
  final double captionSize;
  final double labelSize;
  final double statusLabelSize;
  final double buttonSize;
  final int pageTitleWeight;
  final int sectionTitleWeight;
  final int cardTitleWeight;
  final int bodyWeight;
  final int labelWeight;
  final int buttonWeight;

  @override
  AmitiaTypographyTokens copyWith({
    String? fontFamily,
    double? pageTitleSize,
    double? pageLargeTitleSize,
    double? sectionTitleSize,
    double? cardTitleSize,
    double? bodySize,
    double? bodySmallSize,
    double? captionSize,
    double? labelSize,
    double? statusLabelSize,
    double? buttonSize,
    int? pageTitleWeight,
    int? sectionTitleWeight,
    int? cardTitleWeight,
    int? bodyWeight,
    int? labelWeight,
    int? buttonWeight,
  }) {
    return AmitiaTypographyTokens(
      fontFamily: fontFamily ?? this.fontFamily,
      pageTitleSize: pageTitleSize ?? this.pageTitleSize,
      pageLargeTitleSize: pageLargeTitleSize ?? this.pageLargeTitleSize,
      sectionTitleSize: sectionTitleSize ?? this.sectionTitleSize,
      cardTitleSize: cardTitleSize ?? this.cardTitleSize,
      bodySize: bodySize ?? this.bodySize,
      bodySmallSize: bodySmallSize ?? this.bodySmallSize,
      captionSize: captionSize ?? this.captionSize,
      labelSize: labelSize ?? this.labelSize,
      statusLabelSize: statusLabelSize ?? this.statusLabelSize,
      buttonSize: buttonSize ?? this.buttonSize,
      pageTitleWeight: pageTitleWeight ?? this.pageTitleWeight,
      sectionTitleWeight: sectionTitleWeight ?? this.sectionTitleWeight,
      cardTitleWeight: cardTitleWeight ?? this.cardTitleWeight,
      bodyWeight: bodyWeight ?? this.bodyWeight,
      labelWeight: labelWeight ?? this.labelWeight,
      buttonWeight: buttonWeight ?? this.buttonWeight,
    );
  }

  @override
  AmitiaTypographyTokens lerp(covariant AmitiaTypographyTokens? other, double t) {
    if (other == null) return this;
    double l(double a, double b) => a + (b - a) * t;
    int li(int a, int b) => (a + (b - a) * t).round();
    return AmitiaTypographyTokens(
      fontFamily: t < .5 ? fontFamily : other.fontFamily,
      pageTitleSize: l(pageTitleSize, other.pageTitleSize),
      pageLargeTitleSize: l(pageLargeTitleSize, other.pageLargeTitleSize),
      sectionTitleSize: l(sectionTitleSize, other.sectionTitleSize),
      cardTitleSize: l(cardTitleSize, other.cardTitleSize),
      bodySize: l(bodySize, other.bodySize),
      bodySmallSize: l(bodySmallSize, other.bodySmallSize),
      captionSize: l(captionSize, other.captionSize),
      labelSize: l(labelSize, other.labelSize),
      statusLabelSize: l(statusLabelSize, other.statusLabelSize),
      buttonSize: l(buttonSize, other.buttonSize),
      pageTitleWeight: li(pageTitleWeight, other.pageTitleWeight),
      sectionTitleWeight: li(sectionTitleWeight, other.sectionTitleWeight),
      cardTitleWeight: li(cardTitleWeight, other.cardTitleWeight),
      bodyWeight: li(bodyWeight, other.bodyWeight),
      labelWeight: li(labelWeight, other.labelWeight),
      buttonWeight: li(buttonWeight, other.buttonWeight),
    );
  }
}

@immutable
class AmitiaIconTokens extends ThemeExtension<AmitiaIconTokens> {
  const AmitiaIconTokens({
    this.extraSmall = 14,
    this.small = 16,
    this.medium = 20,
    this.large = 24,
    this.navigation = 20,
  });

  final double extraSmall;
  final double small;
  final double medium;
  final double large;
  final double navigation;

  @override
  AmitiaIconTokens copyWith({
    double? extraSmall,
    double? small,
    double? medium,
    double? large,
    double? navigation,
  }) {
    return AmitiaIconTokens(
      extraSmall: extraSmall ?? this.extraSmall,
      small: small ?? this.small,
      medium: medium ?? this.medium,
      large: large ?? this.large,
      navigation: navigation ?? this.navigation,
    );
  }

  @override
  AmitiaIconTokens lerp(covariant AmitiaIconTokens? other, double t) {
    if (other == null) return this;
    double l(double a, double b) => a + (b - a) * t;
    return AmitiaIconTokens(
      extraSmall: l(extraSmall, other.extraSmall),
      small: l(small, other.small),
      medium: l(medium, other.medium),
      large: l(large, other.large),
      navigation: l(navigation, other.navigation),
    );
  }
}

@immutable
class AmitiaComponentTokens extends ThemeExtension<AmitiaComponentTokens> {
  const AmitiaComponentTokens({
    this.toolbarHeight = 58,
    this.drawerMaxWidth = 340,
    this.borderWidth = .8,
    this.controlHeight = 46,
    this.compactControlHeight = 36,
  });

  final double toolbarHeight;
  final double drawerMaxWidth;
  final double borderWidth;
  final double controlHeight;
  final double compactControlHeight;

  @override
  AmitiaComponentTokens copyWith({
    double? toolbarHeight,
    double? drawerMaxWidth,
    double? borderWidth,
    double? controlHeight,
    double? compactControlHeight,
  }) {
    return AmitiaComponentTokens(
      toolbarHeight: toolbarHeight ?? this.toolbarHeight,
      drawerMaxWidth: drawerMaxWidth ?? this.drawerMaxWidth,
      borderWidth: borderWidth ?? this.borderWidth,
      controlHeight: controlHeight ?? this.controlHeight,
      compactControlHeight: compactControlHeight ?? this.compactControlHeight,
    );
  }

  @override
  AmitiaComponentTokens lerp(covariant AmitiaComponentTokens? other, double t) {
    if (other == null) return this;
    double l(double a, double b) => a + (b - a) * t;
    return AmitiaComponentTokens(
      toolbarHeight: l(toolbarHeight, other.toolbarHeight),
      drawerMaxWidth: l(drawerMaxWidth, other.drawerMaxWidth),
      borderWidth: l(borderWidth, other.borderWidth),
      controlHeight: l(controlHeight, other.controlHeight),
      compactControlHeight: l(compactControlHeight, other.compactControlHeight),
    );
  }
}

@immutable
class AmitiaComponentVariants extends ThemeExtension<AmitiaComponentVariants> {
  const AmitiaComponentVariants({this.values = const <String, Map<String, Object>>{}});

  final Map<String, Map<String, Object>> values;

  Map<String, Object> variant(String key) => values[key] ?? const <String, Object>{};

  @override
  AmitiaComponentVariants copyWith({Map<String, Map<String, Object>>? values}) =>
      AmitiaComponentVariants(values: values ?? this.values);

  @override
  AmitiaComponentVariants lerp(covariant AmitiaComponentVariants? other, double t) {
    if (other == null || t < .5) return this;
    return other;
  }
}

/// Runtime fallback for provider-aware widgets that are built outside a
/// ThemeData scope. AppSpacing/AppRadius are compatibility facades over this
/// state, so legacy pages receive runtime provider values as well.
abstract final class DesignTokenRuntime {
  static AmitiaLayoutTokens _layout = const AmitiaLayoutTokens();
  static AmitiaTypographyTokens _typography = const AmitiaTypographyTokens();
  static AmitiaIconTokens _icons = const AmitiaIconTokens();
  static AmitiaComponentTokens _components = const AmitiaComponentTokens();

  static AmitiaLayoutTokens get layout => _layout;
  static AmitiaTypographyTokens get typography => _typography;
  static AmitiaIconTokens get icons => _icons;
  static AmitiaComponentTokens get components => _components;

  static void activate({
    AmitiaLayoutTokens? layout,
    AmitiaTypographyTokens? typography,
    AmitiaIconTokens? icons,
    AmitiaComponentTokens? components,
  }) {
    if (layout != null) _layout = layout;
    if (typography != null) _typography = typography;
    if (icons != null) _icons = icons;
    if (components != null) _components = components;
  }

  static void activateLayout(AmitiaLayoutTokens layout) => activate(layout: layout);

  static void reset() {
    _layout = const AmitiaLayoutTokens();
    _typography = const AmitiaTypographyTokens();
    _icons = const AmitiaIconTokens();
    _components = const AmitiaComponentTokens();
  }
}

extension AmitiaDesignTokenContext on BuildContext {
  AmitiaLayoutTokens get uiLayout =>
      Theme.of(this).extension<AmitiaLayoutTokens>() ?? DesignTokenRuntime.layout;
  AmitiaTypographyTokens get uiTypography =>
      Theme.of(this).extension<AmitiaTypographyTokens>() ?? DesignTokenRuntime.typography;
  AmitiaIconTokens get uiIcons =>
      Theme.of(this).extension<AmitiaIconTokens>() ?? DesignTokenRuntime.icons;
  AmitiaComponentTokens get uiComponents =>
      Theme.of(this).extension<AmitiaComponentTokens>() ?? DesignTokenRuntime.components;
  AmitiaComponentVariants get uiComponentVariants =>
      Theme.of(this).extension<AmitiaComponentVariants>() ?? const AmitiaComponentVariants();
  Map<String, Object> uiComponentVariant(String key) => uiComponentVariants.variant(key);
}

FontWeight designFontWeight(int weight) {
  if (weight <= 100) return FontWeight.w100;
  if (weight <= 200) return FontWeight.w200;
  if (weight <= 300) return FontWeight.w300;
  if (weight <= 400) return FontWeight.w400;
  if (weight <= 500) return FontWeight.w500;
  if (weight <= 600) return FontWeight.w600;
  if (weight <= 700) return FontWeight.w700;
  if (weight <= 800) return FontWeight.w800;
  return FontWeight.w900;
}

Color? parseDesignColor(Object? raw) {
  if (raw is! String) return null;
  var value = raw.trim().replaceFirst('#', '');
  if (value.length == 6) value = 'FF$value';
  if (value.length != 8) return null;
  final parsed = int.tryParse(value, radix: 16);
  return parsed == null ? null : Color(parsed);
}

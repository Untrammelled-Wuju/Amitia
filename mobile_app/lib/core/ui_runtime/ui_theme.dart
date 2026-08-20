import 'package:flutter/material.dart';

import '../../app/theme/app_colors.dart';
import '../../app/theme/design_tokens.dart';
import 'ui_provider.dart';

Map<String, dynamic> _map(Object? value) {
  if (value is Map) return value.cast<String, dynamic>();
  return const <String, dynamic>{};
}

Map<String, dynamic> _providerTokens(
  UIProviderDefinition? provider,
  Brightness brightness,
) {
  if (provider == null || provider.builtin || !provider.enabled) {
    return const <String, dynamic>{};
  }
  final meta = provider.metadata;
  final raw = _map(meta['tokens']).isNotEmpty
      ? _map(meta['tokens'])
      : _map(meta['cssVariables']);
  final brightnessKey = brightness == Brightness.dark ? 'dark' : 'light';
  final scoped = _map(raw[brightnessKey]);
  return scoped.isEmpty ? raw : <String, dynamic>{...raw, ...scoped};
}

Map<String, dynamic> _globalProviderTokens(UIProviderDefinition? provider) {
  if (provider == null || provider.builtin || !provider.enabled) {
    return const <String, dynamic>{};
  }
  final meta = provider.metadata;
  final raw = _map(meta['tokens']).isNotEmpty
      ? _map(meta['tokens'])
      : _map(meta['cssVariables']);
  // Layout is intentionally brightness-independent. Prefer an explicit layout
  // namespace, then the root token object. Light/dark maps only affect visual
  // tokens such as colors and typography.
  final explicit = _map(meta['layoutTokens']).isNotEmpty
      ? _map(meta['layoutTokens'])
      : _map(raw['layout']);
  return explicit.isEmpty ? raw : <String, dynamic>{...raw, ...explicit};
}

double _double(Map<String, dynamic> source, String key, double fallback) {
  final value = source[key];
  return value is num ? value.toDouble() : fallback;
}

int _int(Map<String, dynamic> source, String key, int fallback) {
  final value = source[key];
  return value is num ? value.toInt() : fallback;
}

String? _string(Map<String, dynamic> source, String key, String? fallback) {
  final value = source[key];
  if (value is String && value.trim().isNotEmpty) return value.trim();
  return fallback;
}

AmitiaLayoutTokens mergeLayoutTokens(
  AmitiaLayoutTokens defaults,
  UIProviderDefinition? provider,
) {
  final source = _globalProviderTokens(provider);
  if (source.isEmpty) return defaults;
  final spacing = _map(source['spacing']);
  final radius = _map(source['radius']);
  double spacingValue(String key, double fallback) =>
      _double(spacing.isEmpty ? source : spacing, key, _double(source, key, fallback));
  double radiusValue(String key, String legacyKey, double fallback) =>
      _double(radius.isEmpty ? source : radius, key, _double(source, legacyKey, fallback));
  return AmitiaLayoutTokens(
    xs: spacingValue('xs', defaults.xs),
    sm: spacingValue('sm', defaults.sm),
    md: spacingValue('md', defaults.md),
    lg: spacingValue('lg', defaults.lg),
    xl: spacingValue('xl', defaults.xl),
    xxl: spacingValue('xxl', defaults.xxl),
    xxxl: spacingValue('xxxl', defaults.xxxl),
    pagePadding: spacingValue('page', _double(source, 'pagePadding', defaults.pagePadding)),
    cardPadding: spacingValue('card', _double(source, 'cardPadding', defaults.cardPadding)),
    sectionGap: spacingValue('section', _double(source, 'sectionGap', defaults.sectionGap)),
    componentGap: spacingValue('component', _double(source, 'componentGap', defaults.componentGap)),
    tightGap: spacingValue('tight', _double(source, 'tightGap', defaults.tightGap)),
    listItemMinHeight: _double(
      source,
      'listItemMinHeight',
      defaults.listItemMinHeight,
    ),
    listItemMaxHeight: _double(
      source,
      'listItemMaxHeight',
      defaults.listItemMaxHeight,
    ),
    buttonHeight: _double(source, 'buttonHeight', defaults.buttonHeight),
    inputMinHeight: _double(
      source,
      'inputMinHeight',
      defaults.inputMinHeight,
    ),
    radiusExtraSmall: radiusValue('xs', 'radiusExtraSmall', defaults.radiusExtraSmall),
    radiusSmall: radiusValue('sm', 'radiusSmall', defaults.radiusSmall),
    radiusMedium: radiusValue('md', 'radiusMedium', defaults.radiusMedium),
    radiusLarge: radiusValue('lg', 'radiusLarge', defaults.radiusLarge),
    radiusTag: radiusValue('tag', 'radiusTag', defaults.radiusTag),
    density: _double(source, 'density', defaults.density),
  );
}

AmitiaTypographyTokens _mergeTypography(
  AmitiaTypographyTokens defaults,
  Map<String, dynamic> source,
) {
  final typography = _map(source['typography']);
  final values = typography.isEmpty ? source : <String, dynamic>{...source, ...typography};
  return AmitiaTypographyTokens(
    fontFamily: _string(values, 'fontFamily', defaults.fontFamily),
    pageTitleSize: _double(values, 'pageTitleSize', defaults.pageTitleSize),
    pageLargeTitleSize: _double(
      values,
      'pageLargeTitleSize',
      defaults.pageLargeTitleSize,
    ),
    sectionTitleSize: _double(
      values,
      'sectionTitleSize',
      defaults.sectionTitleSize,
    ),
    cardTitleSize: _double(values, 'cardTitleSize', defaults.cardTitleSize),
    bodySize: _double(values, 'bodySize', defaults.bodySize),
    bodySmallSize: _double(values, 'bodySmallSize', defaults.bodySmallSize),
    captionSize: _double(values, 'captionSize', defaults.captionSize),
    labelSize: _double(values, 'labelSize', defaults.labelSize),
    statusLabelSize: _double(
      values,
      'statusLabelSize',
      defaults.statusLabelSize,
    ),
    buttonSize: _double(values, 'buttonSize', defaults.buttonSize),
    pageTitleWeight: _int(
      values,
      'pageTitleWeight',
      defaults.pageTitleWeight,
    ),
    sectionTitleWeight: _int(
      values,
      'sectionTitleWeight',
      defaults.sectionTitleWeight,
    ),
    cardTitleWeight: _int(
      values,
      'cardTitleWeight',
      defaults.cardTitleWeight,
    ),
    bodyWeight: _int(values, 'bodyWeight', defaults.bodyWeight),
    labelWeight: _int(values, 'labelWeight', defaults.labelWeight),
    buttonWeight: _int(values, 'buttonWeight', defaults.buttonWeight),
  );
}

AmitiaIconTokens _mergeIcons(
  AmitiaIconTokens defaults,
  Map<String, dynamic> source,
) {
  final icons = _map(source['icons']);
  final values = icons.isEmpty ? source : <String, dynamic>{...source, ...icons};
  return AmitiaIconTokens(
    extraSmall: _double(values, 'extraSmall', _double(values, 'iconExtraSmall', defaults.extraSmall)),
    small: _double(values, 'small', _double(values, 'iconSmall', defaults.small)),
    medium: _double(values, 'medium', _double(values, 'iconMedium', defaults.medium)),
    large: _double(values, 'large', _double(values, 'iconLarge', defaults.large)),
    navigation: _double(values, 'navigationSize', _double(values, 'navigation', _double(values, 'iconNavigation', defaults.navigation))),
  );
}

AmitiaComponentTokens _mergeComponents(
  AmitiaComponentTokens defaults,
  Map<String, dynamic> source,
) {
  final components = _map(source['components']);
  final values = components.isEmpty
      ? source
      : <String, dynamic>{...source, ...components};
  return AmitiaComponentTokens(
    toolbarHeight: _double(values, 'toolbarHeight', defaults.toolbarHeight),
    drawerMaxWidth: _double(values, 'drawerMaxWidth', defaults.drawerMaxWidth),
    borderWidth: _double(values, 'borderWidth', defaults.borderWidth),
    controlHeight: _double(values, 'controlHeight', defaults.controlHeight),
    compactControlHeight: _double(
      values,
      'compactControlHeight',
      defaults.compactControlHeight,
    ),
  );
}

/// Applies one visual provider layer. Multiple capabilities (ui.theme,
/// ui.tokens, ui.icons, ui.components) may be applied in sequence; later layers
/// override only the token keys they declare.
ThemeData applyUIThemeProvider(ThemeData base, UIProviderDefinition? provider) {
  if (provider == null || provider.builtin || !provider.enabled) return base;

  final source = _providerTokens(provider, base.brightness);
  final paletteDefaults = base.brightness == Brightness.dark
      ? defaultDarkColorTokens()
      : defaultLightColorTokens();
  final colorsDefault = base.extension<AmitiaColorTokens>() ?? paletteDefaults;
  final layoutDefault =
      base.extension<AmitiaLayoutTokens>() ?? const AmitiaLayoutTokens();
  final typographyDefault =
      base.extension<AmitiaTypographyTokens>() ??
      const AmitiaTypographyTokens();
  final iconDefault =
      base.extension<AmitiaIconTokens>() ?? const AmitiaIconTokens();
  final componentDefault =
      base.extension<AmitiaComponentTokens>() ??
      const AmitiaComponentTokens();

  final colorSource = _map(source['colors']);
  final colorValues = colorSource.isEmpty ? source : <String, dynamic>{...source, ...colorSource};
  Color c(String key, Color fallback, [String? semanticKey]) =>
      parseDesignColor(colorValues[key] ?? (semanticKey == null ? null : colorValues[semanticKey])) ?? fallback;

  final colors = AmitiaColorTokens(
    backgroundPrimary: c('backgroundPrimary', colorsDefault.backgroundPrimary, 'background'),
    backgroundSecondary: c(
      'backgroundSecondary',
      colorsDefault.backgroundSecondary,
    ),
    surfacePrimary: c('surfacePrimary', colorsDefault.surfacePrimary, 'surface'),
    surfaceSecondary: c('surfaceSecondary', colorsDefault.surfaceSecondary),
    accentPrimary: c('accentPrimary', colorsDefault.accentPrimary, 'primary'),
    accentSecondary: c('accentSecondary', colorsDefault.accentSecondary),
    accentSoft: c('accentSoft', colorsDefault.accentSoft),
    accentPressed: c('accentPressed', colorsDefault.accentPressed),
    textPrimary: c('textPrimary', colorsDefault.textPrimary, 'textPrimary'),
    textSecondary: c('textSecondary', colorsDefault.textSecondary, 'textSecondary'),
    textTertiary: c('textTertiary', colorsDefault.textTertiary, 'textMuted'),
    textDisabled: c('textDisabled', colorsDefault.textDisabled),
    borderPrimary: c('borderPrimary', colorsDefault.borderPrimary, 'border'),
    borderSecondary: c('borderSecondary', colorsDefault.borderSecondary),
    success: c('success', colorsDefault.success, 'success'),
    warning: c('warning', colorsDefault.warning),
    error: c('error', colorsDefault.error, 'danger'),
    info: c('info', colorsDefault.info),
    scrim: c('scrim', colorsDefault.scrim),
    overlay: c('overlay', colorsDefault.overlay),
  );

  final layout = mergeLayoutTokens(layoutDefault, provider);
  final typography = _mergeTypography(typographyDefault, source);
  final icons = _mergeIcons(iconDefault, source);
  final components = _mergeComponents(componentDefault, source);
  DesignTokenRuntime.activate(
    layout: layout,
    typography: typography,
    icons: icons,
    components: components,
  );

  return base.copyWith(
    scaffoldBackgroundColor: colors.backgroundPrimary,
    colorScheme: base.colorScheme.copyWith(
      primary: colors.accentPrimary,
      secondary: colors.accentSecondary,
      surface: colors.surfacePrimary,
      error: colors.error,
      onSurface: colors.textPrimary,
    ),
    appBarTheme: base.appBarTheme.copyWith(
      toolbarHeight: components.toolbarHeight,
      backgroundColor: colors.backgroundPrimary,
      foregroundColor: colors.textPrimary,
    ),
    cardTheme: base.cardTheme.copyWith(
      color: colors.surfacePrimary,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(layout.radiusMedium),
        side: BorderSide(color: colors.borderPrimary, width: components.borderWidth),
      ),
    ),
    inputDecorationTheme: base.inputDecorationTheme.copyWith(
      fillColor: colors.surfaceSecondary,
      contentPadding: EdgeInsets.symmetric(
        horizontal: layout.lg,
        vertical: layout.md,
      ),
      enabledBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(layout.radiusMedium),
        borderSide: BorderSide.none,
      ),
      focusedBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(layout.radiusMedium),
        borderSide: BorderSide(color: colors.accentPrimary, width: 1.5),
      ),
    ),
    dialogTheme: base.dialogTheme.copyWith(
      backgroundColor: colors.surfacePrimary,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(layout.radiusLarge),
      ),
    ),
    bottomSheetTheme: base.bottomSheetTheme.copyWith(
      backgroundColor: colors.surfacePrimary,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(
          top: Radius.circular(layout.radiusLarge),
        ),
      ),
    ),
    listTileTheme: base.listTileTheme.copyWith(
      contentPadding: EdgeInsets.symmetric(
        horizontal: layout.lg,
        vertical: layout.xs,
      ),
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(layout.radiusSmall),
      ),
    ),
    textTheme: typography.fontFamily == null
        ? base.textTheme
        : base.textTheme.apply(fontFamily: typography.fontFamily),
    iconTheme: base.iconTheme.copyWith(size: icons.medium),
    extensions: <ThemeExtension<dynamic>>[
      ...base.extensions.where(
        (extension) =>
            extension is! AmitiaColorTokens &&
            extension is! AmitiaLayoutTokens &&
            extension is! AmitiaTypographyTokens &&
            extension is! AmitiaIconTokens &&
            extension is! AmitiaComponentTokens,
      ),
      colors,
      layout,
      typography,
      icons,
      components,
    ],
  );
}

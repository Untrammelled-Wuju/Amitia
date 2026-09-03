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
      : _map(meta['theme']).isNotEmpty
          ? _map(meta['theme'])
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
      : _map(meta['theme']).isNotEmpty
          ? _map(meta['theme'])
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

double _doubleAlias(Map<String, dynamic> source, List<String> keys, double fallback) {
  for (final key in keys) {
    final value = source[key];
    if (value is num) return value.toDouble();
  }
  return fallback;
}

int _intAlias(Map<String, dynamic> source, List<String> keys, int fallback) {
  for (final key in keys) {
    final value = source[key];
    if (value is num) return value.toInt();
  }
  return fallback;
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
    radiusPill: radiusValue('pill', 'radiusPill', defaults.radiusPill),
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
    pageTitleSize: _doubleAlias(values, const ['pageTitleSize', 'titleSize'], defaults.pageTitleSize),
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
    bodySmallSize: _doubleAlias(values, const ['bodySmallSize', 'smallBodySize'], defaults.bodySmallSize),
    captionSize: _double(values, 'captionSize', defaults.captionSize),
    labelSize: _double(values, 'labelSize', defaults.labelSize),
    statusLabelSize: _double(
      values,
      'statusLabelSize',
      defaults.statusLabelSize,
    ),
    buttonSize: _double(values, 'buttonSize', defaults.buttonSize),
    pageTitleWeight: _intAlias(values, const ['pageTitleWeight', 'weightBold'], defaults.pageTitleWeight),
    sectionTitleWeight: _intAlias(values, const ['sectionTitleWeight', 'weightMedium'], defaults.sectionTitleWeight),
    cardTitleWeight: _intAlias(values, const ['cardTitleWeight', 'weightMedium'], defaults.cardTitleWeight),
    bodyWeight: _intAlias(values, const ['bodyWeight', 'weightRegular'], defaults.bodyWeight),
    labelWeight: _intAlias(values, const ['labelWeight', 'weightRegular'], defaults.labelWeight),
    buttonWeight: _intAlias(values, const ['buttonWeight', 'weightMedium'], defaults.buttonWeight),
  );
}

AmitiaIconTokens _mergeIcons(
  AmitiaIconTokens defaults,
  Map<String, dynamic> source,
) {
  final icons = _map(source['icons']);
  final values = icons.isEmpty ? source : <String, dynamic>{...source, ...icons};
  return AmitiaIconTokens(
    extraSmall: _doubleAlias(values, const ['extraSmall', 'iconExtraSmall'], defaults.extraSmall),
    small: _doubleAlias(values, const ['small', 'iconSmall'], defaults.small),
    medium: _doubleAlias(values, const ['medium', 'size', 'iconMedium'], defaults.medium),
    large: _doubleAlias(values, const ['large', 'iconLarge'], defaults.large),
    navigation: _doubleAlias(values, const ['navigation', 'navigationSize', 'iconNavigation'], defaults.navigation),
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
    drawerMaxWidth: _doubleAlias(values, const ['drawerWidth', 'drawerMaxWidth'], defaults.drawerMaxWidth),
    borderWidth: _double(values, 'borderWidth', defaults.borderWidth),
    controlHeight: _double(values, 'controlHeight', defaults.controlHeight),
    compactControlHeight: _double(
      values,
      'compactControlHeight',
      defaults.compactControlHeight,
    ),
  );
}

AmitiaComponentVariants _mergeComponentVariants(
  AmitiaComponentVariants defaults,
  UIProviderDefinition provider,
) {
  final raw = provider.metadata['componentVariants'];
  if (raw is! Map) return defaults;
  final merged = <String, Map<String, Object>>{...defaults.values};
  for (final entry in raw.entries) {
    if (entry.value is! Map) continue;
    final values = <String, Object>{};
    for (final value in (entry.value as Map).entries) {
      if (value.value is String || value.value is num || value.value is bool) {
        values[value.key.toString()] = value.value as Object;
      }
    }
    if (values.isNotEmpty) merged[entry.key.toString()] = values;
  }
  return AmitiaComponentVariants(values: merged);
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
  final componentVariantsDefault =
      base.extension<AmitiaComponentVariants>() ??
      const AmitiaComponentVariants();

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
  final componentVariants = _mergeComponentVariants(componentVariantsDefault, provider);
  Object? componentValue(String component, String key, {String? fallbackComponent}) {
    final primary = componentVariants.variant(component)[key];
    if (primary != null) return primary;
    if (fallbackComponent != null) {
      return componentVariants.variant(fallbackComponent)[key];
    }
    return null;
  }
  double componentNumber(
    String component,
    String key,
    double fallback, {
    String? fallbackComponent,
  }) {
    final value = componentValue(component, key, fallbackComponent: fallbackComponent);
    return value is num ? value.toDouble() : fallback;
  }
  int componentWeight(
    String component,
    String key,
    int fallback, {
    String? fallbackComponent,
  }) {
    final value = componentValue(component, key, fallbackComponent: fallbackComponent);
    return value is num ? value.toInt() : fallback;
  }
  Color componentColor(
    String component,
    String key,
    Color fallback, {
    String? fallbackComponent,
  }) {
    return parseDesignColor(
          componentValue(component, key, fallbackComponent: fallbackComponent),
        ) ??
        fallback;
  }
  DesignTokenRuntime.activate(
    layout: layout,
    typography: typography,
    icons: icons,
    components: components,
  );

  return base.copyWith(
    scaffoldBackgroundColor: colors.backgroundPrimary,
    canvasColor: colors.backgroundPrimary,
    disabledColor: colors.textDisabled,
    dividerColor: colors.borderSecondary,
    colorScheme: base.colorScheme.copyWith(
      primary: colors.accentPrimary,
      primaryContainer: colors.accentSoft,
      onPrimaryContainer: colors.accentPressed,
      secondary: colors.accentSecondary,
      secondaryContainer: colors.accentSoft,
      onSecondaryContainer: colors.accentPressed,
      surface: colors.surfacePrimary,
      surfaceContainerHighest: colors.surfaceSecondary,
      onSurface: colors.textPrimary,
      onSurfaceVariant: colors.textSecondary,
      outline: colors.borderPrimary,
      outlineVariant: colors.borderSecondary,
      error: colors.error,
      scrim: colors.scrim,
      surfaceTint: Colors.transparent,
    ),
    appBarTheme: base.appBarTheme.copyWith(
      toolbarHeight: components.toolbarHeight,
      backgroundColor: colors.backgroundPrimary,
      foregroundColor: colors.textPrimary,
    ),
    cardTheme: base.cardTheme.copyWith(
      color: colors.surfacePrimary,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(
          componentNumber('card', 'radius', layout.radiusMedium),
        ),
        side: BorderSide(
          color: colors.borderPrimary,
          width: componentNumber('card', 'borderWidth', components.borderWidth),
        ),
      ),
    ),
    inputDecorationTheme: base.inputDecorationTheme.copyWith(
      fillColor: colors.surfaceSecondary,
      contentPadding: EdgeInsets.symmetric(
        horizontal: componentNumber('input', 'paddingX', layout.lg),
        vertical: componentNumber('input', 'paddingY', layout.md),
      ),
      enabledBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(
          componentNumber('input', 'radius', layout.radiusMedium),
        ),
        borderSide: BorderSide.none,
      ),
      focusedBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(
          componentNumber('input', 'radius', layout.radiusMedium),
        ),
        borderSide: BorderSide(
          color: colors.accentPrimary,
          width: componentNumber('input', 'borderWidth', 1.5),
        ),
      ),
    ),
    filledButtonTheme: FilledButtonThemeData(
      style: ButtonStyle(
        minimumSize: WidgetStatePropertyAll(
          Size(0, componentNumber('button', 'minHeight', components.controlHeight)),
        ),
        padding: WidgetStatePropertyAll(
          EdgeInsets.symmetric(
            horizontal: componentNumber('button', 'paddingX', layout.lg),
            vertical: componentNumber('button', 'paddingY', layout.sm),
          ),
        ),
        shape: WidgetStatePropertyAll(
          RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(
              componentNumber('button', 'radius', layout.radiusMedium),
            ),
          ),
        ),
        textStyle: WidgetStatePropertyAll(
          TextStyle(
            fontSize: componentNumber('button', 'fontSize', typography.buttonSize),
            fontWeight: designFontWeight(
              componentWeight('button', 'fontWeight', typography.buttonWeight),
            ),
          ),
        ),
      ),
    ),
    outlinedButtonTheme: OutlinedButtonThemeData(
      style: ButtonStyle(
        minimumSize: WidgetStatePropertyAll(
          Size(
            0,
            componentNumber(
              'outlinedButton',
              'minHeight',
              components.controlHeight,
              fallbackComponent: 'button',
            ),
          ),
        ),
        padding: WidgetStatePropertyAll(
          EdgeInsets.symmetric(
            horizontal: componentNumber(
              'outlinedButton',
              'paddingX',
              layout.lg,
              fallbackComponent: 'button',
            ),
            vertical: componentNumber(
              'outlinedButton',
              'paddingY',
              layout.sm,
              fallbackComponent: 'button',
            ),
          ),
        ),
        shape: WidgetStatePropertyAll(
          RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(
              componentNumber(
                'outlinedButton',
                'radius',
                layout.radiusMedium,
                fallbackComponent: 'button',
              ),
            ),
          ),
        ),
        side: WidgetStatePropertyAll(
          BorderSide(
            color: componentColor(
              'outlinedButton',
              'borderColor',
              colors.borderPrimary,
              fallbackComponent: 'button',
            ),
            width: componentNumber(
              'outlinedButton',
              'borderWidth',
              components.borderWidth,
              fallbackComponent: 'button',
            ),
          ),
        ),
        textStyle: WidgetStatePropertyAll(
          TextStyle(
            fontSize: componentNumber(
              'outlinedButton',
              'fontSize',
              typography.buttonSize,
              fallbackComponent: 'button',
            ),
            fontWeight: designFontWeight(
              componentWeight(
                'outlinedButton',
                'fontWeight',
                typography.buttonWeight,
                fallbackComponent: 'button',
              ),
            ),
          ),
        ),
      ),
    ),
    iconButtonTheme: IconButtonThemeData(
      style: ButtonStyle(
        minimumSize: WidgetStatePropertyAll(
          Size.square(
            componentNumber(
              'iconButton',
              'minHeight',
              components.compactControlHeight,
              fallbackComponent: 'button',
            ),
          ),
        ),
        padding: WidgetStatePropertyAll(
          EdgeInsets.all(
            componentNumber('iconButton', 'padding', layout.sm),
          ),
        ),
        shape: WidgetStatePropertyAll(
          RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(
              componentNumber(
                'iconButton',
                'radius',
                layout.radiusSmall,
                fallbackComponent: 'button',
              ),
            ),
          ),
        ),
        foregroundColor: WidgetStatePropertyAll(
          componentColor('iconButton', 'color', colors.textPrimary),
        ),
      ),
    ),
    switchTheme: SwitchThemeData(
      thumbColor: WidgetStateProperty.resolveWith((states) {
        if (states.contains(WidgetState.disabled)) return colors.textDisabled;
        if (states.contains(WidgetState.selected)) {
          return componentColor('switch', 'thumbColor', Colors.white);
        }
        return componentColor('switch', 'inactiveThumbColor', colors.textTertiary);
      }),
      trackColor: WidgetStateProperty.resolveWith((states) {
        if (states.contains(WidgetState.disabled)) {
          return colors.borderSecondary.withValues(alpha: .5);
        }
        if (states.contains(WidgetState.selected)) {
          return componentColor('switch', 'activeColor', colors.accentPrimary);
        }
        return componentColor('switch', 'inactiveTrackColor', colors.borderSecondary);
      }),
      trackOutlineColor: WidgetStatePropertyAll(
        componentColor('switch', 'borderColor', Colors.transparent),
      ),
    ),
    sliderTheme: base.sliderTheme.copyWith(
      activeTrackColor: componentColor('slider', 'activeColor', colors.accentPrimary),
      inactiveTrackColor: componentColor('slider', 'inactiveColor', colors.borderSecondary),
      thumbColor: componentColor('slider', 'thumbColor', colors.accentPrimary),
      overlayColor: componentColor('slider', 'overlayColor', colors.accentSoft),
      trackHeight: componentNumber('slider', 'trackHeight', base.sliderTheme.trackHeight ?? 4),
      thumbShape: RoundSliderThumbShape(
        enabledThumbRadius: componentNumber('slider', 'thumbRadius', 10),
      ),
    ),
    textButtonTheme: TextButtonThemeData(
      style: ButtonStyle(
        minimumSize: WidgetStatePropertyAll(
          Size(0, componentNumber('button', 'minHeight', components.compactControlHeight)),
        ),
        shape: WidgetStatePropertyAll(
          RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(
              componentNumber('button', 'radius', layout.radiusMedium),
            ),
          ),
        ),
      ),
    ),
    dialogTheme: base.dialogTheme.copyWith(
      backgroundColor: colors.surfacePrimary,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(
          componentNumber('dialog', 'radius', layout.radiusLarge),
        ),
      ),
    ),
    bottomSheetTheme: base.bottomSheetTheme.copyWith(
      backgroundColor: colors.surfacePrimary,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(
          top: Radius.circular(
            componentNumber('bottomSheet', 'radius', layout.radiusLarge),
          ),
        ),
      ),
    ),
    listTileTheme: base.listTileTheme.copyWith(
      contentPadding: EdgeInsets.symmetric(
        horizontal: componentNumber('listTile', 'paddingX', layout.lg),
        vertical: componentNumber('listTile', 'paddingY', layout.xs),
      ),
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(
          componentNumber('listTile', 'radius', layout.radiusSmall),
        ),
      ),
    ),
    textTheme: typography.fontFamily == null
        ? base.textTheme
        : base.textTheme.apply(fontFamily: typography.fontFamily),
    iconTheme: base.iconTheme.copyWith(size: icons.medium),
    extensions: _buildThemeExtensions(base, colors, layout, typography, icons, components, componentVariants),
  );
}

Iterable<ThemeExtension<dynamic>> _buildThemeExtensions(
  ThemeData base,
  AmitiaColorTokens colors,
  AmitiaLayoutTokens layout,
  AmitiaTypographyTokens typography,
  AmitiaIconTokens icons,
  AmitiaComponentTokens components,
  AmitiaComponentVariants componentVariants,
) sync* {
  for (final e in base.extensions.values) {
    if (e is! AmitiaColorTokens &&
        e is! AmitiaLayoutTokens &&
        e is! AmitiaTypographyTokens &&
        e is! AmitiaIconTokens &&
        e is! AmitiaComponentTokens &&
        e is! AmitiaComponentVariants) {
      yield e;
    }
  }
  yield colors;
  yield layout;
  yield typography;
  yield icons;
  yield components;
  yield componentVariants;
}

import 'package:flutter/material.dart';

const Set<String> amitiaStyleStates = <String>{
  'default',
  'hover',
  'focus',
  'pressed',
  'disabled',
  'dark',
  'light',
};

@immutable
class AmitiaStyleScopeData {
  final String? platform;
  final String? page;
  final String? surface;
  final String? instance;

  const AmitiaStyleScopeData({
    this.platform,
    this.page,
    this.surface,
    this.instance,
  });

  factory AmitiaStyleScopeData.fromMap(Object? raw) {
    if (raw is! Map) return const AmitiaStyleScopeData();
    final value = raw.cast<String, dynamic>();
    return AmitiaStyleScopeData(
      platform: _normalizedString(value['platform']),
      page: amitiaStylePageId(_normalizedString(value['page'])),
      surface: _normalizedString(value['surface']),
      instance: _normalizedString(value['instance']),
    );
  }

  Map<String, dynamic> toMap() => <String, dynamic>{
    if (platform != null) 'platform': platform,
    if (page != null) 'page': page,
    if (surface != null) 'surface': surface,
    if (instance != null) 'instance': instance,
  };

  bool matches(AmitiaStyleScopeData current) {
    return _scopeValueMatches(platform, current.platform) &&
        _scopeValueMatches(page, current.page) &&
        _scopeValueMatches(surface, current.surface) &&
        _scopeValueMatches(instance, current.instance);
  }

  bool get isEmpty =>
      platform == null && page == null && surface == null && instance == null;
}

@immutable
class AmitiaStyleRule {
  final String id;
  final String source;
  final AmitiaStyleScopeData scope;
  final String target;
  final List<String> states;
  final Map<String, dynamic> properties;
  final String? transactionId;

  const AmitiaStyleRule({
    required this.id,
    this.source = 'ai',
    this.scope = const AmitiaStyleScopeData(),
    required this.target,
    this.states = const <String>['default'],
    this.properties = const <String, dynamic>{},
    this.transactionId,
  });

  factory AmitiaStyleRule.fromMap(
    Map<String, dynamic> raw, {
    String? fallbackId,
  }) {
    final rawProperties = raw['properties'] ?? raw['set'];
    final properties = rawProperties is Map
        ? rawProperties.cast<String, dynamic>()
        : const <String, dynamic>{};
    final rawStates = raw['states'];
    final states = rawStates is List
        ? rawStates
              .map((item) => _normalizedState(item))
              .whereType<String>()
              .toList(growable: false)
        : <String>['default'];
    return AmitiaStyleRule(
      id:
          _normalizedString(raw['id']) ??
          fallbackId ??
          'rule-${DateTime.now().microsecondsSinceEpoch}',
      source: _normalizedString(raw['source']) ?? 'ai',
      scope: AmitiaStyleScopeData.fromMap(raw['scope']),
      target: _normalizedString(raw['target']) ?? 'global',
      states: states.isEmpty ? const <String>['default'] : states,
      properties: properties,
      transactionId: _normalizedString(raw['transactionId']),
    );
  }

  Map<String, dynamic> toMap() => <String, dynamic>{
    'id': id,
    'source': source,
    if (!scope.isEmpty) 'scope': scope.toMap(),
    'target': target,
    'states': states,
    'properties': properties,
    if (transactionId != null) 'transactionId': transactionId,
  };

  bool matches(
    AmitiaStyleScopeData current,
    String requestedTarget,
    Set<String> requestedStates,
  ) {
    if (!scope.matches(current)) return false;
    if (target != '*' && target != requestedTarget) return false;
    return states.any(requestedStates.contains);
  }
}

@immutable
class AmitiaStyleTransaction {
  final String id;
  final String source;
  final int revision;
  final List<String> ruleIds;
  final String createdAt;

  const AmitiaStyleTransaction({
    required this.id,
    this.source = 'ai',
    this.revision = 0,
    this.ruleIds = const <String>[],
    this.createdAt = '',
  });

  factory AmitiaStyleTransaction.fromMap(Map<String, dynamic> raw) {
    final rawRuleIds = raw['ruleIds'];
    return AmitiaStyleTransaction(
      id: _normalizedString(raw['id']) ?? 'transaction',
      source: _normalizedString(raw['source']) ?? 'ai',
      revision: (raw['revision'] as num?)?.toInt() ?? 0,
      ruleIds: rawRuleIds is List
          ? rawRuleIds.map((item) => item.toString()).toList(growable: false)
          : const <String>[],
      createdAt: _normalizedString(raw['createdAt']) ?? '',
    );
  }

  Map<String, dynamic> toMap() => <String, dynamic>{
    'id': id,
    'source': source,
    'revision': revision,
    'ruleIds': ruleIds,
    if (createdAt.isNotEmpty) 'createdAt': createdAt,
  };
}

@immutable
class AmitiaStyleDocument {
  final int revision;
  final List<AmitiaStyleRule> rules;
  final List<AmitiaStyleTransaction> transactions;

  const AmitiaStyleDocument({
    this.revision = 0,
    this.rules = const <AmitiaStyleRule>[],
    this.transactions = const <AmitiaStyleTransaction>[],
  });

  factory AmitiaStyleDocument.fromMap(
    Map<String, dynamic> raw, {
    int revision = 0,
  }) {
    final rawRules = raw['rules'];
    final rawTransactions = raw['transactions'];
    final rules = rawRules is List
        ? rawRules
              .whereType<Map>()
              .map(
                (item) => AmitiaStyleRule.fromMap(item.cast<String, dynamic>()),
              )
              .toList(growable: false)
        : const <AmitiaStyleRule>[];
    final transactions = rawTransactions is List
        ? rawTransactions
              .whereType<Map>()
              .map(
                (item) => AmitiaStyleTransaction.fromMap(
                  item.cast<String, dynamic>(),
                ),
              )
              .toList(growable: false)
        : const <AmitiaStyleTransaction>[];
    return AmitiaStyleDocument(
      revision: (raw['revision'] as num?)?.toInt() ?? revision,
      rules: rules,
      transactions: transactions,
    );
  }

  Map<String, dynamic> toMap() => <String, dynamic>{
    'revision': revision,
    'rules': rules.map((rule) => rule.toMap()).toList(growable: false),
    'transactions': transactions
        .map((transaction) => transaction.toMap())
        .toList(growable: false),
  };

  AmitiaStyleDocument copyWith({
    int? revision,
    List<AmitiaStyleRule>? rules,
    List<AmitiaStyleTransaction>? transactions,
  }) {
    return AmitiaStyleDocument(
      revision: revision ?? this.revision,
      rules: rules ?? this.rules,
      transactions: transactions ?? this.transactions,
    );
  }
}

@immutable
class AmitiaStyleTargetDescriptor {
  final String target;
  final String? page;
  final List<String> states;
  final Map<String, String> properties;

  const AmitiaStyleTargetDescriptor({
    required this.target,
    this.page,
    this.states = const <String>['default'],
    this.properties = const <String, String>{},
  });

  Map<String, dynamic> toMap() => <String, dynamic>{
    'target': target,
    if (page != null) 'page': page,
    'states': states,
    'properties': <String, dynamic>{
      for (final entry in properties.entries)
        entry.key: <String, dynamic>{'type': entry.value},
    },
  };
}

class AmitiaStyleTargetRegistry {
  static final Map<String, AmitiaStyleTargetDescriptor> _targets =
      <String, AmitiaStyleTargetDescriptor>{
        'component.dialog': const AmitiaStyleTargetDescriptor(
          target: 'component.dialog',
          properties: <String, String>{
            'backgroundColor': 'color',
            'background': 'color',
            'foregroundColor': 'color',
            'textColor': 'color',
            'borderColor': 'color',
            'borderWidth': 'number',
            'borderStyle': 'string',
            'borderRadius': 'length',
            'shadowColor': 'color',
            'boxShadow': 'shadow',
            'padding': 'insets',
            'fontSize': 'number',
            'fontWeight': 'number',
            'elevation': 'number',
          },
        ),
        'component.bottom_sheet': const AmitiaStyleTargetDescriptor(
          target: 'component.bottom_sheet',
          properties: <String, String>{
            'backgroundColor': 'color',
            'background': 'color',
            'foregroundColor': 'color',
            'textColor': 'color',
            'borderColor': 'color',
            'borderWidth': 'number',
            'borderStyle': 'string',
            'borderRadius': 'length',
            'boxShadow': 'shadow',
            'padding': 'insets',
            'fontSize': 'number',
            'fontWeight': 'number',
            'elevation': 'number',
          },
        ),
        'component.popover': const AmitiaStyleTargetDescriptor(
          target: 'component.popover',
          properties: <String, String>{
            'backgroundColor': 'color',
            'background': 'color',
            'foregroundColor': 'color',
            'textColor': 'color',
            'borderColor': 'color',
            'borderWidth': 'number',
            'borderStyle': 'string',
            'borderRadius': 'length',
            'boxShadow': 'shadow',
            'padding': 'insets',
            'fontSize': 'number',
            'fontWeight': 'number',
          },
        ),
        'component.list_item': const AmitiaStyleTargetDescriptor(
          target: 'component.list_item',
          properties: <String, String>{
            'backgroundColor': 'color',
            'background': 'color',
            'foregroundColor': 'color',
            'textColor': 'color',
            'iconColor': 'color',
            'borderColor': 'color',
            'borderWidth': 'number',
            'borderStyle': 'string',
            'borderRadius': 'length',
            'boxShadow': 'shadow',
            'padding': 'insets',
            'fontSize': 'number',
            'fontWeight': 'number',
          },
        ),
        'card': const AmitiaStyleTargetDescriptor(
          target: 'card',
          properties: <String, String>{
            'backgroundColor': 'color',
            'background': 'color',
            'foregroundColor': 'color',
            'borderColor': 'color',
            'borderWidth': 'number',
            'borderStyle': 'string',
            'borderRadius': 'length',
            'boxShadow': 'shadow',
            'padding': 'insets',
            'margin': 'insets',
            'opacity': 'number',
          },
        ),
        'listTile': const AmitiaStyleTargetDescriptor(
          target: 'listTile',
          properties: <String, String>{
            'backgroundColor': 'color',
            'foregroundColor': 'color',
            'iconColor': 'color',
            'borderColor': 'color',
            'borderWidth': 'number',
            'borderRadius': 'length',
            'padding': 'insets',
            'margin': 'insets',
            'fontSize': 'number',
            'fontWeight': 'number',
            'opacity': 'number',
          },
        ),
        'component.icon_button': const AmitiaStyleTargetDescriptor(
          target: 'component.icon_button',
          states: <String>['default', 'hover', 'pressed', 'disabled'],
          properties: <String, String>{
            'backgroundColor': 'color',
            'foregroundColor': 'color',
            'borderColor': 'color',
            'borderWidth': 'number',
            'borderRadius': 'length',
            'padding': 'insets',
            'width': 'length',
            'height': 'length',
            'iconColor': 'color',
            'iconSize': 'number',
            'opacity': 'number',
          },
        ),
        'app.shell': const AmitiaStyleTargetDescriptor(
          target: 'app.shell',
          properties: <String, String>{
            'backgroundColor': 'color',
            'borderColor': 'color',
            'borderRadius': 'length',
          },
        ),
        'editor': const AmitiaStyleTargetDescriptor(
          target: 'editor',
          properties: <String, String>{
            'backgroundColor': 'color',
            'foregroundColor': 'color',
            'borderColor': 'color',
            'borderWidth': 'number',
            'borderRadius': 'length',
            'padding': 'insets',
            'opacity': 'number',
          },
        ),
        'text_field': const AmitiaStyleTargetDescriptor(
          target: 'text_field',
          properties: <String, String>{
            'backgroundColor': 'color',
            'foregroundColor': 'color',
            'placeholderColor': 'color',
            'borderColor': 'color',
            'borderWidth': 'number',
            'borderRadius': 'length',
            'padding': 'insets',
            'fontFamily': 'string',
            'fontSize': 'number',
            'fontWeight': 'number',
            'height': 'length',
            'width': 'length',
            'opacity': 'number',
          },
        ),
        'text_field.search': const AmitiaStyleTargetDescriptor(
          target: 'text_field.search',
          properties: <String, String>{
            'backgroundColor': 'color',
            'foregroundColor': 'color',
            'placeholderColor': 'color',
            'borderColor': 'color',
            'borderWidth': 'number',
            'borderRadius': 'length',
            'padding': 'insets',
            'fontFamily': 'string',
            'fontSize': 'number',
            'iconColor': 'color',
            'iconSize': 'number',
            'height': 'length',
            'opacity': 'number',
          },
        ),
        'switch': const AmitiaStyleTargetDescriptor(
          target: 'switch',
          states: <String>['default', 'checked', 'disabled'],
          properties: <String, String>{
            'backgroundColor': 'color',
            'foregroundColor': 'color',
            'borderColor': 'color',
            'borderWidth': 'number',
            'borderRadius': 'length',
            'padding': 'insets',
            'fontSize': 'number',
            'fontWeight': 'number',
            'opacity': 'number',
          },
        ),
        'segmented': const AmitiaStyleTargetDescriptor(
          target: 'segmented',
          states: <String>['default', 'selected', 'pressed', 'disabled'],
          properties: <String, String>{
            'backgroundColor': 'color',
            'foregroundColor': 'color',
            'borderColor': 'color',
            'borderWidth': 'number',
            'borderRadius': 'length',
            'padding': 'insets',
            'fontSize': 'number',
            'fontWeight': 'number',
            'opacity': 'number',
          },
        ),
        'message.user': const AmitiaStyleTargetDescriptor(
          target: 'message.user',
          page: 'chat',
          properties: <String, String>{
            'backgroundColor': 'color',
            'foregroundColor': 'color',
            'borderColor': 'color',
            'borderWidth': 'number',
            'borderRadius': 'length',
            'padding': 'insets',
            'margin': 'insets',
            'fontFamily': 'string',
            'fontSize': 'number',
            'fontWeight': 'number',
            'lineHeight': 'number',
            'opacity': 'number',
          },
        ),
        'message.assistant': const AmitiaStyleTargetDescriptor(
          target: 'message.assistant',
          page: 'chat',
          states: <String>['default', 'loading'],
          properties: <String, String>{
            'backgroundColor': 'color',
            'foregroundColor': 'color',
            'borderColor': 'color',
            'borderWidth': 'number',
            'borderRadius': 'length',
            'padding': 'insets',
            'margin': 'insets',
            'fontFamily': 'string',
            'fontSize': 'number',
            'fontWeight': 'number',
            'lineHeight': 'number',
            'opacity': 'number',
          },
        ),
        'code_block': const AmitiaStyleTargetDescriptor(
          target: 'code_block',
          page: 'chat',
          properties: <String, String>{
            'backgroundColor': 'color',
            'foregroundColor': 'color',
            'borderColor': 'color',
            'borderWidth': 'number',
            'borderRadius': 'length',
            'padding': 'insets',
            'margin': 'insets',
            'fontFamily': 'string',
            'fontSize': 'number',
            'lineHeight': 'number',
            'width': 'length',
            'opacity': 'number',
          },
        ),
        'attachment': const AmitiaStyleTargetDescriptor(
          target: 'attachment',
          page: 'chat',
          properties: <String, String>{
            'backgroundColor': 'color',
            'foregroundColor': 'color',
            'borderColor': 'color',
            'borderWidth': 'number',
            'borderRadius': 'length',
            'padding': 'insets',
            'margin': 'insets',
            'opacity': 'number',
          },
        ),
        'chat.composer.surface': const AmitiaStyleTargetDescriptor(
          target: 'chat.composer.surface',
          page: 'chat',
          properties: <String, String>{
            'backgroundColor': 'color',
            'borderColor': 'color',
            'borderWidth': 'number',
            'borderRadius': 'length',
            'shadowColor': 'color',
          },
        ),
        'chat.composer.input': const AmitiaStyleTargetDescriptor(
          target: 'chat.composer.input',
          page: 'chat',
          properties: <String, String>{
            'backgroundColor': 'color',
            'foregroundColor': 'color',
            'placeholderColor': 'color',
            'borderColor': 'color',
            'borderWidth': 'number',
            'borderRadius': 'length',
            'fontSize': 'number',
            'fontFamily': 'string',
          },
        ),
        'chat.composer.send': const AmitiaStyleTargetDescriptor(
          target: 'chat.composer.send',
          page: 'chat',
          properties: <String, String>{
            'backgroundColor': 'color',
            'foregroundColor': 'color',
            'borderRadius': 'length',
          },
        ),
        'chat.composer.add': const AmitiaStyleTargetDescriptor(
          target: 'chat.composer.add',
          page: 'chat',
          properties: <String, String>{
            'backgroundColor': 'color',
            'foregroundColor': 'color',
            'borderRadius': 'length',
          },
        ),
        'chat.composer.model': const AmitiaStyleTargetDescriptor(
          target: 'chat.composer.model',
          page: 'chat',
          properties: <String, String>{
            'backgroundColor': 'color',
            'foregroundColor': 'color',
            'borderColor': 'color',
            'borderRadius': 'length',
          },
        ),
        'chat.composer.permission': const AmitiaStyleTargetDescriptor(
          target: 'chat.composer.permission',
          page: 'chat',
          properties: <String, String>{
            'backgroundColor': 'color',
            'foregroundColor': 'color',
            'borderRadius': 'length',
          },
        ),
        'button.primary': const AmitiaStyleTargetDescriptor(
          target: 'button.primary',
          properties: <String, String>{
            'backgroundColor': 'color',
            'foregroundColor': 'color',
            'borderColor': 'color',
            'borderWidth': 'number',
            'borderRadius': 'length',
            'padding': 'insets',
            'fontSize': 'number',
            'fontWeight': 'number',
          },
        ),
      };

  static void register(AmitiaStyleTargetDescriptor descriptor) {
    _targets[descriptor.target] = descriptor;
  }

  static void unregister(String target) {
    _targets.remove(target);
  }

  static AmitiaStyleTargetDescriptor? descriptor(String target) =>
      _targets[target];

  static List<AmitiaStyleTargetDescriptor> all() =>
      _targets.values.toList(growable: false);

  static List<AmitiaStyleTargetDescriptor> forPage(String? page) {
    if (page == null || page.isEmpty) return all();
    final result = _targets.values
        .where((item) => item.page == null || item.page == page)
        .toList(growable: false);
    final pageTarget = amitiaStyleTargetForPage(page);
    if (!_targets.containsKey(pageTarget)) {
      return <AmitiaStyleTargetDescriptor>[
        AmitiaStyleTargetDescriptor(
          target: pageTarget,
          page: page,
          properties: const <String, String>{
            'backgroundColor': 'color',
            'foregroundColor': 'color',
            'borderColor': 'color',
            'borderRadius': 'length',
            'padding': 'insets',
          },
        ),
        ...result,
      ];
    }
    return result;
  }
}

@immutable
class AmitiaResolvedStyle {
  final Map<String, dynamic> properties;
  final List<String> warnings;

  const AmitiaResolvedStyle({
    this.properties = const <String, dynamic>{},
    this.warnings = const <String>[],
  });

  dynamic operator [](String key) => properties[key];

  bool has(String key) => properties.containsKey(key);

  Color? color(String key) => _parseColor(properties[key]);

  double? number(String key) {
    final value = properties[key];
    if (value is num) return value.toDouble();
    return double.tryParse(value?.toString() ?? '');
  }

  String? string(String key) => _normalizedString(properties[key]);

  bool? boolean(String key) {
    final value = properties[key];
    return value is bool ? value : null;
  }

  EdgeInsets? edgeInsets(String key) => _parseEdgeInsets(properties[key]);

  BorderRadius? borderRadius(String key) => _parseBorderRadius(properties[key]);

  BorderSide? borderSide({BorderSide? fallback}) {
    final raw = properties['border'] ?? properties['borderSide'];
    final borderColor = color('borderColor') ?? fallback?.color;
    final width = number('borderWidth') ?? fallback?.width;
    final style = _parseBorderStyle(
      properties['borderStyle'] ?? (raw is Map ? raw['style'] : null),
    );
    if (borderColor == null && width == null && style == null) return fallback;
    return BorderSide(
      color: borderColor ?? fallback?.color ?? Colors.transparent,
      width: width ?? fallback?.width ?? 1,
      style: style ?? fallback?.style ?? BorderStyle.solid,
    );
  }

  List<BoxShadow>? boxShadow() => _parseBoxShadows(
    properties['boxShadow'] ?? properties['boxShadows'] ?? properties['shadow'],
  );

  TextStyle applyTextStyle(TextStyle base) {
    final next = base.copyWith(
      color:
          color('foregroundColor') ??
          color('textColor') ??
          color('color') ??
          base.color,
      fontFamily: string('fontFamily') ?? base.fontFamily,
      fontSize: number('fontSize') ?? base.fontSize,
      fontWeight: _parseFontWeight(properties['fontWeight']) ?? base.fontWeight,
      fontStyle: _parseFontStyle(properties['fontStyle']) ?? base.fontStyle,
      letterSpacing: number('letterSpacing') ?? base.letterSpacing,
      height: number('lineHeight') ?? base.height,
      decoration:
          _parseTextDecoration(properties['decoration']) ?? base.decoration,
    );
    return next;
  }

  IconThemeData applyIconTheme(IconThemeData base) {
    return base.copyWith(
      color:
          color('iconColor') ??
          color('foregroundColor') ??
          color('color') ??
          base.color,
      size: number('iconSize') ?? base.size,
    );
  }

  BoxDecoration applyBoxDecoration(BoxDecoration base) {
    final borderSideValue = borderSide();
    final radius = borderRadius('borderRadius');
    return base.copyWith(
      color: color('backgroundColor') ?? color('background') ?? base.color,
      borderRadius: radius ?? base.borderRadius,
      border: borderSideValue == null
          ? base.border
          : Border.all(
              color: borderSideValue.color,
              width: borderSideValue.width,
              style: borderSideValue.style,
            ),
      boxShadow: boxShadow() ?? base.boxShadow,
      gradient: _parseGradient(properties['gradient']) ?? base.gradient,
    );
  }

  ButtonStyle applyButtonStyle(ButtonStyle base) {
    final side = borderSide();
    final shapeRadius = borderRadius('borderRadius');
    return base.copyWith(
      backgroundColor:
          _stateProperty(color('backgroundColor') ?? color('background')) ??
          base.backgroundColor,
      foregroundColor:
          _stateProperty(color('foregroundColor') ?? color('textColor')) ??
          base.foregroundColor,
      overlayColor: _stateProperty(color('overlayColor')) ?? base.overlayColor,
      elevation: _stateProperty(number('elevation')) ?? base.elevation,
      padding: _stateProperty(edgeInsets('padding')) ?? base.padding,
      minimumSize:
          _stateProperty(_parseSize(properties['minimumSize'])) ??
          base.minimumSize,
      fixedSize:
          _stateProperty(_parseSize(properties['fixedSize'])) ?? base.fixedSize,
      side: _stateProperty(side) ?? base.side,
      shape: shapeRadius == null
          ? base.shape
          : _stateProperty(RoundedRectangleBorder(borderRadius: shapeRadius)) ??
                base.shape,
      textStyle:
          _stateProperty(applyTextStyle(const TextStyle())) ?? base.textStyle,
    );
  }
}

class AmitiaStyleResolver {
  static AmitiaResolvedStyle resolve({
    required AmitiaStyleDocument document,
    required AmitiaStyleScopeData scope,
    required String target,
    List<String> states = const <String>['default'],
  }) {
    final requestedStates = <String>{
      'default',
      for (final state in states) ?_normalizedState(state),
    };
    final selected = <_PrioritizedRule>[];
    for (var index = 0; index < document.rules.length; index++) {
      final rule = document.rules[index];
      if (!rule.matches(scope, target, requestedStates)) continue;
      selected.add(
        _PrioritizedRule(
          rule: rule,
          score: _ruleScore(rule, scope, target, requestedStates) + index,
        ),
      );
    }
    selected.sort((a, b) => a.score.compareTo(b.score));
    final merged = <String, dynamic>{};
    for (final item in selected) {
      _deepMergeInto(merged, item.rule.properties);
    }
    final warnings = <String>[];
    for (final key in merged.keys) {
      if (!_supportedProperties.contains(key)) {
        warnings.add('unsupported property: $key');
      }
    }
    return AmitiaResolvedStyle(
      properties: Map<String, dynamic>.unmodifiable(merged),
      warnings: warnings,
    );
  }
}

class _PrioritizedRule {
  final AmitiaStyleRule rule;
  final int score;

  const _PrioritizedRule({required this.rule, required this.score});
}

class AmitiaStyleScopeHost extends InheritedWidget {
  final AmitiaStyleScopeData scope;

  const AmitiaStyleScopeHost({
    super.key,
    required this.scope,
    required super.child,
  });

  static AmitiaStyleScopeHost? maybeOf(BuildContext context) =>
      context.dependOnInheritedWidgetOfExactType<AmitiaStyleScopeHost>();

  static AmitiaStyleScopeData of(BuildContext context) =>
      maybeOf(context)?.scope ?? const AmitiaStyleScopeData(platform: 'mobile');

  @override
  bool updateShouldNotify(AmitiaStyleScopeHost oldWidget) =>
      oldWidget.scope != scope;
}

class AmitiaStyleDocumentHost extends InheritedWidget {
  final AmitiaStyleDocument document;

  const AmitiaStyleDocumentHost({
    super.key,
    required this.document,
    required super.child,
  });

  static AmitiaStyleDocumentHost? maybeOf(BuildContext context) =>
      context.dependOnInheritedWidgetOfExactType<AmitiaStyleDocumentHost>();

  static AmitiaStyleDocument of(BuildContext context) =>
      maybeOf(context)?.document ?? const AmitiaStyleDocument();

  @override
  bool updateShouldNotify(AmitiaStyleDocumentHost oldWidget) =>
      oldWidget.document != document;
}

class AmitiaStyleTarget extends StatelessWidget {
  final String target;
  final List<String> states;
  final Widget child;
  final Widget Function(BuildContext context, AmitiaResolvedStyle style)?
  builder;

  const AmitiaStyleTarget({
    super.key,
    required this.target,
    this.states = const <String>['default'],
    required this.child,
    this.builder,
  });

  @override
  Widget build(BuildContext context) {
    final style = context.amitiaStyleFor(target, states: states);
    return builder?.call(context, style) ?? child;
  }
}

extension AmitiaStyleBuildContext on BuildContext {
  AmitiaStyleScopeData get amitiaStyleScope => AmitiaStyleScopeHost.of(this);

  AmitiaStyleDocument get amitiaStyleDocument =>
      AmitiaStyleDocumentHost.of(this);

  AmitiaResolvedStyle amitiaStyleFor(
    String target, {
    List<String> states = const <String>['default'],
  }) {
    return AmitiaStyleResolver.resolve(
      document: amitiaStyleDocument,
      scope: amitiaStyleScope,
      target: target,
      states: states,
    );
  }
}

Map<String, String> amitiaStyleVars(
  BuildContext context,
  String target, {
  List<String> states = const <String>['default'],
}) {
  final style = context.amitiaStyleFor(target, states: states);
  return <String, String>{
    for (final entry in style.properties.entries)
      '--amitia-${_cssVariableName(entry.key)}': _cssVariableValue(entry.value),
  };
}

Map<String, dynamic> amitiaStyleBridgePayload(
  BuildContext context,
  String target, {
  List<String> states = const <String>['default'],
  List<String> additionalTargets = const <String>[],
}) {
  final style = context.amitiaStyleFor(target, states: states);
  final targetNames = <String>{target, ...additionalTargets};
  final targets = <String, dynamic>{
    for (final item in targetNames)
      item: <String, dynamic>{
        'target': item,
        'properties': context.amitiaStyleFor(item, states: states).properties,
        'vars': amitiaStyleVars(context, item, states: states),
        'warnings': context.amitiaStyleFor(item, states: states).warnings,
      },
  };
  final scope = context.amitiaStyleScope;
  final scopeRules = context.amitiaStyleDocument.rules
      .where(
        (rule) =>
            rule.scope.matches(scope) &&
            (rule.target == '*' ||
                rule.target.startsWith('extension.') ||
                rule.target.startsWith('plugin.') ||
                rule.target.startsWith('component.') ||
                rule.target.startsWith('page.')),
      )
      .map((rule) => rule.toMap())
      .toList(growable: false);
  return <String, dynamic>{
    'schemaVersion': 1,
    'target': target,
    'states': states,
    'scope': scope.toMap(),
    'properties': style.properties,
    'vars': amitiaStyleVars(context, target, states: states),
    'warnings': style.warnings,
    'targets': targets,
    'rules': scopeRules,
  };
}

String? amitiaStylePageId(String? location) {
  final text = (location ?? '').trim();
  if (text.isEmpty) return null;
  if (!text.contains('/') && !text.contains('\\') && !text.contains('?')) {
    return text.split('.').map(_segmentId).join('.');
  }
  final uri = Uri.tryParse(text);
  final path = (uri?.path ?? text).replaceAll('\\', '/');
  final segments = path
      .split('/')
      .where((segment) => segment.trim().isNotEmpty)
      .toList(growable: false);
  if (segments.isEmpty) return 'root';
  if (segments.length >= 3 &&
      segments[0].toLowerCase() == 'extension' &&
      segments[1].toLowerCase() == 'page') {
    final page = segments.skip(2).map(_segmentId).join('.');
    final extensionId = uri?.queryParameters['extensionId']?.trim();
    return extensionId == null || extensionId.isEmpty
        ? 'plugin.$page'
        : 'plugin.${_segmentId(extensionId)}.$page';
  }
  return segments.map(_segmentId).join('.');
}

String amitiaStyleTargetForPage(String? page) {
  final value = (page ?? '').trim();
  return value.isEmpty ? 'page.root' : 'page.$value';
}

String? _normalizedString(Object? value) {
  final text = value?.toString().trim();
  return text == null || text.isEmpty ? null : text;
}

String? _normalizedState(Object? value) {
  final text = _normalizedString(value)?.toLowerCase();
  if (text == null) return null;
  return amitiaStyleStates.contains(text) ? text : text;
}

String _segmentId(String value) {
  final normalized = value.trim().toLowerCase();
  if (normalized.isEmpty) return 'root';
  return normalized.replaceAll(RegExp(r'[^a-z0-9_-]+'), '-');
}

String _cssVariableName(String key) {
  final buffer = StringBuffer();
  for (var index = 0; index < key.length; index++) {
    final char = key[index];
    final upper = char.toUpperCase();
    if (index > 0 && char == upper && char != char.toLowerCase()) {
      buffer.write('-');
    }
    buffer.write(char.toLowerCase());
  }
  return buffer.toString();
}

String _cssVariableValue(Object? value) {
  if (value is Map) return Map<String, dynamic>.from(value).toString();
  if (value is List) return value.join(',');
  return value?.toString() ?? '';
}

bool _scopeValueMatches(String? expected, String? current) {
  if (expected == null || expected == '*' || expected == 'all') return true;
  if (current == null) return false;
  if (expected == current) return true;
  final expectedPage = amitiaStylePageId(expected);
  final currentPage = amitiaStylePageId(current);
  if (expectedPage != null && currentPage != null) {
    return expectedPage == currentPage;
  }
  return expected == current;
}

int _ruleScore(
  AmitiaStyleRule rule,
  AmitiaStyleScopeData scope,
  String target,
  Set<String> states,
) {
  var score = rule.source == 'ai' ? 200 : 100;
  if (rule.scope.page != null && rule.scope.page == scope.page) score += 40;
  if (rule.scope.surface != null && rule.scope.surface == scope.surface) {
    score += 60;
  }
  if (rule.scope.instance != null && rule.scope.instance == scope.instance) {
    score += 80;
  }
  if (rule.target == target) score += 120;
  if (rule.states.any(
    (state) => states.contains(state) && state != 'default',
  )) {
    score += 24;
  }
  return score;
}

void _deepMergeInto(
  Map<String, dynamic> destination,
  Map<String, dynamic> source,
) {
  for (final entry in source.entries) {
    final current = destination[entry.key];
    if (current is Map && entry.value is Map) {
      final merged = current.cast<String, dynamic>();
      _deepMergeInto(merged, (entry.value as Map).cast<String, dynamic>());
      destination[entry.key] = merged;
    } else {
      destination[entry.key] = entry.value;
    }
  }
}

Color? _parseColor(Object? raw) {
  if (raw == null) return null;
  if (raw is Color) return raw;
  final value = raw.toString().trim().replaceFirst('#', '');
  if (value.length == 6) {
    final parsed = int.tryParse(value, radix: 16);
    return parsed == null ? null : Color(0xFF000000 | parsed);
  }
  if (value.length == 8) {
    final parsed = int.tryParse(value, radix: 16);
    return parsed == null ? null : Color(parsed);
  }
  return null;
}

EdgeInsets? _parseEdgeInsets(Object? raw) {
  if (raw is num) return EdgeInsets.all(raw.toDouble());
  if (raw is! Map) return null;
  final value = raw.cast<String, dynamic>();
  double read(String key) => (value[key] as num?)?.toDouble() ?? 0;
  return EdgeInsets.fromLTRB(
    read('left') + read('x') / 2,
    read('top') + read('y') / 2,
    read('right') + read('x') / 2,
    read('bottom') + read('y') / 2,
  );
}

BorderRadius? _parseBorderRadius(Object? raw) {
  if (raw is num) return BorderRadius.circular(raw.toDouble());
  if (raw is! Map) return null;
  final value = raw.cast<String, dynamic>();
  double read(String key) => (value[key] as num?)?.toDouble() ?? 0;
  return BorderRadius.only(
    topLeft: Radius.circular(read('topLeft') + read('all')),
    topRight: Radius.circular(read('topRight') + read('all')),
    bottomLeft: Radius.circular(read('bottomLeft') + read('all')),
    bottomRight: Radius.circular(read('bottomRight') + read('all')),
  );
}

BorderStyle? _parseBorderStyle(Object? raw) {
  final value = _normalizedString(raw)?.toLowerCase();
  return switch (value) {
    'none' => BorderStyle.none,
    'solid' => BorderStyle.solid,
    _ => null,
  };
}

List<BoxShadow>? _parseBoxShadows(Object? raw) {
  final values = raw is List
      ? raw
      : raw == null
      ? const <Object?>[]
      : <Object?>[raw];
  final shadows = <BoxShadow>[];
  for (final item in values) {
    if (item is! Map) continue;
    final value = item.cast<String, dynamic>();
    final color = _parseColor(value['color']);
    if (color == null) continue;
    shadows.add(
      BoxShadow(
        color: color,
        blurRadius: (value['blurRadius'] as num?)?.toDouble() ?? 0,
        spreadRadius: (value['spreadRadius'] as num?)?.toDouble() ?? 0,
        offset: Offset(
          (value['offsetX'] as num?)?.toDouble() ??
              ((value['offset'] as num?)?.toDouble() ?? 0),
          (value['offsetY'] as num?)?.toDouble() ?? 0,
        ),
      ),
    );
  }
  return shadows.isEmpty ? null : shadows;
}

FontWeight? _parseFontWeight(Object? raw) {
  if (raw is num) {
    final value = raw.toInt().clamp(100, 900);
    return FontWeight.values[((value / 100).round() - 1).clamp(0, 8)];
  }
  final value = _normalizedString(raw)?.toLowerCase();
  return switch (value) {
    '100' => FontWeight.w100,
    '200' => FontWeight.w200,
    '300' => FontWeight.w300,
    '400' || 'normal' => FontWeight.w400,
    '500' => FontWeight.w500,
    '600' || 'semibold' => FontWeight.w600,
    '700' || 'bold' => FontWeight.w700,
    '800' => FontWeight.w800,
    '900' => FontWeight.w900,
    _ => null,
  };
}

FontStyle? _parseFontStyle(Object? raw) {
  return switch (_normalizedString(raw)?.toLowerCase()) {
    'italic' => FontStyle.italic,
    'normal' => FontStyle.normal,
    _ => null,
  };
}

TextDecoration? _parseTextDecoration(Object? raw) {
  return switch (_normalizedString(raw)?.toLowerCase()) {
    'none' => TextDecoration.none,
    'underline' => TextDecoration.underline,
    'line-through' => TextDecoration.lineThrough,
    'overline' => TextDecoration.overline,
    _ => null,
  };
}

Gradient? _parseGradient(Object? raw) {
  if (raw is! Map) return null;
  final value = raw.cast<String, dynamic>();
  final colors = (value['colors'] as List?)
      ?.map(_parseColor)
      .whereType<Color>()
      .toList(growable: false);
  if (colors == null || colors.isEmpty) return null;
  final begin = _parseAlignment(value['begin']) ?? Alignment.topCenter;
  final end = _parseAlignment(value['end']) ?? Alignment.bottomCenter;
  return LinearGradient(
    colors: colors,
    begin: begin,
    end: end,
    stops: (value['stops'] as List?)
        ?.whereType<num>()
        .map((item) => item.toDouble())
        .toList(growable: false),
  );
}

Alignment? _parseAlignment(Object? raw) {
  return switch (_normalizedString(raw)?.toLowerCase()) {
    'top' || 'topcenter' => Alignment.topCenter,
    'bottom' || 'bottomcenter' => Alignment.bottomCenter,
    'left' || 'centerleft' => Alignment.centerLeft,
    'right' || 'centerright' => Alignment.centerRight,
    'topleft' => Alignment.topLeft,
    'topright' => Alignment.topRight,
    'bottomleft' => Alignment.bottomLeft,
    'bottomright' => Alignment.bottomRight,
    _ => null,
  };
}

Size? _parseSize(Object? raw) {
  if (raw is num) return Size(raw.toDouble(), raw.toDouble());
  if (raw is! Map) return null;
  final value = raw.cast<String, dynamic>();
  final width = (value['width'] as num?)?.toDouble();
  final height = (value['height'] as num?)?.toDouble();
  if (width == null && height == null) return null;
  return Size(width ?? 0, height ?? 0);
}

WidgetStateProperty<T>? _stateProperty<T>(T? value) {
  return value == null ? null : WidgetStatePropertyAll<T>(value);
}

const Set<String> _supportedProperties = <String>{
  'backgroundColor',
  'background',
  'foregroundColor',
  'textColor',
  'color',
  'placeholderColor',
  'borderColor',
  'borderWidth',
  'borderStyle',
  'borderRadius',
  'border',
  'borderSide',
  'shadowColor',
  'boxShadow',
  'boxShadows',
  'shadow',
  'fontFamily',
  'fontSize',
  'fontWeight',
  'fontStyle',
  'lineHeight',
  'letterSpacing',
  'decoration',
  'iconColor',
  'iconSize',
  'padding',
  'margin',
  'opacity',
  'gradient',
  'overlayColor',
  'elevation',
  'minimumSize',
  'fixedSize',
  'width',
  'height',
};

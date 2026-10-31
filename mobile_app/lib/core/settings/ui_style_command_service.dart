import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../artifact/artifact_providers.dart';
import '../ui_runtime/mobile_style_runtime.dart';
import 'appearance_preferences.dart';
import 'background_preferences.dart';
import 'custom_palette.dart';
import 'ui_style_profile.dart';

final uiStyleCommandServiceProvider = Provider<UIStyleCommandService>((ref) {
  return UIStyleCommandService(ref);
});

class UIStyleCommandService {
  UIStyleCommandService(this._ref);

  final Ref _ref;
  String? _currentPage;

  UIStylePatchNotifier get _patch => _ref.read(uiStylePatchProvider.notifier);
  AppearancePreferencesNotifier get _appearance =>
      _ref.read(appearancePreferencesProvider.notifier);
  BackgroundPreferencesNotifier get _background =>
      _ref.read(backgroundPreferencesProvider.notifier);
  AppearancePreferences get _appearanceState =>
      _ref.read(appearancePreferencesProvider);
  BackgroundPreferences get _backgroundState =>
      _ref.read(backgroundPreferencesProvider);
  UIStylePatchState get _patchState => _ref.read(uiStylePatchProvider);

  void setCurrentPage(String? page) {
    _currentPage = page;
  }

  Future<void> init() async {
    await Future.wait(<Future<void>>[
      _patch.init(),
      _appearance.init(),
      _background.init(),
    ]);
  }

  Future<Map<String, dynamic>> execute({
    required String action,
    required int expectedRevision,
    required Map<String, dynamic> changes,
  }) async {
    await init();
    switch (action) {
      case 'inspect':
        return _inspect(changes);
      case 'apply':
        return _apply(expectedRevision, changes);
      case 'remove':
        return _remove(expectedRevision, changes);
      case 'reset':
        return _reset(expectedRevision, changes);
      case 'undo':
        return _undo(expectedRevision);
      default:
        throw StateError('unsupported style action: $action');
    }
  }

  Map<String, dynamic> _inspect([Map<String, dynamic> changes = const {}]) {
    final scope = _scopeFrom(changes, fallbackPage: _currentPage);
    final document = _patchState.document;
    final targets = <String, dynamic>{};
    for (final descriptor in AmitiaStyleTargetRegistry.forPage(scope.page)) {
      final resolved = AmitiaStyleResolver.resolve(
        document: document,
        scope: scope,
        target: descriptor.target,
      );
      targets[descriptor.target] = <String, dynamic>{
        ...descriptor.toMap(),
        'current': resolved.properties,
        'warnings': resolved.warnings,
      };
    }
    return <String, dynamic>{
      'ok': true,
      'revision': document.revision,
      'page': scope.page,
      'scope': scope.toMap(),
      'targets': targets,
      'rules': document.rules
          .map((rule) => rule.toMap())
          .toList(growable: false),
      'transactions': document.transactions
          .map((transaction) => transaction.toMap())
          .toList(growable: false),
      'patch': _patchState.patch,
      'appearance': _appearanceState.toMap(),
      'background': _backgroundState.toMap(),
    };
  }

  Future<Map<String, dynamic>> _apply(
    int expectedRevision,
    Map<String, dynamic> changes,
  ) async {
    if (changes.isEmpty) throw StateError('style patch is empty');
    final current = _patchState;
    if (expectedRevision > 0 && expectedRevision != current.revision) {
      throw StateError(
        'style revision conflict: expected $expectedRevision, current ${current.revision}',
      );
    }
    final snapshot = _snapshot(current.revision);
    await _patch.pushHistory(snapshot);
    try {
      final applied = await _applyChanges(current.patch, changes);
      final rules = _rulesFromChanges(changes, revision: current.revision + 1);
      final transaction = AmitiaStyleTransaction(
        id: _transactionId(current.revision + 1),
        source: _stringValue(changes['source']) ?? 'ai',
        revision: current.revision + 1,
        ruleIds: rules.map((rule) => rule.id).toList(growable: false),
        createdAt: DateTime.now().toUtc().toIso8601String(),
      );
      final nextDocument = AmitiaStyleDocument(
        revision: current.revision + 1,
        rules: <AmitiaStyleRule>[...current.rules, ...rules],
        transactions: <AmitiaStyleTransaction>[
          ...current.transactions,
          if (rules.isNotEmpty) transaction,
        ],
      );
      await _patch.replaceDocument(nextDocument, applied);
      return <String, dynamic>{
        'ok': true,
        'revision': nextDocument.revision,
        'previousRevision': current.revision,
        'applied': changes.keys.toList(growable: false),
        'ruleIds': rules.map((rule) => rule.id).toList(growable: false),
        'warnings': _warningsForRules(rules),
        'effective': _inspect(changes),
      };
    } catch (error, stackTrace) {
  print("[original-error] $error\n$stackTrace");
await _restoreSnapshot(snapshot);
      rethrow;
    }
  }

  Future<Map<String, dynamic>> _remove(
    int expectedRevision,
    Map<String, dynamic> changes,
  ) async {
    final current = _patchState;
    if (expectedRevision > 0 && expectedRevision != current.revision) {
      throw StateError(
        'style revision conflict: expected $expectedRevision, current ${current.revision}',
      );
    }
    if (!_hasRuleFilter(changes)) {
      throw StateError('style remove filter is empty');
    }
    final snapshot = _snapshot(current.revision);
    await _patch.pushHistory(snapshot);
    try {
      final result = _removeMatchingRules(current.rules, changes);
      final nextDocument = AmitiaStyleDocument(
        revision: current.revision + 1,
        rules: result.rules,
        transactions: current.transactions,
      );
      await _patch.replaceDocument(nextDocument, current.patch);
      return <String, dynamic>{
        'ok': true,
        'revision': nextDocument.revision,
        'previousRevision': current.revision,
        'removed': result.removedRuleIds,
        'changed': result.changedRuleIds,
        'effective': _inspect(changes),
      };
    } catch (error, stackTrace) {
  print("[original-error] $error\n$stackTrace");
await _restoreSnapshot(snapshot);
      rethrow;
    }
  }

  Future<Map<String, dynamic>> _reset(
    int expectedRevision,
    Map<String, dynamic> changes,
  ) async {
    final current = _patchState;
    if (expectedRevision > 0 && expectedRevision != current.revision) {
      throw StateError(
        'style revision conflict: expected $expectedRevision, current ${current.revision}',
      );
    }
    final snapshot = _snapshot(current.revision);
    await _patch.pushHistory(snapshot);
    try {
      if (_hasRuleFilter(changes)) {
        final result = _removeMatchingRules(current.rules, changes);
        final nextDocument = AmitiaStyleDocument(
          revision: current.revision + 1,
          rules: result.rules,
          transactions: current.transactions,
        );
        await _patch.replaceDocument(nextDocument, current.patch);
        return <String, dynamic>{
          'ok': true,
          'revision': nextDocument.revision,
          'previousRevision': current.revision,
          'reset': true,
          'removed': result.removedRuleIds,
          'changed': result.changedRuleIds,
          'effective': _inspect(changes),
        };
      }
      await Future.wait<void>(<Future<void>>[
        _appearance.reset(),
        _background.reset(),
      ]);
      final revision = current.revision + 1;
      await _patch.clear(revision);
      return <String, dynamic>{
        'ok': true,
        'revision': revision,
        'previousRevision': current.revision,
        'reset': true,
        'effective': _inspect(changes),
      };
    } catch (error, stackTrace) {
  print("[original-error] $error\n$stackTrace");
await _restoreSnapshot(snapshot);
      rethrow;
    }
  }

  Future<Map<String, dynamic>> _undo(int expectedRevision) async {
    final current = _patchState;
    if (expectedRevision > 0 && expectedRevision != current.revision) {
      throw StateError(
        'style revision conflict: expected $expectedRevision, current ${current.revision}',
      );
    }
    final snapshot = await _patch.popHistory();
    if (snapshot == null) throw StateError('no style revision to undo');
    try {
      await _restoreSnapshot(snapshot);
      final revision = current.revision + 1;
      final document = _patchState.document.copyWith(revision: revision);
      await _patch.replaceDocument(document, _patchState.patch);
      return <String, dynamic>{
        'ok': true,
        'revision': revision,
        'previousRevision': current.revision,
        'undo': true,
        'effective': _inspect(),
      };
    } catch (error, stackTrace) {
  print("[original-error] $error\n$stackTrace");
await _patch.pushHistory(snapshot);
      rethrow;
    }
  }

  Future<Map<String, dynamic>> _applyChanges(
    Map<String, dynamic> currentPatch,
    Map<String, dynamic> changes,
  ) async {
    var appearance = _appearanceState;
    var background = _backgroundState;
    final patch = _deepMerge(currentPatch, const <String, dynamic>{
      'schemaVersion': 1,
    });

    final themeMode = changes['themeMode'] ?? changes['preset'];
    if (themeMode != null) {
      appearance = appearance.copyWith(
        themeMode: switch (themeMode.toString()) {
          'light' => ThemeMode.light,
          'dark' => ThemeMode.dark,
          _ => ThemeMode.system,
        },
      );
    }
    if (changes.containsKey('accentColor')) {
      final accent = _color(changes['accentColor']);
      if (accent != null) {
        appearance = appearance.copyWith(
          customPalette: appearance.customPalette.copyWith(
            enabled: true,
            primary: accent,
          ),
        );
      }
    }
    if (changes['fontScale'] is num) {
      appearance = appearance.copyWith(
        fontScale: (changes['fontScale'] as num)
            .toDouble()
            .clamp(0.8, 1.4)
            .toDouble(),
      );
    }
    if (changes['cornerStyle'] is num) {
      appearance = appearance.copyWith(
        cornerStyleIndex: (changes['cornerStyle'] as num)
            .toInt()
            .clamp(0, 2)
            .toInt(),
      );
    }
    if (changes['dynamicEffect'] is bool) {
      appearance = appearance.copyWith(
        dynamicEffect: changes['dynamicEffect'] as bool,
      );
    }
    if (changes['reduceAnimation'] is bool) {
      appearance = appearance.copyWith(
        reduceAnimation: changes['reduceAnimation'] as bool,
      );
    }
    if (changes['customPalette'] case final Map<dynamic, dynamic> raw) {
      final custom = raw.cast<String, dynamic>();
      final current = appearance.customPalette;
      appearance = appearance.copyWith(
        customPalette: current.copyWith(
          enabled: custom['enabled'] is bool
              ? custom['enabled'] as bool
              : current.enabled,
          primary: _color(custom['primary']) ?? current.primary,
          secondary: _color(custom['secondary']) ?? current.secondary,
          text: _color(custom['text']) ?? current.text,
          textMode: ThemeTextMode.values.firstWhere(
            (item) => item.name == custom['textMode'],
            orElse: () => current.textMode,
          ),
        ),
      );
    }
    await _appearance.replace(appearance);

    if (changes['background'] case final Map<dynamic, dynamic> raw) {
      final backgroundChanges = raw.cast<String, dynamic>();
      final assetUri = backgroundChanges['assetUri']?.toString().trim() ?? '';
      if (assetUri.isNotEmpty) {
        final artifacts = await _ref.read(artifactServiceProvider.future);
        final resolved = await artifacts.resolveMediaUri(assetUri, embed: true);
        final bytes = resolved.data?.contentAsBytes();
        if (bytes != null && bytes.isNotEmpty) {
          final mode = backgroundChanges['mode']?.toString();
          final kind = mode == 'video'
              ? BackgroundMediaKind.video
              : BackgroundMediaKind.image;
          final name =
              backgroundChanges['name']?.toString().trim().isNotEmpty == true
              ? backgroundChanges['name'].toString().trim()
              : kind == BackgroundMediaKind.video
              ? 'background.mp4'
              : 'background.png';
          await _background.importBytes(bytes, name, kind, sourceUri: assetUri);
          background = _backgroundState;
        }
      }
      background = _applyBackground(background, backgroundChanges);
      await _background.replace(background);
    }

    for (final key in const <String>[
      'palette',
      'typography',
      'layout',
      'icons',
      'components',
      'componentVariants',
    ]) {
      final value = changes[key];
      if (value is Map) {
        patch[key] = _deepMerge(
          patch[key] is Map
              ? (patch[key] as Map).cast<String, dynamic>()
              : const <String, dynamic>{},
          value.cast<String, dynamic>(),
        );
      }
    }
    return patch;
  }

  BackgroundPreferences _applyBackground(
    BackgroundPreferences current,
    Map<String, dynamic> raw,
  ) {
    final mode = BackgroundDisplayMode.values.firstWhere(
      (item) => item.name == raw['mode'],
      orElse: () => raw.containsKey('assetUri') && raw['assetUri'] != null
          ? BackgroundDisplayMode.image
          : current.mode,
    );
    final kind = mode == BackgroundDisplayMode.video
        ? BackgroundMediaKind.video
        : BackgroundMediaKind.image;
    final hasAsset =
        raw['assetUri'] is String &&
        (raw['assetUri'] as String).trim().isNotEmpty;
    final sourceUri = hasAsset
        ? (raw['assetUri'] as String).trim()
        : current.sourceUri;
    final assetAlreadyLocal =
        hasAsset && current.sourceUri == sourceUri && current.path.isNotEmpty;
    final hasOwnedMedia =
        mode == BackgroundDisplayMode.image ||
        mode == BackgroundDisplayMode.video;
    return current.copyWith(
      enabled: raw['enabled'] is bool
          ? raw['enabled'] as bool
          : mode != BackgroundDisplayMode.defaultMode,
      mode: mode,
      kind: kind,
      name: raw['name']?.toString() ?? current.name,
      color: _color(raw['color']) ?? current.color,
      gradientStart: _color(raw['gradientStart']) ?? current.gradientStart,
      gradientEnd: _color(raw['gradientEnd']) ?? current.gradientEnd,
      gradientAngle: raw['gradientAngle'] is num
          ? (raw['gradientAngle'] as num).toDouble()
          : current.gradientAngle,
      opacity: raw['opacity'] is num
          ? (raw['opacity'] as num).toDouble()
          : current.opacity,
      blurEnabled: raw['blurEnabled'] is bool
          ? raw['blurEnabled'] as bool
          : current.blurEnabled,
      blurRadius: raw['blurRadius'] is num
          ? (raw['blurRadius'] as num).toDouble()
          : current.blurRadius,
      sourceUri: sourceUri,
      path: hasOwnedMedia && hasAsset && !assetAlreadyLocal ? '' : current.path,
      fit: raw['fit']?.toString() ?? current.fit,
      overlayColor: _color(raw['overlayColor']) ?? current.overlayColor,
      overlayOpacity: raw['overlayOpacity'] is num
          ? (raw['overlayOpacity'] as num).toDouble()
          : current.overlayOpacity,
    );
  }

  Map<String, dynamic> _snapshot(int revision) => <String, dynamic>{
    'revision': revision,
    'appearance': _appearanceState.toMap(),
    'background': _backgroundState.toMap(),
    'patch': _patchState.patch,
    'document': _patchState.document.toMap(),
    'createdAt': DateTime.now().toUtc().toIso8601String(),
  };

  Future<void> _restoreSnapshot(Map<String, dynamic> snapshot) async {
    final appearance = snapshot['appearance'];
    final background = snapshot['background'];
    final rawDocument = snapshot['document'];
    await Future.wait<void>(<Future<void>>[
      if (appearance is Map)
        _appearance.replace(
          AppearancePreferences.fromMap(appearance.cast<String, dynamic>()),
        ),
      if (background is Map)
        _background.replace(
          BackgroundPreferences.fromMap(background.cast<String, dynamic>()),
        ),
    ]);
    if (rawDocument is Map) {
      final document = AmitiaStyleDocument.fromMap(
        rawDocument.cast<String, dynamic>(),
      );
      await _patch.replaceDocument(document, _readSnapshotPatch(snapshot));
    } else {
      await _patch.replace(
        _readSnapshotPatch(snapshot),
        (snapshot['revision'] as num?)?.toInt() ?? 0,
        rules: const <AmitiaStyleRule>[],
      );
    }
  }

  Map<String, dynamic> _readSnapshotPatch(Map<String, dynamic> snapshot) {
    final raw = snapshot['patch'];
    return raw is Map ? raw.cast<String, dynamic>() : <String, dynamic>{};
  }

  List<AmitiaStyleRule> _rulesFromChanges(
    Map<String, dynamic> changes, {
    required int revision,
  }) {
    final rawTargets = changes['targets'];
    final targets = <Map<String, dynamic>>[];
    if (rawTargets is List) {
      for (final item in rawTargets) {
        if (item is Map) targets.add(item.cast<String, dynamic>());
      }
    }
    if (changes['target'] != null) targets.add(changes);
    final result = <AmitiaStyleRule>[];
    final source = _stringValue(changes['source']) ?? 'ai';
    final defaultScope = _scopeFrom(changes, fallbackPage: _currentPage);
    for (var index = 0; index < targets.length; index++) {
      final raw = targets[index];
      final target = _stringValue(raw['target']);
      if (target == null || target.isEmpty) {
        throw StateError('style target is required');
      }
      final rawProperties = raw['properties'] ?? raw['set'] ?? raw['changes'];
      final properties = rawProperties is Map
          ? rawProperties.cast<String, dynamic>()
          : const <String, dynamic>{};
      if (properties.isEmpty) continue;
      final scope = raw['scope'] == null
          ? defaultScope
          : _scopeFrom(raw, fallbackPage: defaultScope.page);
      result.add(
        AmitiaStyleRule(
          id:
              _stringValue(raw['id']) ??
              'rule-$revision-${DateTime.now().microsecondsSinceEpoch}-$index',
          source: _stringValue(raw['source']) ?? source,
          scope: scope,
          target: target,
          states: _statesFrom(raw['states']),
          properties: properties,
          transactionId: _stringValue(raw['transactionId']),
        ),
      );
    }
    return result;
  }

  _RemoveResult _removeMatchingRules(
    List<AmitiaStyleRule> rules,
    Map<String, dynamic> filters,
  ) {
    final scope = _scopeFrom(filters, fallbackPage: _currentPage);
    final target = _stringValue(filters['target']);
    final requestedStates = filters.containsKey('states')
        ? _statesFrom(filters['states']).toSet()
        : <String>{};
    final requestedProperties = _propertiesFrom(filters['properties']);
    final transactionId = _stringValue(filters['transactionId']);
    final source = _stringValue(filters['source']) ?? 'ai';
    final result = <AmitiaStyleRule>[];
    final removed = <String>[];
    final changed = <String>[];
    for (final rule in rules) {
      final sourceMatches = filters.containsKey('source')
          ? rule.source == source
          : rule.source == 'ai';
      final matches =
          (target == null || target == rule.target) &&
          (transactionId == null || transactionId == rule.transactionId) &&
          sourceMatches &&
          _ruleScopeMatches(rule.scope, scope) &&
          (requestedStates.isEmpty ||
              rule.states.any(requestedStates.contains));
      if (!matches) {
        result.add(rule);
        continue;
      }
      final split = _splitRule(
        rule,
        requestedStates: requestedStates,
        requestedProperties: requestedProperties,
      );
      if (split.removed) removed.add(rule.id);
      if (split.changed) changed.add(rule.id);
      result.addAll(split.rules);
    }
    return _RemoveResult(
      rules: result,
      removedRuleIds: removed,
      changedRuleIds: changed,
    );
  }

  _SplitResult _splitRule(
    AmitiaStyleRule rule, {
    required Set<String> requestedStates,
    required Set<String> requestedProperties,
  }) {
    final affectedStates = requestedStates.isEmpty
        ? rule.states.toSet()
        : rule.states.where(requestedStates.contains).toSet();
    if (affectedStates.isEmpty) {
      return _SplitResult(rules: <AmitiaStyleRule>[rule]);
    }
    final allProperties = requestedProperties.isEmpty
        ? rule.properties.keys.toSet()
        : rule.properties.keys.where(requestedProperties.contains).toSet();
    if (allProperties.isEmpty) {
      return _SplitResult(rules: <AmitiaStyleRule>[rule]);
    }
    final remainingProperties = <String, dynamic>{
      for (final entry in rule.properties.entries)
        if (!allProperties.contains(entry.key)) entry.key: entry.value,
    };
    final unaffectedStates = rule.states
        .where((state) => !affectedStates.contains(state))
        .toList(growable: false);
    final rebuilt = <AmitiaStyleRule>[];
    if (unaffectedStates.isNotEmpty) {
      rebuilt.add(
        AmitiaStyleRule(
          id: rule.id,
          source: rule.source,
          scope: rule.scope,
          target: rule.target,
          states: unaffectedStates,
          properties: rule.properties,
          transactionId: rule.transactionId,
        ),
      );
    }
    if (remainingProperties.isNotEmpty) {
      rebuilt.add(
        AmitiaStyleRule(
          id: '${rule.id}-${DateTime.now().microsecondsSinceEpoch}',
          source: rule.source,
          scope: rule.scope,
          target: rule.target,
          states: affectedStates.toList(growable: false),
          properties: remainingProperties,
          transactionId: rule.transactionId,
        ),
      );
    }
    return _SplitResult(
      rules: rebuilt,
      changed: true,
      removed: rebuilt.isEmpty,
    );
  }

  bool _ruleScopeMatches(
    AmitiaStyleScopeData ruleScope,
    AmitiaStyleScopeData filter,
  ) {
    return _filterScopeValue(filter.platform, ruleScope.platform) &&
        _filterScopeValue(filter.page, ruleScope.page) &&
        _filterScopeValue(filter.surface, ruleScope.surface) &&
        _filterScopeValue(filter.instance, ruleScope.instance);
  }

  bool _filterScopeValue(String? filter, String? value) {
    if (filter == null || filter == '*' || filter == 'all') return true;
    return value == filter;
  }

  AmitiaStyleScopeData _scopeFrom(
    Map<String, dynamic> raw, {
    String? fallbackPage,
  }) {
    final rawScope = raw['scope'];
    final scope = rawScope is Map
        ? rawScope.cast<String, dynamic>()
        : const <String, dynamic>{};
    return AmitiaStyleScopeData(
      platform:
          _stringValue(scope['platform']) ??
          _stringValue(raw['platform']) ??
          'mobile',
      page:
          amitiaStylePageId(
            _stringValue(scope['page']) ??
                _stringValue(raw['page']) ??
                fallbackPage,
          ),
      surface: _stringValue(scope['surface']) ?? _stringValue(raw['surface']),
      instance:
          _stringValue(scope['instance']) ?? _stringValue(raw['instance']),
    );
  }

  List<String> _statesFrom(Object? raw) {
    final values = raw is List
        ? raw
        : raw == null
        ? const <Object?>[]
        : <Object?>[raw];
    final states = values
        .map((item) => item.toString().trim().toLowerCase())
        .where((item) => item.isNotEmpty)
        .toList(growable: false);
    return states.isEmpty ? const <String>['default'] : states;
  }

  Set<String> _propertiesFrom(Object? raw) {
    if (raw is List) {
      return raw
          .map((item) => item.toString().trim())
          .where((item) => item.isNotEmpty)
          .toSet();
    }
    if (raw is Map) {
      return raw.keys.map((item) => item.toString()).toSet();
    }
    return <String>{};
  }

  bool _hasRuleFilter(Map<String, dynamic> changes) {
    if (changes.containsKey('scope') ||
        changes.containsKey('target') ||
        changes.containsKey('states') ||
        changes.containsKey('properties') ||
        changes.containsKey('transactionId') ||
        changes.containsKey('source')) {
      return true;
    }
    final scope = changes['scope'];
    return scope is Map && scope.isNotEmpty;
  }

  List<String> _warningsForRules(List<AmitiaStyleRule> rules) {
    final warnings = <String>[];
    for (final rule in rules) {
      for (final key in rule.properties.keys) {
        if (!_knownStyleProperties.contains(key)) {
          warnings.add('${rule.target}: unsupported property $key');
        }
      }
    }
    return warnings;
  }

  Color? _color(Object? raw) {
    final value = raw?.toString().trim().replaceFirst('#', '') ?? '';
    if (value.length == 6) {
      return Color(0xFF000000 | int.parse(value, radix: 16));
    }
    if (value.length == 8) {
      return Color(int.parse(value, radix: 16));
    }
    return null;
  }

  String? _stringValue(Object? value) {
    final text = value?.toString().trim();
    return text == null || text.isEmpty ? null : text;
  }

  String _transactionId(int revision) =>
      'transaction-$revision-${DateTime.now().microsecondsSinceEpoch}';

  Map<String, dynamic> _deepMerge(
    Map<String, dynamic> base,
    Map<String, dynamic> overlay,
  ) {
    final merged = base.map((key, value) => MapEntry(key, value));
    for (final entry in overlay.entries) {
      final current = merged[entry.key];
      if (current is Map && entry.value is Map) {
        merged[entry.key] = _deepMerge(
          current.cast<String, dynamic>(),
          (entry.value as Map).cast<String, dynamic>(),
        );
      } else {
        merged[entry.key] = entry.value;
      }
    }
    return merged;
  }
}

class _RemoveResult {
  final List<AmitiaStyleRule> rules;
  final List<String> removedRuleIds;
  final List<String> changedRuleIds;

  const _RemoveResult({
    required this.rules,
    required this.removedRuleIds,
    required this.changedRuleIds,
  });
}

class _SplitResult {
  final List<AmitiaStyleRule> rules;
  final bool changed;
  final bool removed;

  const _SplitResult({
    required this.rules,
    this.changed = false,
    this.removed = false,
  });
}

const Set<String> _knownStyleProperties = <String>{
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

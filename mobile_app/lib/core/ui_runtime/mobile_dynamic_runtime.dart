import 'ui_provider.dart';

class MobileDynamicRuntime {
  const MobileDynamicRuntime._();

  static UISlotSnapshotEntry? slotDefinition({
    required Map<String, dynamic>? sessionState,
    required String slotId,
  }) {
    final id = slotId.trim();
    if (sessionState == null || id.isEmpty) return null;
    final revision = (sessionState['revision'] as num?)?.toInt() ?? 0;
    for (final active in _activePackageVersions(sessionState)) {
      final contributions =
          (active.definition['contributions'] as List?) ?? const <dynamic>[];
      for (final rawContribution in contributions.whereType<Map>()) {
        final contribution = rawContribution.cast<String, dynamic>();
        final parentSlotId =
            (contribution['slotId'] ?? '').toString().trim();
        final children =
            (contribution['children'] as List?) ?? const <dynamic>[];
        for (final rawChild in children.whereType<Map>()) {
          final child = rawChild.cast<String, dynamic>();
          final childId = (child['slotId'] ?? '').toString().trim();
          if (childId != id) continue;
          return UISlotSnapshotEntry(
            slotId: childId,
            contractVersion:
                _int(child['contractVersion']) ?? 1,
            supportedKinds: _strings(child['supportedKinds']),
            kind: _requiredText(child['kind'], 'kind'),
            layout: (child['layout'] ?? 'stack').toString(),
            fallbackPolicy:
                (child['fallbackPolicy'] ?? 'empty')
                    .toString(),
            ownerExtension: 'client-runtime:${active.packageId}',
            parentSlotId:
                (child['parentSlotId'] ?? parentSlotId)
                    .toString(),
            declarationEpoch: revision,
            scope: (child['scope'] ?? 'session').toString(),
            dynamicSlot: true,
            contributions: const <UIContributionSnapshotEntry>[],
          );
        }
      }
    }
    return null;
  }

  static List<UIContributionSnapshotEntry> slotContributions({
    required UIProviderSnapshot snapshot,
    required Map<String, dynamic>? sessionState,
    required String slotId,
  }) {
    if (sessionState == null || slotId.trim().isEmpty) {
      return const <UIContributionSnapshotEntry>[];
    }
    final result = <UIContributionSnapshotEntry>[];
    for (final active in _activePackageVersions(sessionState)) {
      final packageId = active.packageId;
      final version = active.version;
      final rows =
          (active.definition['contributions'] as List?) ?? const <dynamic>[];
      for (final raw in rows.whereType<Map>()) {
        final row = raw.cast<String, dynamic>();
        final targetSlot = (row['slotId'] ?? '').toString().trim();
        if (targetSlot != slotId) continue;
        final sourceExtensionId =
            (row['sourceExtensionId'] ?? '').toString().trim();
        final sourceContributionId =
            (row['sourceContributionId'] ?? '').toString().trim();
        final key = (row['key'] ?? sourceContributionId).toString().trim();
        if (sourceExtensionId.isEmpty ||
            sourceContributionId.isEmpty ||
            key.isEmpty) {
          continue;
        }
        final source = _findContribution(
          snapshot,
          sourceExtensionId,
          sourceContributionId,
        );
        final entryKey = _text(row['entryKey']) ?? key;
        final cellId = _text(row['cellId']) ?? key;
        result.add(
          UIContributionSnapshotEntry(
            contributionId: 'client-runtime:$packageId@$version:$key',
            sourceContributionId: sourceContributionId,
            extensionId: sourceExtensionId,
            moduleId: source?.moduleId ?? sourceExtensionId,
            kind: (row['kind'] ?? source?.kind ?? 'panel').toString(),
            slotId: targetSlot,
            contractVersion:
                snapshot.slot(targetSlot)?.contractVersion ??
                source?.contractVersion ??
                1,
            entryType: source?.entryType.isNotEmpty == true
                ? source!.entryType
                : 'schema_renderer',
            entryPath: source?.entryPath ?? '',
            schemaPath: source?.schemaPath,
            permissions: source?.permissions ?? const <String>[],
            dataContract: source?.dataContract ?? const <String, dynamic>{},
            visibility: source?.visibility ?? const <String, dynamic>{},
            ordering: _int(row['ordering']) ?? source?.ordering ?? 0,
            priority: _int(row['priority']) ?? source?.priority ?? 0,
            entryKey: entryKey,
            cellId: cellId,
            matched: row['matched'],
            matchRules: _mapList(row['match']),
            runtimePackageId: packageId,
            runtimePackageVersion: version,
          ),
        );
      }
    }
    result.sort(_compareStable);
    return result;
  }

  static List<UIContributionSnapshotEntry> conversationNodeContributions({
    required UIProviderSnapshot snapshot,
    required Map<String, dynamic>? sessionState,
  }) {
    if (sessionState == null) return const <UIContributionSnapshotEntry>[];
    final result = <UIContributionSnapshotEntry>[];
    for (final active in _activePackageVersions(sessionState)) {
      final packageId = active.packageId;
      final version = active.version;
      final rows =
          (active.definition['conversationNodes'] as List?) ??
          const <dynamic>[];
      for (final raw in rows.whereType<Map>()) {
        final row = raw.cast<String, dynamic>();
        final sourceExtensionId =
            (row['sourceExtensionId'] ?? '').toString().trim();
        final sourceContributionId =
            (row['sourceContributionId'] ?? '').toString().trim();
        final key = (row['key'] ?? sourceContributionId).toString().trim();
        final projection = (row['projection'] as Map?)?.cast<String, dynamic>();
        if (sourceExtensionId.isEmpty ||
            sourceContributionId.isEmpty ||
            key.isEmpty ||
            projection == null) {
          continue;
        }
        final source = _findContribution(
          snapshot,
          sourceExtensionId,
          sourceContributionId,
        );
        result.add(
          UIContributionSnapshotEntry(
            contributionId:
                'client-runtime:$packageId@$version:conversation:$key',
            sourceContributionId: sourceContributionId,
            extensionId: sourceExtensionId,
            moduleId: source?.moduleId ?? sourceExtensionId,
            kind: 'conversation_node',
            slotId: 'chat.conversation.node',
            contractVersion:
                snapshot.slot('chat.conversation.node')?.contractVersion ?? 1,
            entryType: source?.entryType.isNotEmpty == true
                ? source!.entryType
                : 'schema_renderer',
            entryPath: source?.entryPath ?? '',
            schemaPath: source?.schemaPath,
            permissions: source?.permissions ?? const <String>[],
            dataContract: <String, dynamic>{
              'projection': <String, dynamic>{
                'event_types': projection['eventTypes'],
                'start_events': projection['startEvents'],
                'end_events': projection['endEvents'],
                'node_type': projection['nodeType'],
                'key_path': projection['keyPath'],
                'title_path': projection['titlePath'],
                'max_events': projection['maxEvents'],
              },
            },
            visibility: source?.visibility ?? const <String, dynamic>{},
            ordering: _int(row['ordering']) ?? source?.ordering ?? 0,
            priority: _int(row['priority']) ?? source?.priority ?? 0,
            entryKey: key,
            cellId: key,
            runtimePackageId: packageId,
            runtimePackageVersion: version,
          ),
        );
      }
    }
    result.sort(_compareStable);
    return result;
  }

  /// Merges server and client-runtime contributions with the same dispatch
  /// semantics as the desktop UI runtime.
  static List<UIContributionSnapshotEntry> resolveSlot({
    required UISlotSnapshotEntry slot,
    required List<UIContributionSnapshotEntry> server,
    required List<UIContributionSnapshotEntry> dynamic,
    Map<String, dynamic> owner = const <String, dynamic>{},
    String? dispatchKey,
    String? listOnly,
  }) {
    final supportedKinds = slot.supportedKinds.toSet();
    final merged = <UIContributionSnapshotEntry>[...server, ...dynamic]
        .where((entry) {
          return supportedKinds.isEmpty || supportedKinds.contains(entry.kind);
        })
        .toList(growable: false);
    if (merged.isEmpty) return const <UIContributionSnapshotEntry>[];

    final kind = slot.kind.trim();
    if (kind.isEmpty) {
      return const <UIContributionSnapshotEntry>[];
    }

    switch (kind) {
      case 'single':
        return _pickShadowWinner(merged, 'single cell');
      case 'list':
        return _dispatchList(merged, listOnly);
      case 'keyed':
        return _dispatchKeyed(merged, dispatchKey);
      case 'chain':
        return _dispatchChain(merged, owner);
      default:
        return const <UIContributionSnapshotEntry>[];
    }
  }

  static List<UIContributionSnapshotEntry> _dispatchList(
    List<UIContributionSnapshotEntry> items,
    String? listOnly,
  ) {
    final filter = listOnly?.trim() ?? '';
    final cells = <String, List<UIContributionSnapshotEntry>>{};
    for (final item in items) {
      final cell = _identity(item.cellId, item.contributionId);
      if (filter.isNotEmpty && cell != filter) continue;
      cells.putIfAbsent(cell, () => <UIContributionSnapshotEntry>[]).add(item);
    }
    final winners = <UIContributionSnapshotEntry>[];
    for (final entry in cells.entries) {
      final winner = _shadowWinner(entry.value, 'list cell ${entry.key}');
      if (winner != null) winners.add(winner);
    }
    winners.sort(_compareListDisplay);
    return winners;
  }

  static List<UIContributionSnapshotEntry> _dispatchKeyed(
    List<UIContributionSnapshotEntry> items,
    String? dispatchKey,
  ) {
    final key = dispatchKey?.trim() ?? '';
    if (key.isEmpty) return const <UIContributionSnapshotEntry>[];
    return _pickShadowWinner(
      items
          .where((item) => _identity(item.entryKey, item.contributionId) == key)
          .toList(growable: false),
      'keyed cell $key',
    );
  }

  static List<UIContributionSnapshotEntry> _dispatchChain(
    List<UIContributionSnapshotEntry> items,
    Map<String, dynamic> owner,
  ) {
    final matched = items
        .where((item) {
          if (item.runtimePackageId?.isNotEmpty != true) return true;
          return _matchesDeclarativeOwner(owner, item.matchRules);
        })
        .toList(growable: false);
    if (matched.isEmpty) return const <UIContributionSnapshotEntry>[];
    matched.sort(_compareChain);
    return <UIContributionSnapshotEntry>[matched.first];
  }

  static List<UIContributionSnapshotEntry> _pickShadowWinner(
    List<UIContributionSnapshotEntry> items,
    String cell,
  ) {
    final winner = _shadowWinner(items, cell);
    return winner == null
        ? const <UIContributionSnapshotEntry>[]
        : <UIContributionSnapshotEntry>[winner];
  }

  static UIContributionSnapshotEntry? _shadowWinner(
    List<UIContributionSnapshotEntry> items,
    String cell,
  ) {
    if (items.isEmpty) return null;
    final ordered = [...items]..sort(_compareShadow);
    final first = ordered.first;
    final tied = ordered
        .where((item) => item.priority == first.priority)
        .toList();
    if (tied.length > 1) {
      throw StateError(
        'UI slot $cell has multiple entries at strict priority '
        '${first.priority}: ${tied.map((item) => item.contributionId).join(', ')}',
      );
    }
    return first;
  }

  static UIContributionSnapshotEntry? _findContribution(
    UIProviderSnapshot snapshot,
    String extensionId,
    String contributionId,
  ) {
    for (final slot in snapshot.slots) {
      for (final contribution in slot.contributions) {
        if (contribution.extensionId == extensionId &&
            (contribution.contributionId == contributionId ||
                contribution.sourceContributionId == contributionId)) {
          return contribution;
        }
      }
    }
    return null;
  }

  static Iterable<_ActivePackageVersion> _activePackageVersions(
    Map<String, dynamic> sessionState,
  ) sync* {
    final packages = (sessionState['packages'] as List?) ?? const <dynamic>[];
    for (final rawPackage in packages.whereType<Map>()) {
      final package = rawPackage.cast<String, dynamic>();
      if (package['running'] != true) continue;
      final packageId = (package['id'] ?? '').toString().trim();
      final transition = (package['transitionState'] ?? '')
          .toString()
          .trim()
          .toLowerCase();
      final targetVersion =
          (package['targetVersion'] ?? '').toString().trim();
      final currentVersion =
          (package['activeVersion'] ?? '').toString().trim();
      final activeVersion =
          (transition == 'starting' || transition == 'awaiting_client') &&
              targetVersion.isNotEmpty
          ? targetVersion
          : currentVersion;
      if (packageId.isEmpty || activeVersion.isEmpty) continue;
      final versions = (package['versions'] as List?) ?? const <dynamic>[];
      for (final rawVersion in versions.whereType<Map>()) {
        final definition = rawVersion.cast<String, dynamic>();
        final version = (definition['version'] ?? '').toString().trim();
        if (version == activeVersion) {
          yield _ActivePackageVersion(packageId, version, definition);
          break;
        }
      }
    }
  }
}

class _ActivePackageVersion {
  const _ActivePackageVersion(this.packageId, this.version, this.definition);

  final String packageId;
  final String version;
  final Map<String, dynamic> definition;
}

bool _matchesDeclarativeOwner(
  Map<String, dynamic> owner,
  List<Map<String, dynamic>> rules,
) {
  if (rules.isEmpty) return true;
  for (final rule in rules) {
    final field = (rule['field'] ?? '').toString().trim();
    if (field.isEmpty) return false;
    final actual = _lookup(owner, field);
    final expected = rule['value'];
    final operator = (rule['operator'] ?? 'eq').toString();
    if (operator == 'eq') {
      if (actual != expected) return false;
    } else if (operator == 'ne') {
      if (actual == expected) return false;
    } else if (operator == 'in') {
      if (expected is! List || !expected.contains(actual)) return false;
    } else if (operator == 'not_in') {
      if (expected is! List || expected.contains(actual)) return false;
    } else if (operator == 'contains') {
      if (actual is! String ||
          expected is! String ||
          !actual.contains(expected)) {
        return false;
      }
    } else if (operator == 'exists') {
      if (actual == null) return false;
    } else {
      return false;
    }
  }
  return true;
}

dynamic _lookup(Map<String, dynamic> source, String path) {
  dynamic current = source;
  for (final segment in path.split('.')) {
    if (current is! Map || !current.containsKey(segment)) return null;
    current = current[segment];
  }
  return current;
}

List<Map<String, dynamic>> _mapList(dynamic value) {
  if (value is! List) return const <Map<String, dynamic>>[];
  return value
      .whereType<Map>()
      .map((item) => item.cast<String, dynamic>())
      .toList(growable: false);
}

List<String> _strings(dynamic value) {
  if (value is! List) return const <String>[];
  return value
      .map((item) => item.toString().trim())
      .where((item) => item.isNotEmpty)
      .toList(growable: false);
}

String? _text(dynamic value) {
  final normalized = value?.toString().trim() ?? '';
  return normalized.isEmpty ? null : normalized;
}

String _requiredText(dynamic value, String field) {
  final text = (value ?? '').toString().trim();
  if (text.isEmpty) throw FormatException('UI slot $field is required');
  return text;
}

String _identity(String value, String fallback) {
  final normalized = value.trim();
  return normalized.isEmpty ? fallback : normalized;
}

int? _int(dynamic value) {
  if (value is int) return value;
  if (value is num) return value.toInt();
  return int.tryParse(value?.toString() ?? '');
}

int _compareAssembly(
  UIContributionSnapshotEntry a,
  UIContributionSnapshotEntry b,
) {
  final aClient = a.runtimePackageId?.isNotEmpty == true;
  final bClient = b.runtimePackageId?.isNotEmpty == true;
  if (aClient != bClient) return aClient ? 1 : -1;
  return a.contributionId.compareTo(b.contributionId);
}

int _compareShadow(
  UIContributionSnapshotEntry a,
  UIContributionSnapshotEntry b,
) {
  final priority = a.priority.compareTo(b.priority);
  if (priority != 0) return priority;
  final ordering = a.ordering.compareTo(b.ordering);
  if (ordering != 0) return ordering;
  return _compareAssembly(a, b);
}

int _compareChain(
  UIContributionSnapshotEntry a,
  UIContributionSnapshotEntry b,
) {
  final priority = a.priority.compareTo(b.priority);
  return priority != 0 ? priority : _compareAssembly(a, b);
}

int _compareListDisplay(
  UIContributionSnapshotEntry a,
  UIContributionSnapshotEntry b,
) {
  final ordering = a.ordering.compareTo(b.ordering);
  return ordering != 0 ? ordering : _compareAssembly(a, b);
}

int _compareStable(
  UIContributionSnapshotEntry a,
  UIContributionSnapshotEntry b,
) {
  final ordering = a.ordering.compareTo(b.ordering);
  if (ordering != 0) return ordering;
  final priority = b.priority.compareTo(a.priority);
  if (priority != 0) return priority;
  return a.contributionId.compareTo(b.contributionId);
}

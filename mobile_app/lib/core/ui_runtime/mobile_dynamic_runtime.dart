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
      final contributions = (active.definition['contributions'] as List?) ?? const <dynamic>[];
      for (final rawContribution in contributions.whereType<Map>()) {
        final contribution = rawContribution.cast<String, dynamic>();
        final parentSlotId = (contribution['slotId'] ?? contribution['slot_id'] ?? '').toString().trim();
        final children = (contribution['children'] as List?) ?? const <dynamic>[];
        for (final rawChild in children.whereType<Map>()) {
          final child = rawChild.cast<String, dynamic>();
          final childId = (child['slotId'] ?? child['slot_id'] ?? '').toString().trim();
          if (childId != id) continue;
          return UISlotSnapshotEntry(
            slotId: childId,
            contractVersion: _int(child['contractVersion'] ?? child['contract_version']) ?? 1,
            multiplicity: (child['multiplicity'] ?? 'ordered_multiple').toString(),
            layout: (child['layout'] ?? 'stack').toString(),
            fallbackPolicy: (child['fallbackPolicy'] ?? child['fallback_policy'] ?? 'empty').toString(),
            ownerExtension: 'client-runtime:${active.packageId}',
            parentSlotId: (child['parentSlotId'] ?? child['parent_slot_id'] ?? parentSlotId).toString(),
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
      final rows = (active.definition['contributions'] as List?) ?? const <dynamic>[];
      for (final raw in rows.whereType<Map>()) {
        final row = raw.cast<String, dynamic>();
        final targetSlot = (row['slotId'] ?? row['slot_id'] ?? '').toString().trim();
        if (targetSlot != slotId) continue;
        final sourceExtensionId = (row['sourceExtensionId'] ?? row['source_extension_id'] ?? '').toString().trim();
        final sourceContributionId = (row['sourceContributionId'] ?? row['source_contribution_id'] ?? '').toString().trim();
        final key = (row['key'] ?? sourceContributionId).toString().trim();
        if (sourceExtensionId.isEmpty || sourceContributionId.isEmpty || key.isEmpty) continue;
        final source = _findContribution(snapshot, sourceExtensionId, sourceContributionId);
        result.add(UIContributionSnapshotEntry(
          contributionId: 'client-runtime:$packageId@$version:$key',
          sourceContributionId: sourceContributionId,
          extensionId: sourceExtensionId,
          moduleId: source?.moduleId ?? sourceExtensionId,
          kind: (row['kind'] ?? source?.kind ?? 'panel').toString(),
          slotId: targetSlot,
          contractVersion: snapshot.slot(targetSlot)?.contractVersion ?? source?.contractVersion ?? 1,
          entryType: source?.entryType.isNotEmpty == true ? source!.entryType : 'schema_renderer',
          entryPath: source?.entryPath ?? '',
          schemaPath: source?.schemaPath,
          permissions: source?.permissions ?? const <String>[],
          dataContract: source?.dataContract ?? const <String, dynamic>{},
          visibility: source?.visibility ?? const <String, dynamic>{},
          ordering: _int(row['ordering']) ?? source?.ordering ?? 0,
          priority: _int(row['priority']) ?? source?.priority ?? 0,
          runtimePackageId: packageId,
          runtimePackageVersion: version,
        ));
      }
    }
    result.sort(_compareContribution);
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
      final rows = (active.definition['conversationNodes'] as List?) ?? const <dynamic>[];
      for (final raw in rows.whereType<Map>()) {
        final row = raw.cast<String, dynamic>();
        final sourceExtensionId = (row['sourceExtensionId'] ?? row['source_extension_id'] ?? '').toString().trim();
        final sourceContributionId = (row['sourceContributionId'] ?? row['source_contribution_id'] ?? '').toString().trim();
        final key = (row['key'] ?? sourceContributionId).toString().trim();
        final projection = (row['projection'] as Map?)?.cast<String, dynamic>();
        if (sourceExtensionId.isEmpty || sourceContributionId.isEmpty || key.isEmpty || projection == null) continue;
        final source = _findContribution(snapshot, sourceExtensionId, sourceContributionId);
        result.add(UIContributionSnapshotEntry(
          contributionId: 'client-runtime:$packageId@$version:conversation:$key',
          sourceContributionId: sourceContributionId,
          extensionId: sourceExtensionId,
          moduleId: source?.moduleId ?? sourceExtensionId,
          kind: 'conversation_node',
          slotId: 'chat.conversation.node',
          contractVersion: snapshot.slot('chat.conversation.node')?.contractVersion ?? 1,
          entryType: source?.entryType.isNotEmpty == true ? source!.entryType : 'schema_renderer',
          entryPath: source?.entryPath ?? '',
          schemaPath: source?.schemaPath,
          permissions: source?.permissions ?? const <String>[],
          dataContract: <String, dynamic>{'projection': projection},
          visibility: source?.visibility ?? const <String, dynamic>{},
          ordering: _int(row['ordering']) ?? source?.ordering ?? 0,
          priority: _int(row['priority']) ?? source?.priority ?? 0,
          runtimePackageId: packageId,
          runtimePackageVersion: version,
        ));
      }
    }
    result.sort(_compareContribution);
    return result;
  }

  static List<UIContributionSnapshotEntry> resolveSlot({
    required UISlotSnapshotEntry slot,
    required List<UIContributionSnapshotEntry> server,
    required List<UIContributionSnapshotEntry> dynamic,
  }) {
    final merged = <UIContributionSnapshotEntry>[...server, ...dynamic]..sort(_compareContribution);
    if (merged.isEmpty) return const <UIContributionSnapshotEntry>[];
    switch (slot.multiplicity) {
      case 'single':
      case 'exclusive':
        merged.sort(_compareContribution);
        return <UIContributionSnapshotEntry>[merged.first];
      case 'replaceable_single':
        merged.sort((a, b) {
          final priority = b.priority.compareTo(a.priority);
          if (priority != 0) return priority;
          return a.contributionId.compareTo(b.contributionId);
        });
        return <UIContributionSnapshotEntry>[merged.first];
      default:
        return merged;
    }
  }

  static UIContributionSnapshotEntry? _findContribution(
    UIProviderSnapshot snapshot,
    String extensionId,
    String contributionId,
  ) {
    for (final slot in snapshot.slots) {
      for (final contribution in slot.contributions) {
        if (contribution.extensionId == extensionId &&
            (contribution.contributionId == contributionId || contribution.sourceContributionId == contributionId)) {
          return contribution;
        }
      }
    }
    return null;
  }

  static Iterable<_ActivePackageVersion> _activePackageVersions(Map<String, dynamic> sessionState) sync* {
    final packages = (sessionState['packages'] as List?) ?? const <dynamic>[];
    for (final rawPackage in packages.whereType<Map>()) {
      final package = rawPackage.cast<String, dynamic>();
      if (package['running'] != true) continue;
      final packageId = (package['id'] ?? '').toString().trim();
      final transition = (package['transitionState'] ?? '').toString().trim().toLowerCase();
      final targetVersion = (package['targetVersion'] ?? package['target_version'] ?? '').toString().trim();
      final currentVersion = (package['activeVersion'] ?? package['active_version'] ?? '').toString().trim();
      final activeVersion = (transition == 'starting' || transition == 'awaiting_client') && targetVersion.isNotEmpty
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

int? _int(dynamic value) {
  if (value is int) return value;
  if (value is num) return value.toInt();
  return int.tryParse(value?.toString() ?? '');
}

int _compareContribution(UIContributionSnapshotEntry a, UIContributionSnapshotEntry b) {
  final ordering = a.ordering.compareTo(b.ordering);
  if (ordering != 0) return ordering;
  final priority = b.priority.compareTo(a.priority);
  if (priority != 0) return priority;
  return a.contributionId.compareTo(b.contributionId);
}

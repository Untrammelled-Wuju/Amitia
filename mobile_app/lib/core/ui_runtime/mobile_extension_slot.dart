import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../services/providers.dart';
import '../../features/extensions/schema_ui/models/schema_ui_types.dart';
import '../../features/extensions/schema_ui/renderer/schema_ui_renderer.dart';
import 'renderers/sandbox_web_provider_host.dart';
import 'ui_provider.dart';
import 'ui_runtime_controller.dart';

class MobileExtensionSlot extends ConsumerWidget {
  const MobileExtensionSlot({
    super.key,
    required this.slotId,
    this.contributionId,
    this.context = const {},
    this.actions = const {},
    this.fallback,
  });

  final String slotId;
  final String? contributionId;
  final Map<String, dynamic> context;
  final Map<String, FutureOr<dynamic> Function(dynamic input)> actions;
  final Widget? fallback;

  @override
  Widget build(BuildContext context) {
    final snapshot = ref.watch(uiRuntimeProvider).valueOrNull;
    final slot = snapshot?.slot(slotId);
    if (slot == null) return fallback ?? const SizedBox.shrink();
    var contributions = snapshot!.contributionsForSlot(slotId)
        .where((item) => _matchesVisibility(item, this.context))
        .toList(growable: false);
    if (contributionId != null && contributionId!.isNotEmpty) {
      contributions = contributions.where((item) => item.contributionId == contributionId).toList(growable: false);
    }
    if (contributions.isEmpty) return fallback ?? const SizedBox.shrink();

    final effective = (slot.multiplicity == 'single' ||
            slot.multiplicity == 'replaceable_single' ||
            slot.multiplicity == 'exclusive')
        ? contributions.take(1).toList(growable: false)
        : contributions;

    final children = effective.map((item) => _ContributionHost(
      contribution: item,
      runtimeContext: this.context,
      actions: actions,
    )).toList(growable: false);

    return switch (slot.layout) {
      'row' || 'inline' => Wrap(spacing: 8, runSpacing: 8, children: children),
      'grid' => GridView.count(
          crossAxisCount: 2,
          shrinkWrap: true,
          physics: const NeverScrollableScrollPhysics(),
          crossAxisSpacing: 8,
          mainAxisSpacing: 8,
          children: children,
        ),
      _ => Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: children,
        ),
    };
  }
}


bool _matchesVisibility(UIContributionSnapshotEntry item, Map<String, dynamic> context) {
  final visibility = item.visibility;
  final required = ((visibility['required_context'] ?? visibility['requiredContext']) as List? ?? const [])
      .map((value) => value.toString())
      .where((value) => value.isNotEmpty);
  for (final path in required) {
    if (_lookupContext(context, path) == null) return false;
  }
  final messageTypes = ((visibility['message_types'] ?? visibility['messageTypes']) as List? ?? const [])
      .map((value) => value.toString()).toSet();
  if (messageTypes.isNotEmpty) {
    final actual = (_lookupContext(context, 'messageType') ?? _lookupContext(context, 'message.type'))?.toString();
    if (actual == null || !messageTypes.contains(actual)) return false;
  }
  final conditions = (visibility['conditions'] as List?) ?? const [];
  for (final raw in conditions.whereType<Map>()) {
    final condition = raw.cast<String, dynamic>();
    final actual = _lookupContext(context, (condition['field'] ?? '').toString());
    final expected = condition['value'];
    final operator = (condition['operator'] ?? '==').toString();
    if ((operator == '==' || operator == 'eq') && actual != expected) return false;
    if ((operator == '!=' || operator == 'ne') && actual == expected) return false;
    if (operator == 'not_null' && actual == null) return false;
    if (operator == 'is_null' && actual != null) return false;
    if (operator == 'in' && (expected is! List || !expected.contains(actual))) return false;
  }
  return true;
}

dynamic _lookupContext(Map<String, dynamic> context, String path) {
  dynamic current = context;
  for (final segment in path.split('.')) {
    if (current is! Map) return null;
    current = current[segment];
  }
  return current;
}

class _ContributionHost extends ConsumerWidget {
  const _ContributionHost({
    required this.contribution,
    required this.runtimeContext,
    required this.actions,
  });

  final UIContributionSnapshotEntry contribution;
  final Map<String, dynamic> runtimeContext;
  final Map<String, FutureOr<dynamic> Function(dynamic input)> actions;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    switch (contribution.entryType) {
      case 'schema_renderer':
        return FutureBuilder<Map<String, dynamic>>(
          future: ref.read(extensionServiceProvider).getUISchema(
                contribution.extensionId,
                contribution.contributionId,
              ),
          builder: (context, snapshot) {
            if (snapshot.hasError) return const SizedBox.shrink();
            if (!snapshot.hasData) {
              return const Padding(
                padding: EdgeInsets.all(8),
                child: Center(child: CircularProgressIndicator(strokeWidth: 2)),
              );
            }
            final document = SchemaUIDocument.fromJson(snapshot.data!);
            return SchemaUIRenderer(
              document: document,
              extensionId: contribution.extensionId,
              contributionId: contribution.contributionId,
              moduleId: contribution.moduleId,
              permissions: contribution.permissions,
              initialContext: {'runtime': runtimeContext, ...runtimeContext},
              embedded: true,
              slotBuilder: (slotId, childContributionId, childContext) => MobileExtensionSlot(
                slotId: slotId,
                contributionId: childContributionId,
                context: {...runtimeContext, ...childContext},
                actions: actions,
              ),
              onActionDispatch: (invocation) async {
                final handler = actions[invocation.actionId];
                return handler?.call(invocation.input ?? const <String, dynamic>{});
              },
            );
          },
        );
      case 'web_restricted':
      case 'web_isolated':
        final entryType = contribution.entryType == 'web_isolated'
            ? UIProviderEntryType.webIsolated
            : UIProviderEntryType.webRestricted;
        final entry = UIProviderEntry(
          contributionId: contribution.contributionId,
          type: entryType,
          path: contribution.entryPath,
          schemaPath: contribution.schemaPath,
        );
        final provider = UIProviderDefinition(
          providerId: 'slot:${contribution.contributionId}',
          extensionId: contribution.extensionId,
          moduleId: contribution.moduleId,
          capability: 'slot:${contribution.slotId}',
          mode: UIProviderMode.replace,
          priority: contribution.ordering,
          platforms: const [],
          entries: {'mobile': entry},
          permissions: contribution.permissions,
          placement: UIProviderPlacement.any,
          generation: 0,
          enabled: true,
          builtin: false,
          metadata: const {},
        );
        return SandboxWebProviderHost(
          provider: provider,
          entry: entry,
          context: runtimeContext,
          actions: actions,
          fallback: const SizedBox.shrink(),
        );
      default:
        return const SizedBox.shrink();
    }
  }
}

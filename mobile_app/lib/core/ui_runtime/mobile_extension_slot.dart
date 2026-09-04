import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../services/providers.dart';
import '../../features/extensions/schema_ui/engine/action_dispatcher.dart';
import '../../features/extensions/schema_ui/engine/data_source_loader.dart';
import '../../features/extensions/schema_ui/models/schema_ui_types.dart';
import '../../features/extensions/schema_ui/renderer/schema_ui_renderer.dart';
import 'mobile_dynamic_runtime.dart';
import 'mobile_ui_visibility.dart';
import 'renderers/sandbox_web_provider_host.dart';
import 'schema_ui_bridge_controller.dart';
import 'ui_provider.dart';
import 'ui_runtime_controller.dart';

class MobileExtensionSlot extends ConsumerWidget {
  const MobileExtensionSlot({
    super.key,
    required this.slotId,
    this.contributionId,
    this.dispatchKey,
    this.dispatchOnly,
    this.context = const {},
    this.actions = const {},
    this.fallback,
    this.declaredBySlotId,
    this.declaredByOwner,
  });

  final String slotId;
  final String? contributionId;
  final String? dispatchKey;
  final String? dispatchOnly;
  final Map<String, dynamic> context;
  final Map<String, FutureOr<dynamic> Function(dynamic input)> actions;
  final Widget? fallback;
  final String? declaredBySlotId;
  final String? declaredByOwner;

  String? _normalizedLayout(dynamic value) {
    final layout = value?.toString().trim().toLowerCase() ?? '';
    const allowed = <String>{'inline', 'stack', 'row', 'grid', 'tabs', 'panel', 'drawer', 'modal'};
    return allowed.contains(layout) ? layout : null;
  }

  String? _normalizedFallback(dynamic value) {
    final mode = value?.toString().trim().toLowerCase() ?? '';
    const allowed = <String>{'none', 'skeleton', 'empty', 'default'};
    return allowed.contains(mode) ? mode : null;
  }

  String? _normalizedSurfaceRole(dynamic value) {
    final role = value?.toString().trim().toLowerCase() ?? '';
    const allowed = <String>{'header', 'status', 'sidebar', 'message', 'composer', 'main', 'overlay'};
    return allowed.contains(role) ? role : null;
  }

  String _resolveSurfaceRole() {
    final explicit = _normalizedSurfaceRole(context['surfaceRole']);
    if (explicit != null) return explicit;
    final surface = context['surface'];
    if (surface is Map) {
      final nested = _normalizedSurfaceRole(surface['role']);
      if (nested != null) return nested;
    }
    final text = surface?.toString().trim().toLowerCase() ?? '';
    for (final role in const ['header', 'status', 'sidebar', 'message', 'composer', 'overlay']) {
      if (text == role || text.startsWith('$role-') || text.startsWith('$role.')) return role;
    }
    return 'main';
  }

  Widget _fallbackWidget(BuildContext context, String mode) {
    switch (mode) {
      case 'skeleton':
        return Container(
          height: 48,
          width: double.infinity,
          decoration: BoxDecoration(
            color: Theme.of(context).colorScheme.surfaceContainerHighest.withValues(alpha: 0.35),
            borderRadius: BorderRadius.circular(8),
          ),
        );
      case 'default':
        return fallback ?? const SizedBox.shrink();
      case 'empty':
      case 'none':
      default:
        return const SizedBox.shrink();
    }
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final requestedFallback = _normalizedFallback(this.context['slotFallback']);
    final snapshot = ref.watch(uiRuntimeProvider).valueOrNull;
    if (snapshot == null) {
      return _fallbackWidget(context, requestedFallback ?? (fallback == null ? 'none' : 'default'));
    }
    final conversationId = (this.context['conversationId'] ?? '')
        .toString()
        .trim();
    final runtimeState = conversationId.isEmpty
        ? null
        : ref
              .watch(clientRuntimeSessionStateProvider(conversationId))
              .valueOrNull;
    final slot =
        snapshot.slot(slotId) ??
        MobileDynamicRuntime.slotDefinition(
          sessionState: runtimeState,
          slotId: slotId,
        );
    if (slot == null) {
      return _fallbackWidget(context, requestedFallback ?? (fallback == null ? 'none' : 'default'));
    }
    final fallbackMode = requestedFallback ?? _normalizedFallback(slot.fallbackPolicy) ?? (fallback == null ? 'none' : 'default');
    final layout = _normalizedLayout(this.context['slotLayout']) ?? _normalizedLayout(slot.layout) ?? 'stack';
    final surfaceRole = _resolveSurfaceRole();
    if (declaredBySlotId != null || declaredByOwner != null) {
      if (slot.parentSlotId != declaredBySlotId ||
          slot.ownerExtension != declaredByOwner) {
        return const SizedBox.shrink();
      }
    }
    if (slot.scope == 'session' && conversationId.isEmpty) {
      return const SizedBox.shrink();
    }

    final inheritedSurface = this.context['surface'];
    final slotContext = <String, dynamic>{
      ...this.context,
      'slotScope': slot.scope,
      'surfaceRole': surfaceRole,
      if (inheritedSurface is Map)
        'surface': <String, dynamic>{
          ...inheritedSurface.cast<String, dynamic>(),
          'role': surfaceRole,
        }
      else if (inheritedSurface == null)
        'surface': surfaceRole,
      if (conversationId.isNotEmpty) 'sessionId': conversationId,
    };
    final dynamicContributions = MobileDynamicRuntime.slotContributions(
      snapshot: snapshot,
      sessionState: runtimeState,
      slotId: slotId,
    );
    final serverVisible = snapshot
        .contributionsForSlot(slotId)
        .where((item) => matchesMobileUIContributionVisibility(item, slotContext))
        .toList(growable: false);
    final dynamicVisible = dynamicContributions
        .where((item) => matchesMobileUIContributionVisibility(item, slotContext))
        .toList(growable: false);

    var contributions = MobileDynamicRuntime.resolveSlot(
      slot: slot,
      server: serverVisible,
      dynamic: dynamicVisible,
      owner: slotContext,
      dispatchKey: dispatchKey ?? _text(this.context['dispatchKey']),
      listOnly: dispatchOnly ?? _text(this.context['dispatchOnly']),
    );
    if (contributionId != null && contributionId!.isNotEmpty) {
      contributions = contributions
          .where(
            (item) =>
                item.contributionId == contributionId ||
                item.sourceContributionId == contributionId,
          )
          .toList(growable: false);
    }
    if (contributions.isEmpty) return _fallbackWidget(context, fallbackMode);

    final children = contributions.map((item) {
      final matched = item.matched ??
          (slot.kind == 'chain' && item.runtimePackageId?.isNotEmpty == true
              ? <String, dynamic>{'owner': slotContext}
              : null);
      return _ContributionHost(
        key: ValueKey(item.contributionId),
        contribution: item,
        runtimeContext: <String, dynamic>{
          ...slotContext,
          'slotId': slot.slotId,
          'entryKey': _identity(item.entryKey, item.contributionId),
          'cellId': _identity(item.cellId, item.contributionId),
          if (matched != null) 'matched': matched,
        },
        actions: actions,
      );
    }).toList(growable: false);

    return switch (layout) {
      'row' || 'inline' => Wrap(
        spacing: 8,
        runSpacing: 8,
        children: children,
      ),
      'grid' => GridView.count(
        crossAxisCount: 2,
        shrinkWrap: true,
        physics: const NeverScrollableScrollPhysics(),
        crossAxisSpacing: 8,
        mainAxisSpacing: 8,
        children: children,
      ),
      'tabs' => Row(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: children.map((child) => Expanded(child: child)).toList(growable: false),
      ),
      'panel' => Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: children,
        ),
      ),
      _ => Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: children,
      ),
    };
  }
}


class _ContributionHost extends ConsumerStatefulWidget {
  const _ContributionHost({
    super.key,
    required this.contribution,
    required this.runtimeContext,
    required this.actions,
  });

  final UIContributionSnapshotEntry contribution;
  final Map<String, dynamic> runtimeContext;
  final Map<String, FutureOr<dynamic> Function(dynamic input)> actions;

  @override
  ConsumerState<_ContributionHost> createState() => _ContributionHostState();
}

class _ContributionHostState extends ConsumerState<_ContributionHost> {
  late final SchemaUIBridgeController _bridge;
  late final DataSourceLoader _dataSourceLoader;
  Future<Map<String, dynamic>>? _schemaFuture;

  String get _sourceContributionId =>
      widget.contribution.sourceContributionId ??
      widget.contribution.contributionId;

  String get _characterId {
    final nested = widget.runtimeContext['character'];
    return (widget.runtimeContext['characterId'] ??
            (nested is Map ? nested['id'] : ''))
        .toString()
        .trim();
  }

  String get _conversationId {
    final nested = widget.runtimeContext['conversation'];
    return (widget.runtimeContext['conversationId'] ??
            (nested is Map ? nested['id'] : ''))
        .toString()
        .trim();
  }

  @override
  void initState() {
    super.initState();
    _bridge = SchemaUIBridgeController(ref.read(extensionServiceProvider));
    _dataSourceLoader = DataSourceLoader(fetcher: (request) => _bridge.requestData(
      request.dataSource.id,
      contributionId: _sourceContributionId,
      contractVersion: widget.contribution.contractVersion,
      characterId: _characterId,
      conversationId: _conversationId,
      params: request.input,
    ));
  }

  @override
  void didUpdateWidget(covariant _ContributionHost oldWidget) {
    super.didUpdateWidget(oldWidget);
    final sourceChanged =
        oldWidget.contribution.contributionId !=
            widget.contribution.contributionId ||
        oldWidget.contribution.sourceContributionId !=
            widget.contribution.sourceContributionId ||
        oldWidget.contribution.extensionId != widget.contribution.extensionId ||
        oldWidget.contribution.contractVersion !=
            widget.contribution.contractVersion ||
        oldWidget.contribution.schemaPath != widget.contribution.schemaPath ||
        oldWidget.contribution.entryPath != widget.contribution.entryPath;
    final scopeChanged =
        _runtimeCharacterId(oldWidget.runtimeContext) != _characterId ||
        _runtimeConversationId(oldWidget.runtimeContext) != _conversationId;
    if (sourceChanged || scopeChanged) {
      unawaited(_bridge.reset());
    }
    if (sourceChanged) {
      _schemaFuture = null;
    }
  }

  Future<dynamic> _dispatchAction(ActionInvocation invocation) {
    return _bridge.dispatch(
      invocation,
      contributionId: _sourceContributionId,
      contractVersion: widget.contribution.contractVersion,
      characterId: _characterId,
      conversationId: _conversationId,
      localActions: widget.actions,
    );
  }

  Future<Map<String, dynamic>> _fetchSchema() {
    return ref.read(extensionServiceProvider).getUISchema(
          widget.contribution.extensionId,
          _sourceContributionId,
        );
  }

  Future<void> _reloadSchema() async {
    if (!mounted) return;
    final next = _fetchSchema();
    setState(() => _schemaFuture = next);
    await next;
  }

  @override
  void dispose() {
    unawaited(_bridge.dispose());
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final contribution = widget.contribution;
    final runtimeContext = widget.runtimeContext;
    final actions = widget.actions;
    switch (contribution.entryType) {
      case 'schema_renderer':
        _schemaFuture ??= _fetchSchema();
        return FutureBuilder<Map<String, dynamic>>(
          future: _schemaFuture,
          builder: (context, snapshot) {
            if (snapshot.hasError) return const SizedBox.shrink();
            if (!snapshot.hasData) {
              return const Padding(
                padding: EdgeInsets.all(8),
                child: Center(
                  child: CircularProgressIndicator(strokeWidth: 2),
                ),
              );
            }
            final document = SchemaUIDocument.fromJson(snapshot.data!);
            return SchemaUIRenderer(
              document: document,
              extensionId: contribution.extensionId,
              contributionId: _sourceContributionId,
              moduleId: contribution.moduleId,
              permissions: contribution.permissions,
              initialContext: {'runtime': runtimeContext, ...runtimeContext},
              embedded: true,
              slotBuilder: (slotId, childContributionId, childContext) =>
                  MobileExtensionSlot(
                    slotId: slotId,
                    contributionId: childContributionId,
                    dispatchKey: _text(childContext['dispatchKey']),
                    dispatchOnly: _text(childContext['dispatchOnly']),
                    context: {...runtimeContext, ...childContext},
                    actions: actions,
                    declaredBySlotId: contribution.slotId,
                    declaredByOwner:
                        contribution.runtimePackageId?.isNotEmpty == true
                        ? 'client-runtime:${contribution.runtimePackageId}'
                        : contribution.extensionId,
                  ),
              dataSourceLoader: _dataSourceLoader,
              onActionDispatch: _dispatchAction,
              onReloadSchema: _reloadSchema,
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

String _runtimeCharacterId(Map<String, dynamic> context) {
  final nested = context['character'];
  return (context['characterId'] ?? (nested is Map ? nested['id'] : ''))
      .toString()
      .trim();
}

String _runtimeConversationId(Map<String, dynamic> context) {
  final nested = context['conversation'];
  return (context['conversationId'] ?? (nested is Map ? nested['id'] : ''))
      .toString()
      .trim();
}

String? _text(dynamic value) {
  final normalized = value?.toString().trim() ?? '';
  return normalized.isEmpty ? null : normalized;
}

String _identity(String value, String fallback) {
  final normalized = value.trim();
  return normalized.isEmpty ? fallback : normalized;
}

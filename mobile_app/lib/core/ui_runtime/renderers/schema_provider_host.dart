import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../services/providers.dart';
import '../../../features/extensions/schema_ui/engine/action_dispatcher.dart';
import '../../../features/extensions/schema_ui/engine/data_source_loader.dart';
import '../../../features/extensions/schema_ui/models/schema_ui_types.dart';
import '../../../features/extensions/schema_ui/renderer/schema_ui_renderer.dart';
import '../mobile_extension_slot.dart';
import '../schema_ui_bridge_controller.dart';
import '../ui_provider.dart';

class SchemaProviderHost extends ConsumerStatefulWidget {
  const SchemaProviderHost({
    super.key,
    required this.provider,
    required this.entry,
    this.context = const {},
    this.actions = const {},
    this.fallback,
    this.onFailure,
  });

  final UIProviderDefinition provider;
  final UIProviderEntry entry;
  final Map<String, dynamic> context;
  final Map<String, FutureOr<dynamic> Function(dynamic input)> actions;
  final Widget? fallback;
  final ValueChanged<Object>? onFailure;

  @override
  ConsumerState<SchemaProviderHost> createState() => _SchemaProviderHostState();
}

class _SchemaProviderHostState extends ConsumerState<SchemaProviderHost> {
  SchemaUIDocument? _document;
  Object? _error;
  bool _loading = true;
  int _loadEpoch = 0;
  late final SchemaUIBridgeController _bridge;
  late final DataSourceLoader _dataSourceLoader;

  @override
  void initState() {
    super.initState();
    _bridge = SchemaUIBridgeController(ref.read(extensionServiceProvider));
    _dataSourceLoader = DataSourceLoader(fetcher: (request) => _bridge.requestData(
      request.dataSource.id,
      contributionId: widget.entry.contributionId ?? '',
      contractVersion: _providerContractVersion(widget.provider),
      characterId: _characterId(widget.context),
      conversationId: _conversationId(widget.context),
      params: request.input,
    ));
    unawaited(_load());
  }

  @override
  void didUpdateWidget(covariant SchemaProviderHost oldWidget) {
    super.didUpdateWidget(oldWidget);
    final providerChanged =
        oldWidget.provider.providerId != widget.provider.providerId ||
        oldWidget.provider.extensionId != widget.provider.extensionId ||
        oldWidget.provider.generation != widget.provider.generation ||
        oldWidget.entry.contributionId != widget.entry.contributionId;
    final scopeChanged =
        _characterId(oldWidget.context) != _characterId(widget.context) ||
        _conversationId(oldWidget.context) != _conversationId(widget.context);
    if (providerChanged || scopeChanged) {
      unawaited(_bridge.reset());
    }
    if (providerChanged) {
      unawaited(_load());
    }
  }

  Future<void> _load() async {
    final loadEpoch = ++_loadEpoch;
    final contributionId = widget.entry.contributionId?.trim() ?? '';
    if (contributionId.isEmpty) {
      final error = StateError(
        'UI provider entry does not reference a Schema UI contribution',
      );
      if (!mounted) return;
      setState(() {
        _loading = false;
        _error = error;
        _document = null;
      });
      widget.onFailure?.call(error);
      return;
    }

    if (mounted) {
      setState(() {
        _loading = true;
        _error = null;
      });
    }
    try {
      final raw = await ref
          .read(extensionServiceProvider)
          .getUISchema(widget.provider.extensionId, contributionId);
      if (!mounted || loadEpoch != _loadEpoch) return;
      setState(() {
        _document = SchemaUIDocument.fromJson(raw);
        _loading = false;
      });
    } catch (error) {
      if (!mounted || loadEpoch != _loadEpoch) return;
      setState(() {
        _error = error;
        _loading = false;
      });
      widget.onFailure?.call(error);
    }
  }

  Future<dynamic> _dispatchAction(ActionInvocation invocation) {
    return _bridge.dispatch(
      invocation,
      contributionId: widget.entry.contributionId ?? '',
      contractVersion: _providerContractVersion(widget.provider),
      characterId: _characterId(widget.context),
      conversationId: _conversationId(widget.context),
      localActions: widget.actions,
    );
  }

  @override
  void dispose() {
    unawaited(_bridge.dispose());
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return const Center(child: CircularProgressIndicator(strokeWidth: 2));
    }
    if (_error != null || _document == null) {
      return widget.fallback ??
          Center(
            child: Text(
              'UI provider unavailable: ${_error ?? 'schema missing'}',
            ),
          );
    }

    return SchemaUIRenderer(
      document: _document!,
      extensionId: widget.provider.extensionId,
      contributionId: widget.entry.contributionId!,
      moduleId: widget.provider.moduleId,
      permissions: widget.provider.permissions,
      initialContext: {
        ...widget.context,
        'runtime': widget.context,
        'providerId': widget.provider.providerId,
        'capability': widget.provider.capability,
      },
      slotBuilder: (slotId, contributionId, childContext) => MobileExtensionSlot(
        slotId: slotId,
        contributionId: contributionId,
        dispatchKey: _text(childContext['dispatchKey']),
        dispatchOnly: _text(childContext['dispatchOnly']),
        context: {...widget.context, ...childContext},
        actions: widget.actions,
      ),
      dataSourceLoader: _dataSourceLoader,
      onActionDispatch: _dispatchAction,
      onReloadSchema: _load,
    );
  }
}

String _characterId(Map<String, dynamic> context) {
  final nested = context['character'];
  return (context['characterId'] ?? (nested is Map ? nested['id'] : ''))
      .toString()
      .trim();
}

String _conversationId(Map<String, dynamic> context) {
  final nested = context['conversation'];
  return (context['conversationId'] ?? (nested is Map ? nested['id'] : ''))
      .toString()
      .trim();
}

int _providerContractVersion(UIProviderDefinition provider) {
  final value =
      provider.metadata['contractVersion'] ?? provider.metadata['contract_version'];
  if (value is num) return value.toInt();
  return int.tryParse(value?.toString() ?? '') ?? 1;
}

String? _text(dynamic value) {
  final normalized = value?.toString().trim() ?? '';
  return normalized.isEmpty ? null : normalized;
}

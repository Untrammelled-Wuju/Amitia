import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../services/providers.dart';
import '../../../features/extensions/schema_ui/models/schema_ui_types.dart';
import '../../../features/extensions/schema_ui/renderer/schema_ui_renderer.dart';
import '../ui_provider.dart';
import '../mobile_extension_slot.dart';

class SchemaProviderHost extends ConsumerStatefulWidget {
  const SchemaProviderHost({super.key, required this.provider, required this.entry, this.context = const {}, this.actions = const {}, this.fallback, this.onFailure});
  final UIProviderDefinition provider;
  final UIProviderEntry entry;
  final Map<String, dynamic> context;
  final Map<String, FutureOr<dynamic> Function(dynamic input)> actions;
  final Widget? fallback;
  final ValueChanged<Object>? onFailure;
  @override ConsumerState<SchemaProviderHost> createState() => _SchemaProviderHostState();
}

class _SchemaProviderHostState extends ConsumerState<SchemaProviderHost> {
  SchemaUIDocument? _document;
  Object? _error;
  bool _loading = true;

  @override void initState() { super.initState(); _load(); }
  @override void didUpdateWidget(covariant SchemaProviderHost oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.provider.providerId != widget.provider.providerId || oldWidget.provider.generation != widget.provider.generation) _load();
  }

  Future<void> _load() async {
    final contributionId = widget.entry.contributionId;
    if (contributionId == null || contributionId.isEmpty) {
      final error = StateError('UI provider entry does not reference a Schema UI contribution');
      setState(() { _loading = false; _error = error; });
      widget.onFailure?.call(error);
      return;
    }
    setState(() { _loading = true; _error = null; });
    try {
      final raw = await ref.read(extensionServiceProvider).getUISchema(widget.provider.extensionId, contributionId);
      if (!mounted) return;
      if (raw == null) { setState(() { _error = 'Empty schema response'; _loading = false; }); return; }
      setState(() { _document = SchemaUIDocument.fromJson(raw); _loading = false; });
    } catch (e) {
      if (!mounted) return;
      setState(() { _error = e; _loading = false; });
      widget.onFailure?.call(e);
    }
  }

  @override Widget build(BuildContext context) {
    if (_loading) return const Center(child: CircularProgressIndicator(strokeWidth: 2));
    if (_error != null || _document == null) return widget.fallback ?? Center(child: Text('UI provider unavailable: ${_error ?? 'schema missing'}'));
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
        context: {...widget.context, ...childContext},
        actions: widget.actions,
      ),
      onActionDispatch: (invocation) async {
        final action = widget.actions[invocation.actionId];
        if (action != null) return action(invocation.input ?? const <String, dynamic>{});
        return null;
      },
    );
  }
}

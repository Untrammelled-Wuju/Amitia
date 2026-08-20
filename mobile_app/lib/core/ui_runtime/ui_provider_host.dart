import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'ui_provider.dart';
import 'ui_runtime_controller.dart';
import 'renderers/schema_provider_host.dart';
import 'renderers/sandbox_web_provider_host.dart';

class UIProviderHost extends ConsumerStatefulWidget {
  const UIProviderHost({
    super.key,
    required this.capability,
    required this.fallback,
    this.providerId,
    this.context = const {},
    this.actions = const {},
  });
  final String capability;
  final String? providerId;
  final Widget fallback;
  final Map<String, dynamic> context;
  final Map<String, FutureOr<dynamic> Function(dynamic input)> actions;
  @override
  ConsumerState<UIProviderHost> createState() => _UIProviderHostState();
}

class _UIProviderHostState extends ConsumerState<UIProviderHost> {
  int _fallbackIndex = 0;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) ref.read(uiRuntimeProvider.notifier).ensureLoaded();
    });
  }

  @override
  void didUpdateWidget(covariant UIProviderHost oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.capability != widget.capability ||
        oldWidget.providerId != widget.providerId) {
      _fallbackIndex = 0;
    }
  }

  void _advanceFallback(Object error) {
    final snapshot = ref.read(uiRuntimeProvider).valueOrNull;
    final chain = snapshot?.fallbackChain(
          widget.capability,
          providerId: widget.providerId,
        ) ??
        const <UIProviderDefinition>[];
    if (_fallbackIndex + 1 < chain.length && mounted) {
      setState(() => _fallbackIndex++);
    }
  }

  Widget _externalRenderer(
    UIProviderDefinition provider,
    UIProviderEntry entry,
    Map<String, dynamic> renderContext,
  ) {
    switch (entry.type) {
      case UIProviderEntryType.schemaRenderer:
        return SchemaProviderHost(
          provider: provider,
          entry: entry,
          context: renderContext,
          actions: widget.actions,
          onFailure: _advanceFallback,
        );
      case UIProviderEntryType.webRestricted:
      case UIProviderEntryType.webIsolated:
        return SandboxWebProviderHost(
          provider: provider,
          entry: entry,
          context: renderContext,
          actions: widget.actions,
          onFailure: _advanceFallback,
        );
      case UIProviderEntryType.webModule:
        // Flutter release builds are AOT; downloadable Dart/native modules
        // cannot be loaded dynamically. Cross-platform providers must ship a
        // schema_renderer or sandbox-web entry for mobile.
        return widget.fallback;
      case UIProviderEntryType.builtinNative:
      case UIProviderEntryType.declarative:
      case UIProviderEntryType.unknown:
        return widget.fallback;
    }
  }

  @override
  Widget build(BuildContext context) {
    final snapshot = ref.watch(uiRuntimeProvider).valueOrNull;
    final chain = snapshot?.fallbackChain(
          widget.capability,
          providerId: widget.providerId,
        ) ??
        const <UIProviderDefinition>[];
    if (chain.isEmpty) return widget.fallback;
    final index = _fallbackIndex.clamp(0, chain.length - 1).toInt();
    final provider = chain[index];
    if (provider.builtin || !provider.enabled) return widget.fallback;
    final entry = provider.entryFor(currentUIPlatform());
    if (entry == null ||
        entry.type == UIProviderEntryType.builtinNative ||
        entry.type == UIProviderEntryType.declarative) {
      return widget.fallback;
    }
    final renderContext = <String, dynamic>{
      'capability': widget.capability,
      'providerId': provider.providerId,
      'providerMode': provider.mode.name,
      'platform': currentUIPlatform(),
      ...widget.context,
    };
    final external = _externalRenderer(provider, entry, renderContext);

    switch (provider.mode) {
      case UIProviderMode.replace:
        return external;
      case UIProviderMode.augment:
        return Stack(
          fit: StackFit.passthrough,
          children: [widget.fallback, external],
        );
      case UIProviderMode.compose:
        // Schema/Web providers cannot host an arbitrary native Widget subtree.
        // Keep the built-in surface as the composition base and layer the
        // provider on top. Trusted native providers remain compile-time built-ins.
        return Stack(
          fit: StackFit.passthrough,
          children: [widget.fallback, external],
        );
    }
  }
}

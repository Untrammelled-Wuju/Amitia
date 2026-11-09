import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'ui_page_provider_registry.dart';
import 'ui_provider.dart';
import 'ui_provider_host.dart';
import 'ui_runtime_controller.dart';

class ProviderTabHost extends ConsumerWidget {
  const ProviderTabHost({
    super.key,
    required this.routePath,
    required this.fallback,
  });

  final String routePath;
  final Widget fallback;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final snapshot = ref.watch(effectiveUIRuntimeSnapshotProvider);
    final provider = UIPageProviderRegistry.resolve(
      snapshot,
      capability: UICapability.pageProvider,
      route: routePath,
    );
    if (provider == null) {
      return fallback;
    }
    return UIProviderHost(
      capability: UICapability.pageProvider,
      providerId: provider.providerId,
      context: {'route': routePath, 'shellRoute': routePath},
      fallback: fallback,
    );
  }
}

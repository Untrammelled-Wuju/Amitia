import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../app/theme/app_colors.dart';
import '../../app/theme/app_spacing.dart';
import '../../app/theme/app_typography.dart';
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
      return _ProviderTabUnavailable(routePath: routePath);
    }
    return UIProviderHost(
      capability: UICapability.pageProvider,
      providerId: provider.providerId,
      context: {'route': routePath, 'shellRoute': routePath},
      fallback: fallback,
    );
  }
}

class _ProviderTabUnavailable extends StatelessWidget {
  const _ProviderTabUnavailable({required this.routePath});

  final String routePath;

  @override
  Widget build(BuildContext context) {
    return ColoredBox(
      color: context.backgroundPrimary,
      child: Center(
        child: Padding(
          padding: EdgeInsets.all(AppSpacing.xl),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(
                Icons.extension_off_outlined,
                size: 42,
                color: context.textTertiary,
              ),
              SizedBox(height: AppSpacing.md),
              Text('页面插件不可用', style: AppTypography.cardTitle(context)),
              SizedBox(height: AppSpacing.xs),
              Text(
                routePath,
                style: AppTypography.caption(context),
                textAlign: TextAlign.center,
              ),
            ],
          ),
        ),
      ),
    );
  }
}

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'theme/app_theme.dart';
import '../core/ui_runtime/route_surface_provider_host.dart';
import '../core/ui_runtime/mobile_extension_slot.dart';
import '../core/ui_runtime/routing/builtin_route_catalog.dart';
import '../core/ui_runtime/ui_provider.dart';
import '../core/ui_runtime/ui_provider_host.dart';
import '../core/ui_runtime/ui_route_registry.dart';
import '../core/ui_runtime/ui_runtime_controller.dart';
import '../core/widgets/application_background.dart';
import '../features/error/presentation/pages/not_found_page.dart';
import '../features/onboarding/presentation/pages/onboarding_page.dart';
import '../features/privacy/presentation/pages/privacy_page.dart';
import '../features/settings/presentation/pages/ui_provider_settings_page.dart';
import 'app_routes.dart';
import 'route_transitions.dart';

final appNavigatorKey = GlobalKey<NavigatorState>(debugLabel: 'app-shell');

class AppShell extends ConsumerWidget {
  final Widget child;
  final String currentRoute;

  const AppShell({super.key, required this.child, required this.currentRoute});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return UIProviderHost(
      capability: UICapability.appWorkspace,
      context: {'route': currentRoute},
      fallback: child,
    );
  }
}

RoutingConfig _routingConfigFor(UIProviderSnapshot? providerSnapshot) {
  return RoutingConfig(
    redirect: (context, state) async {
      final location = state.matchedLocation;
      if (location == '/about') return '/settings/about';
      if (location == '/toolbox') return '/settings/toolbox';
      return null;
    },
    routes: <RouteBase>[
      GoRoute(path: '/', redirect: (context, state) => AppRoutes.conversations),
      GoRoute(
        path: '/onboarding',
        pageBuilder: (context, state) => slideFadePage(
          context: context,
          state: state,
          child: const OnboardingPage(),
        ),
      ),
      GoRoute(
        path: '/privacy',
        pageBuilder: (context, state) => slideFadePage(
          context: context,
          state: state,
          child: const PrivacyPage(),
        ),
      ),
      // Recovery route deliberately bypasses replaceable appShell.
      GoRoute(
        path: '/settings/ui-providers',
        pageBuilder: (context, state) => slideFadePage(
          context: context,
          state: state,
          child: Theme(
            data: Theme.of(context).brightness == Brightness.dark
                ? AppTheme.darkTheme()
                : AppTheme.lightTheme(),
            child: const UIProviderSettingsPage(),
          ),
        ),
      ),
      ShellRoute(
        builder: (context, state, child) {
          final surface = RouteSurfaceProviderHost(
            route: state.matchedLocation,
            child: child,
          );
          final shell = UIProviderHost(
            capability: UICapability.appShell,
            context: {'route': state.matchedLocation},
            fallback: AppShell(
              currentRoute: state.matchedLocation,
              child: surface,
            ),
          );
          return ApplicationBackground(
            child: MobileExtensionSlot(
              slotId: 'root',
              context: {
                'route': state.matchedLocation,
                'surfaceRole': 'main',
                'slotFallback': 'default',
              },
              fallback: shell,
            ),
          );
        },
        routes: <RouteBase>[
          ...buildBuiltinBusinessRoutes(),
          ...buildProviderRoutes(providerSnapshot),
        ],
      ),
    ],
  );
}

final goRouterProvider = Provider<GoRouter>((ref) {
  final initialSnapshot = ref.read(uiRuntimeProvider).valueOrNull;
  final routingConfig = ValueNotifier<RoutingConfig>(
    _routingConfigFor(initialSnapshot),
  );
  ref.onDispose(routingConfig.dispose);
  ref.listen<String>(
    uiRuntimeProvider.select(
      (state) => uiRouteRegistrySignature(state.valueOrNull),
    ),
    (previous, next) {
      if (previous == next) return;
      routingConfig.value = _routingConfigFor(
        ref.read(uiRuntimeProvider).valueOrNull,
      );
    },
  );
  return GoRouter.routingConfig(
    routingConfig: routingConfig,
    initialLocation: AppRoutes.conversations,
    navigatorKey: appNavigatorKey,
    errorBuilder: (context, state) =>
        NotFoundPage(attemptedPath: state.uri.toString()),
  );
});

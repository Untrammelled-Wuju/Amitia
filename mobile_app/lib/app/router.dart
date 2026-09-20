import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
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
import '../core/widgets/amitia_drawer.dart';
import '../core/widgets/amitia_scaffold.dart';
import '../features/error/presentation/pages/not_found_page.dart';
import '../features/onboarding/presentation/pages/onboarding_page.dart';
import '../features/privacy/presentation/pages/privacy_page.dart';
import '../features/settings/presentation/pages/ui_provider_settings_page.dart';
import 'app_routes.dart';
import 'route_transitions.dart';

final appNavigatorKey = GlobalKey<NavigatorState>(debugLabel: 'app-shell');

class AppShell extends ConsumerStatefulWidget {
  final Widget child;
  final String currentRoute;

  const AppShell({super.key, required this.child, required this.currentRoute});

  @override
  ConsumerState<AppShell> createState() => _AppShellState();
}

class _AppShellState extends ConsumerState<AppShell> {
  final _scaffoldKey = GlobalKey<ScaffoldState>();
  DateTime? _lastBackPress;
  bool _diagnosticLogged = false;

  @override
  void didUpdateWidget(AppShell oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.currentRoute != widget.currentRoute) {
      _lastBackPress = null;
    }
  }

  void _handleChatBack() {
    final now = DateTime.now();
    if (_lastBackPress != null &&
        now.difference(_lastBackPress!) < const Duration(seconds: 2)) {
      _lastBackPress = null;
      SystemNavigator.pop();
      return;
    }
    _lastBackPress = now;
    ScaffoldMessenger.of(context)
      ..clearSnackBars()
      ..showSnackBar(
        const SnackBar(
          content: Text('再按一次退出应用'),
          duration: Duration(seconds: 2),
          behavior: SnackBarBehavior.floating,
        ),
      );
  }

  @override
  Widget build(BuildContext context) {
    if (!_diagnosticLogged) {
      _diagnosticLogged = true;
      debugPrint('[diag] AppShell route=${widget.currentRoute}');
    }
    final isChatRoute = widget.currentRoute == AppRoutes.chat;
    final router = GoRouter.of(context);
    return PopScope(
      canPop: !isChatRoute && router.canPop(),
      onPopInvokedWithResult: (didPop, result) {
        if (didPop) return;
        if (isChatRoute) {
          _handleChatBack();
        } else {
          router.go(AppRoutes.chat);
        }
      },
      child: ShellDrawerScope(
        openDrawer: () {
          FocusManager.instance.primaryFocus?.unfocus();
          _scaffoldKey.currentState?.openDrawer();
        },
        child: Scaffold(
          key: _scaffoldKey,
          resizeToAvoidBottomInset: true,
          drawer: UIProviderHost(
            capability: UICapability.appNavigation,
            context: {'route': widget.currentRoute, 'surface': 'drawer'},
            fallback: AmitiaDrawer(currentRoute: widget.currentRoute),
          ),
          body: UIProviderHost(
            capability: UICapability.appWorkspace,
            context: {'route': widget.currentRoute},
            fallback: widget.child,
          ),
        ),
      ),
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
      GoRoute(path: '/', redirect: (context, state) => AppRoutes.chat),
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
          return MobileExtensionSlot(
            slotId: 'root',
            context: {
              'route': state.matchedLocation,
              'surfaceRole': 'main',
              'slotFallback': 'default',
            },
            fallback: shell,
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
    initialLocation: AppRoutes.chat,
    navigatorKey: appNavigatorKey,
    errorBuilder: (context, state) =>
        NotFoundPage(attemptedPath: state.uri.toString()),
  );
});

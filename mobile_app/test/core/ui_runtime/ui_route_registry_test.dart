import 'package:amitia_app/app/app_routes.dart';
import 'package:amitia_app/core/ui_runtime/ui_provider.dart';
import 'package:amitia_app/core/ui_runtime/ui_route_registry.dart';
import 'package:flutter_test/flutter_test.dart';

UIProviderDefinition _provider({
  required String id,
  required String extensionId,
  required String capability,
  bool enabled = true,
  Map<String, dynamic> metadata = const <String, dynamic>{},
}) {
  return UIProviderDefinition(
    providerId: id,
    extensionId: extensionId,
    capability: capability,
    mode: UIProviderMode.replace,
    priority: 1,
    platforms: const <String>[],
    entries: const <String, UIProviderEntry>{
      '*': UIProviderEntry(type: UIProviderEntryType.declarative),
    },
    permissions: const <String>[],
    placement: UIProviderPlacement.any,
    generation: 1,
    enabled: enabled,
    builtin: false,
    metadata: metadata,
  );
}

UIProviderSnapshot _snapshot(List<UIProviderDefinition> providers) {
  return UIProviderSnapshot(
    providers: providers,
    slots: const <UISlotSnapshotEntry>[],
    profile: const UIProfile(
      profileId: 'default',
      name: 'Default',
      selections: <String, String>{},
    ),
    profileLayers: const <UIProfile>[],
    context: const UIProviderResolveContext(localRuntime: true),
    resolved: const <String, UIProviderDefinition>{},
    version: 1,
  );
}

void main() {
  test('section shells protect their root and accept child provider routes', () {
    expect(isProtectedProviderRoutePath(AppRoutes.extensions), isTrue);
    expect(
      isProtectedProviderRoutePath('${AppRoutes.extensions}/market'),
      isFalse,
    );
    expect(
      isProtectedProviderRoutePath('${AppRoutes.extensions}/third-party'),
      isFalse,
    );
    expect(isProtectedProviderRoutePath('/decision-viz'), isFalse);
    expect(isProtectedProviderRoutePath('/long-running'), isFalse);
  });

  test('closed built-in namespaces remain protected', () {
    expect(isProtectedProviderRoutePath(AppRoutes.settings), isFalse);
    expect(
      isProtectedProviderRoutePath('${AppRoutes.settings}/plugin'),
      isFalse,
    );
    expect(
      isProtectedProviderRoutePath('${AppRoutes.memoryHub}/plugin'),
      isTrue,
    );
    expect(
      isProtectedProviderRoutePath('${AppRoutes.gameCenter}/plugin'),
      isFalse,
    );
  });

  test(
    'core business roots are protected while remaining plugin roots stay open',
    () {
      expect(isProtectedProviderRoutePath(AppRoutes.characters), isTrue);
      expect(isProtectedProviderRoutePath(AppRoutes.memory), isTrue);
      expect(isProtectedProviderRoutePath(AppRoutes.tasks), isTrue);
      expect(isProtectedProviderRoutePath(AppRoutes.developer), isFalse);
    },
  );

  test('settings root resolves from the provider snapshot', () {
    final snapshot = _snapshot([
      _provider(
        id: 'user-space-routes',
        extensionId: 'com.amitia/user-space-hub',
        capability: UICapability.routeRegistry,
        metadata: const {
          'routes': [
            {
              'id': 'settings-root',
              'path': AppRoutes.settings,
              'providerId': 'user-space-page-provider',
              'capability': UICapability.pageProvider,
            },
          ],
        },
      ),
      _provider(
        id: 'user-space-page-provider',
        extensionId: 'com.amitia/user-space-hub',
        capability: UICapability.pageProvider,
      ),
    ]);

    expect(
      resolveProviderRoutes(snapshot).map((route) => route.path),
      contains(AppRoutes.settings),
    );
  });

  test(
    'remaining plugin business roots resolve from the provider snapshot',
    () {
      const entries = <(String, String, String)>[
        (
          '/developer',
          'com.amitia/runtime-insights',
          'runtime-insights-mobile-page-provider',
        ),
      ];
      final providers = <UIProviderDefinition>[];
      for (final (index, (route, extensionId, pageProviderId))
          in entries.indexed) {
        providers.addAll([
          _provider(
            id: 'route-registry-$index',
            extensionId: extensionId,
            capability: UICapability.routeRegistry,
            metadata: {
              'routes': [
                {
                  'id': 'root-$index',
                  'path': route,
                  'providerId': pageProviderId,
                  'capability': UICapability.pageProvider,
                },
              ],
            },
          ),
          _provider(
            id: pageProviderId,
            extensionId: extensionId,
            capability: UICapability.pageProvider,
          ),
        ]);
      }

      expect(
        resolveProviderRoutes(_snapshot(providers)).map((route) => route.path),
        containsAll(entries.map((entry) => entry.$1)),
      );
    },
  );

  test(
    'disabled or uninstalled providers remove routes and invalidate router signature',
    () {
      final enabledSnapshot = _snapshot([
        _provider(
          id: 'routes',
          extensionId: 'com.example/plugin',
          capability: UICapability.routeRegistry,
          metadata: const {
            'routes': [
              {
                'id': 'main',
                'path': '/extensions-ui/plugin',
                'providerId': 'page',
                'capability': UICapability.pageProvider,
              },
            ],
          },
        ),
        _provider(
          id: 'page',
          extensionId: 'com.example/plugin',
          capability: UICapability.pageProvider,
          metadata: const {
            'routes': ['/extensions-ui/plugin'],
          },
        ),
      ]);
      expect(
        resolveProviderRoutes(enabledSnapshot).map((route) => route.path),
        contains('/extensions-ui/plugin'),
      );
      final disabledSnapshot = _snapshot([
        _provider(
          id: 'routes',
          extensionId: 'com.example/plugin',
          capability: UICapability.routeRegistry,
          enabled: false,
        ),
        _provider(
          id: 'page',
          extensionId: 'com.example/plugin',
          capability: UICapability.pageProvider,
          enabled: false,
        ),
      ]);
      expect(resolveProviderRoutes(disabledSnapshot), isEmpty);
      expect(uiRouteRegistrySignature(disabledSnapshot), 'builtin');
      final uninstalledSnapshot = _snapshot(const []);
      expect(resolveProviderRoutes(uninstalledSnapshot), isEmpty);
      expect(uiRouteRegistrySignature(uninstalledSnapshot), 'builtin');
    },
  );
}

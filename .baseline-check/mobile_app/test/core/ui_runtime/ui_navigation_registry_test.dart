import 'package:amitia_app/app/app_routes.dart';
import 'package:amitia_app/core/ui_runtime/ui_navigation_registry.dart';
import 'package:amitia_app/core/ui_runtime/ui_provider.dart';
import 'package:flutter_test/flutter_test.dart';

UIProviderDefinition _provider({
  required String id,
  required String extensionId,
  required String capability,
  int priority = 0,
  Map<String, dynamic> metadata = const <String, dynamic>{},
}) {
  return UIProviderDefinition(
    providerId: id,
    extensionId: extensionId,
    capability: capability,
    mode: UIProviderMode.augment,
    priority: priority,
    platforms: const <String>[],
    entries: const <String, UIProviderEntry>{
      '*': UIProviderEntry(type: UIProviderEntryType.declarative),
    },
    permissions: const <String>[],
    placement: UIProviderPlacement.any,
    generation: 1,
    enabled: true,
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

UIProviderDefinition _page(String extensionId, String id) => _provider(
      id: id,
      extensionId: extensionId,
      capability: UICapability.pageProvider,
    );

UIProviderDefinition _routes({
  required String extensionId,
  required String id,
  required String path,
  required String pageProviderId,
  required int priority,
  bool includeNavigation = false,
}) {
  return _provider(
    id: id,
    extensionId: extensionId,
    capability: UICapability.routeRegistry,
    priority: priority,
    metadata: {
      'routes': [
        {
          'id': 'main',
          'path': path,
          'providerId': pageProviderId,
          'capability': UICapability.pageProvider,
        },
      ],
      if (includeNavigation)
        'navigationItems': [
          {'id': 'main', 'label': extensionId, 'route': path},
        ],
    },
  );
}

void main() {
  test('dashboard is a normal built-in navigation entry', () {
    final items = UINavigationRegistry.resolve(_snapshot(const []));
    final dashboard = items.singleWhere((item) => item.id == 'builtin.dashboard');
    expect(dashboard.route, AppRoutes.dashboard);
    expect(dashboard.panel, UINavigationPanel.main);
  });

  test('hides route.registry navigation entries that lost route arbitration', () {
    final snapshot = _snapshot([
      _routes(
        extensionId: 'winner',
        id: 'routes.winner',
        path: '/extensions-ui/shared',
        pageProviderId: 'page.winner',
        priority: 100,
        includeNavigation: true,
      ),
      _page('winner', 'page.winner'),
      _routes(
        extensionId: 'loser',
        id: 'routes.loser',
        path: '/extensions-ui/shared',
        pageProviderId: 'page.loser',
        priority: 1,
        includeNavigation: true,
      ),
      _page('loser', 'page.loser'),
    ]);

    final extensions = UINavigationRegistry.resolve(snapshot)
        .where((item) => item.extensionId != null)
        .toList();
    expect(extensions.map((item) => item.extensionId), ['winner']);
  });

  test('hides app.navigation links when the extension route is not effective', () {
    final snapshot = _snapshot([
      _provider(
        id: 'nav.loser',
        extensionId: 'loser',
        capability: UICapability.appNavigation,
        metadata: const {
          'navigationItems': [
            {
              'id': 'main',
              'label': 'Loser',
              'route': '/extensions-ui/shared',
            },
          ],
        },
      ),
      _routes(
        extensionId: 'loser',
        id: 'routes.loser',
        path: '/extensions-ui/shared',
        pageProviderId: 'page.loser',
        priority: 1,
      ),
      _page('loser', 'page.loser'),
      _routes(
        extensionId: 'winner',
        id: 'routes.winner',
        path: '/extensions-ui/shared',
        pageProviderId: 'page.winner',
        priority: 100,
      ),
      _page('winner', 'page.winner'),
    ]);

    expect(
      UINavigationRegistry.resolve(snapshot)
          .where((item) => item.extensionId == 'loser'),
      isEmpty,
    );
  });

  test('preserves the more panel contract for effective extension routes', () {
    final snapshot = _snapshot([
      _provider(
        id: 'nav.tools',
        extensionId: 'tools',
        capability: UICapability.appNavigation,
        metadata: const {
          'navigationItems': [
            {
              'id': 'diagnostics',
              'label': 'Diagnostics',
              'route': '/extensions-ui/tools',
              'panel': 'more',
            },
          ],
        },
      ),
      _routes(
        extensionId: 'tools',
        id: 'routes.tools',
        path: '/extensions-ui/tools',
        pageProviderId: 'page.tools',
        priority: 10,
      ),
      _page('tools', 'page.tools'),
    ]);

    final item = UINavigationRegistry.resolve(snapshot)
        .singleWhere((entry) => entry.extensionId == 'tools');
    expect(item.panel, UINavigationPanel.more);
  });
}

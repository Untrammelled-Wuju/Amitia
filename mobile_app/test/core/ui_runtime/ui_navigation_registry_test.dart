import 'package:amitia_app/app/app_routes.dart';
import 'package:amitia_app/core/ui_runtime/ui_navigation_registry.dart';
import 'package:amitia_app/core/ui_runtime/ui_provider.dart';
import 'package:amitia_app/core/ui_runtime/ui_route_registry.dart';
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
          {'id': 'main', 'label': extensionId, 'route': path, 'panel': 'main'},
        ],
    },
  );
}

void main() {
  test('empty provider route tables keep the builtin router signature', () {
    expect(uiRouteRegistrySignature(null), 'builtin');
    expect(uiRouteRegistrySignature(_snapshot(const [])), 'builtin');
  });

  test('shows channel messages only when a channel presentation exists', () {
    final empty = UINavigationRegistry.resolve(_snapshot(const []));
    expect(
      empty.any((item) => item.route == AppRoutes.channelMessages),
      isFalse,
    );
    final withChannel = UINavigationRegistry.resolve(
      _snapshot([
        _provider(
          id: 'channel.wechat.presentation',
          extensionId: 'com.amitia/channel-wechat',
          capability: UICapability.channelPresentation,
          metadata: const {
            'channelId': 'wechat_personal',
            'displayName': '个人微信',
          },
        ),
      ]),
    );
    expect(
      withChannel.any((item) => item.route == AppRoutes.channelMessages),
      isTrue,
    );
  });

  test('dashboard is not a drawer navigation entry', () {
    final items = UINavigationRegistry.resolve(_snapshot(const []));
    expect(
      items.any(
        (item) =>
            item.id == 'builtin.dashboard' || item.route == AppRoutes.dashboard,
      ),
      isFalse,
    );
  });

  test(
    'retired active task provider entries are ignored by core navigation',
    () {
      final items = UINavigationRegistry.resolve(
        _snapshot([
          _routes(
            extensionId: 'com.amitia/active-task',
            id: 'active-task-routes',
            path: AppRoutes.continuity,
            pageProviderId: 'continuity-page',
            priority: 100,
            includeNavigation: true,
          ),
          _page('com.amitia/active-task', 'continuity-page'),
        ]),
      );
      expect(
        items.where((item) => item.extensionId == 'com.amitia/active-task'),
        isEmpty,
      );
      expect(
        items.any((item) => item.route == AppRoutes.tasks && item.builtin),
        isTrue,
      );
      expect(
        UINavigationRegistry.resolve(
          _snapshot(const []),
        ).any((item) => item.route == AppRoutes.continuity),
        isFalse,
      );
      expect(
        UINavigationRegistry.resolve(
          _snapshot(const []),
        ).map((item) => item.route),
        containsAll(<String>[
          AppRoutes.tasks,
          AppRoutes.memoryHub,
          AppRoutes.characters,
        ]),
      );
    },
  );

  test(
    'separates primary, settings, profile, workspace and contextual items',
    () {
      final snapshot = _snapshot([
        _provider(
          id: 'placement-routes',
          extensionId: 'com.example/placement',
          capability: UICapability.routeRegistry,
          metadata: const {
            'routes': [
              {
                'id': 'hub',
                'path': '/hub',
                'providerId': 'placement-pages',
                'capability': UICapability.pageProvider,
              },
              {
                'id': 'settings',
                'path': '/storage',
                'providerId': 'placement-pages',
                'capability': UICapability.pageProvider,
              },
              {
                'id': 'profile',
                'path': '/logs',
                'providerId': 'placement-pages',
                'capability': UICapability.pageProvider,
              },
              {
                'id': 'workspace',
                'path': '/workspaces',
                'providerId': 'placement-pages',
                'capability': UICapability.pageProvider,
              },
              {
                'id': 'contextual',
                'path': '/import',
                'providerId': 'placement-pages',
                'capability': UICapability.pageProvider,
              },
            ],
            'navigationItems': [
              {
                'id': 'hub',
                'label': '业务中心',
                'route': '/hub',
                'group': 'character',
                'profile': true,
              },
              {
                'id': 'settings',
                'label': '存储维护',
                'route': '/storage',
                'group': 'settings',
              },
              {
                'id': 'profile',
                'label': '归档对话',
                'route': '/logs',
                'group': 'home',
              },
              {
                'id': 'workspace',
                'label': '工作区',
                'route': '/workspaces',
                'group': 'workspace',
              },
              {
                'id': 'contextual',
                'label': '聊天导入',
                'route': '/import',
                'group': 'character',
              },
            ],
          },
        ),
        _page('com.example/placement', 'placement-pages'),
      ]);
      final items = UINavigationRegistry.resolve(snapshot);
      expect(
        items.singleWhere((item) => item.route == '/hub').placement,
        UINavigationPlacement.primary,
      );
      expect(
        items.singleWhere((item) => item.route == '/storage').placement,
        UINavigationPlacement.settings,
      );
      expect(
        items.singleWhere((item) => item.route == '/logs').placement,
        UINavigationPlacement.profile,
      );
      expect(
        items.singleWhere((item) => item.route == '/workspaces').placement,
        UINavigationPlacement.workspace,
      );
      expect(
        items.singleWhere((item) => item.route == '/import').placement,
        UINavigationPlacement.contextual,
      );
      expect(
        UINavigationRegistry.resolvePrimary(
          snapshot,
        ).where((item) => item.extensionId != null).map((item) => item.route),
        ['/hub'],
      );
    },
  );

  test('mobile tabs only accept primary and profile placements', () {
    final snapshot = _snapshot([
      _provider(
        id: 'mobile-placement-routes',
        extensionId: 'com.example/mobile-placement',
        capability: UICapability.routeRegistry,
        metadata: const {
          'routes': [
            {
              'id': 'primary',
              'path': '/tasks',
              'providerId': 'mobile-placement-pages',
              'capability': UICapability.pageProvider,
            },
            {
              'id': 'profile',
              'path': '/settings',
              'providerId': 'mobile-placement-pages',
              'capability': UICapability.pageProvider,
            },
            {
              'id': 'settings',
              'path': '/runtime-debug',
              'providerId': 'mobile-placement-pages',
              'capability': UICapability.pageProvider,
            },
            {
              'id': 'workspace',
              'path': '/workspaces',
              'providerId': 'mobile-placement-pages',
              'capability': UICapability.pageProvider,
            },
          ],
          'navigationItems': [
            {
              'id': 'primary',
              'label': '任务',
              'route': '/tasks',
              'group': 'tasks',
              'tabLabel': '任务',
              'mobile': true,
            },
            {
              'id': 'profile',
              'label': '设置',
              'route': '/settings',
              'group': 'space',
              'tabLabel': '我的',
              'mobile': true,
              'profile': true,
            },
            {
              'id': 'settings',
              'label': '运行时调试',
              'route': '/runtime-debug',
              'group': 'settings',
              'mobile': true,
            },
            {
              'id': 'workspace',
              'label': '工作区',
              'route': '/workspaces',
              'group': 'workspace',
              'mobile': true,
            },
          ],
        },
      ),
      _page('com.example/mobile-placement', 'mobile-placement-pages'),
    ]);
    expect(
      UINavigationRegistry.resolveMobile(
        snapshot,
      ).where((item) => item.extensionId != null).map((item) => item.route),
      ['/settings'],
    );
  });

  test(
    'hides route.registry navigation entries that lost route arbitration',
    () {
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

      final extensions = UINavigationRegistry.resolve(
        snapshot,
      ).where((item) => item.extensionId != null).toList();
      expect(extensions.map((item) => item.extensionId), ['winner']);
    },
  );

  test('backs a section root navigation item with its effective child route', () {
    final snapshot = _snapshot([
      _provider(
        id: 'plugin-market-routes',
        extensionId: 'com.amitia/plugin-market',
        capability: UICapability.routeRegistry,
        priority: 260,
        metadata: const {
          'routes': [
            {
              'id': 'plugin-market',
              'path': '/extensions/market',
              'providerId': 'plugin-market-page-provider',
              'capability': UICapability.pageProvider,
            },
          ],
          'navigationItems': [
            {
              'id': 'extensions-hub',
              'label': '扩展中心',
              'route': AppRoutes.extensions,
              'placement': 'primary',
              'group': 'extensions',
              'order': 90,
            },
          ],
        },
      ),
      _page('com.amitia/plugin-market', 'plugin-market-page-provider'),
    ]);

    final item = UINavigationRegistry.resolve(
      snapshot,
    ).singleWhere((item) => item.route == AppRoutes.extensions);
    expect(item.extensionId, 'com.amitia/plugin-market');
    expect(item.placement, UINavigationPlacement.primary);
  });

  test(
    'hides app.navigation links when the extension route is not effective',
    () {
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
        UINavigationRegistry.resolve(
          snapshot,
        ).where((item) => item.extensionId == 'loser'),
        isEmpty,
      );
    },
  );

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

    final item = UINavigationRegistry.resolve(
      snapshot,
    ).singleWhere((entry) => entry.extensionId == 'tools');
    expect(item.panel, UINavigationPanel.more);
  });
}

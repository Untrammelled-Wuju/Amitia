import 'package:amitia_app/core/ui_runtime/channel_plugin_navigation.dart';
import 'package:amitia_app/core/ui_runtime/ui_provider.dart';
import 'package:amitia_app/core/ui_runtime/ui_workspace_tabs.dart';
import 'package:flutter_test/flutter_test.dart';

UIProviderDefinition _provider({
  required String id,
  required String extensionId,
  required String capability,
  int priority = 0,
  Map<String, UIProviderEntry> entries = const <String, UIProviderEntry>{
    'mobile': UIProviderEntry(type: UIProviderEntryType.declarative),
  },
  Map<String, dynamic> metadata = const <String, dynamic>{},
}) {
  return UIProviderDefinition(
    providerId: id,
    extensionId: extensionId,
    capability: capability,
    mode: UIProviderMode.augment,
    priority: priority,
    platforms: const <String>[],
    entries: entries,
    permissions: const <String>[],
    placement: UIProviderPlacement.device,
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
    context: const UIProviderResolveContext(
      platform: 'android',
      localRuntime: true,
      deviceOnline: true,
      deviceId: 'device',
    ),
    resolved: const <String, UIProviderDefinition>{},
    version: 1,
  );
}

void main() {
  test(
    'channel page entry requires presentation and effective provider route',
    () {
      const extensionId = 'com.amitia/channel-test';
      final snapshot = _snapshot([
        _provider(
          id: 'channel-presentation',
          extensionId: extensionId,
          capability: UICapability.channelPresentation,
          metadata: const {
            'channelId': 'test_channel',
            'displayName': '测试渠道',
            'order': 10,
          },
        ),
        _provider(
          id: 'channel-routes',
          extensionId: extensionId,
          capability: UICapability.routeRegistry,
          priority: 100,
          metadata: const {
            'navigationItems': [
              {
                'id': 'channel-page',
                'label': '测试渠道',
                'route': '/channels/test',
                'mobile': true,
              },
            ],
            'routes': [
              {
                'id': 'channel-page',
                'path': '/channels/test',
                'providerId': 'channel-page-provider',
                'capability': UICapability.pageProvider,
              },
            ],
          },
        ),
        _provider(
          id: 'channel-page-provider',
          extensionId: extensionId,
          capability: UICapability.pageProvider,
          entries: const {
            'mobile': UIProviderEntry(
              contributionId: 'channel-mobile-page',
              type: UIProviderEntryType.schemaRenderer,
            ),
          },
        ),
      ]);

      final items = ChannelPluginNavigationRegistry.resolve(snapshot);
      expect(items, hasLength(1));
      expect(items.single.route, '/channels/test');
      expect(items.single.label, '测试渠道');
      expect(items.single.channelId, 'test_channel');
      expect(items.single.extensionId, extensionId);
      expect(
        ChannelPluginNavigationRegistry.extensionIds(snapshot),
        contains(extensionId),
      );
    },
  );

  test(
    'presentation without a registered page route does not expose an entry',
    () {
      final snapshot = _snapshot([
        _provider(
          id: 'channel-presentation',
          extensionId: 'com.amitia/channel-test',
          capability: UICapability.channelPresentation,
          metadata: const {'channelId': 'test_channel', 'displayName': '测试渠道'},
        ),
      ]);

      expect(ChannelPluginNavigationRegistry.resolve(snapshot), isEmpty);
    },
  );

  test('workspace tabs use the generic provider registry contract', () {
    final snapshot = _snapshot([
      _provider(
        id: 'channel-workspace-tab',
        extensionId: 'com.amitia/channel-test',
        capability: UICapability.workspaceTab,
        priority: 200,
        entries: const {
          'mobile': UIProviderEntry(
            contributionId: 'channel-extra-tab',
            type: UIProviderEntryType.schemaRenderer,
          ),
        },
        metadata: const {
          'workspace': 'channel.workspace',
          'tabId': 'extra',
          'title': '插件扩展页',
          'routePrefix': '/channels/test',
          'contextMatch': {
            'channelId': ['test_channel'],
          },
          'order': 10,
        },
      ),
    ]);

    final tabs = UIWorkspaceTabRegistry.resolve(
      snapshot,
      workspace: 'channel.workspace',
      context: const {
        'channelId': 'test_channel',
        'channelExtensionId': 'com.amitia/channel-test',
      },
      route: '/channels/test',
      reservedTabIds: const {'plugin', 'messages', 'settings'},
    );

    expect(tabs, hasLength(1));
    expect(tabs.single.tabId, 'extra');
    expect(tabs.single.label, '插件扩展页');
    expect(tabs.single.providerId, 'channel-workspace-tab');
    expect(tabs.single.entry.contributionId, 'channel-extra-tab');
  });
}

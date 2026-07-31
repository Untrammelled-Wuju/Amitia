import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/app/drawer_route_state.dart';

void main() {
  group('DrawerRouteState', () {
    test('MainDrawerItem contains all expected items', () {
      expect(MainDrawerItem.values, contains(MainDrawerItem.chat));
      expect(MainDrawerItem.values, contains(MainDrawerItem.tasks));
      expect(MainDrawerItem.values, contains(MainDrawerItem.characters));
      expect(MainDrawerItem.values, contains(MainDrawerItem.memory));
      expect(MainDrawerItem.values, contains(MainDrawerItem.more));
      expect(MainDrawerItem.values, contains(MainDrawerItem.none));
    });

    test('MoreDrawerItem contains all expected items', () {
      expect(MoreDrawerItem.values, contains(MoreDrawerItem.extensions));
      expect(MoreDrawerItem.values, contains(MoreDrawerItem.workshop));
      expect(MoreDrawerItem.values, contains(MoreDrawerItem.gameCenter));
      expect(MoreDrawerItem.values, contains(MoreDrawerItem.channels));
      expect(MoreDrawerItem.values, contains(MoreDrawerItem.desktopPet));
      expect(MoreDrawerItem.values, contains(MoreDrawerItem.reminders));
      expect(MoreDrawerItem.values, contains(MoreDrawerItem.dashboard));
      expect(MoreDrawerItem.values, contains(MoreDrawerItem.chatLogs));
      expect(MoreDrawerItem.values, contains(MoreDrawerItem.chatImport));
      expect(MoreDrawerItem.values, contains(MoreDrawerItem.emotes));
      expect(MoreDrawerItem.values, contains(MoreDrawerItem.developer));
      expect(MoreDrawerItem.values, contains(MoreDrawerItem.none));
    });
  });

  group('isRouteFamily', () {
    test('matches exact route', () {
      expect(isRouteFamily('/chat', '/chat'), isTrue);
    });

    test('matches child route', () {
      expect(isRouteFamily('/chat/123', '/chat'), isTrue);
      expect(isRouteFamily('/characters/c1/voice', '/characters'), isTrue);
    });

    test('does not match unrelated route', () {
      expect(isRouteFamily('/chat', '/characters'), isFalse);
      expect(isRouteFamily('/characters', '/chat'), isFalse);
    });

    test('does not match partial prefix', () {
      expect(isRouteFamily('/chatroom', '/chat'), isFalse);
      expect(isRouteFamily('/channels', '/chat'), isFalse);
    });
  });

  group('resolveDrawerRouteState', () {
    test('chat route resolves to main panel with chat item', () {
      final state = resolveDrawerRouteState('/chat');
      expect(state.initialPanel, DrawerPanel.main);
      expect(state.mainItem, MainDrawerItem.chat);
      expect(state.moreItem, MoreDrawerItem.none);
      expect(state.settingsSelected, isFalse);
    });

    test('conversations route resolves to main panel with chat item', () {
      final state = resolveDrawerRouteState('/conversations');
      expect(state.initialPanel, DrawerPanel.main);
      expect(state.mainItem, MainDrawerItem.chat);
    });

    test('agent route resolves to main panel with tasks item', () {
      final state = resolveDrawerRouteState('/agent');
      expect(state.initialPanel, DrawerPanel.main);
      expect(state.mainItem, MainDrawerItem.tasks);
    });

    test('characters route resolves to main panel with characters item', () {
      final state = resolveDrawerRouteState('/characters');
      expect(state.initialPanel, DrawerPanel.main);
      expect(state.mainItem, MainDrawerItem.characters);
    });

    test('character detail route resolves to main panel with characters item', () {
      final state = resolveDrawerRouteState('/characters/c1');
      expect(state.mainItem, MainDrawerItem.characters);
    });

    test('memory route resolves to main panel with memory item', () {
      final state = resolveDrawerRouteState('/memory');
      expect(state.initialPanel, DrawerPanel.main);
      expect(state.mainItem, MainDrawerItem.memory);
    });

    test('memory child route resolves to main panel with memory item', () {
      final state = resolveDrawerRouteState('/memory/graph');
      expect(state.mainItem, MainDrawerItem.memory);
    });

    test('settings route resolves to main panel with settings selected', () {
      final state = resolveDrawerRouteState('/settings');
      expect(state.initialPanel, DrawerPanel.main);
      expect(state.settingsSelected, isTrue);
      expect(state.mainItem, MainDrawerItem.none);
    });

    test('settings child route resolves to main panel with settings selected', () {
      final state = resolveDrawerRouteState('/settings/about');
      expect(state.settingsSelected, isTrue);
    });

    test('settings toolbox route resolves to main panel with settings selected', () {
      final state = resolveDrawerRouteState('/settings/toolbox');
      expect(state.settingsSelected, isTrue);
    });

    test('extensions route resolves to more panel with extensions item', () {
      final state = resolveDrawerRouteState('/extensions');
      expect(state.initialPanel, DrawerPanel.more);
      expect(state.mainItem, MainDrawerItem.more);
      expect(state.moreItem, MoreDrawerItem.extensions);
    });

    test('extensions child route resolves to more panel with extensions item', () {
      final state = resolveDrawerRouteState('/extensions/mcp');
      expect(state.initialPanel, DrawerPanel.more);
      expect(state.moreItem, MoreDrawerItem.extensions);
    });

    test('workshop route resolves to more panel with workshop item', () {
      final state = resolveDrawerRouteState('/workshop');
      expect(state.initialPanel, DrawerPanel.more);
      expect(state.moreItem, MoreDrawerItem.workshop);
    });

    test('game-center route resolves to more panel with gameCenter item', () {
      final state = resolveDrawerRouteState('/game-center');
      expect(state.initialPanel, DrawerPanel.more);
      expect(state.moreItem, MoreDrawerItem.gameCenter);
    });

    test('channels route resolves to more panel with channels item', () {
      final state = resolveDrawerRouteState('/channels');
      expect(state.initialPanel, DrawerPanel.more);
      expect(state.moreItem, MoreDrawerItem.channels);
    });

    test('desktop-pet route resolves to more panel with desktopPet item', () {
      final state = resolveDrawerRouteState('/desktop-pet');
      expect(state.initialPanel, DrawerPanel.more);
      expect(state.moreItem, MoreDrawerItem.desktopPet);
    });

    test('reminders route resolves to more panel with reminders item', () {
      final state = resolveDrawerRouteState('/reminders');
      expect(state.initialPanel, DrawerPanel.more);
      expect(state.moreItem, MoreDrawerItem.reminders);
    });

    test('dashboard route resolves to more panel with dashboard item', () {
      final state = resolveDrawerRouteState('/dashboard');
      expect(state.initialPanel, DrawerPanel.more);
      expect(state.moreItem, MoreDrawerItem.dashboard);
    });

    test('chat-logs route resolves to more panel with chatLogs item', () {
      final state = resolveDrawerRouteState('/chat-logs');
      expect(state.initialPanel, DrawerPanel.more);
      expect(state.moreItem, MoreDrawerItem.chatLogs);
    });

    test('chat-import route resolves to more panel with chatImport item', () {
      final state = resolveDrawerRouteState('/chat-import');
      expect(state.initialPanel, DrawerPanel.more);
      expect(state.moreItem, MoreDrawerItem.chatImport);
    });

    test('emotes route resolves to more panel with emotes item', () {
      final state = resolveDrawerRouteState('/emotes');
      expect(state.initialPanel, DrawerPanel.more);
      expect(state.moreItem, MoreDrawerItem.emotes);
    });

    test('developer route resolves to more panel with developer item', () {
      final state = resolveDrawerRouteState('/developer');
      expect(state.initialPanel, DrawerPanel.more);
      expect(state.moreItem, MoreDrawerItem.developer);
    });

    test('unknown route resolves to main panel with none items', () {
      final state = resolveDrawerRouteState('/unknown-route');
      expect(state.initialPanel, DrawerPanel.main);
      expect(state.mainItem, MainDrawerItem.none);
      expect(state.moreItem, MoreDrawerItem.none);
      expect(state.settingsSelected, isFalse);
    });
  });
}

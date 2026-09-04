import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/app/drawer_route_state.dart';

void main() {
  group('DrawerRouteState', () {
    test('MainDrawerItem contains current single-panel navigation items', () {
      expect(MainDrawerItem.values, contains(MainDrawerItem.chat));
      expect(MainDrawerItem.values, contains(MainDrawerItem.characters));
      expect(MainDrawerItem.values, contains(MainDrawerItem.memory));
      expect(MainDrawerItem.values, contains(MainDrawerItem.devices));
      expect(MainDrawerItem.values, contains(MainDrawerItem.extensions));
      expect(MainDrawerItem.values, contains(MainDrawerItem.workshop));
      expect(MainDrawerItem.values, contains(MainDrawerItem.none));
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
    test('chat route selects chat', () {
      final state = resolveDrawerRouteState('/chat');
      expect(state.initialPanel, DrawerPanel.main);
      expect(state.mainItem, MainDrawerItem.chat);
      expect(state.settingsSelected, isFalse);
    });

    test('conversations route belongs to chat', () {
      final state = resolveDrawerRouteState('/conversations');
      expect(state.mainItem, MainDrawerItem.chat);
    });

    test('agent route no longer has a drawer item', () {
      final state = resolveDrawerRouteState('/agent');
      expect(state.initialPanel, DrawerPanel.main);
      expect(state.mainItem, MainDrawerItem.none);
    });

    test('characters family selects characters', () {
      expect(resolveDrawerRouteState('/characters').mainItem, MainDrawerItem.characters);
      expect(resolveDrawerRouteState('/characters/c1').mainItem, MainDrawerItem.characters);
    });

    test('memory family selects memory', () {
      expect(resolveDrawerRouteState('/memory').mainItem, MainDrawerItem.memory);
      expect(resolveDrawerRouteState('/memory/graph').mainItem, MainDrawerItem.memory);
    });

    test('devices route selects the first-level devices item', () {
      final state = resolveDrawerRouteState('/settings/devices');
      expect(state.mainItem, MainDrawerItem.devices);
      expect(state.settingsSelected, isFalse);
    });

    test('extensions family is a main drawer item', () {
      expect(resolveDrawerRouteState('/extensions').mainItem, MainDrawerItem.extensions);
      expect(resolveDrawerRouteState('/extensions/mcp').mainItem, MainDrawerItem.extensions);
      expect(resolveDrawerRouteState('/extension/page/demo').mainItem, MainDrawerItem.extensions);
    });

    test('workshop family is a main drawer item', () {
      expect(resolveDrawerRouteState('/workshop').mainItem, MainDrawerItem.workshop);
      expect(resolveDrawerRouteState('/workshop/skills').mainItem, MainDrawerItem.workshop);
    });

    test('settings-owned pages keep settings selected', () {
      for (final route in <String>[
        '/settings',
        '/settings/about',
        '/settings/toolbox',
        '/channels',
        '/reminders',
        '/dashboard',
        '/chat-logs',
        '/chat-import',
        '/emotes',
      ]) {
        final state = resolveDrawerRouteState(route);
        expect(state.initialPanel, DrawerPanel.main, reason: route);
        expect(state.settingsSelected, isTrue, reason: route);
      }
    });

    test('removed drawer destinations remain unselected', () {
      for (final route in <String>['/game-center', '/desktop-pet', '/developer']) {
        final state = resolveDrawerRouteState(route);
        expect(state.mainItem, MainDrawerItem.none, reason: route);
        expect(state.moreItem, MoreDrawerItem.none, reason: route);
      }
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

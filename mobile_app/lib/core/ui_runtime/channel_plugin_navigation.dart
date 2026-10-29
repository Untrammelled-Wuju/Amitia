import 'channel_presentation.dart';
import 'ui_navigation_registry.dart';
import 'ui_provider.dart';

class ChannelPluginNavigationItem {
  final String id;
  final String route;
  final String label;
  final String channelId;
  final String extensionId;
  final int order;

  const ChannelPluginNavigationItem({
    required this.id,
    required this.route,
    required this.label,
    required this.channelId,
    required this.extensionId,
    required this.order,
  });
}

abstract final class ChannelPluginNavigationRegistry {
  static Set<String> extensionIds(UIProviderSnapshot? snapshot) {
    return resolve(snapshot).map((item) => item.extensionId).toSet();
  }

  static List<ChannelPluginNavigationItem> resolve(
    UIProviderSnapshot? snapshot,
  ) {
    final presentations = ChannelPresentationRegistry.resolve(snapshot);
    if (presentations.isEmpty || snapshot == null) {
      return const <ChannelPluginNavigationItem>[];
    }
    final navigationItems = UINavigationRegistry.resolve(snapshot)
        .where(
          (item) =>
              item.extensionId != null && item.route.startsWith('/channels/'),
        )
        .toList(growable: false);
    final result = <ChannelPluginNavigationItem>[];
    for (final presentation in presentations) {
      final navigation = navigationItems
          .where((item) => item.extensionId == presentation.extensionId)
          .firstOrNull;
      final route = navigation?.route ?? '';
      if (route.isEmpty) continue;
      result.add(
        ChannelPluginNavigationItem(
          id: 'channel:${presentation.extensionId}:${presentation.channelId}',
          route: route,
          label: presentation.displayName,
          channelId: presentation.channelId,
          extensionId: presentation.extensionId,
          order: presentation.order,
        ),
      );
    }
    result.sort((a, b) {
      final order = a.order.compareTo(b.order);
      if (order != 0) return order;
      final label = a.label.compareTo(b.label);
      return label != 0 ? label : a.channelId.compareTo(b.channelId);
    });
    return List<ChannelPluginNavigationItem>.unmodifiable(result);
  }

  static ChannelPluginNavigationItem? findByRoute(
    UIProviderSnapshot? snapshot,
    String route,
  ) {
    return resolve(snapshot).where((item) => item.route == route).firstOrNull;
  }
}

extension _FirstOrNull<T> on Iterable<T> {
  T? get firstOrNull {
    final iterator = this.iterator;
    return iterator.moveNext() ? iterator.current : null;
  }
}

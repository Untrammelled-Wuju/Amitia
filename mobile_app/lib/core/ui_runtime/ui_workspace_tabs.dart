import 'ui_provider.dart';

class UIWorkspaceTabDefinition {
  final String providerId;
  final String extensionId;
  final String? moduleId;
  final String tabId;
  final String label;
  final String icon;
  final int order;
  final bool keepMounted;
  final UIProviderDefinition provider;
  final UIProviderEntry entry;

  const UIWorkspaceTabDefinition({
    required this.providerId,
    required this.extensionId,
    required this.moduleId,
    required this.tabId,
    required this.label,
    required this.icon,
    required this.order,
    required this.keepMounted,
    required this.provider,
    required this.entry,
  });
}

abstract final class UIWorkspaceTabRegistry {
  static List<UIWorkspaceTabDefinition> resolve(
    UIProviderSnapshot? snapshot, {
    required String workspace,
    required Map<String, dynamic> context,
    required String route,
    Set<String> reservedTabIds = const <String>{},
  }) {
    if (snapshot == null) return const <UIWorkspaceTabDefinition>[];
    final platform = currentUIPlatform();
    final candidates = <UIWorkspaceTabDefinition>[];
    for (final provider in snapshot.providers) {
      if (!provider.enabled ||
          provider.capability != UICapability.workspaceTab ||
          !provider.compatibleWith(snapshot.context, platform)) {
        continue;
      }
      final metadata = provider.metadata;
      if ((metadata['workspace'] ?? '').toString().trim() != workspace) {
        continue;
      }
      final routePrefix = (metadata['routePrefix'] ?? '').toString().trim();
      if (routePrefix.isNotEmpty &&
          route != routePrefix &&
          !route.startsWith('$routePrefix/')) {
        continue;
      }
      if (!_contextMatches(metadata['contextMatch'], context)) continue;
      final entry = provider.entryFor(platform);
      final tabId = (metadata['tabId'] ?? '').toString().trim();
      if (entry == null || tabId.isEmpty || reservedTabIds.contains(tabId)) {
        continue;
      }
      candidates.add(
        UIWorkspaceTabDefinition(
          providerId: provider.providerId,
          extensionId: provider.extensionId,
          moduleId: provider.moduleId,
          tabId: tabId,
          label: (metadata['title'] ?? tabId).toString().trim(),
          icon: (metadata['icon'] ?? 'extension').toString().trim(),
          order: _number(metadata['order'], provider.priority),
          keepMounted: metadata['keepMounted'] != false,
          provider: provider,
          entry: entry,
        ),
      );
    }
    candidates.sort((left, right) {
      final order = right.order.compareTo(left.order);
      if (order != 0) return order;
      final priority = right.provider.priority.compareTo(
        left.provider.priority,
      );
      if (priority != 0) return priority;
      final tab = left.tabId.compareTo(right.tabId);
      return tab != 0 ? tab : left.providerId.compareTo(right.providerId);
    });
    final seen = <String>{};
    return List<UIWorkspaceTabDefinition>.unmodifiable(
      candidates.where((tab) => seen.add(tab.tabId)),
    );
  }

  static bool _contextMatches(dynamic rawMatch, Map<String, dynamic> context) {
    if (rawMatch is! Map) return true;
    for (final entry in rawMatch.entries) {
      final actual = (context[entry.key] ?? '').toString().trim();
      final expected = entry.value is List
          ? (entry.value as List).map((item) => item.toString().trim())
          : <String>[entry.value.toString().trim()];
      if (!expected.contains(actual)) return false;
    }
    return true;
  }

  static int _number(dynamic value, int fallback) {
    if (value is num) return value.toInt();
    return int.tryParse(value?.toString() ?? '') ?? fallback;
  }
}

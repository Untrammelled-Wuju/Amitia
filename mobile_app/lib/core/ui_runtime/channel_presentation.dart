import 'ui_provider.dart';

class ChannelPresentationDescriptor {
  final String providerId;
  final String extensionId;
  final String channelId;
  final String displayName;
  final String icon;
  final int order;
  final int priority;
  final Map<String, dynamic> capabilities;

  const ChannelPresentationDescriptor({
    required this.providerId,
    required this.extensionId,
    required this.channelId,
    required this.displayName,
    required this.icon,
    required this.order,
    required this.priority,
    required this.capabilities,
  });
}

abstract final class ChannelPresentationRegistry {
  static List<ChannelPresentationDescriptor> resolve(
    UIProviderSnapshot? snapshot,
  ) {
    if (snapshot == null) return const <ChannelPresentationDescriptor>[];
    final platform = currentUIPlatform();
    final providers =
        snapshot.providers
            .where(
              (provider) =>
                  provider.enabled &&
                  !provider.builtin &&
                  provider.capability == UICapability.channelPresentation &&
                  provider.compatibleWith(snapshot.context, platform),
            )
            .toList()
          ..sort(
            (a, b) => b.priority.compareTo(a.priority) != 0
                ? b.priority.compareTo(a.priority)
                : a.providerId.compareTo(b.providerId),
          );
    final byChannel = <String, ChannelPresentationDescriptor>{};
    for (final provider in providers) {
      final channelId = _text(provider.metadata['channelId']);
      if (channelId.isEmpty || byChannel.containsKey(channelId)) continue;
      final rawCapabilities = provider.metadata['capabilities'];
      byChannel[channelId] = ChannelPresentationDescriptor(
        providerId: provider.providerId,
        extensionId: provider.extensionId,
        channelId: channelId,
        displayName: _text(provider.metadata['displayName']).isEmpty
            ? channelId
            : _text(provider.metadata['displayName']),
        icon: _text(provider.metadata['icon']).isEmpty
            ? 'channel'
            : _text(provider.metadata['icon']),
        order: _integer(provider.metadata['order'], 1000),
        priority: provider.priority,
        capabilities: rawCapabilities is Map
            ? Map<String, dynamic>.from(rawCapabilities)
            : const <String, dynamic>{},
      );
    }
    final result = byChannel.values.toList()
      ..sort((a, b) {
        final order = a.order.compareTo(b.order);
        if (order != 0) return order;
        final name = a.displayName.compareTo(b.displayName);
        return name != 0 ? name : a.channelId.compareTo(b.channelId);
      });
    return List<ChannelPresentationDescriptor>.unmodifiable(result);
  }

  static String _text(dynamic value) => value?.toString().trim() ?? '';

  static int _integer(dynamic value, int fallback) =>
      value is num && value.isFinite ? value.toInt() : fallback;
}

import 'ui_provider.dart';

/// Resolves selector-specific renderers from every enabled compatible plugin.
/// Selector-less renderers remain profile-global replacements.
abstract final class UIMessageRendererRegistry {
  static UIProviderDefinition? resolve(
    UIProviderSnapshot? snapshot, {
    required String messageType,
    String? role,
    String? mimeType,
    String? extensionType,
  }) {
    if (snapshot == null) return null;
    final platform = currentUIPlatform();
    final candidates = snapshot.providers
        .where((provider) =>
            provider.enabled &&
            !provider.builtin &&
            provider.capability == UICapability.conversationMessageRenderer &&
            provider.compatibleWith(snapshot.context, platform) &&
            _hasSelectors(provider) &&
            _supports(provider, messageType, role, mimeType, extensionType))
        .toList()
      ..sort((a, b) {
        final aScore = _score(a);
        final bScore = _score(b);
        final specificity = bScore.compareTo(aScore);
        if (specificity != 0) return specificity;
        return a.providerId.compareTo(b.providerId);
      });
    if (candidates.isNotEmpty) return candidates.first;

    final selected = snapshot.resolve(UICapability.conversationMessageRenderer);
    if (selected != null &&
        selected.enabled &&
        selected.compatibleWith(snapshot.context, platform) &&
        (selected.builtin || !_hasSelectors(selected))) {
      return selected;
    }
    return null;
  }

  static bool _hasSelectors(UIProviderDefinition provider) {
    final metadata = provider.metadata;
    return <String>['messageTypes', 'roles', 'mimeTypes', 'extensionTypes']
        .any((key) => (metadata[key] as List?)?.isNotEmpty == true);
  }

  static int _score(UIProviderDefinition provider) {
    final metadata = provider.metadata;
    var specificity = provider.priority * 100;
    if ((metadata['messageTypes'] as List?)?.isNotEmpty == true) specificity += 8;
    if ((metadata['roles'] as List?)?.isNotEmpty == true) specificity += 4;
    if ((metadata['mimeTypes'] as List?)?.isNotEmpty == true) specificity += 4;
    if ((metadata['extensionTypes'] as List?)?.isNotEmpty == true) specificity += 8;
    return specificity;
  }

  static bool _supports(
    UIProviderDefinition provider,
    String messageType,
    String? role,
    String? mimeType,
    String? extensionType,
  ) {
    final metadata = provider.metadata;
    Set<String> strings(String key) =>
        ((metadata[key] as List?) ?? const <dynamic>[])
            .map((e) => e.toString().toLowerCase())
            .toSet();
    final messageTypes = strings('messageTypes');
    final roles = strings('roles');
    final mimeTypes = strings('mimeTypes');
    final extensionTypes = strings('extensionTypes');
    final normalizedType = messageType.toLowerCase();
    final normalizedRole = role?.toLowerCase();
    final normalizedMime = mimeType?.toLowerCase();
    final normalizedExtension = extensionType?.toLowerCase();

    if (messageTypes.isNotEmpty &&
        !messageTypes.contains('*') &&
        !messageTypes.contains(normalizedType)) return false;
    if (roles.isNotEmpty &&
        !roles.contains('*') &&
        (normalizedRole == null || !roles.contains(normalizedRole))) return false;
    if (extensionTypes.isNotEmpty &&
        !extensionTypes.contains('*') &&
        (normalizedExtension == null || !extensionTypes.contains(normalizedExtension))) return false;
    if (mimeTypes.isNotEmpty) {
      if (normalizedMime == null) return false;
      final matched = mimeTypes.any((pattern) {
        if (pattern == '*/*' || pattern == '*') return true;
        if (pattern.endsWith('/*')) {
          return normalizedMime.startsWith(pattern.substring(0, pattern.length - 1));
        }
        return pattern == normalizedMime;
      });
      if (!matched) return false;
    }
    return true;
  }
}

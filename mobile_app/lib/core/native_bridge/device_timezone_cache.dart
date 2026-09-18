/// Process-wide cache of the device's IANA timezone identifier.
///
/// The native providers return canonical IANA values such as
/// `Asia/Shanghai` or `America/Los_Angeles`. HTTP transport reads this cache
/// synchronously so every request can consistently send X-Device-Timezone.
final class DeviceTimezoneCache {
  DeviceTimezoneCache._();

  static String _ianaTimezone = '';

  static String get ianaTimezone => _ianaTimezone;

  static bool get hasValue => _looksLikeIanaTimezone(_ianaTimezone);

  static void update(String value) {
    final candidate = value.trim();
    if (_looksLikeIanaTimezone(candidate)) {
      _ianaTimezone = candidate;
    }
  }

  static bool _looksLikeIanaTimezone(String value) {
    if (value == 'UTC') return true;
    if (value.isEmpty || value.length > 128 || !value.contains('/')) {
      return false;
    }
    if (value.contains('..') || value.startsWith('/') || value.endsWith('/')) {
      return false;
    }
    final parts = value.split('/');
    if (parts.length < 2 || parts.any((part) => part.isEmpty)) {
      return false;
    }
    final validPart = RegExp(r'^[A-Za-z0-9._+-]+$');
    return parts.every(validPart.hasMatch);
  }
}

import 'ui_provider.dart';

/// Evaluates the portable visibility subset used by server-driven UI
/// contributions. Keep this aligned with the desktop extension UI store so a
/// contribution is neither exposed nor shadowed differently across clients.
bool matchesMobileUIContributionVisibility(
  UIContributionSnapshotEntry item,
  Map<String, dynamic> context, {
  String? platform,
}) {
  final visibility = item.visibility;
  if (visibility.isEmpty) return true;

  final platforms = _strings(
    visibility['platforms'] ?? visibility['platform'],
  ).toSet();
  final activePlatform =
      (platform ?? context['platform']?.toString() ?? currentUIPlatform()).trim();
  if (platforms.isNotEmpty && !platforms.contains(activePlatform)) {
    return false;
  }

  final required = _strings(
    visibility['required_context'] ?? visibility['requiredContext'],
  );
  for (final path in required) {
    if (!_hasPath(context, path)) return false;
  }

  final messageTypes = _strings(
    visibility['message_types'] ?? visibility['messageTypes'],
  ).toSet();
  if (messageTypes.isNotEmpty) {
    final actual = (context['messageType'] ??
            context['type'] ??
            _lookup(context, 'message.type'))
        ?.toString();
    if (actual == null || !messageTypes.contains(actual)) return false;
  }

  final conditions = (visibility['conditions'] as List?) ?? const <dynamic>[];
  for (final raw in conditions.whereType<Map>()) {
    final condition = raw.cast<String, dynamic>();
    final field = (condition['field'] ?? '').toString().trim();
    if (field.isEmpty) return false;
    final actual = _lookup(context, field);
    final expected = condition['value'];
    final matches = switch ((condition['operator'] ?? '==').toString()) {
      '==' || 'eq' => actual == expected,
      '!=' || 'ne' => actual != expected,
      'in' => expected is List && expected.contains(actual),
      'not_in' => expected is List && !expected.contains(actual),
      'not_null' => actual != null,
      'is_null' => actual == null,
      'contains' =>
        actual is String && expected is String && actual.contains(expected),
      _ => false,
    };
    if (!matches) return false;
  }
  return true;
}

bool _hasPath(Map<String, dynamic> context, String path) {
  if (path.trim().isEmpty) return false;
  dynamic current = context;
  for (final segment in path.split('.')) {
    if (current is! Map || !current.containsKey(segment)) return false;
    current = current[segment];
  }
  return true;
}

dynamic _lookup(Map<String, dynamic> context, String path) {
  if (path.trim().isEmpty) return null;
  dynamic current = context;
  for (final segment in path.split('.')) {
    if (current is! Map || !current.containsKey(segment)) return null;
    current = current[segment];
  }
  return current;
}

List<String> _strings(dynamic value) {
  if (value is! List) return const <String>[];
  return value
      .map((item) => item.toString().trim())
      .where((item) => item.isNotEmpty)
      .toList(growable: false);
}

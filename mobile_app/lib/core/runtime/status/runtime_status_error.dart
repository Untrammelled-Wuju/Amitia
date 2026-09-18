enum RuntimeStatusErrorSource {
  runtime,
  manifest,
  backendConnection,
  http,
  webSocket,
  consistency,
}

final class RuntimeStatusError {
  final RuntimeStatusErrorSource source;
  final String code;
  final String message;
  final Map<String, String> details;

  const RuntimeStatusError({
    required this.source,
    required this.code,
    required this.message,
    this.details = const <String, String>{},
  });

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is RuntimeStatusError &&
          source == other.source &&
          code == other.code &&
          message == other.message &&
          _mapEquals(details, other.details);

  @override
  int get hashCode =>
      source.hashCode ^
      code.hashCode ^
      message.hashCode ^
      Object.hashAllUnordered(
        details.entries.map((e) => Object.hash(e.key, e.value)),
      );

  @override
  String toString() {
    final detailText = details.isEmpty ? '' : ', details=$details';
    return 'RuntimeStatusError(source=${source.name}, code=$code, '
        'message=$message$detailText)';
  }
}

bool _mapEquals(Map<String, String> a, Map<String, String> b) {
  if (identical(a, b)) return true;
  if (a.length != b.length) return false;
  for (final entry in a.entries) {
    if (b[entry.key] != entry.value) return false;
  }
  return true;
}

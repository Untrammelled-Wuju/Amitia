class RuntimeBridgeError {
  final String code;
  final String message;
  final bool retryable;
  final Map<String, String> details;

  const RuntimeBridgeError({
    required this.code,
    required this.message,
    required this.retryable,
    this.details = const <String, String>{},
  });

  factory RuntimeBridgeError.fromMap(Map<String, dynamic>? map) {
    if (map == null) {
      return const RuntimeBridgeError(
        code: 'BRIDGE_ERROR_MISSING',
        message: 'Runtime error payload is missing.',
        retryable: false,
      );
    }

    final rawCode = map['code'];
    final normalizedCode = rawCode is String && rawCode.trim().isNotEmpty
        ? rawCode.trim()
        : 'BRIDGE_ERROR_INVALID';
    final rawMessage = map['message'];
    final normalizedMessage =
        rawMessage is String && rawMessage.trim().isNotEmpty
        ? rawMessage.trim()
        : 'Runtime error: $normalizedCode';

    return RuntimeBridgeError(
      code: normalizedCode,
      message: normalizedMessage,
      retryable: map['retryable'] is bool ? map['retryable'] as bool : false,
      details: map['details'] is Map
          ? Map<String, dynamic>.from(
              map['details'] as Map,
            ).map((key, value) => MapEntry(key, value.toString()))
          : const <String, String>{},
    );
  }

  static RuntimeBridgeError? tryFromMap(Map<String, dynamic>? map) {
    if (map == null) return null;
    return RuntimeBridgeError.fromMap(map);
  }

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is RuntimeBridgeError &&
          code == other.code &&
          message == other.message &&
          retryable == other.retryable &&
          _mapEquals(details, other.details);

  @override
  int get hashCode =>
      code.hashCode ^
      message.hashCode ^
      retryable.hashCode ^
      Object.hashAllUnordered(
        details.entries.map((e) => Object.hash(e.key, e.value)),
      );

  @override
  String toString() {
    final detailText = details.isEmpty ? '' : ', details=$details';
    return 'RuntimeBridgeError(code=$code, retryable=$retryable, '
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

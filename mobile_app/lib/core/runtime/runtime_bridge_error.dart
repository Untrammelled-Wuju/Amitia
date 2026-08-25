class RuntimeBridgeError {
  final String code;
  final String message;
  final bool retryable;

  const RuntimeBridgeError({
    required this.code,
    required this.message,
    required this.retryable,
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
    final normalizedMessage = rawMessage is String && rawMessage.trim().isNotEmpty
        ? rawMessage.trim()
        : 'Runtime error: $normalizedCode';

    return RuntimeBridgeError(
      code: normalizedCode,
      message: normalizedMessage,
      retryable: map['retryable'] is bool ? map['retryable'] as bool : false,
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
          retryable == other.retryable;

  @override
  int get hashCode => code.hashCode ^ message.hashCode ^ retryable.hashCode;
}

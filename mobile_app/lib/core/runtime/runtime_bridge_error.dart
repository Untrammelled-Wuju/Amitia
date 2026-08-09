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
        code: 'UNKNOWN',
        message: 'Unknown error',
        retryable: false,
      );
    }
    return RuntimeBridgeError(
      code: map['code'] as String? ?? 'UNKNOWN',
      message: map['message'] as String? ?? 'Unknown error',
      retryable: map['retryable'] as bool? ?? false,
    );
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

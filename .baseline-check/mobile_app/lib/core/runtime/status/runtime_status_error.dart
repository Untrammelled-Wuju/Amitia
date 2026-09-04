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

  const RuntimeStatusError({
    required this.source,
    required this.code,
    required this.message,
  });

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is RuntimeStatusError &&
          source == other.source &&
          code == other.code &&
          message == other.message;

  @override
  int get hashCode => source.hashCode ^ code.hashCode ^ message.hashCode;
}

enum BackendConnectionErrorCode {
  RUNTIME_NOT_READY,
  BACKEND_NOT_READY,
  ENDPOINT_UNAVAILABLE,
  ENDPOINT_INVALID,
  CREDENTIAL_UNAVAILABLE,
  CREDENTIAL_INVALID,
  GENERATION_INVALID,
  BRIDGE_UNAVAILABLE,
  BRIDGE_PAYLOAD_INVALID,
  UNSUPPORTED_CONNECTION_MODE,
  INTERNAL_ERROR,
}

class BackendConnectionError {
  final BackendConnectionErrorCode code;
  final String message;
  BackendConnectionError(this.code, this.message);

  @override
  String toString() => 'BackendConnectionError(${code.name}: $message)';
}

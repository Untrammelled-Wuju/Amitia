import 'backend_transport_error_code.dart';

class BackendTransportError implements Exception {
  final BackendTransportErrorCode code;
  final String? method;
  final String? path;
  final int? statusCode;
  final int? generation;
  final Object? cause;

  BackendTransportError({
    required this.code,
    this.method,
    this.path,
    this.statusCode,
    this.generation,
    this.cause,
  });

  @override
  String toString() {
    final buffer = StringBuffer('BackendTransportError(');
    buffer.write('code: ${code.name}');
    if (method != null) buffer.write(', method: $method');
    if (path != null) buffer.write(', path: $path');
    if (statusCode != null) buffer.write(', statusCode: $statusCode');
    if (generation != null) buffer.write(', generation: $generation');
    buffer.write(')');
    return buffer.toString();
  }
}

import 'backend_connection_config.dart';

class BackendUriBuilder {
  Uri httpBase(BackendConnectionConfig config) {
    return Uri(
      scheme: config.endpoint.httpScheme,
      host: config.endpoint.host,
      port: config.endpoint.port,
    );
  }

  Uri http(
    BackendConnectionConfig config,
    String path, {
    Map<String, dynamic>? queryParameters,
  }) {
    _validatePath(path);
    return Uri(
      scheme: config.endpoint.httpScheme,
      host: config.endpoint.host,
      port: config.endpoint.port,
      path: path,
      queryParameters: (queryParameters?.isEmpty ?? true) ? null : queryParameters,
    );
  }

  Uri webSocketBase(BackendConnectionConfig config) {
    return Uri(
      scheme: config.endpoint.webSocketScheme,
      host: config.endpoint.host,
      port: config.endpoint.port,
    );
  }

  Uri webSocket(
    BackendConnectionConfig config,
    String path, {
    Map<String, dynamic>? queryParameters,
  }) {
    _validatePath(path);
    return Uri(
      scheme: config.endpoint.webSocketScheme,
      host: config.endpoint.host,
      port: config.endpoint.port,
      path: path,
      queryParameters: (queryParameters?.isEmpty ?? true) ? null : queryParameters,
    );
  }

  void _validatePath(String path) {
    if (path.isEmpty) throw ArgumentError('path must not be empty');
    if (!path.startsWith('/')) throw ArgumentError.value(path, 'path', 'must start with /');
    if (path.contains('://')) throw ArgumentError.value(path, 'path', 'must not contain scheme');
    if (path.contains('?')) throw ArgumentError.value(path, 'path', 'must not contain query');
    if (path.contains('#')) throw ArgumentError.value(path, 'path', 'must not contain fragment');
    if (path.contains('\u0000')) throw ArgumentError.value(path, 'path', 'must not contain NUL');
    if (path.contains('\r')) throw ArgumentError.value(path, 'path', 'must not contain CR');
    if (path.contains('\n')) throw ArgumentError.value(path, 'path', 'must not contain LF');
  }
}

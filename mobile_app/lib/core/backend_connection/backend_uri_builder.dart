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
      queryParameters: _normalizeQueryParameters(queryParameters),
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
      queryParameters: _normalizeQueryParameters(queryParameters),
    );
  }

  Map<String, dynamic>? _normalizeQueryParameters(
    Map<String, dynamic>? queryParameters,
  ) {
    if (queryParameters == null || queryParameters.isEmpty) return null;
    final normalized = <String, dynamic>{};
    for (final entry in queryParameters.entries) {
      final value = entry.value;
      if (value == null) continue;
      if (value is String) {
        normalized[entry.key] = value;
      } else if (value is Iterable) {
        final values = value
            .where((item) => item != null)
            .map((item) => item.toString())
            .toList(growable: false);
        if (values.isNotEmpty) normalized[entry.key] = values;
      } else {
        normalized[entry.key] = value.toString();
      }
    }
    return normalized.isEmpty ? null : normalized;
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

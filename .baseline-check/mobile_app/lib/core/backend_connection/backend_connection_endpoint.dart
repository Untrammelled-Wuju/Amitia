class BackendConnectionEndpoint {
  final String host;
  final int port;
  final String httpScheme;
  final String webSocketScheme;
  final String livenessPath;
  final String readinessPath;

  BackendConnectionEndpoint({
    required this.host,
    required this.port,
    required this.httpScheme,
    required this.webSocketScheme,
    required this.livenessPath,
    required this.readinessPath,
  }) {
    if (host.isEmpty) throw ArgumentError('host must not be empty');
    if (host == 'localhost') throw ArgumentError.value(host, 'host', 'must not be localhost');
    if (host == '0.0.0.0') throw ArgumentError.value(host, 'host', 'must not be 0.0.0.0');
    if (host == '::') throw ArgumentError.value(host, 'host', 'must not be ::');
    if (host.contains('://')) throw ArgumentError.value(host, 'host', 'must not contain scheme');
    if (host.contains(':')) throw ArgumentError.value(host, 'host', 'must not contain port');
    if (host.contains('/')) throw ArgumentError.value(host, 'host', 'must not contain path');
    if (host.contains('?')) throw ArgumentError.value(host, 'host', 'must not contain query');
    if (host.contains('#')) throw ArgumentError.value(host, 'host', 'must not contain fragment');
    if (host.contains('\u0000')) throw ArgumentError.value(host, 'host', 'must not contain NUL');
    if (port < 1 || port > 65535) throw ArgumentError.value(port, 'port', 'must be in range 1..65535');
    if (httpScheme != 'http' && httpScheme != 'https') {
      throw ArgumentError.value(httpScheme, 'httpScheme', 'must be http or https');
    }
    final expectedWs = httpScheme == 'http' ? 'ws' : 'wss';
    if (webSocketScheme != expectedWs) {
      throw ArgumentError.value(webSocketScheme, 'webSocketScheme', 'must match httpScheme');
    }
    if (!livenessPath.startsWith('/')) {
      throw ArgumentError.value(livenessPath, 'livenessPath', 'must start with /');
    }
    if (!readinessPath.startsWith('/')) {
      throw ArgumentError.value(readinessPath, 'readinessPath', 'must start with /');
    }
    for (final p in [livenessPath, readinessPath]) {
      if (p.contains('://') || p.contains('?') || p.contains('#')) {
        throw ArgumentError.value(p, 'path', 'must not contain scheme, query, or fragment');
      }
      if (p.contains('\u0000')) throw ArgumentError.value(p, 'path', 'must not contain NUL');
    }
  }
}

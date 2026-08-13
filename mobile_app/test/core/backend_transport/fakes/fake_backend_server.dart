import 'dart:convert';
import 'dart:io';

class FakeBackendServer {
  late HttpServer _server;
  late int port;
  final List<RecordedRequest> requests = [];
  bool requireToken = true;
  String validToken = 'a' * 32;
  int authFailures = 0;
  bool healthy = true;

  Future<void> start() async {
    _server = await HttpServer.bind('127.0.0.1', 0);
    port = _server.port;
    _server.listen(_handleRequest);
  }

  Future<void> _handleRequest(HttpRequest request) async {
    final path = request.uri.path;
    final method = request.method;
    final headers = <String, String>{};
    request.headers.forEach((name, values) {
      headers[name] = values.join(', ');
    });

    requests.add(RecordedRequest(
      method: method,
      path: path,
      queryParameters: request.uri.queryParameters,
      headers: headers,
      body: null,
    ));

    final isWsUpgrade = headers['upgrade']?.toLowerCase() == 'websocket';
    if (isWsUpgrade && path.startsWith('/ws')) {
      await _handleWebSocketUpgrade(request, headers);
      return;
    }

    if (requireToken) {
      final token = headers['x-amitia-local-token'];
      if (token == validToken) {
        // OK
      } else {
        authFailures++;
        request.response.statusCode = 401;
        request.response.headers.contentType = ContentType.json;
        request.response.write(jsonEncode({
          'code': 401,
          'message': 'Unauthorized',
        }));
        await request.response.close();
        return;
      }
    }

    if (path == '/readyz' && method == 'GET') {
      request.response.statusCode = healthy ? 200 : 503;
      request.response.write('OK');
      await request.response.close();
      return;
    }

    if (path == '/livez' && method == 'GET') {
      request.response.statusCode = 200;
      request.response.write('OK');
      await request.response.close();
      return;
    }

    if (path.startsWith('/api/') || path.startsWith('/internal/')) {
      final body = await utf8.decoder.bind(request).join();
      requests.last = RecordedRequest(
        method: method,
        path: path,
        queryParameters: request.uri.queryParameters,
        headers: headers,
        body: body.isEmpty ? null : body,
      );

      if (method == 'GET') {
        request.response.statusCode = 200;
        request.response.headers.contentType = ContentType.json;
        request.response.write(jsonEncode({
          'code': 200,
          'message': 'success',
          'data': {'path': path, 'items': []},
        }));
      } else if (method == 'POST' || method == 'PUT') {
        request.response.statusCode = 200;
        request.response.headers.contentType = ContentType.json;
        request.response.write(jsonEncode({
          'code': 200,
          'message': 'success',
          'data': body.isNotEmpty ? jsonDecode(body) : null,
        }));
      } else if (method == 'DELETE') {
        request.response.statusCode = 200;
        request.response.headers.contentType = ContentType.json;
        request.response.write(jsonEncode({
          'code': 200,
          'message': 'success',
        }));
      } else if (method == 'PATCH') {
        request.response.statusCode = 200;
        request.response.headers.contentType = ContentType.json;
        request.response.write(jsonEncode({
          'code': 200,
          'message': 'success',
        }));
      } else if (method == 'HEAD') {
        request.response.statusCode = 200;
      } else {
        request.response.statusCode = 405;
      }
      await request.response.close();
      return;
    }

    request.response.statusCode = 404;
    request.response.headers.contentType = ContentType.json;
    request.response.write(jsonEncode({
      'code': 404,
      'message': 'Not found',
    }));
    await request.response.close();
  }

  Future<void> _handleWebSocketUpgrade(
      HttpRequest request, Map<String, String> headers) async {
    if (requireToken) {
      final token = headers['x-amitia-local-token'];
      if (token != validToken) {
        authFailures++;
        request.response.statusCode = 401;
        request.response.headers.contentType = ContentType.json;
        request.response.write(jsonEncode({
          'code': 401,
          'message': 'Unauthorized',
        }));
        await request.response.close();
        return;
      }
    }

    try {
      final socket = await WebSocketTransformer.upgrade(request);
      socket.listen(
        (data) {
          if (data is String) {
            socket.add('echo: $data');
          } else {
            socket.add(data);
          }
        },
        onDone: () => socket.close(),
        onError: (_) => socket.close(),
      );
    } catch (_) {
      // WebSocket upgrade failed, silently ignore
    }
  }

  Future<void> stop() async {
    await _server.close(force: true);
    requests.clear();
  }
}

class RecordedRequest {
  final String method;
  final String path;
  final Map<String, String> queryParameters;
  final Map<String, String> headers;
  final String? body;

  RecordedRequest({
    required this.method,
    required this.path,
    required this.queryParameters,
    required this.headers,
    this.body,
  });
}

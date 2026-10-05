import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/backend_connection/backend_uri_builder.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_config.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_endpoint.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_credential.dart';

void main() {
  BackendConnectionConfig _config({
    String host = '127.0.0.1',
    int port = 18899,
    String httpScheme = 'http',
    String webSocketScheme = 'ws',
    String pathPrefix = '',
  }) {
    return BackendConnectionConfig(
      schemaVersion: 1,
      generation: 1,
      endpoint: BackendConnectionEndpoint(
        host: host,
        port: port,
        httpScheme: httpScheme,
        webSocketScheme: webSocketScheme,
        livenessPath: '/healthz',
        readinessPath: '/readyz',
        pathPrefix: pathPrefix,
      ),
      authStrategy: BackendAuthStrategy.localToken,
      credential: BackendConnectionCredential.tryCreate('a' * 32)!,
    );
  }

  group('BackendUriBuilder', () {
    late BackendUriBuilder builder;

    setUp(() {
      builder = BackendUriBuilder();
    });

    test('provider connection keeps HTTP and WebSocket routes inside the local authenticated channel', () {
      final config = _config(pathPrefix: '/internal/device-mesh/provider');
      final http = builder.http(config, '/api/device-mesh/v1/business/messages', queryParameters: {'roleId': 'role-a'});
      expect(http.host, '127.0.0.1');
      expect(http.port, 18899);
      expect(http.path, '/internal/device-mesh/provider/api/device-mesh/v1/business/messages');
      expect(http.queryParameters['roleId'], 'role-a');
      expect(builder.webSocket(config, '/api/events/ws').path, '/internal/device-mesh/provider/api/events/ws');
      expect(() => _config(host: '192.168.1.2', pathPrefix: '/internal/device-mesh/provider'), throwsArgumentError);
      expect(() => _config(pathPrefix: '/arbitrary-proxy'), throwsArgumentError);
    });

    group('httpBase', () {
      test('builds http base URI', () {
        final uri = builder.httpBase(_config());
        expect(uri.scheme, 'http');
        expect(uri.host, '127.0.0.1');
        expect(uri.port, 18899);
        expect(uri.path, isEmpty);
      });

      test('builds https base URI', () {
        final uri = builder.httpBase(_config(
          httpScheme: 'https',
          webSocketScheme: 'wss',
        ));
        expect(uri.scheme, 'https');
      });
    });

    group('http', () {
      test('builds URI with path', () {
        final uri = builder.http(_config(), '/api/v1/chat');
        expect(uri.scheme, 'http');
        expect(uri.host, '127.0.0.1');
        expect(uri.port, 18899);
        expect(uri.path, '/api/v1/chat');
      });

      test('builds URI with query parameters', () {
        final uri = builder.http(
          _config(),
          '/api/v1/chat',
          queryParameters: {'foo': 'bar', 'baz': 'qux'},
        );
        expect(uri.queryParameters['foo'], 'bar');
        expect(uri.queryParameters['baz'], 'qux');
      });

      test('omits empty query parameters', () {
        final uri = builder.http(
          _config(),
          '/api/v1/chat',
          queryParameters: <String, dynamic>{},
        );
        expect(uri.hasQuery, isFalse);
      });

      test('normalizes scalar query parameter values', () {
        final uri = builder.http(
          _config(),
          '/api/v1/chat',
          queryParameters: <String, dynamic>{
            'page': 2,
            'pageSize': 50,
            'enabled': true,
          },
        );
        expect(uri.queryParameters['page'], '2');
        expect(uri.queryParameters['pageSize'], '50');
        expect(uri.queryParameters['enabled'], 'true');
      });

      test('normalizes iterable query parameter values', () {
        final uri = builder.http(
          _config(),
          '/api/v1/chat',
          queryParameters: <String, dynamic>{
            'tag': ['alpha', 2, true],
          },
        );
        expect(uri.queryParametersAll['tag'], ['alpha', '2', 'true']);
      });

      test('omits null query parameter values', () {
        final uri = builder.http(
          _config(),
          '/api/v1/chat',
          queryParameters: <String, dynamic>{
            'empty': null,
            'value': 'present',
          },
        );
        expect(uri.queryParameters.containsKey('empty'), isFalse);
        expect(uri.queryParameters['value'], 'present');
      });

      test('rejects empty path', () {
        expect(
          () => builder.http(_config(), ''),
          throwsArgumentError,
        );
      });

      test('rejects path not starting with /', () {
        expect(
          () => builder.http(_config(), 'api/v1/chat'),
          throwsArgumentError,
        );
      });

      test('rejects path containing scheme', () {
        expect(
          () => builder.http(_config(), '/api://v1'),
          throwsArgumentError,
        );
      });

      test('rejects path containing query', () {
        expect(
          () => builder.http(_config(), '/api?v=1'),
          throwsArgumentError,
        );
      });

      test('rejects path containing fragment', () {
        expect(
          () => builder.http(_config(), '/api#frag'),
          throwsArgumentError,
        );
      });

      test('rejects path containing NUL', () {
        expect(
          () => builder.http(_config(), '/api\u0000v1'),
          throwsArgumentError,
        );
      });

      test('rejects path containing CR', () {
        expect(
          () => builder.http(_config(), '/api\rv1'),
          throwsArgumentError,
        );
      });

      test('rejects path containing LF', () {
        expect(
          () => builder.http(_config(), '/api\nv1'),
          throwsArgumentError,
        );
      });
    });

    group('webSocketBase', () {
      test('builds ws base URI', () {
        final uri = builder.webSocketBase(_config());
        expect(uri.scheme, 'ws');
        expect(uri.host, '127.0.0.1');
        expect(uri.port, 18899);
      });

      test('builds wss base URI', () {
        final uri = builder.webSocketBase(_config(
          httpScheme: 'https',
          webSocketScheme: 'wss',
        ));
        expect(uri.scheme, 'wss');
      });
    });

    group('webSocket', () {
      test('builds ws URI with path', () {
        final uri = builder.webSocket(_config(), '/ws/chat');
        expect(uri.scheme, 'ws');
        expect(uri.host, '127.0.0.1');
        expect(uri.port, 18899);
        expect(uri.path, '/ws/chat');
      });

      test('builds ws URI with query parameters', () {
        final uri = builder.webSocket(
          _config(),
          '/ws/chat',
          queryParameters: {'token': 'abc123'},
        );
        expect(uri.queryParameters['token'], 'abc123');
      });
    });
  });
}

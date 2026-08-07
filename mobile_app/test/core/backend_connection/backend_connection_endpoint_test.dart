import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_endpoint.dart';

void main() {
  group('BackendConnectionEndpoint', () {
    group('valid construction', () {
      test('accepts 127.0.0.1 with http/ws', () {
        final endpoint = BackendConnectionEndpoint(
          host: '127.0.0.1',
          port: 18899,
          httpScheme: 'http',
          webSocketScheme: 'ws',
          livenessPath: '/healthz',
          readinessPath: '/readyz',
        );
        expect(endpoint.host, '127.0.0.1');
        expect(endpoint.port, 18899);
        expect(endpoint.httpScheme, 'http');
        expect(endpoint.webSocketScheme, 'ws');
      });

      test('accepts https/wss pairing', () {
        final endpoint = BackendConnectionEndpoint(
          host: '10.0.0.1',
          port: 443,
          httpScheme: 'https',
          webSocketScheme: 'wss',
          livenessPath: '/healthz',
          readinessPath: '/readyz',
        );
        expect(endpoint.httpScheme, 'https');
        expect(endpoint.webSocketScheme, 'wss');
      });

      test('accepts minimum valid port 1', () {
        final endpoint = BackendConnectionEndpoint(
          host: '127.0.0.1',
          port: 1,
          httpScheme: 'http',
          webSocketScheme: 'ws',
          livenessPath: '/healthz',
          readinessPath: '/readyz',
        );
        expect(endpoint.port, 1);
      });

      test('accepts maximum valid port 65535', () {
        final endpoint = BackendConnectionEndpoint(
          host: '127.0.0.1',
          port: 65535,
          httpScheme: 'http',
          webSocketScheme: 'ws',
          livenessPath: '/healthz',
          readinessPath: '/readyz',
        );
        expect(endpoint.port, 65535);
      });
    });

    group('host validation', () {
      test('rejects empty host', () {
        expect(
          () => BackendConnectionEndpoint(
            host: '',
            port: 18899,
            httpScheme: 'http',
            webSocketScheme: 'ws',
            livenessPath: '/healthz',
            readinessPath: '/readyz',
          ),
          throwsArgumentError,
        );
      });

      test('rejects localhost', () {
        expect(
          () => BackendConnectionEndpoint(
            host: 'localhost',
            port: 18899,
            httpScheme: 'http',
            webSocketScheme: 'ws',
            livenessPath: '/healthz',
            readinessPath: '/readyz',
          ),
          throwsArgumentError,
        );
      });

      test('rejects 0.0.0.0', () {
        expect(
          () => BackendConnectionEndpoint(
            host: '0.0.0.0',
            port: 18899,
            httpScheme: 'http',
            webSocketScheme: 'ws',
            livenessPath: '/healthz',
            readinessPath: '/readyz',
          ),
          throwsArgumentError,
        );
      });

      test('rejects ::', () {
        expect(
          () => BackendConnectionEndpoint(
            host: '::',
            port: 18899,
            httpScheme: 'http',
            webSocketScheme: 'ws',
            livenessPath: '/healthz',
            readinessPath: '/readyz',
          ),
          throwsArgumentError,
        );
      });

      test('rejects host containing scheme', () {
        expect(
          () => BackendConnectionEndpoint(
            host: 'http://127.0.0.1',
            port: 18899,
            httpScheme: 'http',
            webSocketScheme: 'ws',
            livenessPath: '/healthz',
            readinessPath: '/readyz',
          ),
          throwsArgumentError,
        );
      });

      test('rejects host containing port', () {
        expect(
          () => BackendConnectionEndpoint(
            host: '127.0.0.1:8080',
            port: 18899,
            httpScheme: 'http',
            webSocketScheme: 'ws',
            livenessPath: '/healthz',
            readinessPath: '/readyz',
          ),
          throwsArgumentError,
        );
      });

      test('rejects host containing path', () {
        expect(
          () => BackendConnectionEndpoint(
            host: '127.0.0.1/path',
            port: 18899,
            httpScheme: 'http',
            webSocketScheme: 'ws',
            livenessPath: '/healthz',
            readinessPath: '/readyz',
          ),
          throwsArgumentError,
        );
      });

      test('rejects host containing query', () {
        expect(
          () => BackendConnectionEndpoint(
            host: '127.0.0.1?a=1',
            port: 18899,
            httpScheme: 'http',
            webSocketScheme: 'ws',
            livenessPath: '/healthz',
            readinessPath: '/readyz',
          ),
          throwsArgumentError,
        );
      });

      test('rejects host containing fragment', () {
        expect(
          () => BackendConnectionEndpoint(
            host: '127.0.0.1#frag',
            port: 18899,
            httpScheme: 'http',
            webSocketScheme: 'ws',
            livenessPath: '/healthz',
            readinessPath: '/readyz',
          ),
          throwsArgumentError,
        );
      });

      test('rejects host containing NUL', () {
        expect(
          () => BackendConnectionEndpoint(
            host: '127.0.0\u0000.1',
            port: 18899,
            httpScheme: 'http',
            webSocketScheme: 'ws',
            livenessPath: '/healthz',
            readinessPath: '/readyz',
          ),
          throwsArgumentError,
        );
      });
    });

    group('port validation', () {
      test('rejects port 0', () {
        expect(
          () => BackendConnectionEndpoint(
            host: '127.0.0.1',
            port: 0,
            httpScheme: 'http',
            webSocketScheme: 'ws',
            livenessPath: '/healthz',
            readinessPath: '/readyz',
          ),
          throwsArgumentError,
        );
      });

      test('rejects negative port', () {
        expect(
          () => BackendConnectionEndpoint(
            host: '127.0.0.1',
            port: -1,
            httpScheme: 'http',
            webSocketScheme: 'ws',
            livenessPath: '/healthz',
            readinessPath: '/readyz',
          ),
          throwsArgumentError,
        );
      });

      test('rejects port 65536', () {
        expect(
          () => BackendConnectionEndpoint(
            host: '127.0.0.1',
            port: 65536,
            httpScheme: 'http',
            webSocketScheme: 'ws',
            livenessPath: '/healthz',
            readinessPath: '/readyz',
          ),
          throwsArgumentError,
        );
      });
    });

    group('scheme validation', () {
      test('rejects non-http/https httpScheme', () {
        expect(
          () => BackendConnectionEndpoint(
            host: '127.0.0.1',
            port: 18899,
            httpScheme: 'ftp',
            webSocketScheme: 'ws',
            livenessPath: '/healthz',
            readinessPath: '/readyz',
          ),
          throwsArgumentError,
        );
      });

      test('rejects mismatched ws when httpScheme is http', () {
        expect(
          () => BackendConnectionEndpoint(
            host: '127.0.0.1',
            port: 18899,
            httpScheme: 'http',
            webSocketScheme: 'wss',
            livenessPath: '/healthz',
            readinessPath: '/readyz',
          ),
          throwsArgumentError,
        );
      });

      test('rejects mismatched ws when httpScheme is https', () {
        expect(
          () => BackendConnectionEndpoint(
            host: '127.0.0.1',
            port: 18899,
            httpScheme: 'https',
            webSocketScheme: 'ws',
            livenessPath: '/healthz',
            readinessPath: '/readyz',
          ),
          throwsArgumentError,
        );
      });
    });

    group('path validation', () {
      test('rejects livenessPath not starting with /', () {
        expect(
          () => BackendConnectionEndpoint(
            host: '127.0.0.1',
            port: 18899,
            httpScheme: 'http',
            webSocketScheme: 'ws',
            livenessPath: 'healthz',
            readinessPath: '/readyz',
          ),
          throwsArgumentError,
        );
      });

      test('rejects readinessPath not starting with /', () {
        expect(
          () => BackendConnectionEndpoint(
            host: '127.0.0.1',
            port: 18899,
            httpScheme: 'http',
            webSocketScheme: 'ws',
            livenessPath: '/healthz',
            readinessPath: 'readyz',
          ),
          throwsArgumentError,
        );
      });

      test('rejects livenessPath containing scheme', () {
        expect(
          () => BackendConnectionEndpoint(
            host: '127.0.0.1',
            port: 18899,
            httpScheme: 'http',
            webSocketScheme: 'ws',
            livenessPath: '/healthz://bad',
            readinessPath: '/readyz',
          ),
          throwsArgumentError,
        );
      });

      test('rejects livenessPath containing NUL', () {
        expect(
          () => BackendConnectionEndpoint(
            host: '127.0.0.1',
            port: 18899,
            httpScheme: 'http',
            webSocketScheme: 'ws',
            livenessPath: '/health\u0000z',
            readinessPath: '/readyz',
          ),
          throwsArgumentError,
        );
      });
    });
  });
}

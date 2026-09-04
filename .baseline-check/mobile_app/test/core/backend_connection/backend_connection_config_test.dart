import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_config.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_endpoint.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_credential.dart';

void main() {
  BackendConnectionEndpoint _validEndpoint() => BackendConnectionEndpoint(
        host: '127.0.0.1',
        port: 18899,
        httpScheme: 'http',
        webSocketScheme: 'ws',
        livenessPath: '/healthz',
        readinessPath: '/readyz',
      );

  BackendConnectionCredential _validCredential() =>
      BackendConnectionCredential.tryCreate('a' * 32)!;

  group('BackendConnectionConfig', () {
    group('valid construction', () {
      test('constructs with valid parameters', () {
        final config = BackendConnectionConfig(
          schemaVersion: 1,
          generation: 1,
          endpoint: _validEndpoint(),
          authStrategy: BackendAuthStrategy.localToken,
          credential: _validCredential(),
        );
        expect(config.schemaVersion, 1);
        expect(config.generation, 1);
        expect(config.endpoint.host, '127.0.0.1');
      });

      test('constructs with large generation', () {
        final config = BackendConnectionConfig(
          schemaVersion: 1,
          generation: 999999,
          endpoint: _validEndpoint(),
          authStrategy: BackendAuthStrategy.localToken,
          credential: _validCredential(),
        );
        expect(config.generation, 999999);
      });
    });

    group('schemaVersion validation', () {
      test('rejects schemaVersion 0', () {
        expect(
          () => BackendConnectionConfig(
            schemaVersion: 0,
            generation: 1,
            endpoint: _validEndpoint(),
            authStrategy: BackendAuthStrategy.localToken,
            credential: _validCredential(),
          ),
          throwsArgumentError,
        );
      });

      test('rejects schemaVersion 2', () {
        expect(
          () => BackendConnectionConfig(
            schemaVersion: 2,
            generation: 1,
            endpoint: _validEndpoint(),
            authStrategy: BackendAuthStrategy.localToken,
            credential: _validCredential(),
          ),
          throwsArgumentError,
        );
      });

      test('rejects negative schemaVersion', () {
        expect(
          () => BackendConnectionConfig(
            schemaVersion: -1,
            generation: 1,
            endpoint: _validEndpoint(),
            authStrategy: BackendAuthStrategy.localToken,
            credential: _validCredential(),
          ),
          throwsArgumentError,
        );
      });
    });

    group('generation validation', () {
      test('rejects generation 0', () {
        expect(
          () => BackendConnectionConfig(
            schemaVersion: 1,
            generation: 0,
            endpoint: _validEndpoint(),
            authStrategy: BackendAuthStrategy.localToken,
            credential: _validCredential(),
          ),
          throwsArgumentError,
        );
      });

      test('rejects negative generation', () {
        expect(
          () => BackendConnectionConfig(
            schemaVersion: 1,
            generation: -1,
            endpoint: _validEndpoint(),
            authStrategy: BackendAuthStrategy.localToken,
            credential: _validCredential(),
          ),
          throwsArgumentError,
        );
      });
    });
  });
}

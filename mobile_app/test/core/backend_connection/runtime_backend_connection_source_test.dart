import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_availability.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_credential.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_config.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_endpoint.dart';

void main() {
  group('RuntimeBackendConnectionSource expectedGeneration', () {
    test('accepts matching generation', () {
      final config = _buildConfig(7);
      expect(config, isNotNull);
      expect(config!.generation, 7);
    });

    test('rejects generation mismatch', () {
      final payload = _validPayload(generation: 7);
      final rejected = _simulateParse(payload, expectedGeneration: 8);
      expect(rejected, isA<BackendConnectionUnavailable>());
    });

    test('accepts when expectedGeneration is 0 (any)', () {
      final payload = _validPayload(generation: 5);
      final parsed = _simulateParse(payload, expectedGeneration: 0);
      expect(parsed, isA<BackendConnectionAvailable>());
      expect((parsed as BackendConnectionAvailable).config.generation, 5);
    });

    test('rejects generation 0 in payload', () {
      final payload = _validPayload(generation: 0);
      final parsed = _simulateParse(payload, expectedGeneration: 0);
      expect(parsed, isA<BackendConnectionUnavailable>());
    });

    test('rejects schema mismatch', () {
      final payload = Map<String, dynamic>.from(_validPayload(generation: 1));
      payload['schemaVersion'] = 99;
      final parsed = _simulateParse(payload, expectedGeneration: 1);
      expect(parsed, isA<BackendConnectionUnavailable>());
    });

    test('rejects missing endpoint', () {
      final payload = Map<String, dynamic>.from(_validPayload(generation: 1));
      payload.remove('endpoint');
      final parsed = _simulateParse(payload, expectedGeneration: 1);
      expect(parsed, isA<BackendConnectionUnavailable>());
    });

    test('rejects wrong auth type', () {
      final payload = Map<String, dynamic>.from(_validPayload(generation: 1));
      final auth = payload['authentication'] as Map<String, dynamic>;
      auth['type'] = 'bearer';
      final parsed = _simulateParse(payload, expectedGeneration: 1);
      expect(parsed, isA<BackendConnectionUnavailable>());
    });

    test('rejects wrong auth header', () {
      final payload = Map<String, dynamic>.from(_validPayload(generation: 1));
      final auth = payload['authentication'] as Map<String, dynamic>;
      auth['header'] = 'Authorization';
      final parsed = _simulateParse(payload, expectedGeneration: 1);
      expect(parsed, isA<BackendConnectionUnavailable>());
    });

    test('rejects missing token', () {
      final payload = Map<String, dynamic>.from(_validPayload(generation: 1));
      final auth = payload['authentication'] as Map<String, dynamic>;
      auth.remove('token');
      final parsed = _simulateParse(payload, expectedGeneration: 1);
      expect(parsed, isA<BackendConnectionUnavailable>());
    });

    test('rejects unavailable payload', () {
      final payload = <String, dynamic>{
        'schemaVersion': 1,
        'status': 'unavailable',
        'error': {'code': 'RUNTIME_NOT_READY'},
      };
      final parsed = _simulateParse(payload, expectedGeneration: 0);
      expect(parsed, isA<BackendConnectionUnavailable>());
    });

    test('rejects invalid token', () {
      final payload = Map<String, dynamic>.from(_validPayload(generation: 1));
      final auth = payload['authentication'] as Map<String, dynamic>;
      auth['token'] = 'short';
      final parsed = _simulateParse(payload, expectedGeneration: 1);
      expect(parsed, isA<BackendConnectionUnavailable>());
    });
  });

  group('Credential redaction', () {
    test('toString does not expose token', () {
      final credential = BackendConnectionCredential.tryCreate('x' * 40)!;
      expect(credential.toString(), contains('REDACTED'));
      expect(credential.toString(), isNot(contains('x' * 40)));
    });

    test('revealForTransport returns raw token', () {
      final token = 'y' * 32;
      final credential = BackendConnectionCredential.tryCreate(token)!;
      expect(credential.revealForTransport(), token);
    });
  });

  group('Config immutability', () {
    test('config fields are final', () {
      final config = _buildConfig(42);
      expect(config!.schemaVersion, 1);
      expect(config.generation, 42);
      expect(config.endpoint.host, '127.0.0.1');
      expect(config.endpoint.port, 18899);
    });
  });
}

Map<String, dynamic> _validPayload({required int generation}) {
  return {
    'schemaVersion': 1,
    'status': 'available',
    'generation': generation,
    'endpoint': {
      'host': '127.0.0.1',
      'port': 18899,
      'httpScheme': 'http',
      'webSocketScheme': 'ws',
      'livenessPath': '/livez',
      'readinessPath': '/readyz',
    },
    'authentication': {
      'type': 'local_token',
      'header': 'X-Amitia-Local-Token',
      'token': 'a' * 40,
    },
  };
}

BackendConnectionConfig? _buildConfig(int generation) {
  final payload = _validPayload(generation: generation);
  final parsed = _simulateParse(payload, expectedGeneration: 0);
  if (parsed is BackendConnectionAvailable) {
    return parsed.config;
  }
  return null;
}

BackendConnectionAvailability _simulateParse(
  Map<String, dynamic> decoded, {
  int expectedGeneration = 0,
}) {
  try {
    final schemaVersion = decoded['schemaVersion'];
    if (schemaVersion is! int || schemaVersion != 1) {
      return BackendConnectionUnavailable();
    }
    final status = decoded['status'];
    if (status == 'available') {
      final generationRaw = decoded['generation'];
      if (generationRaw is! int || generationRaw <= 0) {
        return BackendConnectionUnavailable();
      }
      if (expectedGeneration > 0 && generationRaw != expectedGeneration) {
        return BackendConnectionUnavailable();
      }
      final endpointMap = decoded['endpoint'];
      if (endpointMap is! Map) return BackendConnectionUnavailable();
      final authMap = decoded['authentication'];
      if (authMap is! Map) return BackendConnectionUnavailable();
      final authType = authMap['type'];
      final authHeader = authMap['header'];
      if (authType != 'local_token' || authHeader != 'X-Amitia-Local-Token') {
        return BackendConnectionUnavailable();
      }
      final token = authMap['token'];
      if (token is! String) return BackendConnectionUnavailable();
      final credential = BackendConnectionCredential.tryCreate(token);
      if (credential == null) return BackendConnectionUnavailable();

      final host = endpointMap['host'];
      final port = endpointMap['port'];
      final httpScheme = endpointMap['httpScheme'];
      final webSocketScheme = endpointMap['webSocketScheme'];
      final livenessPath = endpointMap['livenessPath'];
      final readinessPath = endpointMap['readinessPath'];
      if (host is! String ||
          port is! int ||
          httpScheme is! String ||
          webSocketScheme is! String ||
          livenessPath is! String ||
          readinessPath is! String) {
        return BackendConnectionUnavailable();
      }

      final endpoint = BackendConnectionEndpoint(
        host: host,
        port: port,
        httpScheme: httpScheme,
        webSocketScheme: webSocketScheme,
        livenessPath: livenessPath,
        readinessPath: readinessPath,
      );

      final config = BackendConnectionConfig(
        schemaVersion: 1,
        generation: generationRaw,
        endpoint: endpoint,
        credential: credential,
      );
      return BackendConnectionAvailable(config);
    }
    return BackendConnectionUnavailable();
  } catch (_) {
    return BackendConnectionUnavailable();
  }
}

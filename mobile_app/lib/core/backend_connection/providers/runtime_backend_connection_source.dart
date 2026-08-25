import 'package:flutter/services.dart';

import '../backend_connection_availability.dart';
import '../backend_connection_config.dart';
import '../backend_connection_credential.dart';
import '../backend_connection_endpoint.dart';
import '../backend_connection_error.dart';
import '../backend_connection_source.dart';

class RuntimeBackendConnectionSource implements BackendConnectionSource {
  static const MethodChannel _channel = MethodChannel('com.amitia.runtime/bridge');
  static const String _methodGetBackendConnection = 'runtime.getBackendConnection';

  const RuntimeBackendConnectionSource();

  @override
  Future<BackendConnectionAvailability> resolve({int? expectedRuntimeGeneration}) async {
    try {
      final result = await _channel.invokeMethod<Map<Object?, Object?>>(
        _methodGetBackendConnection,
      );
      if (result == null) {
        return const BackendConnectionUnavailable(
          BackendConnectionError(
            BackendConnectionErrorCode.BRIDGE_PAYLOAD_INVALID,
            'runtime bridge returned an empty backend connection payload',
          ),
        );
      }
      final converted = <String, dynamic>{};
      for (final entry in result.entries) {
        converted[entry.key.toString()] = entry.value;
      }
      return _parsePayload(
        converted,
        expectedRuntimeGeneration: expectedRuntimeGeneration,
      );
    } on PlatformException catch (error) {
      return BackendConnectionUnavailable(
        BackendConnectionError(
          BackendConnectionErrorCode.BRIDGE_UNAVAILABLE,
          _nonEmpty(
            error.message,
            'runtime bridge failed while resolving the backend connection',
          ),
        ),
      );
    } on MissingPluginException {
      return const BackendConnectionUnavailable(
        BackendConnectionError(
          BackendConnectionErrorCode.BRIDGE_UNAVAILABLE,
          'runtime bridge plugin is not registered',
        ),
      );
    } catch (error) {
      return BackendConnectionUnavailable(
        BackendConnectionError(
          BackendConnectionErrorCode.INTERNAL_ERROR,
          'unexpected backend connection bridge error: $error',
        ),
      );
    }
  }

  BackendConnectionAvailability _parsePayload(
    Map<String, dynamic> decoded, {
    int? expectedRuntimeGeneration,
  }) {
    try {
      final schemaVersion = decoded['schemaVersion'];
      if (schemaVersion is! int || schemaVersion != 1) {
        return const BackendConnectionUnavailable(
          BackendConnectionError(
            BackendConnectionErrorCode.BRIDGE_PAYLOAD_INVALID,
            'unsupported backend connection payload schema',
          ),
        );
      }

      final status = decoded['status'];
      if (status == 'unavailable') {
        return BackendConnectionUnavailable(_parseError(decoded['error']));
      }
      if (status != 'available') {
        return const BackendConnectionUnavailable(
          BackendConnectionError(
            BackendConnectionErrorCode.BRIDGE_PAYLOAD_INVALID,
            'backend connection payload contains an invalid status',
          ),
        );
      }

      final generationRaw = decoded['generation'];
      if (generationRaw is! int || generationRaw <= 0) {
        return const BackendConnectionUnavailable(
          BackendConnectionError(
            BackendConnectionErrorCode.GENERATION_INVALID,
            'backend connection payload contains an invalid generation',
          ),
        );
      }
      if (expectedRuntimeGeneration != null &&
          expectedRuntimeGeneration > 0 &&
          generationRaw != expectedRuntimeGeneration) {
        return BackendConnectionUnavailable(
          BackendConnectionError(
            BackendConnectionErrorCode.GENERATION_INVALID,
            'backend connection generation $generationRaw does not match runtime generation $expectedRuntimeGeneration',
          ),
        );
      }

      final endpointMap = decoded['endpoint'];
      if (endpointMap is! Map) {
        return const BackendConnectionUnavailable(
          BackendConnectionError(
            BackendConnectionErrorCode.ENDPOINT_INVALID,
            'backend connection payload is missing endpoint data',
          ),
        );
      }
      final authMap = decoded['authentication'];
      if (authMap is! Map) {
        return const BackendConnectionUnavailable(
          BackendConnectionError(
            BackendConnectionErrorCode.CREDENTIAL_UNAVAILABLE,
            'backend connection payload is missing authentication data',
          ),
        );
      }

      final authType = authMap['type'];
      final authHeader = authMap['header'];
      if (authType != 'local_token' || authHeader != 'X-Amitia-Local-Token') {
        return const BackendConnectionUnavailable(
          BackendConnectionError(
            BackendConnectionErrorCode.CREDENTIAL_INVALID,
            'runtime bridge returned an unsupported local authentication contract',
          ),
        );
      }
      final token = authMap['token'];
      if (token is! String) {
        return const BackendConnectionUnavailable(
          BackendConnectionError(
            BackendConnectionErrorCode.CREDENTIAL_UNAVAILABLE,
            'runtime bridge did not return a local backend credential',
          ),
        );
      }
      final credential = BackendConnectionCredential.tryCreate(token);
      if (credential == null) {
        return const BackendConnectionUnavailable(
          BackendConnectionError(
            BackendConnectionErrorCode.CREDENTIAL_INVALID,
            'runtime bridge returned an invalid local backend credential',
          ),
        );
      }

      final host = endpointMap['host'];
      final port = endpointMap['port'];
      final httpScheme = endpointMap['httpScheme'];
      final webSocketScheme = endpointMap['webSocketScheme'];
      final livenessPath = endpointMap['livenessPath'];
      final readinessPath = endpointMap['readinessPath'];
      if (host is! String ||
          host.trim().isEmpty ||
          port is! int ||
          port <= 0 ||
          port > 65535 ||
          httpScheme is! String ||
          webSocketScheme is! String ||
          livenessPath is! String ||
          readinessPath is! String) {
        return const BackendConnectionUnavailable(
          BackendConnectionError(
            BackendConnectionErrorCode.ENDPOINT_INVALID,
            'runtime bridge returned an invalid backend endpoint',
          ),
        );
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
        authStrategy: BackendAuthStrategy.localToken,
        credential: credential,
      );
      return BackendConnectionAvailable(config);
    } catch (error) {
      return BackendConnectionUnavailable(
        BackendConnectionError(
          BackendConnectionErrorCode.BRIDGE_PAYLOAD_INVALID,
          'failed to parse backend connection payload: $error',
        ),
      );
    }
  }

  BackendConnectionError _parseError(Object? raw) {
    if (raw is! Map) {
      return const BackendConnectionError(
        BackendConnectionErrorCode.ENDPOINT_UNAVAILABLE,
        'runtime backend connection is unavailable',
      );
    }
    final codeName = raw['code']?.toString();
    final message = raw['message']?.toString();
    final code = BackendConnectionErrorCode.values.firstWhere(
      (candidate) => candidate.name == codeName,
      orElse: () => BackendConnectionErrorCode.ENDPOINT_UNAVAILABLE,
    );
    return BackendConnectionError(
      code,
      _nonEmpty(message, 'runtime backend connection is unavailable'),
    );
  }

  String _nonEmpty(String? value, String fallback) {
    final normalized = value?.trim();
    return normalized == null || normalized.isEmpty ? fallback : normalized;
  }
}

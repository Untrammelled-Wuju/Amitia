import 'package:flutter/services.dart';
import '../backend_connection_availability.dart';
import '../backend_connection_config.dart';
import '../backend_connection_credential.dart';
import '../backend_connection_endpoint.dart';
import '../backend_connection_source.dart';

class RuntimeBackendConnectionSource implements BackendConnectionSource {
  static const MethodChannel _channel = MethodChannel('com.amitia.runtime/bridge');
  static const String _methodGetBackendConnection = 'runtime.getBackendConnection';

  const RuntimeBackendConnectionSource();

  @override
  Future<BackendConnectionAvailability> resolve() async {
    try {
      final result = await _channel.invokeMethod<Map<Object?, Object?>>(_methodGetBackendConnection);
      if (result == null) return BackendConnectionUnavailable();
      final converted = <String, dynamic>{};
      for (final entry in result.entries) {
        converted[entry.key.toString()] = entry.value;
      }
      return _parsePayload(converted);
    } on PlatformException {
      return BackendConnectionUnavailable();
    } on MissingPluginException {
      return BackendConnectionUnavailable();
    }
  }

  BackendConnectionAvailability _parsePayload(Map<String, dynamic> decoded) {
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
        if (host is! String || port is! int || httpScheme is! String ||
            webSocketScheme is! String || livenessPath is! String || readinessPath is! String) {
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
}

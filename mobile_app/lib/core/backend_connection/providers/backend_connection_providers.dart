import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend_connection_availability.dart';
import '../backend_connection_config.dart';
import '../backend_connection_credential.dart';
import '../backend_connection_endpoint.dart';
import '../backend_connection_error.dart';
import '../backend_connection_repository.dart';
import '../backend_connection_source.dart';
import 'runtime_backend_connection_source.dart';
import '../../runtime/runtime_bridge_provider.dart';
import '../../runtime/runtime_bridge_state.dart';
import '../../runtime/backend/mobile_backend_providers.dart';
import '../../runtime/backend/mobile_deployment_mode.dart';
import '../../runtime/backend/backend_topology_resolver.dart';

final backendConnectionRepositoryProvider = Provider<BackendConnectionRepository>((ref) {
  final source = ref.watch(backendConnectionSourceProvider);
  return DefaultBackendConnectionRepository(source);
});

final backendConnectionSourceProvider = Provider<BackendConnectionSource>((ref) {
  final config = ref.watch(mobileDeploymentConfigProvider);
  if (config.mode == MobileDeploymentMode.local) {
    return const RuntimeBackendConnectionSource();
  }
  return _CloudBackendConnectionSource(config);
});

final backendConnectionProvider = FutureProvider<BackendConnectionAvailability>((ref) async {
  final config = ref.watch(mobileDeploymentConfigProvider);
  final repo = ref.watch(backendConnectionRepositoryProvider);

  if (config.mode == MobileDeploymentMode.local) {
    final runtimeAsync = ref.watch(runtimeSnapshotProvider);
    final runtime = runtimeAsync.valueOrNull;
    if (runtime == null ||
        runtime.state != RuntimeBridgeState.ready ||
        runtime.generation <= 0) {
      repo.invalidate();
      return const BackendConnectionUnavailable(
        BackendConnectionError(
          BackendConnectionErrorCode.RUNTIME_NOT_READY,
          'embedded runtime is not ready',
        ),
      );
    }
    return repo.resolve(expectedRuntimeGeneration: runtime.generation);
  }

  return repo.resolve(expectedRuntimeGeneration: null);
});

/// Repository-level generation semantics:
/// - local mode: Native Runtime generation is the single source of truth and
///   must never be rewritten by Flutter;
/// - cloud mode: the repository owns a monotonic business-generation counter
///   because there is no embedded runtime generation to bind against.
class DefaultBackendConnectionRepository implements BackendConnectionRepository {
  final BackendConnectionSource _source;
  BackendConnectionConfig? _cached;
  int _resolutionEpoch = 0;
  int _cloudBusinessGeneration = 0;
  String? _lastCloudCacheKey;

  DefaultBackendConnectionRepository(this._source);

  @override
  Future<BackendConnectionAvailability> resolve({
    int? expectedRuntimeGeneration,
  }) async {
    final epoch = ++_resolutionEpoch;
    final result = await _source.resolve(
      expectedRuntimeGeneration: expectedRuntimeGeneration,
    );

    if (epoch != _resolutionEpoch) {
      return const BackendConnectionUnavailable(
        BackendConnectionError(
          BackendConnectionErrorCode.GENERATION_INVALID,
          'backend connection resolution was superseded by a newer runtime state',
        ),
      );
    }

    if (result is! BackendConnectionAvailable) {
      _cached = null;
      return result;
    }

    if (expectedRuntimeGeneration != null) {
      if (expectedRuntimeGeneration <= 0 ||
          result.config.generation != expectedRuntimeGeneration) {
        _cached = null;
        return BackendConnectionUnavailable(
          BackendConnectionError(
            BackendConnectionErrorCode.GENERATION_INVALID,
            'backend connection generation ${result.config.generation} does not match runtime generation $expectedRuntimeGeneration',
          ),
        );
      }
      // Local mode must preserve the Native Runtime generation exactly.
      _cached = result.config;
      return BackendConnectionAvailable(result.config);
    }

    final cacheKey = _cacheKeyFor(result.config);
    if (_cloudBusinessGeneration <= 0 || cacheKey != _lastCloudCacheKey) {
      _cloudBusinessGeneration++;
      _lastCloudCacheKey = cacheKey;
    }
    final stamped = BackendConnectionConfig(
      schemaVersion: result.config.schemaVersion,
      generation: _cloudBusinessGeneration,
      endpoint: result.config.endpoint,
      authStrategy: result.config.authStrategy,
      credential: result.config.credential,
      spaceId: result.config.spaceId,
      deviceId: result.config.deviceId,
      runtimeId: result.config.runtimeId,
    );
    _cached = stamped;
    return BackendConnectionAvailable(stamped);
  }

  String _cacheKeyFor(BackendConnectionConfig config) {
    return [
      config.endpoint.host,
      config.endpoint.port,
      config.endpoint.httpScheme,
      config.endpoint.webSocketScheme,
      config.endpoint.livenessPath,
      config.endpoint.readinessPath,
      config.authStrategy.name,
      config.credential.revealForTransport(),
      config.spaceId,
      config.deviceId,
      config.runtimeId,
    ].join('|');
  }

  @override
  BackendConnectionConfig? get cached => _cached;

  @override
  void invalidate() {
    // Invalidation only revokes the current resolution/cache authority. It must
    // never mutate a generation, otherwise STARTING-state rebuilds can drift
    // away from the Native Runtime generation before READY is observed.
    _resolutionEpoch++;
    _cached = null;
  }
}

class _CloudBackendConnectionSource implements BackendConnectionSource {
  final MobileDeploymentConfig _config;

  const _CloudBackendConnectionSource(this._config);

  @override
  Future<BackendConnectionAvailability> resolve({int? expectedRuntimeGeneration}) async {
    final uri = _config.remoteCoreUri;
    if (uri == null || uri.trim().isEmpty) {
      return const BackendConnectionUnavailable(
        BackendConnectionError(
          BackendConnectionErrorCode.ENDPOINT_UNAVAILABLE,
          'remote core URI is not configured',
        ),
      );
    }

    try {
      final parsed = normalizeRemoteCoreUri(uri);
      final localAvailability = await const RuntimeBackendConnectionSource().resolve();
      if (localAvailability is! BackendConnectionAvailable) {
        return const BackendConnectionUnavailable(
          BackendConnectionError(
            BackendConnectionErrorCode.RUNTIME_NOT_READY,
            'device agent runtime is not ready',
          ),
        );
      }

      final local = localAvailability.config;
      final localBase = Uri(
        scheme: local.endpoint.httpScheme,
        host: local.endpoint.host,
        port: local.endpoint.port,
      );
      final dio = Dio(
        BaseOptions(
          baseUrl: localBase.toString(),
          connectTimeout: const Duration(seconds: 5),
          receiveTimeout: const Duration(seconds: 5),
          headers: <String, String>{
            'Accept': 'application/json',
            'X-Amitia-Local-Token': local.credential.revealForTransport(),
            'X-Amitia-Client-Type': 'mobile',
          },
        ),
      );

      Map<String, dynamic> auth;
      try {
        final response = await dio.get<dynamic>('/internal/device-mesh/cloud-auth');
        final raw = response.data;
        if (raw is! Map) {
          return const BackendConnectionUnavailable(
            BackendConnectionError(
              BackendConnectionErrorCode.CREDENTIAL_UNAVAILABLE,
              'device agent returned an invalid cloud credential response',
            ),
          );
        }
        auth = Map<String, dynamic>.from(raw);
        if (auth['data'] is Map) {
          auth = Map<String, dynamic>.from(auth['data'] as Map);
        }
      } on DioException catch (error) {
        final status = error.response?.statusCode;
        return BackendConnectionUnavailable(
          BackendConnectionError(
            status == 401
                ? BackendConnectionErrorCode.CREDENTIAL_INVALID
                : BackendConnectionErrorCode.CREDENTIAL_UNAVAILABLE,
            status == 404
                ? 'this device is not paired with the configured Cloud Core'
                : 'failed to load Device Mesh cloud credential',
          ),
        );
      } finally {
        dio.close(force: true);
      }

      final cloudBaseUrl = (auth['cloudBaseUrl'] ?? '').toString().trim();
      final authorization = (auth['authorization'] ?? '').toString().trim();
      const prefix = 'AmitiaDevice ';
      if (!authorization.startsWith(prefix)) {
        return const BackendConnectionUnavailable(
          BackendConnectionError(
            BackendConnectionErrorCode.CREDENTIAL_INVALID,
            'device agent returned an invalid DeviceCredential',
          ),
        );
      }
      if (cloudBaseUrl.isEmpty || normalizeRemoteCoreUri(cloudBaseUrl).origin != parsed.origin) {
        return const BackendConnectionUnavailable(
          BackendConnectionError(
            BackendConnectionErrorCode.CREDENTIAL_INVALID,
            'paired Cloud Core does not match the configured remote core URI',
          ),
        );
      }

      final credential = BackendConnectionCredential.tryCreate(
        authorization.substring(prefix.length),
      );
      final spaceId = (auth['spaceId'] ?? '').toString().trim();
      final deviceId = (auth['deviceId'] ?? '').toString().trim();
      final runtimeId = (auth['runtimeId'] ?? '').toString().trim();
      if (credential == null || spaceId.isEmpty || deviceId.isEmpty || runtimeId.isEmpty) {
        return const BackendConnectionUnavailable(
          BackendConnectionError(
            BackendConnectionErrorCode.CREDENTIAL_INVALID,
            'DeviceCredential identity is incomplete',
          ),
        );
      }

      final scheme = parsed.scheme.toLowerCase();
      var port = parsed.port;
      if (port == 0) port = scheme == 'https' ? 443 : 80;
      final endpoint = BackendConnectionEndpoint(
        host: parsed.host,
        port: port,
        httpScheme: scheme,
        webSocketScheme: scheme == 'https' ? 'wss' : 'ws',
        livenessPath: '/readyz',
        readinessPath: '/readyz',
      );
      return BackendConnectionAvailable(
        BackendConnectionConfig(
          schemaVersion: 1,
          generation: 1,
          endpoint: endpoint,
          authStrategy: BackendAuthStrategy.deviceCredential,
          credential: credential,
          spaceId: spaceId,
          deviceId: deviceId,
          runtimeId: runtimeId,
        ),
      );
    } catch (error) {
      return BackendConnectionUnavailable(
        BackendConnectionError(
          BackendConnectionErrorCode.ENDPOINT_INVALID,
          'failed to resolve cloud Device Mesh connection: $error',
        ),
      );
    }
  }
}

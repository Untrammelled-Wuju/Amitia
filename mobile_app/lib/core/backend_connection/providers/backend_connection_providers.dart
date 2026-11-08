import 'dart:io';

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend_connection_availability.dart';
import '../backend_connection_config.dart';
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
import '../../device_mesh/mobile_cloud_device_credential_store.dart';

final backendConnectionRepositoryProvider =
    Provider<BackendConnectionRepository>((ref) {
      final source = ref.watch(backendConnectionSourceProvider);
      return DefaultBackendConnectionRepository(source);
    });

final backendConnectionSourceProvider = Provider<BackendConnectionSource>((
  ref,
) {
  final config = ref.watch(mobileDeploymentConfigProvider);
  if (config.mode == MobileDeploymentMode.local) {
    return const RuntimeBackendConnectionSource();
  }
  return _CloudBackendConnectionSource(config);
});

final backendConnectionProvider = FutureProvider<BackendConnectionAvailability>(
  (ref) async {
    final config = ref.watch(mobileDeploymentConfigProvider);
    final repo = ref.watch(backendConnectionRepositoryProvider);
    final runtimeAsync = ref.watch(runtimeSnapshotProvider);

    if (config.mode == MobileDeploymentMode.local) {
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

    final runtime = runtimeAsync.valueOrNull;
    if (runtime == null ||
        runtime.state != RuntimeBridgeState.ready ||
        runtime.generation <= 0) {
      repo.invalidate();
      return const BackendConnectionUnavailable(
        BackendConnectionError(
          BackendConnectionErrorCode.RUNTIME_NOT_READY,
          'embedded device agent runtime is not ready',
        ),
      );
    }

    return repo.resolve(expectedRuntimeGeneration: null);
  },
);

/// Repository-level generation semantics:
/// - local mode: Native Runtime generation is the single source of truth and
///   must never be rewritten by Flutter;
/// - cloud mode: the repository owns a monotonic business-generation counter
///   because there is no embedded runtime generation to bind against.
class DefaultBackendConnectionRepository
    implements BackendConnectionRepository {
  final BackendConnectionSource _source;
  BackendConnectionConfig? _cached;
  int _resolutionEpoch = 0;
  int _cloudBusinessGeneration = 0;
  String? _lastCloudCacheKey;
  final Map<String, Future<BackendConnectionAvailability>> _inFlight = {};

  DefaultBackendConnectionRepository(this._source);

  @override
  Future<BackendConnectionAvailability> resolve({
    int? expectedRuntimeGeneration,
  }) {
    final key = expectedRuntimeGeneration?.toString() ?? 'cloud';
    final existing = _inFlight[key];
    if (existing != null) return existing;

    final epoch = ++_resolutionEpoch;
    late final Future<BackendConnectionAvailability> future;
    future =
        _resolveOnce(
          expectedRuntimeGeneration: expectedRuntimeGeneration,
          epoch: epoch,
        ).whenComplete(() {
          if (identical(_inFlight[key], future)) {
            _inFlight.remove(key);
          }
        });
    _inFlight[key] = future;
    return future;
  }

  Future<BackendConnectionAvailability> _resolveOnce({
    required int? expectedRuntimeGeneration,
    required int epoch,
  }) async {
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
      fingerprint: result.config.fingerprint,
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
      config.endpoint.pathPrefix,
      config.endpoint.livenessPath,
      config.endpoint.readinessPath,
      config.authStrategy.name,
      config.credential.revealForTransport(),
      config.spaceId,
      config.deviceId,
      config.runtimeId,
      config.fingerprint,
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
    _inFlight.clear();
  }
}

class _CloudBackendConnectionSource implements BackendConnectionSource {
  final MobileDeploymentConfig _config;

  const _CloudBackendConnectionSource(this._config);

  @override
  Future<BackendConnectionAvailability> resolve({
    int? expectedRuntimeGeneration,
  }) async {
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
      final embedded = Platform.isIOS || Platform.isAndroid
          ? await _loadEmbeddedCredential(parsed)
          : null;
      if (embedded == null) {
        return const BackendConnectionUnavailable(
          BackendConnectionError(
            BackendConnectionErrorCode.CREDENTIAL_UNAVAILABLE,
            '当前设备尚未与配置的统筹中心配对',
          ),
        );
      }
      final auth = embedded.auth;

      if (auth.spaceId.isEmpty ||
          auth.deviceId.isEmpty ||
          auth.runtimeId.isEmpty) {
        return const BackendConnectionUnavailable(
          BackendConnectionError(
            BackendConnectionErrorCode.CREDENTIAL_INVALID,
            'DeviceCredential identity is incomplete',
          ),
        );
      }
      if (auth.isExpired) {
        return const BackendConnectionUnavailable(
          BackendConnectionError(
            BackendConnectionErrorCode.CREDENTIAL_INVALID,
            'DeviceCredential has expired; pair this device again',
          ),
        );
      }
      if (normalizeRemoteCoreUri(auth.cloudBaseUrl).origin != parsed.origin) {
        return const BackendConnectionUnavailable(
          BackendConnectionError(
            BackendConnectionErrorCode.CREDENTIAL_INVALID,
            '已配对的统筹中心与当前远程中心地址不一致',
          ),
        );
      }

      return BackendConnectionAvailable(
        cloudProxyConnectionConfig(local: embedded.local, auth: auth),
      );
    } on BackendConnectionError catch (error) {
      return BackendConnectionUnavailable(error);
    } catch (error) {
      return BackendConnectionUnavailable(
        BackendConnectionError(
          BackendConnectionErrorCode.ENDPOINT_INVALID,
          '无法解析统筹中心直连地址: $error',
        ),
      );
    }
  }

  Future<({MobileCloudDeviceCredential auth, BackendConnectionConfig local})?>
  _loadEmbeddedCredential(Uri cloud) async {
    final localAvailability = await const RuntimeBackendConnectionSource()
        .resolve();
    if (localAvailability is! BackendConnectionAvailable) {
      throw const BackendConnectionError(
        BackendConnectionErrorCode.RUNTIME_NOT_READY,
        'device agent runtime is not ready',
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

    try {
      final response = await dio.get<dynamic>(
        '/internal/device-mesh/cloud-auth',
      );
      var raw = response.data;
      if (raw is! Map) {
        throw const BackendConnectionError(
          BackendConnectionErrorCode.CREDENTIAL_UNAVAILABLE,
          'device agent returned an invalid cloud credential response',
        );
      }
      var auth = Map<String, dynamic>.from(raw);
      if (auth['data'] is Map) {
        auth = Map<String, dynamic>.from(auth['data'] as Map);
      }
      final authorization = (auth['authorization'] ?? '').toString().trim();
      const prefix = 'AmitiaDevice ';
      if (!authorization.startsWith(prefix)) {
        throw const BackendConnectionError(
          BackendConnectionErrorCode.CREDENTIAL_INVALID,
          'device agent returned an invalid DeviceCredential',
        );
      }
      final cloudBaseUrl = (auth['cloudBaseUrl'] ?? '').toString().trim();
      if (cloudBaseUrl.isEmpty ||
          normalizeRemoteCoreUri(cloudBaseUrl).origin != cloud.origin) {
        throw const BackendConnectionError(
          BackendConnectionErrorCode.CREDENTIAL_INVALID,
          '已配对的统筹中心与当前远程中心地址不一致',
        );
      }
      final expiresAt =
          DateTime.tryParse((auth['expiresAt'] ?? '').toString())?.toUtc() ??
          DateTime.now().toUtc().add(const Duration(days: 30));
      return (
        auth: MobileCloudDeviceCredential(
          cloudBaseUrl: cloudBaseUrl,
          credentialId: (auth['credentialId'] ?? '').toString().trim(),
          credential: authorization.substring(prefix.length).trim(),
          spaceId: (auth['spaceId'] ?? '').toString().trim(),
          deviceId: (auth['deviceId'] ?? '').toString().trim(),
          runtimeId: (auth['runtimeId'] ?? '').toString().trim(),
          fingerprint: (auth['fingerprint'] ?? '').toString().trim(),
          expiresAt: expiresAt,
          protocol: 'amitia.device-runtime',
          envelopeVersion: 1,
          schemaVersion: '1.0.0',
          websocketPath: '/api/device-mesh/v1/runtime/ws',
        ),
        local: local,
      );
    } on DioException catch (error) {
      final status = error.response?.statusCode;
      throw BackendConnectionError(
        status == 401
            ? BackendConnectionErrorCode.CREDENTIAL_INVALID
            : BackendConnectionErrorCode.CREDENTIAL_UNAVAILABLE,
        status == 404
            ? '当前设备尚未与配置的统筹中心配对'
            : 'failed to load Device Mesh cloud credential',
      );
    } finally {
      dio.close(force: true);
    }
  }
}

BackendConnectionConfig cloudProxyConnectionConfig({
  required BackendConnectionConfig local,
  required MobileCloudDeviceCredential auth,
}) {
  return BackendConnectionConfig(
    schemaVersion: 1,
    generation: 1,
    endpoint: BackendConnectionEndpoint(
      host: local.endpoint.host,
      port: local.endpoint.port,
      httpScheme: local.endpoint.httpScheme,
      webSocketScheme: local.endpoint.webSocketScheme,
      pathPrefix: '/internal/device-mesh/provider',
      livenessPath: '/readyz',
      readinessPath: '/readyz',
    ),
    authStrategy: BackendAuthStrategy.localToken,
    credential: local.credential,
    spaceId: auth.spaceId,
    deviceId: auth.deviceId,
    runtimeId: auth.runtimeId,
    fingerprint: auth.fingerprint,
  );
}

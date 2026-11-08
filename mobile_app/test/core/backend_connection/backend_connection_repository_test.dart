import 'dart:async';

import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_availability.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_config.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_endpoint.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_credential.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_error.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_source.dart';
import 'package:amitia_app/core/backend_connection/providers/backend_connection_providers.dart';
import 'package:amitia_app/core/runtime/runtime_bridge_snapshot.dart';
import 'package:amitia_app/core/runtime/runtime_bridge_provider.dart';
import 'package:amitia_app/core/runtime/runtime_bridge_state.dart';
import 'package:amitia_app/core/runtime/status/runtime_status_provider.dart';
import 'package:amitia_app/core/runtime/backend/mobile_deployment_config_repository.dart';
import 'package:amitia_app/core/runtime/backend/mobile_deployment_mode.dart';
import 'package:amitia_app/core/runtime/backend/mobile_backend_providers.dart';
import 'package:amitia_app/core/device_mesh/mobile_cloud_device_credential_store.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

class _FakeSource implements BackendConnectionSource {
  final List<BackendConnectionAvailability> responses;
  final List<int> requestedGenerations = [];
  int _index = 0;

  _FakeSource(this.responses);

  @override
  Future<BackendConnectionAvailability> resolve({
    int? expectedRuntimeGeneration,
  }) async {
    requestedGenerations.add(expectedRuntimeGeneration ?? 0);
    if (_index < responses.length) {
      return responses[_index++];
    }
    return BackendConnectionUnavailable();
  }
}

BackendConnectionConfig _makeConfig(int generation, {String fingerprint = ''}) {
  return BackendConnectionConfig(
    schemaVersion: 1,
    generation: generation,
    endpoint: BackendConnectionEndpoint(
      host: '127.0.0.1',
      port: 18899,
      httpScheme: 'http',
      webSocketScheme: 'ws',
      livenessPath: '/livez',
      readinessPath: '/readyz',
    ),
    authStrategy: BackendAuthStrategy.localToken,
    credential: BackendConnectionCredential.tryCreate('a' * 32)!,
    fingerprint: fingerprint,
  );
}

void main() {
  group('BackendConnectionRepository resolve', () {
    test('cloud resolution assigns repository business generation', () async {
      final source = _FakeSource([BackendConnectionAvailable(_makeConfig(1))]);
      final repo = DefaultBackendConnectionRepository(source);

      final result = await repo.resolve(expectedRuntimeGeneration: null);
      expect(result, isA<BackendConnectionAvailable>());
      expect(source.requestedGenerations, [0]);
    });

    test('cloud resolution preserves certificate fingerprint', () async {
      final fingerprint = 'a' * 64;
      final source = _FakeSource([
        BackendConnectionAvailable(_makeConfig(1, fingerprint: fingerprint)),
      ]);
      final repo = DefaultBackendConnectionRepository(source);

      final result = await repo.resolve(expectedRuntimeGeneration: null);
      expect(result, isA<BackendConnectionAvailable>());
      expect(
        (result as BackendConnectionAvailable).config.fingerprint,
        fingerprint,
      );
    });

    test('runtime status uses repository business generation', () async {
      final first = _makeConfig(1, fingerprint: 'a' * 64);
      final second = _makeConfig(1, fingerprint: 'b' * 64);
      final source = _FakeSource([
        BackendConnectionAvailable(first),
        BackendConnectionAvailable(second),
        BackendConnectionAvailable(second),
      ]);
      final repository = DefaultBackendConnectionRepository(source);

      await repository.resolve(expectedRuntimeGeneration: null);
      final advanced = await repository.resolve(
        expectedRuntimeGeneration: null,
      );
      final observed = await RepositoryBackendConnectionSource(
        repository,
      ).resolve(expectedRuntimeGeneration: 3);

      expect((advanced as BackendConnectionAvailable).config.generation, 2);
      expect((observed as BackendConnectionAvailable).config.generation, 2);
    });

    test(
      'certificate fingerprint change advances business generation',
      () async {
        final first = _makeConfig(1, fingerprint: 'a' * 64);
        final second = _makeConfig(1, fingerprint: 'b' * 64);
        final source = _FakeSource([
          BackendConnectionAvailable(first),
          BackendConnectionAvailable(second),
        ]);
        final repo = DefaultBackendConnectionRepository(source);

        final firstResult = await repo.resolve(expectedRuntimeGeneration: null);
        final secondResult = await repo.resolve(
          expectedRuntimeGeneration: null,
        );
        expect(
          (firstResult as BackendConnectionAvailable).config.generation,
          1,
        );
        expect(
          (secondResult as BackendConnectionAvailable).config.generation,
          2,
        );
      },
    );

    test('accepts matching generation', () async {
      final config = _makeConfig(4);
      final source = _FakeSource([BackendConnectionAvailable(config)]);
      final repo = DefaultBackendConnectionRepository(source);

      final result = await repo.resolve(expectedRuntimeGeneration: 4);
      expect(result, isA<BackendConnectionAvailable>());
      expect((result as BackendConnectionAvailable).config.generation, 4);
      expect(repo.cached?.generation, 4);
    });

    test('rejects wrong generation', () async {
      final config = _makeConfig(7);
      final source = _FakeSource([BackendConnectionAvailable(config)]);
      final repo = DefaultBackendConnectionRepository(source);

      final result = await repo.resolve(expectedRuntimeGeneration: 8);
      expect(result, isA<BackendConnectionUnavailable>());
      expect(repo.cached, isNull);
    });

    test('rejects unavailable result', () async {
      final source = _FakeSource([BackendConnectionUnavailable()]);
      final repo = DefaultBackendConnectionRepository(source);

      final result = await repo.resolve(expectedRuntimeGeneration: 5);
      expect(result, isA<BackendConnectionUnavailable>());
      expect(repo.cached, isNull);
    });
  });

  test('云端业务连接通过本机代理并保留中心指纹', () {
    final local = _makeConfig(4);
    final protected = cloudProxyConnectionConfig(
      local: local,
      auth: MobileCloudDeviceCredential(
        cloudBaseUrl: 'https://172.16.176.114:54796',
        credentialId: 'credential-a',
        credential: 'credential-secret',
        spaceId: 'space-a',
        deviceId: 'device-a',
        runtimeId: 'runtime-a',
        fingerprint: 'a' * 64,
        expiresAt: DateTime.now().toUtc().add(const Duration(hours: 1)),
        protocol: 'amitia.device-runtime',
        envelopeVersion: 1,
        schemaVersion: '1.0.0',
        websocketPath: '/api/device-mesh/v1/runtime/ws',
      ),
    );
    expect(protected.endpoint.host, local.endpoint.host);
    expect(protected.endpoint.port, local.endpoint.port);
    expect(protected.endpoint.pathPrefix, '/internal/device-mesh/provider');
    expect(protected.authStrategy, BackendAuthStrategy.localToken);
    expect(protected.fingerprint, 'a' * 64);
    expect(protected.credential.revealForTransport(), 'a' * 32);
  });

  group('BackendConnectionRepository epoch', () {
    test('late resolve cannot overwrite cache after newer resolve', () async {
      final completer1 = Completer<BackendConnectionAvailability>();
      final completer2 = Completer<BackendConnectionAvailability>();
      late List<Completer<BackendConnectionAvailability>> completers;
      completers = [completer1, completer2];

      final source = _DeferredSource(completers);
      final repo = DefaultBackendConnectionRepository(source);

      final future1 = repo.resolve(expectedRuntimeGeneration: 6);
      final future2 = repo.resolve(expectedRuntimeGeneration: 7);

      completer2.complete(BackendConnectionAvailable(_makeConfig(7)));
      await Future.delayed(const Duration(milliseconds: 1));

      completer1.complete(BackendConnectionAvailable(_makeConfig(6)));
      await Future.delayed(const Duration(milliseconds: 1));

      final result1 = await future1;
      final result2 = await future2;

      expect(result2, isA<BackendConnectionAvailable>());
      expect((result2 as BackendConnectionAvailable).config.generation, 7);
      expect(result1, isA<BackendConnectionUnavailable>());
      expect(repo.cached?.generation, 7);
    });

    test('invalidate cancels old commit authority', () async {
      final completer = Completer<BackendConnectionAvailability>();
      final source = _ControllableSource(completer);
      final repo = DefaultBackendConnectionRepository(source);

      final future = repo.resolve(expectedRuntimeGeneration: 5);
      repo.invalidate();
      completer.complete(BackendConnectionAvailable(_makeConfig(5)));
      await Future.delayed(const Duration(milliseconds: 1));

      final result = await future;
      expect(result, isA<BackendConnectionUnavailable>());
      expect(repo.cached, isNull);
    });

    test('invalidate clears cache and increments epoch', () async {
      final config = _makeConfig(3);
      final source = _FakeSource([
        BackendConnectionAvailable(config),
        BackendConnectionAvailable(config),
      ]);
      final repo = DefaultBackendConnectionRepository(source);

      await repo.resolve(expectedRuntimeGeneration: 3);
      expect(repo.cached?.generation, 3);

      repo.invalidate();
      expect(repo.cached, isNull);
    });
  });

  group('backendConnectionProvider watches runtimeSnapshotProvider', () {
    test('STOPPED does not resolve native connection', () async {
      final source = _CountingSource();
      final container = ProviderContainer(
        overrides: [
          runtimeSnapshotProvider.overrideWith((ref) async* {
            yield const RuntimeBridgeSnapshot(
              schemaVersion: 1,
              state: RuntimeBridgeState.stopped,
              generation: 0,
              runtimeInstalled: true,
              runtimeAvailable: true,
            );
          }),
          backendConnectionSourceProvider.overrideWithValue(source),
        ],
      );
      addTearDown(container.dispose);

      await container.read(runtimeSnapshotProvider.future);
      await container
          .read(backendConnectionProvider.future)
          .catchError((_) => const BackendConnectionUnavailable());

      expect(source.resolveCallCount, 0);
    });

    test('cloud provider resolves after runtime becomes ready', () async {
      final runtimeEvents = StreamController<RuntimeBridgeSnapshot>();
      final source = _FakeSource([BackendConnectionAvailable(_makeConfig(1))]);
      final container = ProviderContainer(
        overrides: [
          mobileDeploymentConfigRepositoryProvider.overrideWithValue(
            const _CloudConfigRepository(),
          ),
          runtimeSnapshotProvider.overrideWith((ref) => runtimeEvents.stream),
          backendConnectionSourceProvider.overrideWithValue(source),
        ],
      );
      addTearDown(() async {
        await runtimeEvents.close();
        container.dispose();
      });

      await container.read(mobileDeploymentConfigProvider.notifier).init();
      final states =
          StreamController<BackendConnectionAvailability>.broadcast();
      final subscription = container.listen(backendConnectionProvider, (
        previous,
        next,
      ) {
        final value = next.value;
        if (value != null) states.add(value);
      });
      addTearDown(subscription.close);
      addTearDown(states.close);

      runtimeEvents.add(
        const RuntimeBridgeSnapshot(
          schemaVersion: 1,
          state: RuntimeBridgeState.starting,
          generation: 1,
          runtimeInstalled: true,
          runtimeAvailable: true,
        ),
      );
      final starting = await states.stream
          .firstWhere(
            (value) =>
                value is BackendConnectionUnavailable &&
                value.error?.code ==
                    BackendConnectionErrorCode.RUNTIME_NOT_READY,
          )
          .timeout(const Duration(seconds: 2));
      expect(starting, isA<BackendConnectionUnavailable>());
      expect(source.requestedGenerations, isEmpty);

      final readyCompleter = Completer<BackendConnectionAvailability>();
      final readySubscription = states.stream.listen((value) {
        if (value is BackendConnectionAvailable &&
            !readyCompleter.isCompleted) {
          readyCompleter.complete(value);
        }
      });
      addTearDown(readySubscription.cancel);
      runtimeEvents.add(
        const RuntimeBridgeSnapshot(
          schemaVersion: 1,
          state: RuntimeBridgeState.ready,
          generation: 1,
          runtimeInstalled: true,
          runtimeAvailable: true,
        ),
      );

      final ready = await readyCompleter.future.timeout(
        const Duration(seconds: 2),
      );
      expect(ready, isA<BackendConnectionAvailable>());
      expect((ready as BackendConnectionAvailable).config.generation, 1);
      expect(source.requestedGenerations, [0]);
    });

    test(
      'cloud provider invalidates stale connection after runtime leaves ready',
      () async {
        final runtimeEvents = StreamController<RuntimeBridgeSnapshot>();
        final source = _FakeSource([
          BackendConnectionAvailable(_makeConfig(1)),
        ]);
        final container = ProviderContainer(
          overrides: [
            mobileDeploymentConfigRepositoryProvider.overrideWithValue(
              const _CloudConfigRepository(),
            ),
            runtimeSnapshotProvider.overrideWith((ref) => runtimeEvents.stream),
            backendConnectionSourceProvider.overrideWithValue(source),
          ],
        );
        addTearDown(() async {
          await runtimeEvents.close();
          container.dispose();
        });

        await container.read(mobileDeploymentConfigProvider.notifier).init();
        final states =
            StreamController<BackendConnectionAvailability>.broadcast();
        final subscription = container.listen(backendConnectionProvider, (
          previous,
          next,
        ) {
          final value = next.value;
          if (value != null) states.add(value);
        });
        addTearDown(subscription.close);
        addTearDown(states.close);

        runtimeEvents.add(
          const RuntimeBridgeSnapshot(
            schemaVersion: 1,
            state: RuntimeBridgeState.ready,
            generation: 1,
            runtimeInstalled: true,
            runtimeAvailable: true,
          ),
        );
        await states.stream
            .firstWhere((value) => value is BackendConnectionAvailable)
            .timeout(const Duration(seconds: 2));

        runtimeEvents.add(
          const RuntimeBridgeSnapshot(
            schemaVersion: 1,
            state: RuntimeBridgeState.stopped,
            generation: 1,
            runtimeInstalled: true,
            runtimeAvailable: true,
          ),
        );

        final stopped = await states.stream
            .firstWhere((value) => value is BackendConnectionUnavailable)
            .timeout(const Duration(seconds: 2));
        expect(
          (stopped as BackendConnectionUnavailable).error?.code,
          BackendConnectionErrorCode.RUNTIME_NOT_READY,
        );
      },
    );
  });

  test('cloud resolves share one in-flight request', () async {
    final completer = Completer<BackendConnectionAvailability>();
    final source = _DeferredSource([completer]);
    final repository = DefaultBackendConnectionRepository(source);

    final first = repository.resolve(expectedRuntimeGeneration: null);
    final second = repository.resolve(expectedRuntimeGeneration: null);

    expect(identical(first, second), isTrue);
    completer.complete(
      BackendConnectionAvailable(_makeConfig(1, fingerprint: 'a' * 64)),
    );
    final results = await Future.wait([first, second]);

    expect(source.requestCount, 1);
    expect(results[0], isA<BackendConnectionAvailable>());
    expect(results[1], isA<BackendConnectionAvailable>());
    expect(
      (results[0] as BackendConnectionAvailable).config.generation,
      (results[1] as BackendConnectionAvailable).config.generation,
    );
  });
}

class _CloudConfigRepository implements MobileDeploymentConfigRepository {
  const _CloudConfigRepository();

  @override
  Future<MobileDeploymentConfig> load() async {
    return const MobileDeploymentConfig(
      mode: MobileDeploymentMode.cloud,
      remoteCoreUri: 'https://core.example:18899',
    );
  }

  @override
  Future<void> save(MobileDeploymentConfig config) async {}
}

class _DeferredSource implements BackendConnectionSource {
  final List<Completer<BackendConnectionAvailability>> completers;
  int _index = 0;
  int requestCount = 0;

  _DeferredSource(this.completers);

  @override
  Future<BackendConnectionAvailability> resolve({
    int? expectedRuntimeGeneration,
  }) async {
    requestCount++;
    if (_index < completers.length) {
      return completers[_index++].future;
    }
    return const BackendConnectionUnavailable();
  }
}

class _ControllableSource implements BackendConnectionSource {
  final Completer<BackendConnectionAvailability> completer;

  _ControllableSource(this.completer);

  @override
  Future<BackendConnectionAvailability> resolve({
    int? expectedRuntimeGeneration,
  }) {
    return completer.future;
  }
}

class _CountingSource implements BackendConnectionSource {
  int resolveCallCount = 0;

  @override
  Future<BackendConnectionAvailability> resolve({
    int? expectedRuntimeGeneration,
  }) async {
    resolveCallCount++;
    return const BackendConnectionUnavailable();
  }
}

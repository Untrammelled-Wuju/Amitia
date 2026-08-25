import 'dart:async';

import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_availability.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_config.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_endpoint.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_credential.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_repository.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_source.dart';
import 'package:amitia_app/core/backend_connection/providers/backend_connection_providers.dart';
import 'package:amitia_app/core/runtime/runtime_bridge_snapshot.dart';
import 'package:amitia_app/core/runtime/runtime_bridge_state.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

class _FakeSource implements BackendConnectionSource {
  final List<BackendConnectionAvailability> responses;
  final List<int> requestedGenerations = [];
  int _index = 0;

  _FakeSource(this.responses);

  @override
  Future<BackendConnectionAvailability> resolve({int? expectedRuntimeGeneration}) async {
    requestedGenerations.add(expectedRuntimeGeneration ?? 0);
    if (_index < responses.length) {
      return responses[_index++];
    }
    return BackendConnectionUnavailable();
  }
}

BackendConnectionConfig _makeConfig(int generation) {
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
  );
}

void main() {
  group('BackendConnectionRepository resolve', () {
    test('cloud resolution assigns repository business generation', () async {
      final source = _FakeSource([
        BackendConnectionAvailable(_makeConfig(1)),
      ]);
      final repo = DefaultBackendConnectionRepository(source);

      final result = await repo.resolve(expectedRuntimeGeneration: null);
      expect(result, isA<BackendConnectionAvailable>());
      expect(source.requestedGenerations, [0]);
    });

    test('accepts matching generation', () async {
      final config = _makeConfig(4);
      final source = _FakeSource([
        BackendConnectionAvailable(config),
      ]);
      final repo = DefaultBackendConnectionRepository(source);

      final result = await repo.resolve(expectedRuntimeGeneration: 4);
      expect(result, isA<BackendConnectionAvailable>());
      expect((result as BackendConnectionAvailable).config.generation, 4);
      expect(repo.cached?.generation, 4);
    });

    test('rejects wrong generation', () async {
      final config = _makeConfig(7);
      final source = _FakeSource([
        BackendConnectionAvailable(config),
      ]);
      final repo = DefaultBackendConnectionRepository(source);

      final result = await repo.resolve(expectedRuntimeGeneration: 8);
      expect(result, isA<BackendConnectionUnavailable>());
      expect(repo.cached, isNull);
    });

    test('rejects unavailable result', () async {
      final source = _FakeSource([
        BackendConnectionUnavailable(),
      ]);
      final repo = DefaultBackendConnectionRepository(source);

      final result = await repo.resolve(expectedRuntimeGeneration: 5);
      expect(result, isA<BackendConnectionUnavailable>());
      expect(repo.cached, isNull);
    });
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

      await container
          .read(backendConnectionProvider.future)
          .catchError((_) => const BackendConnectionUnavailable());

      expect(source.resolveCallCount, 0);
    });
  });
}

class _DeferredSource implements BackendConnectionSource {
  final List<Completer<BackendConnectionAvailability>> completers;
  int _index = 0;

  _DeferredSource(this.completers);

  @override
  Future<BackendConnectionAvailability> resolve({int? expectedRuntimeGeneration}) async {
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
  Future<BackendConnectionAvailability> resolve({int? expectedRuntimeGeneration}) {
    return completer.future;
  }
}

class _CountingSource implements BackendConnectionSource {
  int resolveCallCount = 0;

  @override
  Future<BackendConnectionAvailability> resolve({int? expectedRuntimeGeneration}) async {
    resolveCallCount++;
    return const BackendConnectionUnavailable();
  }
}

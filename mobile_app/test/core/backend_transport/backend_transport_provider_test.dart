import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_availability.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_source.dart';
import 'package:amitia_app/core/backend_connection/providers/backend_connection_providers.dart';
import 'package:amitia_app/core/backend_transport/providers/backend_transport_providers.dart';
import 'package:amitia_app/core/backend_transport/state/backend_transport_state.dart';
import 'package:amitia_app/core/runtime/runtime_bridge_snapshot.dart';
import 'package:amitia_app/core/runtime/runtime_bridge_state.dart';
import 'package:amitia_app/core/runtime/runtime_bridge_provider.dart';

void main() {
  group('TransportAvailable generation', () {
    test('TransportAvailable carries generation', () {
      final state = TransportAvailable(generation: 11);
      expect(state.generation, 11);
    });

    test('TransportAvailable asserts generation > 0', () {
      expect(
        () => TransportAvailable(generation: 0),
        throwsA(isA<AssertionError>()),
      );
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
      final result = await container
          .read(backendConnectionProvider.future)
          .catchError((_) => const BackendConnectionUnavailable());

      expect(result, isA<BackendConnectionUnavailable>());
      expect(source.resolveCallCount, 0);
    });

    test('STARTING does not resolve native connection', () async {
      final source = _CountingSource();
      final container = ProviderContainer(
        overrides: [
          runtimeSnapshotProvider.overrideWith((ref) async* {
            yield const RuntimeBridgeSnapshot(
              schemaVersion: 1,
              state: RuntimeBridgeState.starting,
              generation: 2,
              runtimeInstalled: true,
              runtimeAvailable: true,
            );
          }),
          backendConnectionSourceProvider.overrideWithValue(source),
        ],
      );
      addTearDown(container.dispose);

      await container.read(runtimeSnapshotProvider.future);
      final result = await container
          .read(backendConnectionProvider.future)
          .catchError((_) => const BackendConnectionUnavailable());

      expect(result, isA<BackendConnectionUnavailable>());
      expect(source.resolveCallCount, 0);
    });
  });

  group('backendTransportProvider state mapping', () {
    test('transport generation provider returns 0 when unavailable', () async {
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
        ],
      );

      await Future.delayed(const Duration(milliseconds: 5));

      final generation = container.read(backendTransportGenerationProvider);
      expect(generation, 0);
    });
  });
}

class _CountingSource implements BackendConnectionSource {
  int resolveCallCount = 0;

  @override
  Future<BackendConnectionAvailability> resolve({int? expectedRuntimeGeneration}) async {
    resolveCallCount++;
    return const BackendConnectionUnavailable();
  }
}

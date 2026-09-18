import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_availability.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_config.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_endpoint.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_credential.dart';
import 'package:amitia_app/core/backend_connection/providers/backend_connection_providers.dart';
import 'package:amitia_app/core/backend_transport/providers/backend_transport_providers.dart';
import 'package:amitia_app/core/backend_transport/state/backend_transport_state.dart';
import 'package:amitia_app/core/runtime/runtime_bridge_provider.dart';
import 'package:amitia_app/core/runtime/runtime_bridge_snapshot.dart';
import 'package:amitia_app/core/runtime/runtime_bridge_state.dart';
import 'package:amitia_app/core/runtime/status/runtime_status_provider.dart';

import 'fakes/fake_runtime_bridge.dart';
import 'fakes/fake_backend_connection_source.dart';

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
  TestWidgetsFlutterBinding.ensureInitialized();

  group('RuntimeStatus TransportStateSource', () {
    test('maps TransportAvailable generation to snapshot', () async {
      final connectionSource = FakeBackendConnectionSource()
        ..setAvailability(BackendConnectionAvailable(_makeConfig(11)));
      final bridge = FakeRuntimeBridge()
        ..setSnapshot(
          const RuntimeBridgeSnapshot(
            schemaVersion: 1,
            state: RuntimeBridgeState.ready,
            generation: 11,
            runtimeInstalled: true,
            runtimeAvailable: true,
          ),
        );
      final container = ProviderContainer(
        overrides: [
          runtimeBridgeProvider.overrideWithValue(bridge),
          backendConnectionSourceProvider.overrideWithValue(connectionSource),
          deviceLocalBackendTransportProvider.overrideWith(
            _AvailableTransportNotifier11.new,
          ),
        ],
      );
      addTearDown(container.dispose);

      await container.read(deviceLocalBackendTransportProvider.future);
      await _waitForGeneration(container, 11);
      final status = container.read(runtimeStatusCurrentProvider);

      expect(status.generation, 11);
    });

    test('TransportUnavailable maps to generation 0', () async {
      final connectionSource = FakeBackendConnectionSource()
        ..setAvailability(const BackendConnectionUnavailable());
      final bridge = FakeRuntimeBridge()
        ..setSnapshot(
          const RuntimeBridgeSnapshot(
            schemaVersion: 1,
            state: RuntimeBridgeState.stopped,
            generation: 0,
            runtimeInstalled: true,
            runtimeAvailable: true,
          ),
        );
      final container = ProviderContainer(
        overrides: [
          runtimeBridgeProvider.overrideWithValue(bridge),
          backendConnectionSourceProvider.overrideWithValue(connectionSource),
          deviceLocalBackendTransportProvider.overrideWith(
            _UnavailableTransportNotifier.new,
          ),
        ],
      );
      addTearDown(container.dispose);

      await Future.delayed(const Duration(milliseconds: 5));

      final status = container.read(runtimeStatusCurrentProvider);

      expect(status.generation, 0);
    });

    test('does not side-read transport generation from notifier', () async {
      final connectionSource = FakeBackendConnectionSource()
        ..setAvailability(BackendConnectionAvailable(_makeConfig(12)));
      final bridge = FakeRuntimeBridge()
        ..setSnapshot(
          const RuntimeBridgeSnapshot(
            schemaVersion: 1,
            state: RuntimeBridgeState.ready,
            generation: 12,
            runtimeInstalled: true,
            runtimeAvailable: true,
          ),
        );
      final container = ProviderContainer(
        overrides: [
          runtimeBridgeProvider.overrideWithValue(bridge),
          backendConnectionSourceProvider.overrideWithValue(connectionSource),
          deviceLocalBackendTransportProvider.overrideWith(
            _AvailableTransportNotifier12.new,
          ),
        ],
      );
      addTearDown(container.dispose);

      await container.read(deviceLocalBackendTransportProvider.future);
      await _waitForGeneration(container, 12);
      final status = container.read(runtimeStatusCurrentProvider);

      expect(status.generation, 12);
      expect(status.httpAvailable, true);
    });
  });
}

Future<void> _waitForGeneration(ProviderContainer container, int generation) async {
  for (var attempt = 0; attempt < 50; attempt++) {
    if (container.read(runtimeStatusCurrentProvider).generation == generation) {
      return;
    }
    await Future<void>.delayed(const Duration(milliseconds: 10));
  }
}

class _AvailableTransportNotifier11 extends DeviceLocalBackendTransportNotifier {
  @override
  Future<BackendTransportState> build() async {
    return const TransportAvailable(generation: 11);
  }
}

class _AvailableTransportNotifier12 extends DeviceLocalBackendTransportNotifier {
  @override
  Future<BackendTransportState> build() async {
    return const TransportAvailable(generation: 12);
  }
}

class _UnavailableTransportNotifier extends DeviceLocalBackendTransportNotifier {
  @override
  Future<BackendTransportState> build() async {
    return const TransportUnavailable();
  }
}

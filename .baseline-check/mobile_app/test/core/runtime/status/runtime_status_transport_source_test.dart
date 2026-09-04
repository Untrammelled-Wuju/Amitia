import 'dart:async';

import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_availability.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_config.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_endpoint.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_credential.dart';
import 'package:amitia_app/core/backend_connection/providers/backend_connection_providers.dart';
import 'package:amitia_app/core/backend_transport/providers/backend_transport_providers.dart';
import 'package:amitia_app/core/backend_transport/state/backend_transport_state.dart';
import 'package:amitia_app/core/runtime/runtime_bridge_snapshot.dart';
import 'package:amitia_app/core/runtime/runtime_bridge_state.dart';
import 'package:amitia_app/core/runtime/status/runtime_status_provider.dart';
import 'package:amitia_app/core/runtime/status/runtime_status_snapshot.dart';

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
  group('RuntimeStatus TransportStateSource', () {
    test('maps TransportAvailable generation to snapshot', () async {
      final container = ProviderContainer(
        overrides: [
          runtimeSnapshotProvider.overrideWith((ref) async* {
            yield const RuntimeBridgeSnapshot(
              schemaVersion: 1,
              state: RuntimeBridgeState.ready,
              generation: 11,
              runtimeInstalled: true,
              runtimeAvailable: true,
            );
          }),
        ],
      );

      await container.read(backendTransportProvider.future);

      final statusFuture = container.read(runtimeStatusSnapshotProvider.future);

      final status = await statusFuture;

      expect(status.generation, 11);
    });

    test('TransportUnavailable maps to generation 0', () async {
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

      final status = container.read(runtimeStatusCurrentProvider);

      expect(status.generation, 0);
    });

    test('does not side-read transport generation from notifier', () async {
      final container = ProviderContainer(
        overrides: [
          runtimeSnapshotProvider.overrideWith((ref) async* {
            yield const RuntimeBridgeSnapshot(
              schemaVersion: 1,
              state: RuntimeBridgeState.ready,
              generation: 12,
              runtimeInstalled: true,
              runtimeAvailable: true,
            );
          }),
        ],
      );

      await container.read(backendTransportProvider.future);

      final statusFuture = container.read(runtimeStatusSnapshotProvider.future);
      final status = await statusFuture;

      expect(status.generation, 12);
      expect(status.httpAvailable, true);
    });
  });
}

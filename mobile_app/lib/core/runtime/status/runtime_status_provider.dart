import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../runtime_bridge_provider.dart';
import '../../backend_connection/providers/backend_connection_providers.dart';
import '../../backend_transport/providers/backend_transport_providers.dart';
import '../../backend_transport/state/backend_transport_state.dart';
import '../backend/mobile_backend_providers.dart';
import '../backend/mobile_deployment_mode.dart';
import 'default_runtime_status_projection.dart';
import 'runtime_status_projection.dart';
import 'runtime_status_snapshot.dart';

final runtimeStatusProjectionProvider = Provider<RuntimeStatusProjection>((
  ref,
) {
  final bridge = ref.watch(runtimeBridgeProvider);
  final connectionSource = ref.watch(backendConnectionSourceProvider);
  final mode = ref.watch(mobileDeploymentConfigProvider).mode;
  final transportSource = _TransportStateSourceImpl(ref, mode);
  final projection = DefaultRuntimeStatusProjection(
    bridge: bridge,
    connectionSource: connectionSource,
    transportStateSource: transportSource,
  );

  ref.onDispose(() async {
    await projection.dispose();
    await transportSource.close();
  });

  unawaited(projection.initialize());

  return projection;
});

final runtimeStatusSnapshotProvider = StreamProvider<RuntimeStatusSnapshot>((
  ref,
) {
  final projection = ref.watch(runtimeStatusProjectionProvider);
  return projection.snapshots.distinct();
});

final runtimeStatusCurrentProvider = Provider<RuntimeStatusSnapshot>((ref) {
  final async = ref.watch(runtimeStatusSnapshotProvider);
  return async.value ?? ref.read(runtimeStatusProjectionProvider).current;
});

class _TransportStateSourceImpl implements TransportStateSource {
  final Ref _ref;
  final bool _usesDeviceLocalTransport;
  final StreamController<TransportStateSnapshot> _controller =
      StreamController<TransportStateSnapshot>.broadcast();
  TransportStateSnapshot _current = TransportStateSnapshot.initial();
  bool _closed = false;

  _TransportStateSourceImpl(this._ref, MobileDeploymentMode mode)
    : _usesDeviceLocalTransport = mode == MobileDeploymentMode.local {
    final transportProvider = _usesDeviceLocalTransport
        ? deviceLocalBackendTransportProvider
        : backendTransportProvider;
    _ref.listen<AsyncValue<BackendTransportState>>(transportProvider, (
      previous,
      next,
    ) {
      next.when(
        data: (state) {
          _current = _mapTransportState(state);
          _controller.add(_current);
        },
        loading: () {
          _current = const TransportStateSnapshot(
            generation: 0,
            httpState: BackendHttpState.idle,
            webSocketState: BackendWebSocketState.idle,
          );
          _controller.add(_current);
        },
        error: (_, __) {
          _current = const TransportStateSnapshot(
            generation: 0,
            httpState: BackendHttpState.unavailable,
            webSocketState: BackendWebSocketState.disconnected,
          );
          _controller.add(_current);
        },
      );
    }, fireImmediately: true);
  }

  TransportStateSnapshot _mapTransportState(BackendTransportState state) {
    switch (state) {
      case TransportAvailable(:final generation):
        return TransportStateSnapshot(
          generation: generation,
          httpState: BackendHttpState.available,
          webSocketState: BackendWebSocketState.idle,
        );
      case TransportIdle():
        return const TransportStateSnapshot(
          generation: 0,
          httpState: BackendHttpState.idle,
          webSocketState: BackendWebSocketState.idle,
        );
      case TransportUnavailable():
        return const TransportStateSnapshot(
          generation: 0,
          httpState: BackendHttpState.unavailable,
          webSocketState: BackendWebSocketState.disconnected,
        );
      case TransportClosed():
        return const TransportStateSnapshot(
          generation: 0,
          httpState: BackendHttpState.closed,
          webSocketState: BackendWebSocketState.closed,
        );
    }
  }

  @override
  Stream<TransportStateSnapshot> get snapshots => _controller.stream;

  @override
  TransportStateSnapshot get current => _current;

  Future<void> close() async {
    if (_closed) return;
    _closed = true;
    await _controller.close();
  }
}

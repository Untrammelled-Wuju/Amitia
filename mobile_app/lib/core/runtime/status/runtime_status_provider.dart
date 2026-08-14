import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../runtime_bridge_provider.dart';
import '../../backend_connection/providers/backend_connection_providers.dart';
import '../../backend_transport/providers/backend_transport_providers.dart';
import '../../backend_transport/state/backend_transport_state.dart';
import 'default_runtime_status_projection.dart';
import 'runtime_status_projection.dart';
import 'runtime_status_snapshot.dart';

final runtimeStatusProjectionProvider =
    Provider<RuntimeStatusProjection>((ref) {
  final bridge = ref.watch(runtimeBridgeProvider);
  final connectionSource = ref.watch(backendConnectionSourceProvider);
  final transportSource = _TransportStateSourceImpl(ref);
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

final runtimeStatusSnapshotProvider =
    StreamProvider<RuntimeStatusSnapshot>((ref) {
  final projection = ref.watch(runtimeStatusProjectionProvider);
  return projection.snapshots.distinct();
});

final runtimeStatusCurrentProvider = Provider<RuntimeStatusSnapshot>((ref) {
  final projection = ref.watch(runtimeStatusProjectionProvider);
  return projection.current;
});

class _TransportStateSourceImpl implements TransportStateSource {
  final Ref _ref;
  final StreamController<TransportStateSnapshot> _controller =
      StreamController<TransportStateSnapshot>.broadcast();
  bool _closed = false;

  _TransportStateSourceImpl(this._ref) {
    _ref.listen<AsyncValue<BackendTransportState>>(
      backendTransportProvider,
      (previous, next) {
        next.when(
          data: (state) {
            _controller.add(_mapTransportState(state));
          },
          loading: () {
            _controller.add(const TransportStateSnapshot(
              generation: 0,
              httpState: BackendHttpState.idle,
              webSocketState: BackendWebSocketState.idle,
            ));
          },
          error: (_, __) {
            _controller.add(const TransportStateSnapshot(
              generation: 0,
              httpState: BackendHttpState.unavailable,
              webSocketState: BackendWebSocketState.disconnected,
            ));
          },
        );
      },
      fireImmediately: true,
    );
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
  TransportStateSnapshot get current {
    final transportAsync = _ref.read(backendTransportProvider);
    return transportAsync.when(
      data: (state) => _mapTransportState(state),
      loading: () => const TransportStateSnapshot(
        generation: 0,
        httpState: BackendHttpState.idle,
        webSocketState: BackendWebSocketState.idle,
      ),
      error: (_, __) => const TransportStateSnapshot(
        generation: 0,
        httpState: BackendHttpState.unavailable,
        webSocketState: BackendWebSocketState.disconnected,
      ),
    );
  }

  Future<void> close() async {
    if (_closed) return;
    _closed = true;
    await _controller.close();
  }
}

import 'dart:async';

import '../runtime_bridge.dart';
import '../runtime_bridge_snapshot.dart';
import '../runtime_bridge_state.dart';
import '../runtime_bridge_error.dart';
import '../../backend_connection/backend_connection_availability.dart';
import '../../backend_connection/backend_connection_source.dart';
import 'runtime_status_snapshot.dart';
import 'runtime_status_phase.dart';
import 'runtime_status_error.dart';
import 'runtime_status_projection.dart';

abstract interface class TransportStateSource {
  Stream<TransportStateSnapshot> get snapshots;
  TransportStateSnapshot get current;
}

final class TransportStateSnapshot {
  final int generation;
  final BackendHttpState httpState;
  final BackendWebSocketState webSocketState;

  const TransportStateSnapshot({
    required this.generation,
    required this.httpState,
    required this.webSocketState,
  });

  factory TransportStateSnapshot.initial() {
    return const TransportStateSnapshot(
      generation: 0,
      httpState: BackendHttpState.idle,
      webSocketState: BackendWebSocketState.idle,
    );
  }
}

enum BackendHttpState {
  idle,
  available,
  unavailable,
  closed,
}

enum BackendWebSocketState {
  idle,
  connecting,
  connected,
  disconnected,
  closed,
}

class DefaultRuntimeStatusProjection implements RuntimeStatusProjection {
  final RuntimeBridge _bridge;
  final BackendConnectionSource _connectionSource;
  final TransportStateSource? _transportStateSource;

  final StreamController<RuntimeStatusSnapshot> _snapshotController =
      StreamController<RuntimeStatusSnapshot>.broadcast();

  StreamSubscription<RuntimeBridgeSnapshot>? _bridgeSubscription;
  StreamSubscription<TransportStateSnapshot>? _transportSubscription;

  RuntimeBridgeSnapshot _lastBridge = RuntimeBridgeSnapshot.initial();
  BackendConnectionAvailability _lastConnection =
      BackendConnectionUnavailable();
  TransportStateSnapshot _lastTransport = TransportStateSnapshot.initial();

  RuntimeStatusSnapshot _current = RuntimeStatusSnapshot.initial();
  bool _disposed = false;

  DefaultRuntimeStatusProjection({
    required RuntimeBridge bridge,
    required BackendConnectionSource connectionSource,
    TransportStateSource? transportStateSource,
  })  : _bridge = bridge,
        _connectionSource = connectionSource,
        _transportStateSource = transportStateSource;

  Future<void> initialize() async {
    if (_disposed) return;

    _bridgeSubscription = _bridge.snapshots.listen(
      _handleBridgeSnapshot,
      onError: (_) => _handleBridgeError(),
    );

    final currentBridge = await _bridge.snapshot();
    _handleBridgeSnapshot(currentBridge);

    if (_transportStateSource case final source?) {
      _transportSubscription = source.snapshots.listen(
        _handleTransportState,
        onError: (_) => _handleTransportError(),
      );
      _handleTransportState(source.current);
    }
  }

  Future<void> _refreshConnection() async {
    try {
      final expectedGen = _lastBridge.state == RuntimeBridgeState.ready ? _lastBridge.generation : 0;
      final result = await _connectionSource.resolve(expectedGeneration: expectedGen);
      if (!_disposed) {
        _lastConnection = result;
        _rederive();
      }
    } catch (_) {
      if (!_disposed) {
        _lastConnection = BackendConnectionUnavailable();
        _rederive();
      }
    }
  }

  void _invalidateConnection() {
    if (!_disposed) {
      _lastConnection = BackendConnectionUnavailable();
      _rederive();
    }
  }

  void _handleBridgeSnapshot(RuntimeBridgeSnapshot snapshot) {
    if (_disposed) return;
    if (snapshot.generation < _lastBridge.generation) return;
    final previousState = _lastBridge.state;
    final previousGeneration = _lastBridge.generation;
    _lastBridge = snapshot;

    final generationChanged = snapshot.generation != previousGeneration;
    final enteredReady = snapshot.state == RuntimeBridgeState.ready && previousState != RuntimeBridgeState.ready;
    final leftReady = snapshot.state != RuntimeBridgeState.ready && previousState == RuntimeBridgeState.ready;
    final isTerminalState = snapshot.state == RuntimeBridgeState.stopping ||
        snapshot.state == RuntimeBridgeState.stopped ||
        snapshot.state == RuntimeBridgeState.failed;

    if (enteredReady || (snapshot.state == RuntimeBridgeState.ready && generationChanged)) {
      _refreshConnection();
    } else if (leftReady || isTerminalState) {
      _invalidateConnection();
    } else {
      _rederive();
    }
  }

  void _handleTransportState(TransportStateSnapshot snapshot) {
    if (_disposed) return;
    _lastTransport = snapshot;
    _rederive();
  }

  void _handleBridgeError() {
    if (_disposed) return;
    final failedSnapshot = RuntimeBridgeSnapshot(
      schemaVersion: _lastBridge.schemaVersion,
      state: RuntimeBridgeState.failed,
      generation: _lastBridge.generation,
      runtimeInstalled: _lastBridge.runtimeInstalled,
      runtimeAvailable: _lastBridge.runtimeAvailable,
      lastError: _lastBridge.lastError,
      manifest: _lastBridge.manifest,
    );
    _lastBridge = failedSnapshot;
    _rederive();
  }

  void _handleTransportError() {
    if (_disposed) return;
    _lastTransport = const TransportStateSnapshot(
      generation: 0,
      httpState: BackendHttpState.unavailable,
      webSocketState: BackendWebSocketState.disconnected,
    );
    _rederive();
  }

  void _rederive() {
    final derived = deriveRuntimeStatus(
      runtime: _lastBridge,
      connection: _lastConnection,
      transport: _lastTransport,
    );

    if (derived != _current) {
      _current = derived;
      _snapshotController.add(_current);
    }
  }

  @override
  Stream<RuntimeStatusSnapshot> get snapshots => _snapshotController.stream;

  @override
  RuntimeStatusSnapshot get current => _current;

  @override
  Future<void> dispose() async {
    if (_disposed) return;
    _disposed = true;
    await _bridgeSubscription?.cancel();
    _bridgeSubscription = null;
    await _transportSubscription?.cancel();
    _transportSubscription = null;
    await _snapshotController.close();
  }
}

RuntimeStatusSnapshot deriveRuntimeStatus({
  required RuntimeBridgeSnapshot runtime,
  required BackendConnectionAvailability connection,
  required TransportStateSnapshot transport,
}) {
  if (!runtime.runtimeAvailable && runtime.state == RuntimeBridgeState.unavailable) {
    return RuntimeStatusSnapshot(
      phase: RuntimeStatusPhase.unavailable,
      runtimeState: runtime.state,
      runtimeReady: false,
      runtimeInstalled: runtime.runtimeInstalled,
      backendConfigured: false,
      httpAvailable: false,
      webSocketConnected: false,
      businessAvailable: false,
      generation: runtime.generation,
      runtimeVersion: runtime.manifest?.runtimeVersion ?? '',
      primaryError: _mapRuntimeError(runtime.lastError),
    );
  }

  if (!runtime.runtimeInstalled) {
    return RuntimeStatusSnapshot(
      phase: RuntimeStatusPhase.installRequired,
      runtimeState: runtime.state,
      runtimeReady: false,
      runtimeInstalled: false,
      backendConfigured: _isConnectionConfigured(connection),
      httpAvailable: false,
      webSocketConnected: false,
      businessAvailable: false,
      generation: runtime.generation,
      runtimeVersion: runtime.manifest?.runtimeVersion ?? '',
      primaryError: _deriveConnectionError(connection),
    );
  }

  switch (runtime.state) {
    case RuntimeBridgeState.unavailable:
      return RuntimeStatusSnapshot(
        phase: RuntimeStatusPhase.unavailable,
        runtimeState: runtime.state,
        runtimeReady: false,
        runtimeInstalled: runtime.runtimeInstalled,
        backendConfigured: false,
        httpAvailable: false,
        webSocketConnected: false,
        businessAvailable: false,
        generation: runtime.generation,
        runtimeVersion: runtime.manifest?.runtimeVersion ?? '',
        primaryError: _mapRuntimeError(runtime.lastError),
      );

    case RuntimeBridgeState.notInstalled:
      return RuntimeStatusSnapshot(
        phase: RuntimeStatusPhase.installRequired,
        runtimeState: runtime.state,
        runtimeReady: false,
        runtimeInstalled: false,
        backendConfigured: _isConnectionConfigured(connection),
        httpAvailable: false,
        webSocketConnected: false,
        businessAvailable: false,
        generation: runtime.generation,
        runtimeVersion: runtime.manifest?.runtimeVersion ?? '',
      );

    case RuntimeBridgeState.stopped:
      return RuntimeStatusSnapshot(
        phase: RuntimeStatusPhase.initializing,
        runtimeState: runtime.state,
        runtimeReady: false,
        runtimeInstalled: true,
        backendConfigured: _isConnectionConfigured(connection),
        httpAvailable: false,
        webSocketConnected: false,
        businessAvailable: false,
        generation: runtime.generation,
        runtimeVersion: runtime.manifest?.runtimeVersion ?? '',
      );

    case RuntimeBridgeState.installing:
    case RuntimeBridgeState.starting:
      return RuntimeStatusSnapshot(
        phase: RuntimeStatusPhase.starting,
        runtimeState: runtime.state,
        runtimeReady: false,
        runtimeInstalled: runtime.runtimeInstalled,
        backendConfigured: false,
        httpAvailable: false,
        webSocketConnected: false,
        businessAvailable: false,
        generation: runtime.generation,
        runtimeVersion: runtime.manifest?.runtimeVersion ?? '',
        primaryError: _mapRuntimeError(runtime.lastError),
      );

    case RuntimeBridgeState.stopping:
      return RuntimeStatusSnapshot(
        phase: RuntimeStatusPhase.stopping,
        runtimeState: runtime.state,
        runtimeReady: false,
        runtimeInstalled: runtime.runtimeInstalled,
        backendConfigured: false,
        httpAvailable: false,
        webSocketConnected: false,
        businessAvailable: false,
        generation: runtime.generation,
        runtimeVersion: runtime.manifest?.runtimeVersion ?? '',
      );

    case RuntimeBridgeState.failed:
      return RuntimeStatusSnapshot(
        phase: RuntimeStatusPhase.failed,
        runtimeState: runtime.state,
        runtimeReady: false,
        runtimeInstalled: runtime.runtimeInstalled,
        backendConfigured: false,
        httpAvailable: false,
        webSocketConnected: false,
        businessAvailable: false,
        generation: runtime.generation,
        runtimeVersion: runtime.manifest?.runtimeVersion ?? '',
        primaryError: _mapRuntimeError(runtime.lastError) ??
            const RuntimeStatusError(
              source: RuntimeStatusErrorSource.runtime,
              code: 'RUNTIME_FAILED',
              message: 'Runtime failed',
            ),
      );

    case RuntimeBridgeState.ready:
      return _deriveReadyStatus(runtime, connection, transport);
  }
}

RuntimeStatusSnapshot _deriveReadyStatus(
  RuntimeBridgeSnapshot runtime,
  BackendConnectionAvailability connection,
  TransportStateSnapshot transport,
) {
  final connectionConfigured = _isConnectionConfigured(connection);
  final runtimeVersion = runtime.manifest?.runtimeVersion ?? '';

  if (!connectionConfigured) {
    final connectionError = _deriveConnectionError(connection);
    if (_hasManifestInconsistency(runtime, connection)) {
      return RuntimeStatusSnapshot(
        phase: RuntimeStatusPhase.degraded,
        runtimeState: runtime.state,
        runtimeReady: true,
        runtimeInstalled: true,
        backendConfigured: false,
        httpAvailable: false,
        webSocketConnected: false,
        businessAvailable: false,
        generation: runtime.generation,
        runtimeVersion: runtimeVersion,
        primaryError: connectionError ??
            const RuntimeStatusError(
              source: RuntimeStatusErrorSource.consistency,
              code: 'RUNTIME_STATUS_INCONSISTENT',
              message: 'Runtime ready but backend connection unavailable',
            ),
      );
    }
    return RuntimeStatusSnapshot(
      phase: RuntimeStatusPhase.degraded,
      runtimeState: runtime.state,
      runtimeReady: true,
      runtimeInstalled: true,
      backendConfigured: false,
      httpAvailable: false,
      webSocketConnected: false,
      businessAvailable: false,
      generation: runtime.generation,
      runtimeVersion: runtimeVersion,
      primaryError: connectionError,
    );
  }

  final generationConsistent = _isGenerationConsistent(runtime, connection, transport);
  if (!generationConsistent) {
    return RuntimeStatusSnapshot(
      phase: RuntimeStatusPhase.degraded,
      runtimeState: runtime.state,
      runtimeReady: true,
      runtimeInstalled: true,
      backendConfigured: true,
      httpAvailable: false,
      webSocketConnected: false,
      businessAvailable: false,
      generation: runtime.generation,
      runtimeVersion: runtimeVersion,
      primaryError: const RuntimeStatusError(
        source: RuntimeStatusErrorSource.consistency,
        code: 'GENERATION_MISMATCH',
        message: 'Backend transport generation mismatch',
      ),
    );
  }

  final httpAvailable = transport.httpState == BackendHttpState.available;
  final webSocketConnected =
      transport.webSocketState == BackendWebSocketState.connected;

  if (!httpAvailable) {
    return RuntimeStatusSnapshot(
      phase: RuntimeStatusPhase.degraded,
      runtimeState: runtime.state,
      runtimeReady: true,
      runtimeInstalled: true,
      backendConfigured: true,
      httpAvailable: false,
      webSocketConnected: false,
      businessAvailable: false,
      generation: runtime.generation,
      runtimeVersion: runtimeVersion,
      primaryError: const RuntimeStatusError(
        source: RuntimeStatusErrorSource.http,
        code: 'HTTP_UNAVAILABLE',
        message: 'HTTP transport unavailable',
      ),
    );
  }

  if (!webSocketConnected) {
    return RuntimeStatusSnapshot(
      phase: RuntimeStatusPhase.degraded,
      runtimeState: runtime.state,
      runtimeReady: true,
      runtimeInstalled: true,
      backendConfigured: true,
      httpAvailable: true,
      webSocketConnected: false,
      businessAvailable: true,
      generation: runtime.generation,
      runtimeVersion: runtimeVersion,
      primaryError: const RuntimeStatusError(
        source: RuntimeStatusErrorSource.webSocket,
        code: 'WEBSOCKET_DISCONNECTED',
        message: 'WebSocket disconnected',
      ),
    );
  }

  return RuntimeStatusSnapshot(
    phase: RuntimeStatusPhase.ready,
    runtimeState: runtime.state,
    runtimeReady: true,
    runtimeInstalled: true,
    backendConfigured: true,
    httpAvailable: true,
    webSocketConnected: true,
    businessAvailable: true,
    generation: runtime.generation,
    runtimeVersion: runtimeVersion,
  );
}

bool _isConnectionConfigured(BackendConnectionAvailability connection) {
  return connection is BackendConnectionAvailable;
}

bool _hasManifestInconsistency(
  RuntimeBridgeSnapshot runtime,
  BackendConnectionAvailability connection,
) {
  if (runtime.runtimeAvailable &&
      runtime.manifest != null &&
      connection is BackendConnectionUnavailable) {
    return true;
  }
  return false;
}

RuntimeStatusError? _deriveConnectionError(
  BackendConnectionAvailability connection,
) {
  if (connection is BackendConnectionUnavailable) {
    return const RuntimeStatusError(
      source: RuntimeStatusErrorSource.backendConnection,
      code: 'CONNECTION_UNAVAILABLE',
      message: 'Backend connection unavailable',
    );
  }
  if (connection is BackendConnectionResolving) {
    return const RuntimeStatusError(
      source: RuntimeStatusErrorSource.backendConnection,
      code: 'CONNECTION_RESOLVING',
      message: 'Backend connection resolving',
    );
  }
  return null;
}

RuntimeStatusError? _mapRuntimeError(RuntimeBridgeError? lastError) {
  if (lastError == null) return null;
  return RuntimeStatusError(
    source: RuntimeStatusErrorSource.runtime,
    code: lastError.code,
    message: lastError.message,
  );
}

bool _isGenerationConsistent(
  RuntimeBridgeSnapshot runtime,
  BackendConnectionAvailability connection,
  TransportStateSnapshot transport,
) {
  if (transport.generation != 0 && transport.generation != runtime.generation) {
    return false;
  }
  if (connection is BackendConnectionAvailable) {
    if (connection.config.generation != runtime.generation) {
      return false;
    }
  }
  return true;
}

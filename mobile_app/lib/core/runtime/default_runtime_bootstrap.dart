import 'dart:async';
import 'runtime_bridge.dart';
import 'runtime_bridge_snapshot.dart';
import 'runtime_bridge_state.dart';
import 'runtime_bootstrap.dart';
import 'runtime_bootstrap_snapshot.dart';
import 'runtime_bootstrap_phase.dart';
import 'runtime_bootstrap_policy.dart';

class DefaultRuntimeBootstrap implements RuntimeBootstrap {
  final RuntimeBridge _bridge;
  final RuntimeBootstrapPolicy _policy;

  final StreamController<RuntimeBootstrapSnapshot> _snapshotController =
      StreamController<RuntimeBootstrapSnapshot>.broadcast();

  StreamSubscription<RuntimeBridgeSnapshot>? _bridgeSubscription;
  Future<void>? _initialization;
  bool _disposed = false;
  bool _autoStartDecided = false;
  int _lastGeneration = 0;

  RuntimeBootstrapSnapshot _current = RuntimeBootstrapSnapshot.initial();

  DefaultRuntimeBootstrap({
    required RuntimeBridge bridge,
    RuntimeBootstrapPolicy policy = const RuntimeBootstrapPolicy(),
  }) : _bridge = bridge,
       _policy = policy;

  @override
  Stream<RuntimeBootstrapSnapshot> get snapshots async* {
    yield _current;
    yield* _snapshotController.stream;
  }

  @override
  Future<void> initialize() async {
    if (_disposed) return;
    if (_initialization != null) return await _initialization;

    _initialization = _performInitialization();
    await _initialization;
  }

  Future<void> _performInitialization() async {
    _bridgeSubscription = _bridge.snapshots.listen(
      _handleRuntimeSnapshot,
      onError: (_) => _handleBridgeError(),
    );

    final current = await _bridge.snapshot();
    _handleRuntimeSnapshot(current);
  }

  void _handleRuntimeSnapshot(RuntimeBridgeSnapshot snapshot) {
    if (_disposed) return;
    if (snapshot.generation < _lastGeneration) return;
    _lastGeneration = snapshot.generation;

    final phase = _mapToBootstrapPhase(snapshot);
    _updatePhase(phase, snapshot);
  }

  RuntimeBootstrapPhase _mapToBootstrapPhase(RuntimeBridgeSnapshot snapshot) {
    switch (snapshot.state) {
      case RuntimeBridgeState.unavailable:
        return RuntimeBootstrapPhase.unavailable;
      case RuntimeBridgeState.notInstalled:
        return RuntimeBootstrapPhase.installRequired;
      case RuntimeBridgeState.stopped:
        return RuntimeBootstrapPhase.stopped;
      case RuntimeBridgeState.installing:
        return RuntimeBootstrapPhase.initializing;
      case RuntimeBridgeState.starting:
        return RuntimeBootstrapPhase.starting;
      case RuntimeBridgeState.ready:
        return RuntimeBootstrapPhase.ready;
      case RuntimeBridgeState.stopping:
        return RuntimeBootstrapPhase.stopping;
      case RuntimeBridgeState.failed:
        return RuntimeBootstrapPhase.failed;
    }
  }

  void _updatePhase(
    RuntimeBootstrapPhase phase,
    RuntimeBridgeSnapshot snapshot,
  ) {
    _current = RuntimeBootstrapSnapshot(
      phase: phase,
      runtime: snapshot,
      error: snapshot.lastError,
    );
    _snapshotController.add(_current);
  }

  Future<void> _requestStart() async {
    await _bridge.start();
  }

  void _handleBridgeError() {
    if (_disposed) return;
    _current = RuntimeBootstrapSnapshot(
      phase: RuntimeBootstrapPhase.failed,
      runtime: _current.runtime,
      error: _current.runtime.lastError,
    );
    _snapshotController.add(_current);
  }

  @override
  Future<void> dispose() async {
    if (_disposed) return;
    _disposed = true;
    await _bridgeSubscription?.cancel();
    _bridgeSubscription = null;
    await _snapshotController.close();
  }
}

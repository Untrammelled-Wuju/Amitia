import 'dart:async';

import 'runtime_bridge.dart';
import 'runtime_bridge_error.dart';
import 'runtime_bridge_snapshot.dart';
import 'runtime_bridge_state.dart';
import 'runtime_bootstrap.dart';
import 'runtime_bootstrap_phase.dart';
import 'runtime_bootstrap_policy.dart';
import 'runtime_bootstrap_snapshot.dart';

/// Initializes and, when configured, installs the embedded runtime.
///
/// Runtime process startup is deliberately not owned here. Profile-aware
/// startup belongs to MobileBackendLifecycle, which prevents the bootstrap
/// path and deployment lifecycle from racing each other with duplicate start
/// commands.
class DefaultRuntimeBootstrap implements RuntimeBootstrap {
  final RuntimeBridge _bridge;
  final RuntimeBootstrapPolicy _policy;

  final StreamController<RuntimeBootstrapSnapshot> _snapshotController =
      StreamController<RuntimeBootstrapSnapshot>.broadcast();

  StreamSubscription<RuntimeBridgeSnapshot>? _bridgeSubscription;
  Future<void>? _initialization;
  bool _disposed = false;
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

    if (_disposed) return;
    if (_policy.autoInstallRuntime &&
        current.state == RuntimeBridgeState.notInstalled) {
      final installed = await _requestInstall();

      if (_disposed || !installed) return;
      final refreshed = await _bridge.snapshot();
      _handleRuntimeSnapshot(refreshed);
    }
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

  Future<bool> _requestInstall() async {
    try {
      final result = await _bridge.install().timeout(_policy.installTimeout);
      if (_disposed) return false;

      if (!result.accepted || result.error != null) {
        _emitBootstrapFailure(
          result.error ??
              const RuntimeBridgeError(
                code: 'INSTALL_REJECTED',
                message: 'Runtime installation command was rejected',
                retryable: true,
              ),
          runtime: result.snapshot,
        );
        return false;
      }

      _handleRuntimeSnapshot(result.snapshot);
      return true;
    } on TimeoutException {
      if (_disposed) return false;
      _emitBootstrapFailure(
        RuntimeBridgeError(
          code: 'INSTALL_TIMEOUT',
          message:
              'Runtime installation timed out after ${_policy.installTimeout.inSeconds}s',
          retryable: true,
        ),
      );
      return false;
    }
  }

  void _emitBootstrapFailure(
    RuntimeBridgeError error, {
    RuntimeBridgeSnapshot? runtime,
  }) {
    if (_disposed) return;
    _current = RuntimeBootstrapSnapshot(
      phase: RuntimeBootstrapPhase.failed,
      runtime: runtime ?? _current.runtime,
      error: error,
    );
    _snapshotController.add(_current);
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

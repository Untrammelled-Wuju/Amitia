import 'dart:async';
import 'package:flutter/services.dart';
import 'runtime_bridge.dart';
import 'runtime_bridge_state.dart';
import 'runtime_bridge_snapshot.dart';
import 'runtime_bridge_error.dart';
import 'runtime_manifest_summary.dart';

class RuntimeBridgeContract {
  static const String methodChannelName = 'com.amitia.runtime/bridge';
  static const String eventChannelName = 'com.amitia.runtime/events';

  static const String methodSnapshot = 'runtime.snapshot';
  static const String methodStart = 'runtime.start';
  static const String methodStop = 'runtime.stop';
  static const String methodInstall = 'runtime.install';
  static const String methodVerify = 'runtime.verify';
  static const String methodRepair = 'runtime.repair';
  static const String methodManifestSummary = 'runtime.manifestSummary';
}

class MethodChannelRuntimeBridge implements RuntimeBridge {
  static const MethodChannel _methodChannel = MethodChannel(
    RuntimeBridgeContract.methodChannelName,
  );
  static const EventChannel _eventChannel = EventChannel(
    RuntimeBridgeContract.eventChannelName,
  );

  final StreamController<RuntimeBridgeSnapshot> _snapshotController =
      StreamController<RuntimeBridgeSnapshot>.broadcast();

  StreamSubscription<dynamic>? _eventSubscription;
  Timer? _reconnectTimer;
  Timer? _snapshotPollTimer;
  Duration? _snapshotPollInterval;
  int _reconnectAttempt = 0;
  int _snapshotListenerCount = 0;
  bool _snapshotPollInFlight = false;
  bool _disposed = false;
  RuntimeBridgeSnapshot _lastSnapshot = RuntimeBridgeSnapshot.initial();

  MethodChannelRuntimeBridge() {
    _subscribeToEvents();
  }

  void _subscribeToEvents() {
    if (_disposed || _eventSubscription != null) return;
    _eventSubscription = _eventChannel.receiveBroadcastStream().listen(
      (dynamic event) {
        if (_disposed) return;
        _reconnectAttempt = 0;
        if (event is Map) {
          try {
            final snapshot = RuntimeBridgeSnapshot.fromMap(
              Map<String, dynamic>.from(event),
            );
            _emitSnapshot(snapshot);
          } catch (_) {
            // A malformed event must not permanently tear down runtime
            // observation; the next native snapshot can still recover state.
          }
        }
      },
      onError: (Object error) {
        _handleEventStreamClosed();
      },
      onDone: _handleEventStreamClosed,
      cancelOnError: false,
    );
  }

  void _handleEventStreamClosed() {
    if (_disposed) return;
    final old = _eventSubscription;
    if (old == null) return;
    _eventSubscription = null;
    unawaited(_completeEventStreamRestart(old));
  }

  Future<void> _completeEventStreamRestart(
    StreamSubscription<dynamic> old,
  ) async {
    try {
      await old.cancel();
    } catch (_) {
      // Reconciliation below is the authority even if cancellation itself
      // reports an error. Waiting here prevents old native onCancel from
      // racing a replacement onListen and tearing down the new subscription.
    }
    if (_disposed) return;
    try {
      await _reconcileSnapshotAfterStreamLoss();
    } catch (_) {
      // Snapshot reconciliation is best-effort. A malformed/transient method
      // response must never disable EventChannel recovery permanently.
    } finally {
      _scheduleEventReconnect();
    }
  }

  Future<void> _reconcileSnapshotAfterStreamLoss() async {
    if (_disposed) return;
    final current = await snapshot();
    if (_disposed) return;
    _snapshotController.add(current);
  }

  void _scheduleEventReconnect() {
    if (_disposed || _reconnectTimer != null) return;
    final exponent = _reconnectAttempt.clamp(0, 5).toInt();
    final delayMs = (250 * (1 << exponent)).clamp(250, 5000).toInt();
    _reconnectAttempt++;
    _reconnectTimer = Timer(Duration(milliseconds: delayMs), () {
      _reconnectTimer = null;
      _subscribeToEvents();
    });
  }

  @override
  Stream<RuntimeBridgeSnapshot> get snapshots =>
      Stream<RuntimeBridgeSnapshot>.multi((controller) {
        final initialSnapshot = _lastSnapshot;
        late final StreamSubscription<RuntimeBridgeSnapshot> subscription;
        controller.onCancel = () {
          _snapshotListenerCount--;
          _syncSnapshotPolling();
          unawaited(subscription.cancel());
        };
        subscription = _snapshotController.stream.listen(
          controller.add,
          onError: controller.addError,
          onDone: controller.close,
        );
        _snapshotListenerCount++;
        _syncSnapshotPolling();
        unawaited(_refreshSnapshot());
        controller.add(initialSnapshot);
      }, isBroadcast: true);

  @override
  Future<RuntimeBridgeSnapshot> snapshot() async {
    final result = await _readSnapshot();
    if (result == null) return RuntimeBridgeSnapshot.initial();
    _emitSnapshot(result);
    return result;
  }

  Future<RuntimeBridgeSnapshot?> _readSnapshot() async {
    try {
      final result = await _methodChannel.invokeMethod<Map<Object?, Object?>>(
        RuntimeBridgeContract.methodSnapshot,
      );
      if (result == null) return null;
      final map = <String, dynamic>{};
      for (final entry in result.entries) {
        map[entry.key.toString()] = entry.value;
      }
      return RuntimeBridgeSnapshot.fromMap(map);
    } on PlatformException {
      return null;
    } on MissingPluginException {
      return null;
    } catch (_) {
      // Treat malformed/forward-incompatible payloads as bridge unavailable
      // rather than letting bootstrap fail with an uncaught TypeError.
      return null;
    }
  }

  void _emitSnapshot(RuntimeBridgeSnapshot snapshot) {
    if (_disposed || snapshot.generation < _lastSnapshot.generation) return;
    if (snapshot == _lastSnapshot) return;
    _lastSnapshot = snapshot;
    _snapshotController.add(snapshot);
    _syncSnapshotPolling();
  }

  void _syncSnapshotPolling() {
    if (_disposed) return;
    if (_snapshotListenerCount <= 0) {
      _snapshotPollTimer?.cancel();
      _snapshotPollTimer = null;
      _snapshotPollInterval = null;
      return;
    }
    final interval =
        _lastSnapshot.state == RuntimeBridgeState.ready ||
            _lastSnapshot.state == RuntimeBridgeState.failed
        ? const Duration(seconds: 5)
        : const Duration(seconds: 1);
    if (_snapshotPollTimer != null && _snapshotPollInterval == interval) return;
    _snapshotPollTimer?.cancel();
    _snapshotPollInterval = interval;
    _snapshotPollTimer = Timer.periodic(
      interval,
      (_) => unawaited(_refreshSnapshot()),
    );
  }

  Future<void> _refreshSnapshot() async {
    if (_disposed || _snapshotPollInFlight) return;
    _snapshotPollInFlight = true;
    try {
      final snapshot = await _readSnapshot();
      if (snapshot != null) {
        _emitSnapshot(snapshot);
      }
    } finally {
      _snapshotPollInFlight = false;
    }
  }

  @override
  Future<RuntimeBridgeCommandResult> start() async {
    try {
      final result = await _methodChannel.invokeMethod<Map<Object?, Object?>>(
        RuntimeBridgeContract.methodStart,
      );
      if (result == null) {
        return RuntimeBridgeCommandResult(
          accepted: false,
          snapshot: RuntimeBridgeSnapshot.initial(),
          error: const RuntimeBridgeError(
            code: 'BRIDGE_UNAVAILABLE',
            message: 'Runtime bridge returned null',
            retryable: false,
          ),
        );
      }
      final map = <String, dynamic>{};
      for (final entry in result.entries) {
        map[entry.key.toString()] = entry.value;
      }
      return RuntimeBridgeCommandResult.fromMap(map);
    } on PlatformException catch (e) {
      return RuntimeBridgeCommandResult(
        accepted: false,
        snapshot: RuntimeBridgeSnapshot.initial(),
        error: RuntimeBridgeError(
          code: e.code,
          message: e.message ?? 'Platform error',
          retryable: false,
        ),
      );
    } on MissingPluginException {
      return RuntimeBridgeCommandResult(
        accepted: false,
        snapshot: RuntimeBridgeSnapshot.initial(),
        error: const RuntimeBridgeError(
          code: 'BRIDGE_UNAVAILABLE',
          message: 'Runtime bridge not available',
          retryable: false,
        ),
      );
    } catch (error) {
      return RuntimeBridgeCommandResult(
        accepted: false,
        snapshot: RuntimeBridgeSnapshot.initial(),
        error: RuntimeBridgeError(
          code: 'BRIDGE_PROTOCOL_ERROR',
          message: 'Invalid runtime bridge response: ${error.runtimeType}',
          retryable: true,
        ),
      );
    }
  }

  @override
  Future<RuntimeBridgeCommandResult> stop() async {
    try {
      final result = await _methodChannel.invokeMethod<Map<Object?, Object?>>(
        RuntimeBridgeContract.methodStop,
      );
      if (result == null) {
        return RuntimeBridgeCommandResult(
          accepted: false,
          snapshot: RuntimeBridgeSnapshot.initial(),
          error: const RuntimeBridgeError(
            code: 'BRIDGE_UNAVAILABLE',
            message: 'Runtime bridge returned null',
            retryable: false,
          ),
        );
      }
      final map = <String, dynamic>{};
      for (final entry in result.entries) {
        map[entry.key.toString()] = entry.value;
      }
      return RuntimeBridgeCommandResult.fromMap(map);
    } on PlatformException catch (e) {
      return RuntimeBridgeCommandResult(
        accepted: false,
        snapshot: RuntimeBridgeSnapshot.initial(),
        error: RuntimeBridgeError(
          code: e.code,
          message: e.message ?? 'Platform error',
          retryable: false,
        ),
      );
    } on MissingPluginException {
      return RuntimeBridgeCommandResult(
        accepted: false,
        snapshot: RuntimeBridgeSnapshot.initial(),
        error: const RuntimeBridgeError(
          code: 'BRIDGE_UNAVAILABLE',
          message: 'Runtime bridge not available',
          retryable: false,
        ),
      );
    } catch (error) {
      return RuntimeBridgeCommandResult(
        accepted: false,
        snapshot: RuntimeBridgeSnapshot.initial(),
        error: RuntimeBridgeError(
          code: 'BRIDGE_PROTOCOL_ERROR',
          message: 'Invalid runtime bridge response: ${error.runtimeType}',
          retryable: true,
        ),
      );
    }
  }

  @override
  Future<RuntimeBridgeCommandResult> install() async {
    try {
      final result = await _methodChannel.invokeMethod<Map<Object?, Object?>>(
        RuntimeBridgeContract.methodInstall,
      );
      if (result == null) {
        return RuntimeBridgeCommandResult(
          accepted: false,
          snapshot: RuntimeBridgeSnapshot.initial(),
          error: const RuntimeBridgeError(
            code: 'BRIDGE_UNAVAILABLE',
            message: 'Runtime bridge returned null',
            retryable: false,
          ),
        );
      }
      final map = <String, dynamic>{};
      for (final entry in result.entries) {
        map[entry.key.toString()] = entry.value;
      }
      return RuntimeBridgeCommandResult.fromMap(map);
    } on PlatformException catch (e) {
      return RuntimeBridgeCommandResult(
        accepted: false,
        snapshot: RuntimeBridgeSnapshot.initial(),
        error: RuntimeBridgeError(
          code: e.code,
          message: e.message ?? 'Platform error',
          retryable: false,
        ),
      );
    } on MissingPluginException {
      return RuntimeBridgeCommandResult(
        accepted: false,
        snapshot: RuntimeBridgeSnapshot.initial(),
        error: const RuntimeBridgeError(
          code: 'BRIDGE_UNAVAILABLE',
          message: 'Runtime bridge not available',
          retryable: false,
        ),
      );
    } catch (error) {
      return RuntimeBridgeCommandResult(
        accepted: false,
        snapshot: RuntimeBridgeSnapshot.initial(),
        error: RuntimeBridgeError(
          code: 'BRIDGE_PROTOCOL_ERROR',
          message: 'Invalid runtime bridge response: ${error.runtimeType}',
          retryable: true,
        ),
      );
    }
  }

  @override
  Future<RuntimeBridgeCommandResult> verify() async {
    try {
      final result = await _methodChannel.invokeMethod<Map<Object?, Object?>>(
        RuntimeBridgeContract.methodVerify,
      );
      if (result == null) {
        return RuntimeBridgeCommandResult(
          accepted: false,
          snapshot: RuntimeBridgeSnapshot.initial(),
          error: const RuntimeBridgeError(
            code: 'BRIDGE_UNAVAILABLE',
            message: 'Runtime bridge returned null',
            retryable: false,
          ),
        );
      }
      final map = <String, dynamic>{};
      for (final entry in result.entries) {
        map[entry.key.toString()] = entry.value;
      }
      return RuntimeBridgeCommandResult.fromMap(map);
    } on PlatformException catch (e) {
      return RuntimeBridgeCommandResult(
        accepted: false,
        snapshot: RuntimeBridgeSnapshot.initial(),
        error: RuntimeBridgeError(
          code: e.code,
          message: e.message ?? 'Platform error',
          retryable: false,
        ),
      );
    } on MissingPluginException {
      return RuntimeBridgeCommandResult(
        accepted: false,
        snapshot: RuntimeBridgeSnapshot.initial(),
        error: const RuntimeBridgeError(
          code: 'BRIDGE_UNAVAILABLE',
          message: 'Runtime bridge not available',
          retryable: false,
        ),
      );
    } catch (error) {
      return RuntimeBridgeCommandResult(
        accepted: false,
        snapshot: RuntimeBridgeSnapshot.initial(),
        error: RuntimeBridgeError(
          code: 'BRIDGE_PROTOCOL_ERROR',
          message: 'Invalid runtime bridge response: ${error.runtimeType}',
          retryable: true,
        ),
      );
    }
  }

  @override
  Future<RuntimeBridgeCommandResult> repair() async {
    try {
      final result = await _methodChannel.invokeMethod<Map<Object?, Object?>>(
        RuntimeBridgeContract.methodRepair,
      );
      if (result == null) {
        return RuntimeBridgeCommandResult(
          accepted: false,
          snapshot: RuntimeBridgeSnapshot.initial(),
          error: const RuntimeBridgeError(
            code: 'BRIDGE_UNAVAILABLE',
            message: 'Runtime bridge returned null',
            retryable: false,
          ),
        );
      }
      final map = <String, dynamic>{};
      for (final entry in result.entries) {
        map[entry.key.toString()] = entry.value;
      }
      return RuntimeBridgeCommandResult.fromMap(map);
    } on PlatformException catch (e) {
      return RuntimeBridgeCommandResult(
        accepted: false,
        snapshot: RuntimeBridgeSnapshot.initial(),
        error: RuntimeBridgeError(
          code: e.code,
          message: e.message ?? 'Platform error',
          retryable: false,
        ),
      );
    } on MissingPluginException {
      return RuntimeBridgeCommandResult(
        accepted: false,
        snapshot: RuntimeBridgeSnapshot.initial(),
        error: const RuntimeBridgeError(
          code: 'BRIDGE_UNAVAILABLE',
          message: 'Runtime bridge not available',
          retryable: false,
        ),
      );
    } catch (error) {
      return RuntimeBridgeCommandResult(
        accepted: false,
        snapshot: RuntimeBridgeSnapshot.initial(),
        error: RuntimeBridgeError(
          code: 'BRIDGE_PROTOCOL_ERROR',
          message: 'Invalid runtime bridge response: ${error.runtimeType}',
          retryable: true,
        ),
      );
    }
  }

  @override
  Future<RuntimeManifestSummary?> manifestSummary() async {
    try {
      final result = await _methodChannel.invokeMethod<Map<Object?, Object?>>(
        RuntimeBridgeContract.methodManifestSummary,
      );
      if (result == null) return null;
      final map = <String, dynamic>{};
      for (final entry in result.entries) {
        map[entry.key.toString()] = entry.value;
      }
      return RuntimeManifestSummary.fromMap(map);
    } on PlatformException {
      return null;
    } on MissingPluginException {
      return null;
    } catch (_) {
      return null;
    }
  }

  @override
  Future<void> dispose() async {
    _disposed = true;
    _reconnectTimer?.cancel();
    _reconnectTimer = null;
    _snapshotPollTimer?.cancel();
    _snapshotPollTimer = null;
    _snapshotListenerCount = 0;
    await _eventSubscription?.cancel();
    _eventSubscription = null;
    await _snapshotController.close();
  }
}

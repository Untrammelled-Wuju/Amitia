import 'dart:async';
import 'package:flutter/services.dart';
import 'runtime_bridge.dart';
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
  static const MethodChannel _methodChannel =
      MethodChannel(RuntimeBridgeContract.methodChannelName);
  static const EventChannel _eventChannel =
      EventChannel(RuntimeBridgeContract.eventChannelName);

  final StreamController<RuntimeBridgeSnapshot> _snapshotController =
      StreamController<RuntimeBridgeSnapshot>.broadcast();

  StreamSubscription<dynamic>? _eventSubscription;
  bool _disposed = false;

  MethodChannelRuntimeBridge() {
    _subscribeToEvents();
  }

  void _subscribeToEvents() {
    _eventSubscription = _eventChannel.receiveBroadcastStream().listen(
      (dynamic event) {
        if (_disposed) return;
        if (event is Map) {
          try {
            final snapshot = RuntimeBridgeSnapshot.fromMap(
              Map<String, dynamic>.from(event),
            );
            _snapshotController.add(snapshot);
          } catch (_) {
          }
        }
      },
      onError: (Object error) {
        if (_disposed) return;
      },
    );
  }

  @override
  Stream<RuntimeBridgeSnapshot> get snapshots => _snapshotController.stream;

  @override
  Future<RuntimeBridgeSnapshot> snapshot() async {
    try {
      final result = await _methodChannel.invokeMethod<Map<Object?, Object?>>(
        RuntimeBridgeContract.methodSnapshot,
      );
      if (result == null) return RuntimeBridgeSnapshot.initial();
      final map = <String, dynamic>{};
      for (final entry in result.entries) {
        map[entry.key.toString()] = entry.value;
      }
      return RuntimeBridgeSnapshot.fromMap(map);
    } on PlatformException {
      return RuntimeBridgeSnapshot.initial();
    } on MissingPluginException {
      return RuntimeBridgeSnapshot.initial();
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
    }
  }

  @override
  Future<void> dispose() async {
    _disposed = true;
    await _eventSubscription?.cancel();
    _eventSubscription = null;
    await _snapshotController.close();
  }
}

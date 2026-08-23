import 'dart:async';

import 'package:flutter/services.dart';

import '../backend/backend_topology.dart';
import 'embedded_runtime_controller.dart';

const String _methodStartWithProfile = 'runtime.startWithProfile';
const String _methodStop = 'runtime.stop';
const String _methodSnapshot = 'runtime.snapshot';

class AndroidEmbeddedRuntimeController implements EmbeddedRuntimeController {
  static const MethodChannel _channel = MethodChannel(
    'com.amitia.runtime/bridge',
  );

  BackendEndpoint? _lastEndpoint;
  int _startGeneration = 0;

  @override
  Future<EmbeddedRuntimeStatus> ensureRunning(
    EmbeddedRuntimeProfile profile,
  ) async {
    final capturedGeneration = ++_startGeneration;

    try {
      final currentStatus = await getStatus();
      if (capturedGeneration != _startGeneration) {
        return EmbeddedRuntimeStatus.stopped;
      }

      // Never issue a second start while Native is already transitioning.
      if (currentStatus == EmbeddedRuntimeStatus.ready ||
          currentStatus == EmbeddedRuntimeStatus.starting ||
          currentStatus == EmbeddedRuntimeStatus.installing ||
          currentStatus == EmbeddedRuntimeStatus.stopping ||
          currentStatus == EmbeddedRuntimeStatus.notInstalled ||
          currentStatus == EmbeddedRuntimeStatus.unsupported) {
        return currentStatus;
      }

      final result = await _channel.invokeMethod<Map<Object?, Object?>>(
        _methodStartWithProfile,
        {'profile': profile.runtimeProfileArg},
      );

      if (capturedGeneration != _startGeneration) {
        return EmbeddedRuntimeStatus.stopped;
      }
      if (result == null) {
        return EmbeddedRuntimeStatus.failed;
      }

      final map = _stringKeyMap(result);
      final accepted = map['accepted'] as bool? ?? false;
      final snapshot = _stringKeyMap(map['snapshot']);
      final status = _mapBridgeState(snapshot['state'] as String?);

      if (status == EmbeddedRuntimeStatus.ready) {
        _lastEndpoint = await getEndpoint();
      }

      // A start command may legally report STARTING before the native startup
      // detector reaches READY. Rejection is only fatal when Native also
      // reports a terminal/non-startable state.
      if (!accepted &&
          status != EmbeddedRuntimeStatus.ready &&
          status != EmbeddedRuntimeStatus.starting) {
        return EmbeddedRuntimeStatus.failed;
      }

      return status;
    } on MissingPluginException {
      return EmbeddedRuntimeStatus.unsupported;
    } on PlatformException {
      return EmbeddedRuntimeStatus.failed;
    } catch (_) {
      return EmbeddedRuntimeStatus.failed;
    }
  }

  @override
  Future<void> stop() async {
    _startGeneration++;
    try {
      await _channel.invokeMethod<void>(_methodStop, null);
    } on PlatformException {
      // Native already stopped/failed. The lifecycle will observe the snapshot.
    } on MissingPluginException {
      // Unsupported platform.
    }
  }

  @override
  Future<EmbeddedRuntimeStatus> getStatus() async {
    try {
      final result = await _channel.invokeMethod<Map<Object?, Object?>>(
        _methodSnapshot,
        null,
      );
      if (result == null) return EmbeddedRuntimeStatus.failed;
      final map = _stringKeyMap(result);
      return _mapBridgeState(map['state'] as String?);
    } on MissingPluginException {
      return EmbeddedRuntimeStatus.unsupported;
    } on PlatformException {
      return EmbeddedRuntimeStatus.failed;
    } catch (_) {
      return EmbeddedRuntimeStatus.failed;
    }
  }

  @override
  Future<BackendEndpoint> getEndpoint() async {
    if (_lastEndpoint != null) return _lastEndpoint!;
    final endpoint = BackendEndpoint(
      role: BackendEndpointRole.localRuntime,
      httpBaseUri: Uri(scheme: 'http', host: '127.0.0.1', port: 18899),
      websocketBaseUri: Uri(scheme: 'ws', host: '127.0.0.1', port: 18899),
      isRemote: false,
    );
    _lastEndpoint = endpoint;
    return endpoint;
  }

  EmbeddedRuntimeStatus _mapBridgeState(String? bridgeState) {
    switch (bridgeState) {
      case 'READY':
      case 'DEGRADED':
        return EmbeddedRuntimeStatus.ready;
      case 'INSTALLING':
      case 'VERIFYING':
      case 'REPAIRING':
        return EmbeddedRuntimeStatus.installing;
      case 'STARTING':
        return EmbeddedRuntimeStatus.starting;
      case 'STOPPING':
        return EmbeddedRuntimeStatus.stopping;
      case 'STOPPED':
      case 'INSTALLED':
        return EmbeddedRuntimeStatus.stopped;
      case 'NOT_INSTALLED':
        return EmbeddedRuntimeStatus.notInstalled;
      case 'FAILED':
      case 'CORRUPTED':
        return EmbeddedRuntimeStatus.failed;
      default:
        return EmbeddedRuntimeStatus.failed;
    }
  }

  Map<String, dynamic> _stringKeyMap(Object? value) {
    if (value is! Map) return <String, dynamic>{};
    final map = <String, dynamic>{};
    for (final entry in value.entries) {
      map[entry.key.toString()] = entry.value;
    }
    return map;
  }
}

import 'dart:async';
import 'package:flutter/services.dart';
import '../backend/backend_topology.dart';
import 'embedded_runtime_controller.dart';

const String _methodStartWithProfile = 'runtime.startWithProfile';
const String _methodStop = 'runtime.stop';
const String _methodSnapshot = 'runtime.snapshot';
const int _defaultProbeTimeoutMs = 5000;

class AndroidEmbeddedRuntimeController implements EmbeddedRuntimeController {
  static const MethodChannel _channel =
      MethodChannel('com.amitia.runtime/bridge');

  int _currentProfileGeneration = 0;
  EmbeddedRuntimeProfile? _pendingProfile;
  BackendEndpoint? _lastEndpoint;
  int _startGeneration = 0;

  @override
  Future<EmbeddedRuntimeStatus> ensureRunning(
    EmbeddedRuntimeProfile profile,
  ) async {
    final capturedGeneration = ++_startGeneration;
    _pendingProfile = profile;

    try {
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

      final map = _convertMap(result);
      final accepted = map['accepted'] as bool? ?? false;
      final snapshot = map['snapshot'] as Map<String, dynamic>? ?? {};

      final bridgeState = snapshot['state'] as String?;
      final status = _mapBridgeState(bridgeState);

      if (status == EmbeddedRuntimeStatus.ready) {
        _currentProfileGeneration = capturedGeneration;
        _lastEndpoint = await getEndpoint();
      }

      if (!accepted &&
          status != EmbeddedRuntimeStatus.ready &&
          status != EmbeddedRuntimeStatus.starting) {
        return EmbeddedRuntimeStatus.failed;
      }

      return status;
    } on PlatformException {
      return EmbeddedRuntimeStatus.failed;
    } on MissingPluginException {
      return EmbeddedRuntimeStatus.unsupported;
    }
  }

  @override
  Future<void> stop() async {
    _startGeneration++;
    try {
      await _channel.invokeMethod<void>(_methodStop, null);
    } on PlatformException {
    } on MissingPluginException {
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
      final map = _convertMap(result);
      final bridgeState = map['state'] as String?;
      return _mapBridgeState(bridgeState);
    } on PlatformException {
      return EmbeddedRuntimeStatus.failed;
    } on MissingPluginException {
      return EmbeddedRuntimeStatus.unsupported;
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
        return EmbeddedRuntimeStatus.ready;
      case 'STARTING':
        return EmbeddedRuntimeStatus.starting;
      case 'STOPPING':
        return EmbeddedRuntimeStatus.stopping;
      case 'STOPPED':
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

  Map<String, dynamic> _convertMap(Map<Object?, Object?> result) {
    final map = <String, dynamic>{};
    for (final entry in result.entries) {
      map[entry.key.toString()] = entry.value;
    }
    return map;
  }
}


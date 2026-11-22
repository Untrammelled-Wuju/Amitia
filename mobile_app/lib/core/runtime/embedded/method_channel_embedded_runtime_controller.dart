import 'dart:async';

import 'package:flutter/services.dart';

import '../backend/backend_topology.dart';
import 'embedded_runtime_controller.dart';

const String _methodStartWithProfile = 'runtime.startWithProfile';
const String _methodStop = 'runtime.stop';
const String _methodSnapshot = 'runtime.snapshot';
const Duration _profileSwitchTimeout = Duration(seconds: 20);
const Duration _profileSwitchPollInterval = Duration(milliseconds: 100);

class MethodChannelEmbeddedRuntimeController
    implements EmbeddedRuntimeController {
  static const MethodChannel _channel = MethodChannel(
    'com.amitia.runtime/bridge',
  );

  BackendEndpoint? _lastEndpoint;
  int? _lastEndpointGeneration;
  String? _lastEndpointProfile;
  int _startGeneration = 0;

  @override
  Future<EmbeddedRuntimeStatus> ensureRunning(
    EmbeddedRuntimeProfile profile,
  ) async {
    final capturedGeneration = ++_startGeneration;
    final requestedProfile = profile.runtimeProfileArg;

    try {
      var snapshot = await _readSnapshot();
      if (capturedGeneration != _startGeneration) {
        return EmbeddedRuntimeStatus.stopped;
      }

      var currentStatus = _mapSnapshotStatus(snapshot);
      final currentProfile = _activeProfile(snapshot);

      if (currentStatus == EmbeddedRuntimeStatus.ready &&
          currentProfile == requestedProfile) {
        await _rememberEndpoint(snapshot);
        return EmbeddedRuntimeStatus.ready;
      }

      // Native generations are profile-bound. Never reuse a READY/STARTING
      // generation for a different runtime profile: stop it completely first.
      if ((currentStatus == EmbeddedRuntimeStatus.ready ||
              currentStatus == EmbeddedRuntimeStatus.starting) &&
          currentProfile != null &&
          currentProfile != requestedProfile) {
        await _requestStop();
        snapshot = await _waitForStopped(capturedGeneration);
        currentStatus = _mapSnapshotStatus(snapshot);
      } else if (currentStatus == EmbeddedRuntimeStatus.stopping) {
        snapshot = await _waitForStopped(capturedGeneration);
        currentStatus = _mapSnapshotStatus(snapshot);
      }

      if (capturedGeneration != _startGeneration) {
        return EmbeddedRuntimeStatus.stopped;
      }

      if (currentStatus == EmbeddedRuntimeStatus.stopping) {
        // The previous profile did not stop within the bounded switch window.
        // Never issue a new start into a generation that still owns teardown.
        return EmbeddedRuntimeStatus.stopping;
      }

      if (currentStatus == EmbeddedRuntimeStatus.ready) {
        // READY without a profile is invalid for a profile-specific request.
        if (_activeProfile(snapshot) == requestedProfile) {
          await _rememberEndpoint(snapshot);
          return EmbeddedRuntimeStatus.ready;
        }
        return EmbeddedRuntimeStatus.failed;
      }

      if (currentStatus == EmbeddedRuntimeStatus.starting ||
          currentStatus == EmbeddedRuntimeStatus.installing ||
          currentStatus == EmbeddedRuntimeStatus.notInstalled ||
          currentStatus == EmbeddedRuntimeStatus.unsupported) {
        return currentStatus;
      }

      final result = await _channel.invokeMethod<Map<Object?, Object?>>(
        _methodStartWithProfile,
        {'profile': requestedProfile},
      );

      if (capturedGeneration != _startGeneration) {
        return EmbeddedRuntimeStatus.stopped;
      }
      if (result == null) {
        return EmbeddedRuntimeStatus.failed;
      }

      final map = _stringKeyMap(result);
      final accepted = map['accepted'] as bool? ?? false;
      final resultSnapshot = _stringKeyMap(map['snapshot']);
      final status = _mapSnapshotStatus(resultSnapshot);

      if (status == EmbeddedRuntimeStatus.ready) {
        if (_activeProfile(resultSnapshot) != requestedProfile) {
          return EmbeddedRuntimeStatus.failed;
        }
        await _rememberEndpoint(resultSnapshot);
      }

      // STARTING is an asynchronous accepted lifecycle even when a duplicate
      // command is rejected by Native as already running.
      if (!accepted &&
          status != EmbeddedRuntimeStatus.ready &&
          status != EmbeddedRuntimeStatus.starting) {
        return status == EmbeddedRuntimeStatus.unsupported
            ? EmbeddedRuntimeStatus.unsupported
            : EmbeddedRuntimeStatus.failed;
      }

      return status;
    } on MissingPluginException {
      return EmbeddedRuntimeStatus.unsupported;
    } on PlatformException catch (error) {
      if (error.code == 'UNSUPPORTED_ABI' ||
          error.code == 'UNSUPPORTED_PLATFORM') {
        return EmbeddedRuntimeStatus.unsupported;
      }
      if (error.code == 'OPERATION_ALREADY_RUNNING') {
        final snapshot = await _readSnapshot();
        final status = _mapSnapshotStatus(snapshot);
        if (status == EmbeddedRuntimeStatus.ready &&
            _activeProfile(snapshot) == requestedProfile) {
          await _rememberEndpoint(snapshot);
          return EmbeddedRuntimeStatus.ready;
        }
        return EmbeddedRuntimeStatus.starting;
      }
      return EmbeddedRuntimeStatus.failed;
    } catch (error, stackTrace) {
      print("[original-error] $error\n$stackTrace");
      return EmbeddedRuntimeStatus.failed;
    }
  }

  @override
  Future<void> stop() async {
    _startGeneration++;
    _clearEndpointCache();
    try {
      await _requestStop();
    } on PlatformException {
      // Native already stopped/failed. The lifecycle will observe the snapshot.
    } on MissingPluginException {
      // Unsupported platform.
    }
  }

  Future<void> _requestStop() async {
    await _channel.invokeMethod<Object?>(_methodStop, null);
  }

  Future<Map<String, dynamic>> _waitForStopped(int capturedGeneration) async {
    final elapsed = Stopwatch()..start();
    var last = await _readSnapshot();
    while (capturedGeneration == _startGeneration &&
        elapsed.elapsed < _profileSwitchTimeout) {
      final status = _mapSnapshotStatus(last);
      if (status == EmbeddedRuntimeStatus.stopped ||
          status == EmbeddedRuntimeStatus.failed ||
          status == EmbeddedRuntimeStatus.notInstalled ||
          status == EmbeddedRuntimeStatus.unsupported) {
        return last;
      }
      await Future<void>.delayed(_profileSwitchPollInterval);
      last = await _readSnapshot();
    }
    return last;
  }

  @override
  Future<EmbeddedRuntimeStatus> getStatus() async {
    try {
      return _mapSnapshotStatus(await _readSnapshot());
    } on MissingPluginException {
      return EmbeddedRuntimeStatus.unsupported;
    } on PlatformException catch (error) {
      if (error.code == 'UNSUPPORTED_ABI' ||
          error.code == 'UNSUPPORTED_PLATFORM') {
        return EmbeddedRuntimeStatus.unsupported;
      }
      return EmbeddedRuntimeStatus.failed;
    } catch (error, stackTrace) {
      print("[original-error] $error\n$stackTrace");
      return EmbeddedRuntimeStatus.failed;
    }
  }

  Future<Map<String, dynamic>> _readSnapshot() async {
    final result = await _channel.invokeMethod<Map<Object?, Object?>>(
      _methodSnapshot,
      null,
    );
    if (result == null) return <String, dynamic>{};
    return _stringKeyMap(result);
  }

  @override
  Future<BackendEndpoint> getEndpoint() async {
    Map<String, dynamic> snapshot;
    try {
      snapshot = await _readSnapshot();
    } catch (error, stackTrace) {
      print("[original-error] $error\n$stackTrace");
      throw StateError('embedded runtime endpoint unavailable: $error');
    }
    final generation = _snapshotGeneration(snapshot);
    final profile = _activeProfile(snapshot);
    final cached = _lastEndpoint;
    if (cached != null &&
        _mapSnapshotStatus(snapshot) == EmbeddedRuntimeStatus.ready &&
        _lastEndpointGeneration == generation &&
        _lastEndpointProfile == profile) {
      return cached;
    }
    return _fetchEndpoint(generation: generation, profile: profile);
  }

  Future<void> _rememberEndpoint(Map<String, dynamic> snapshot) async {
    try {
      await _fetchEndpoint(
        generation: _snapshotGeneration(snapshot),
        profile: _activeProfile(snapshot),
      );
    } catch (error, stackTrace) {
      print("[original-error] $error\n$stackTrace");
      _clearEndpointCache();
    }
  }

  Future<BackendEndpoint> _fetchEndpoint({
    required int? generation,
    required String? profile,
  }) async {
    try {
      final result = await _channel.invokeMethod<Map<Object?, Object?>>(
        'runtime.getBackendConnection',
        <String, Object?>{'expectedRuntimeGeneration': generation},
      );
      final payload = result == null
          ? <String, dynamic>{}
          : _stringKeyMap(result);
      final endpoint = payload['endpoint'];
      if (payload['status'] == 'available' && endpoint is Map) {
        final map = _stringKeyMap(endpoint);
        final host = map['host'] as String? ?? '127.0.0.1';
        final port = map['port'] as int? ?? 0;
        if (host == '127.0.0.1' && port > 0 && port <= 65535) {
          final httpBaseUri = Uri(scheme: 'http', host: host, port: port);
          final endpointValue = BackendEndpoint(
            role: BackendEndpointRole.localRuntime,
            httpBaseUri: httpBaseUri,
            websocketBaseUri: httpBaseUri.replace(scheme: 'ws'),
            isRemote: false,
          );
          _lastEndpoint = endpointValue;
          _lastEndpointGeneration = generation;
          _lastEndpointProfile = profile;
          return endpointValue;
        }
      }
      final nativeError = _stringKeyMap(payload['error']);
      final nativeMessage = (nativeError['message'] as String? ?? '').trim();
      throw StateError(
        nativeMessage.isEmpty
            ? 'embedded runtime endpoint is unavailable'
            : 'embedded runtime endpoint is unavailable: $nativeMessage',
      );
    } on StateError {
      rethrow;
    } catch (error, stackTrace) {
      print("[original-error] $error\n$stackTrace");
      throw StateError('embedded runtime endpoint unavailable: $error');
    }
  }

  void _clearEndpointCache() {
    _lastEndpoint = null;
    _lastEndpointGeneration = null;
    _lastEndpointProfile = null;
  }

  int? _snapshotGeneration(Map<String, dynamic> snapshot) {
    final value = snapshot['generation'];
    if (value is int) return value;
    if (value is num) return value.toInt();
    if (value is String) return int.tryParse(value.trim());
    return null;
  }

  String? _activeProfile(Map<String, dynamic> snapshot) {
    final value = snapshot['activeProfile'];
    if (value is! String) return null;
    final normalized = value.trim();
    return normalized.isEmpty ? null : normalized;
  }

  EmbeddedRuntimeStatus _mapSnapshotStatus(Map<String, dynamic> snapshot) {
    final bridgeState = snapshot['state'] as String?;
    if (bridgeState == 'FAILED' || bridgeState == 'CORRUPTED') {
      final error = _stringKeyMap(snapshot['lastError']);
      final code = error['code'] as String?;
      if (code == 'UNSUPPORTED_ABI' || code == 'UNSUPPORTED_PLATFORM') {
        return EmbeddedRuntimeStatus.unsupported;
      }
    }
    return _mapBridgeState(bridgeState);
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

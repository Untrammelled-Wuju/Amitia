import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:math';

import 'package:crypto/crypto.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../../../core/backend_connection/backend_connection_availability.dart';
import '../../../core/backend_connection/backend_connection_config.dart';
import '../../../core/backend_connection/backend_uri_builder.dart';
import '../../../core/backend_transport/backend_service_api.dart';
import '../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../core/native_bridge/native_bridge_platform_dispatcher.dart';
import '../../../core/native_bridge/providers/native_bridge_relay_provider.dart';
import '../../../core/ui_runtime/ui_client_info.dart';
import '../../../core/ui_runtime/ui_runtime_controller.dart';

const _runtimeVersion = '2.0.0';
const _runtimeContractVersion = '2.0.0';
const _runtimeProtocol = 'amitia.desktop-pet.runtime';
const _runtimeWsSubprotocol = 'amitia.runtime.v2';
const _runtimeBootstrapPrefix = 'amitia.runtime.bootstrap.';
const _runtimeWsPath = '/internal/desktop-pet/runtime/ws';
const _prefsAlpha = 'desktopPet.mobile.alpha.v1';
const _defaultAlpha = 0.85;

const _mandatoryCapabilities = <String>[
  'runtime.sync_desired_v2',
  'runtime.play_action_v2',
  'runtime.renderer_ack_v2',
  'runtime.expiry_rfc3339_v1',
  'platform:android',
];

class DesktopPetMobileRuntimeState {
  final bool supported;
  final bool connected;
  final bool permissionGranted;
  final bool rendererLoaded;
  final bool visible;
  final bool paused;
  final String deviceId;
  final String runtimeId;
  final String installationId;
  final String petName;
  final String currentActionKey;
  final String playbackId;
  final double alpha;
  final String phase;
  final String error;
  final DateTime? updatedAt;

  const DesktopPetMobileRuntimeState({
    this.supported = false,
    this.connected = false,
    this.permissionGranted = false,
    this.rendererLoaded = false,
    this.visible = false,
    this.paused = false,
    this.deviceId = '',
    this.runtimeId = '',
    this.installationId = '',
    this.petName = '',
    this.currentActionKey = '',
    this.playbackId = '',
    this.alpha = _defaultAlpha,
    this.phase = 'idle',
    this.error = '',
    this.updatedAt,
  });

  DesktopPetMobileRuntimeState copyWith({
    bool? supported,
    bool? connected,
    bool? permissionGranted,
    bool? rendererLoaded,
    bool? visible,
    bool? paused,
    String? deviceId,
    String? runtimeId,
    String? installationId,
    String? petName,
    String? currentActionKey,
    String? playbackId,
    double? alpha,
    String? phase,
    String? error,
    DateTime? updatedAt,
  }) {
    return DesktopPetMobileRuntimeState(
      supported: supported ?? this.supported,
      connected: connected ?? this.connected,
      permissionGranted: permissionGranted ?? this.permissionGranted,
      rendererLoaded: rendererLoaded ?? this.rendererLoaded,
      visible: visible ?? this.visible,
      paused: paused ?? this.paused,
      deviceId: deviceId ?? this.deviceId,
      runtimeId: runtimeId ?? this.runtimeId,
      installationId: installationId ?? this.installationId,
      petName: petName ?? this.petName,
      currentActionKey: currentActionKey ?? this.currentActionKey,
      playbackId: playbackId ?? this.playbackId,
      alpha: alpha ?? this.alpha,
      phase: phase ?? this.phase,
      error: error ?? this.error,
      updatedAt: updatedAt ?? this.updatedAt,
    );
  }
}

class _RuntimeCursor {
  int lastAppliedDesiredRevision;
  int lastProcessedCommandSequence;
  int lastEventSequence;
  String actualStateHash;
  String appliedDesiredHash;
  int appliedSettingsRevision;

  _RuntimeCursor({
    this.lastAppliedDesiredRevision = 0,
    this.lastProcessedCommandSequence = 0,
    this.lastEventSequence = 0,
    this.actualStateHash = '',
    this.appliedDesiredHash = '',
    this.appliedSettingsRevision = 0,
  });

}

class _PendingPosition {
  final String installationId;
  final int x;
  final int y;

  const _PendingPosition(this.installationId, this.x, this.y);
}

class _TrackedPlayback {
  final String commandId;
  final int commandSequence;
  final String playbackId;
  final String actionKey;
  final String installationId;
  final String characterId;
  final String decisionId;
  final String completionPolicy;
  final int interruptAfterMs;
  final int minimumPlayMs;
  final int? maximumPlayMs;
  final bool interruptible;
  bool firstCycleSent = false;
  Timer? pollTimer;

  _TrackedPlayback({
    required this.commandId,
    required this.commandSequence,
    required this.playbackId,
    required this.actionKey,
    required this.installationId,
    required this.characterId,
    required this.decisionId,
    required this.completionPolicy,
    required this.interruptAfterMs,
    required this.minimumPlayMs,
    required this.maximumPlayMs,
    required this.interruptible,
  });

  void cancel() {
    pollTimer?.cancel();
    pollTimer = null;
  }
}

final desktopPetMobileRuntimeProvider = StateNotifierProvider<
    DesktopPetMobileRuntimeNotifier, DesktopPetMobileRuntimeState>((ref) {
  final notifier = DesktopPetMobileRuntimeNotifier(ref);
  ref.onDispose(notifier.dispose);
  return notifier;
});

/// Keeps the Android Runtime V2 renderer attached to the embedded Device Agent
/// even when the desktop-pet page is not open.
final desktopPetMobileRuntimeBootstrapProvider = Provider<int?>((ref) {
  if (kIsWeb || !Platform.isAndroid) {
    ref.read(desktopPetMobileRuntimeProvider.notifier).attach(null);
    return null;
  }
  final localConnection = ref.watch(deviceLocalBackendConnectionProvider).valueOrNull;
  final notifier = ref.read(desktopPetMobileRuntimeProvider.notifier);
  if (localConnection is! BackendConnectionAvailable) {
    notifier.attach(null);
    return null;
  }
  notifier.attach(localConnection.config);
  return localConnection.config.generation;
});

class DesktopPetMobileRuntimeNotifier
    extends StateNotifier<DesktopPetMobileRuntimeState> {
  DesktopPetMobileRuntimeNotifier(this.ref)
      : super(DesktopPetMobileRuntimeState(
          supported: !kIsWeb && Platform.isAndroid,
          phase: !kIsWeb && Platform.isAndroid ? 'waiting_runtime' : 'unsupported',
        ));

  final Ref ref;
  final Random _random = Random.secure();
  final BackendUriBuilder _uriBuilder = BackendUriBuilder();

  BackendConnectionConfig? _config;
  WebSocket? _socket;
  StreamSubscription<dynamic>? _socketSubscription;
  Timer? _reconnectTimer;
  Timer? _heartbeatTimer;
  Timer? _watchdogTimer;
  Timer? _snapshotTimer;
  Timer? _interactionTimer;
  bool _disposed = false;
  bool _connecting = false;
  int _attachEpoch = 0;
  Future<void> _inboundSerial = Future<void>.value();

  String _userId = '';
  String _deviceId = '';
  String _runtimeId = '';
  String _sessionId = '';
  int _connectionGeneration = 0;
  int _outboundSequence = 0;
  int _lastServerSequence = 0;
  int _heartbeatIntervalMs = 15000;
  int _heartbeatTimeoutMs = 30000;
  int _maxMessageBytes = 1048576;
  DateTime _lastServerMessageAt = DateTime.fromMillisecondsSinceEpoch(0);
  _RuntimeCursor _cursor = _RuntimeCursor();
  final Map<String, Map<String, dynamic>> _durableReplay =
      <String, Map<String, dynamic>>{};
  _TrackedPlayback? _playback;
  bool _drainingInteractions = false;
  bool _persistingPosition = false;
  _PendingPosition? _pendingPosition;
  Future<void> _localPlaybackQuiesce = Future<void>.value();
  Map<String, dynamic> _lastConfirmedNativeStatus = <String, dynamic>{};
  bool _nativeStatusAvailable = false;
  String _nativeStatusLastError = '';

  bool get nativeStatusAvailable => _nativeStatusAvailable;
  Map<String, dynamic> get lastConfirmedNativeStatus => Map<String, dynamic>.from(_lastConfirmedNativeStatus);
  String get nativeStatusLastError => _nativeStatusLastError;

  void attach(BackendConnectionConfig? config) {
    if (_disposed) return;
    if (config == null) {
      if (_config != null || _socket != null) {
        _attachEpoch++;
        _config = null;
        _disconnect('device_runtime_unavailable');
      }
      state = state.copyWith(
        connected: false,
        phase: 'waiting_runtime',
        error: '',
        updatedAt: DateTime.now(),
      );
      return;
    }
    if (_config?.generation == config.generation &&
        _config?.endpoint.host == config.endpoint.host &&
        _config?.endpoint.port == config.endpoint.port &&
        (_socket != null || _connecting)) {
      return;
    }
    _attachEpoch++;
    final epoch = _attachEpoch;
    _config = config;
    _disconnect('reattach');
    unawaited(_connect(epoch));
  }

  Future<void> refreshStatus() async {
    if (!Platform.isAndroid) return;
    Map<String, dynamic> result = <String, dynamic>{};
    try {
      result = await _native('desktop.pet.renderer.status');
      _lastConfirmedNativeStatus = Map<String, dynamic>.from(result);
      _nativeStatusAvailable = true;
      _nativeStatusLastError = '';
    } catch (e) {
      _nativeStatusAvailable = false;
      _nativeStatusLastError = e.toString();
      state = state.copyWith(
        error: 'native_status_unavailable: ${e.toString()}',
        updatedAt: DateTime.now(),
      );
      return;
    }
    final overlay = await _native('system.overlay.status');
    final permission = overlay['permissionGranted'] == true ||
        result['permissionGranted'] == true;
    final loaded = result['loaded'] == true;
    await _loadActiveInstallationName();
    state = state.copyWith(
      permissionGranted: permission,
      rendererLoaded: loaded,
      visible: loaded ? (result['visible'] == true) : false,
      paused: loaded ? (result['paused'] == true) : false,
      installationId: loaded
          ? (result['installationId']?.toString() ?? '')
          : '',
      currentActionKey: loaded
          ? (result['currentActionKey']?.toString() ?? '')
          : '',
      playbackId: loaded
          ? (result['playbackId']?.toString() ?? '')
          : '',
      alpha: _double(result['alpha'], state.alpha).clamp(0.2, 1.0).toDouble(),
      updatedAt: DateTime.now(),
      error: _nativeStatusLastError.isEmpty ? '' : state.error,
    );
  }

  Future<void> requestOverlayPermission() async {
    if (!Platform.isAndroid) return;
    try {
      await _native('system.overlay.permission.request');
      await refreshStatus();
    } catch (error) {
      state = state.copyWith(error: error.toString(), updatedAt: DateTime.now());
      rethrow;
    }
  }

  Future<void> setAlpha(double value) async {
    final normalized = value.clamp(0.2, 1.0).toDouble();
    final prefs = await SharedPreferences.getInstance();
    await prefs.setDouble(_prefsAlpha, normalized);
    if (state.rendererLoaded) {
      await _native('desktop.pet.renderer.settings', <String, dynamic>{
        'alpha': normalized,
      });
    }
    state = state.copyWith(alpha: normalized, updatedAt: DateTime.now());
    await _sendStateSnapshot();
  }

  Future<void> setVisible(bool visible) async {
    if (!Platform.isAndroid) return;
    if (visible && !state.permissionGranted) {
      await requestOverlayPermission();
      await refreshStatus();
      if (!state.permissionGranted) {
        throw const _RuntimeCommandFailure(
          'OVERLAY_PERMISSION_REQUIRED',
          'Android overlay permission is required before showing the desktop pet',
        );
      }
    }
    if (!state.rendererLoaded) {
      throw const _RuntimeCommandFailure(
        'PET_NOT_READY',
        'desktop pet renderer is not loaded',
      );
    }
    await _native(visible ? 'desktop.pet.renderer.show' : 'desktop.pet.renderer.hide');
    await refreshStatus();
    await _sendStateSnapshot();
  }

  Future<void> _connect(int epoch) async {
    final config = _config;
    if (_disposed || config == null || _connecting || epoch != _attachEpoch) {
      return;
    }
    _connecting = true;
    state = state.copyWith(phase: 'connecting', error: '', updatedAt: DateTime.now());
    try {
      await _localPlaybackQuiesce;
      if (_disposed || epoch != _attachEpoch || _config?.generation != config.generation) {
        return;
      }
      _deviceId = await ref.read(uiRuntimeProvider.notifier).deviceId;
      final prefs = await SharedPreferences.getInstance();
      // Runtime identity and cursor are incarnation-scoped. Android destroys the
      // WindowManager surface with the app process, so restoring an old runtime
      // identity/cursor would incorrectly claim that a desired revision is still
      // applied after process restart and suppress the backend reconcile.
      if (_runtimeId.isEmpty) {
        _runtimeId = 'rt_mobile_${_randomToken(24)}';
        _cursor = _RuntimeCursor();
        _durableReplay.clear();
        _outboundSequence = 0;
      }
      final alpha = (prefs.getDouble(_prefsAlpha) ?? _defaultAlpha).clamp(0.2, 1.0);
      state = state.copyWith(
        deviceId: _deviceId,
        runtimeId: _runtimeId,
        alpha: alpha.toDouble(),
      );

      final api = _localApiRequired();
      await api.post<dynamic>(
        '/api/local/devices/register',
        data: <String, dynamic>{
          'deviceId': _deviceId,
          'platform': 'android',
          'appVersion': currentUIClientInfo().appVersion,
        },
      );
      final ticket = await api.post<Map<String, dynamic>>(
        '/api/local/devices/${Uri.encodeComponent(_deviceId)}/runtime-bootstrap-tickets',
        data: <String, dynamic>{'runtimeId': _runtimeId},
        fromJson: (dynamic value) => Map<String, dynamic>.from(value as Map),
      );
      if (ticket == null) throw StateError('runtime bootstrap ticket is empty');
      final rawTicket = ticket['ticket']?.toString().trim() ?? '';
      _userId = ticket['userId']?.toString().trim() ?? '';
      if (rawTicket.isEmpty || _userId.isEmpty) {
        throw StateError('runtime bootstrap ticket is invalid');
      }
      if (!RegExp(r'^[A-Za-z0-9._~-]+$').hasMatch(rawTicket)) {
        throw StateError('runtime bootstrap ticket contains invalid characters');
      }

      final uri = _uriBuilder.webSocket(
        config,
        _runtimeWsPath,
        queryParameters: <String, dynamic>{
          'deviceId': _deviceId,
          'runtimeId': _runtimeId,
        },
      );
      final socket = await WebSocket.connect(
        uri.toString(),
        protocols: <String>[
          _runtimeWsSubprotocol,
          '$_runtimeBootstrapPrefix$rawTicket',
        ],
      ).timeout(const Duration(seconds: 10));
      if (_disposed || epoch != _attachEpoch || _config?.generation != config.generation) {
        await socket.close(1000, 'stale_attach');
        return;
      }
      _socket = socket;
      _lastServerSequence = 0;
      _lastServerMessageAt = DateTime.now();
      _socketSubscription = socket.listen(
        (dynamic data) {
          _inboundSerial = _inboundSerial.then<void>((_) async {
            await _handleSocketMessage(data, epoch);
          });
        },
        onError: (Object error, StackTrace stack) =>
            unawaited(_onSocketClosed(epoch, error.toString())),
        onDone: () => unawaited(_onSocketClosed(epoch, 'socket_closed')),
        cancelOnError: false,
      );
      await _sendHello();
    } catch (error) {
      if (epoch == _attachEpoch && !_disposed) {
        state = state.copyWith(
          connected: false,
          phase: 'degraded',
          error: error.toString(),
          updatedAt: DateTime.now(),
        );
        _scheduleReconnect(epoch);
      }
    } finally {
      _connecting = false;
    }
  }

  Future<void> _handleSocketMessage(dynamic raw, int epoch) async {
    if (_disposed || epoch != _attachEpoch) return;
    try {
      if (raw is! String) throw const FormatException('runtime envelope must be text');
      if (utf8.encode(raw).length > _maxMessageBytes) {
        throw const FormatException('runtime envelope exceeds maxMessageBytes');
      }
      final decoded = jsonDecode(raw);
      if (decoded is! Map) throw const FormatException('runtime envelope must be an object');
      final envelope = Map<String, dynamic>.from(decoded);
      _validateServerEnvelope(envelope);
      _lastServerSequence = _positiveInt(envelope['sequence']);
      _lastServerMessageAt = DateTime.now();
      final type = envelope['messageType']?.toString() ?? '';
      switch (type) {
        case 'hello_ack':
          await _handleHelloAck(envelope);
          break;
        case 'command':
          await _handleCommand(envelope);
          break;
        case 'pong':
        case 'state_snapshot':
          break;
        case 'error':
          final payload = _map(envelope['payload']);
          throw StateError(
            'runtime server error: ${payload['code'] ?? ''} ${payload['message'] ?? ''}',
          );
        default:
          break;
      }
    } catch (error) {
      state = state.copyWith(error: error.toString(), updatedAt: DateTime.now());
      final socket = _socket;
      if (socket != null) {
        await socket.close(4003, 'protocol_violation');
      }
    }
  }

  void _validateServerEnvelope(Map<String, dynamic> envelope) {
    if (envelope['envelopeVersion'] != 2 || envelope['protocol'] != _runtimeProtocol) {
      throw const FormatException('invalid runtime protocol envelope');
    }
    if (envelope['userId']?.toString() != _userId ||
        envelope['deviceId']?.toString() != _deviceId ||
        envelope['runtimeId']?.toString() != _runtimeId) {
      throw const FormatException('runtime envelope identity mismatch');
    }
    final generation = _positiveInt(envelope['connectionGeneration']);
    final sequence = _positiveInt(envelope['sequence']);
    if (generation <= 0 || sequence <= 0) {
      throw const FormatException('runtime envelope generation/sequence invalid');
    }
    if (sequence <= _lastServerSequence) {
      throw const FormatException('runtime envelope sequence is stale or duplicated');
    }
    final payload = envelope['payload'];
    if (envelope['payloadHash']?.toString() != _payloadHash(payload)) {
      throw const FormatException('runtime envelope payload hash mismatch');
    }
    if (envelope['messageType']?.toString() == 'hello_ack') {
      if (_lastServerSequence != 0) {
        throw const FormatException('runtime hello_ack must be the first server envelope');
      }
      return;
    }
    if (_sessionId.isEmpty || envelope['runtimeSessionId']?.toString() != _sessionId) {
      throw const FormatException('runtime envelope session mismatch');
    }
    if (generation != _connectionGeneration) {
      throw const FormatException('runtime envelope connection generation mismatch');
    }
  }

  Future<void> _handleHelloAck(Map<String, dynamic> envelope) async {
    final payload = _map(envelope['payload']);
    if (payload['accepted'] != true) {
      throw StateError(
        'runtime hello rejected: ${payload['errorCode'] ?? ''} ${payload['errorMessage'] ?? ''}',
      );
    }
    final sessionId = payload['sessionId']?.toString().trim() ?? '';
    if (sessionId.isEmpty || envelope['runtimeSessionId']?.toString() != sessionId) {
      throw const FormatException('runtime hello session mismatch');
    }
    _sessionId = sessionId;
    _connectionGeneration = _positiveInt(envelope['connectionGeneration']);
    _heartbeatIntervalMs = _positiveInt(payload['heartbeatIntervalMs'], 15000);
    _heartbeatTimeoutMs = _positiveInt(payload['heartbeatTimeoutMs'], 30000);
    _maxMessageBytes = _positiveInt(payload['maxMessageBytes'], 1048576);
    state = state.copyWith(
      connected: true,
      phase: 'ready',
      error: '',
      updatedAt: DateTime.now(),
    );
    _startHeartbeat();
    await refreshStatus();
    await _sendStateSnapshot();
  }

  Future<void> _handleCommand(Map<String, dynamic> envelope) async {
    final outer = _map(envelope['payload']);
    final commandId = outer['commandId']?.toString().trim() ?? '';
    final commandType = outer['commandType']?.toString().trim() ?? '';
    final commandSequence = _positiveInt(outer['commandSequence']);
    final desiredRevision = _nonNegativeInt(outer['desiredRevision']);
    final inner = _map(outer['payload']);
    final desiredHash = inner['desiredHash']?.toString().trim() ?? '';
    if (commandId.isEmpty || commandType.isEmpty || commandSequence <= 0) {
      throw const FormatException('runtime command identity is invalid');
    }

    final cached = _durableReplay[commandId];
    if (cached != null && _isDurable(commandType)) {
      if (cached['ok'] == true) {
        // Mirror the canonical Electron Runtime V2 replay sequence. Successful
        // durable replays may repeat received/accepted before desired_applied.
        await _sendCommandAck(commandId, commandSequence, 'runtime_received');
        await _sendCommandAck(commandId, commandSequence, 'runtime_accepted');
        await _sendDesiredApplied(commandId, desiredRevision, desiredHash);
      } else {
        // A rejected desired-state command is already terminal. Sending a new
        // runtime_received/runtime_accepted first would regress the backend
        // command state machine from its terminal state.
        await _sendDesiredRejected(
          commandId,
          desiredRevision,
          desiredHash,
          cached['errorCode']?.toString() ?? 'RUNTIME_REJECTED',
          cached['errorMessage']?.toString() ?? 'runtime rejected desired state',
        );
      }
      return;
    }

    await _sendCommandAck(commandId, commandSequence, 'runtime_received');

    if (_isEphemeral(commandType)) {
      final expiry = outer['expiresAt']?.toString().trim() ?? '';
      final expiresAt = DateTime.tryParse(expiry)?.toUtc();
      if (expiry.isEmpty || expiresAt == null) {
        await _sendCommandAck(
          commandId,
          commandSequence,
          'failed_terminal',
          errorCode: 'COMMAND_EXPIRY_INVALID',
          errorMessage: 'ephemeral runtime command has invalid expiresAt',
        );
        _cursor.lastProcessedCommandSequence =
            max(_cursor.lastProcessedCommandSequence, commandSequence);
        await _persistCursor();
        await _sendStateSnapshot();
        return;
      }
      if (!expiresAt.isAfter(DateTime.now().toUtc())) {
        await _sendCommandAck(
          commandId,
          commandSequence,
          'expired',
          errorCode: 'COMMAND_EXPIRED',
          errorMessage: 'ephemeral runtime command expired before local execution',
        );
        _cursor.lastProcessedCommandSequence =
            max(_cursor.lastProcessedCommandSequence, commandSequence);
        await _persistCursor();
        await _sendStateSnapshot();
        return;
      }
    }

    try {
      switch (commandType) {
        case 'runtime.command.sync_desired_state':
        case 'runtime.command.reload_release':
          if (desiredRevision <= 0 || desiredHash.isEmpty) {
            throw const _RuntimeCommandFailure(
              'DESIRED_STATE_INVALID',
              'desiredRevision and desiredHash are required',
            );
          }
          if (_cursor.lastAppliedDesiredRevision > desiredRevision) {
            throw const _RuntimeCommandFailure(
              'DESIRED_REVISION_STALE',
              'desired revision is older than the applied revision',
            );
          }
          if (_cursor.lastAppliedDesiredRevision == desiredRevision &&
              _cursor.appliedDesiredHash.isNotEmpty &&
              _cursor.appliedDesiredHash != desiredHash) {
            throw const _RuntimeCommandFailure(
              'DESIRED_HASH_MISMATCH',
              'equal desired revision carries a different desired hash',
            );
          }
          await _applyDesired(outer, inner);
          await _sendCommandAck(commandId, commandSequence, 'runtime_accepted');
          _cursor.lastAppliedDesiredRevision =
              max(_cursor.lastAppliedDesiredRevision, desiredRevision);
          _cursor.appliedDesiredHash = desiredHash;
          _cursor.appliedSettingsRevision = max(
            _cursor.appliedSettingsRevision,
            _nonNegativeInt(inner['settingsRevision'] ?? outer['settingsRevision']),
          );
          _cursor.lastProcessedCommandSequence =
              max(_cursor.lastProcessedCommandSequence, commandSequence);
          _durableReplay[commandId] = <String, dynamic>{'ok': true};
          _trimReplay();
          await _persistCursor();
          await _sendDesiredApplied(commandId, desiredRevision, desiredHash);
          await _sendStateSnapshot(currentCommandId: commandId);
          break;
        case 'runtime.command.ensure_absent':
          if (desiredRevision <= 0 || desiredHash.isEmpty) {
            throw const _RuntimeCommandFailure(
              'DESIRED_STATE_INVALID',
              'desiredRevision and desiredHash are required',
            );
          }
          if (_cursor.lastAppliedDesiredRevision > desiredRevision) {
            throw const _RuntimeCommandFailure(
              'DESIRED_REVISION_STALE',
              'desired revision is older than the applied revision',
            );
          }
          if (_cursor.lastAppliedDesiredRevision == desiredRevision &&
              _cursor.appliedDesiredHash.isNotEmpty &&
              _cursor.appliedDesiredHash != desiredHash) {
            throw const _RuntimeCommandFailure(
              'DESIRED_HASH_MISMATCH',
              'equal desired revision carries a different desired hash',
            );
          }
          await _interruptPlayback('user_disable');
          await _native('desktop.pet.renderer.unload');
          state = state.copyWith(
            rendererLoaded: false,
            visible: false,
            installationId: '',
            petName: '',
            currentActionKey: '',
            playbackId: '',
            updatedAt: DateTime.now(),
          );
          await _sendCommandAck(commandId, commandSequence, 'runtime_accepted');
          _cursor.lastAppliedDesiredRevision =
              max(_cursor.lastAppliedDesiredRevision, desiredRevision);
          _cursor.appliedDesiredHash = desiredHash;
          _cursor.appliedSettingsRevision = max(
            _cursor.appliedSettingsRevision,
            _nonNegativeInt(inner['settingsRevision'] ?? outer['settingsRevision']),
          );
          _cursor.lastProcessedCommandSequence =
              max(_cursor.lastProcessedCommandSequence, commandSequence);
          _durableReplay[commandId] = <String, dynamic>{'ok': true};
          _trimReplay();
          await _persistCursor();
          await _sendDesiredApplied(commandId, desiredRevision, desiredHash);
          await _sendStateSnapshot(currentCommandId: commandId);
          break;
        case 'runtime.command.play_action':
          await _playAction(commandId, commandSequence, outer, inner);
          break;
        case 'runtime.command.stop_action':
          await _interruptPlayback('runtime_stop');
          await _sendCommandAck(commandId, commandSequence, 'runtime_accepted');
          await _sendCommandAck(commandId, commandSequence, 'completed');
          _cursor.lastProcessedCommandSequence =
              max(_cursor.lastProcessedCommandSequence, commandSequence);
          await _persistCursor();
          await refreshStatus();
          await _sendStateSnapshot();
          break;
        case 'runtime.command.pause_action':
          await _native('desktop.pet.renderer.pause');
          await _sendCommandAck(commandId, commandSequence, 'runtime_accepted');
          await _sendCommandAck(commandId, commandSequence, 'completed');
          _cursor.lastProcessedCommandSequence =
              max(_cursor.lastProcessedCommandSequence, commandSequence);
          await _persistCursor();
          await refreshStatus();
          await _sendStateSnapshot();
          break;
        case 'runtime.command.resume_action':
          await _native('desktop.pet.renderer.resume');
          await _sendCommandAck(commandId, commandSequence, 'runtime_accepted');
          await _sendCommandAck(commandId, commandSequence, 'completed');
          _cursor.lastProcessedCommandSequence =
              max(_cursor.lastProcessedCommandSequence, commandSequence);
          await _persistCursor();
          await refreshStatus();
          await _sendStateSnapshot();
          break;
        case 'runtime.command.recenter_once':
          final recentered = await _native('desktop.pet.renderer.recenter');
          try {
            await _persistRuntimePosition(
              state.installationId,
              _int(recentered['x'], 0),
              _int(recentered['y'], 0),
              preservePositionMode: true,
            );
          } catch (error) {
            // Recenter is an ephemeral physical command. The window has already
            // moved successfully, so coordinate persistence is best-effort just
            // like the desktop renderer path; desired reconciliation can repair
            // settings later without falsely failing the physical command.
            state = state.copyWith(
              error: '桌宠居中位置保存失败: $error',
              updatedAt: DateTime.now(),
            );
          }
          await _sendCommandAck(commandId, commandSequence, 'runtime_accepted');
          await _sendCommandAck(commandId, commandSequence, 'completed');
          _cursor.lastProcessedCommandSequence =
              max(_cursor.lastProcessedCommandSequence, commandSequence);
          await _persistCursor();
          await refreshStatus();
          await _sendStateSnapshot();
          break;
        default:
          await _sendCommandAck(
            commandId,
            commandSequence,
            'failed_terminal',
            errorCode: 'UNKNOWN_COMMAND_TYPE',
            errorMessage: 'unsupported runtime command: $commandType',
          );
          _cursor.lastProcessedCommandSequence =
              max(_cursor.lastProcessedCommandSequence, commandSequence);
          await _persistCursor();
          await _sendStateSnapshot();
      }
    } on _RuntimeCommandFailure catch (error) {
      if (_isDurable(commandType)) {
        _durableReplay[commandId] = <String, dynamic>{
          'ok': false,
          'errorCode': error.code,
          'errorMessage': error.message,
        };
        _trimReplay();
        _cursor.lastProcessedCommandSequence =
            max(_cursor.lastProcessedCommandSequence, commandSequence);
        await _persistCursor();
        await _sendDesiredRejected(
          commandId,
          desiredRevision,
          desiredHash,
          error.code,
          error.message,
        );
      } else {
        await _sendCommandAck(
          commandId,
          commandSequence,
          'failed_terminal',
          errorCode: error.code,
          errorMessage: error.message,
        );
        _cursor.lastProcessedCommandSequence =
            max(_cursor.lastProcessedCommandSequence, commandSequence);
        await _persistCursor();
      }
      state = state.copyWith(error: error.message, updatedAt: DateTime.now());
      await _sendStateSnapshot();
    } catch (error) {
      final message = error.toString();
      if (_isDurable(commandType)) {
        _durableReplay[commandId] = <String, dynamic>{
          'ok': false,
          'errorCode': 'COMMAND_EXECUTION_FAILED',
          'errorMessage': message,
        };
        _trimReplay();
        _cursor.lastProcessedCommandSequence =
            max(_cursor.lastProcessedCommandSequence, commandSequence);
        await _persistCursor();
        await _sendDesiredRejected(
          commandId,
          desiredRevision,
          desiredHash,
          'COMMAND_EXECUTION_FAILED',
          message,
        );
      } else {
        await _sendCommandAck(
          commandId,
          commandSequence,
          'failed_terminal',
          errorCode: 'COMMAND_EXECUTION_FAILED',
          errorMessage: message,
        );
        _cursor.lastProcessedCommandSequence =
            max(_cursor.lastProcessedCommandSequence, commandSequence);
        await _persistCursor();
      }
      state = state.copyWith(error: message, updatedAt: DateTime.now());
      await _sendStateSnapshot();
    }
  }

  Future<void> _applyDesired(
    Map<String, dynamic> outer,
    Map<String, dynamic> inner,
  ) async {
    if (inner['ensureAbsent'] == true) {
      await _interruptPlayback('user_disable');
      await _native('desktop.pet.renderer.unload');
      state = state.copyWith(
        rendererLoaded: false,
        visible: false,
        installationId: '',
        petName: '',
        currentActionKey: '',
        playbackId: '',
        updatedAt: DateTime.now(),
      );
      return;
    }

    final installationId =
        (inner['installationId'] ?? outer['installationId'])?.toString().trim() ?? '';
    if (installationId.isEmpty) {
      throw const _RuntimeCommandFailure(
        'MISSING_INSTALLATION_ID',
        'desired state does not identify an installation',
      );
    }
    final expectedContract = inner['runtimeContractVersion']?.toString().trim() ?? '';
    if (expectedContract.isNotEmpty && expectedContract != _runtimeContractVersion) {
      throw _RuntimeCommandFailure(
        'RUNTIME_CONTRACT_MISMATCH',
        'desired state requires runtime contract $expectedContract',
      );
    }

    final detail = await _loadInstallationDetail(installationId);
    // CoordinatorHandler.GetInstallation embeds Installation anonymously, so
    // installation fields are flattened at the response root alongside the
    // nested settings/manifest objects. Keep compatibility with a future
    // explicitly nested shape, but use the authoritative flattened response
    // today instead of treating it as an empty installation.
    final nestedInstallation = _map(detail['installation'] ?? detail['Installation']);
    final installation = nestedInstallation.isNotEmpty ? nestedInstallation : detail;
    final manifest = _map(detail['manifest'] ?? detail['Manifest']);
    final manifestIntegrity = _map(manifest['integrity']);
    final persistedSettings = _map(detail['settings'] ?? detail['Settings']);
    final desiredSettings = _map(inner['settingsSnapshot']);
    final settings = desiredSettings.isNotEmpty ? desiredSettings : persistedSettings;

    void requireIdentity(String field, String actual) {
      final expected = inner[field]?.toString().trim() ?? '';
      if (expected.isNotEmpty && expected != actual.trim()) {
        throw _RuntimeCommandFailure(
          'DESIRED_IDENTITY_MISMATCH',
          'desired $field does not match the installed desktop pet',
        );
      }
    }

    requireIdentity('petId', installation['petId']?.toString() ?? '');
    requireIdentity('characterId', installation['characterId']?.toString() ?? '');
    requireIdentity('releaseId', installation['currentReleaseId']?.toString() ?? '');

    final authoritativePetId = installation['petId']?.toString().trim() ?? '';
    final authoritativeReleaseId =
        installation['currentReleaseId']?.toString().trim() ?? '';
    final authoritativeCharacterId =
        installation['characterId']?.toString().trim() ?? '';
    final manifestPetId = manifest['petId']?.toString().trim() ?? '';
    final manifestReleaseId = manifest['releaseId']?.toString().trim() ?? '';
    if (manifest.isEmpty ||
        manifestPetId != authoritativePetId ||
        manifestReleaseId != authoritativeReleaseId) {
      throw const _RuntimeCommandFailure(
        'PACKAGE_AUTHORITY_MISMATCH',
        'installation manifest does not match the authoritative release',
      );
    }

    String expectedAuthority(String innerKey, String manifestValue) {
      final fromDesired = inner[innerKey]?.toString().trim() ?? '';
      if (fromDesired.isNotEmpty && manifestValue.isNotEmpty && fromDesired != manifestValue) {
        throw _RuntimeCommandFailure(
          'PACKAGE_AUTHORITY_MISMATCH',
          'desired $innerKey does not match the authoritative release manifest',
        );
      }
      return fromDesired.isNotEmpty ? fromDesired : manifestValue;
    }

    final expectedReleaseVersion =
        expectedAuthority('releaseVersion', manifest['version']?.toString().trim() ?? '');
    final expectedManifestHash = expectedAuthority(
      'manifestHash',
      manifestIntegrity['manifestHash']?.toString().trim() ?? '',
    );
    final expectedContentRootHash = expectedAuthority(
      'contentRootHash',
      manifestIntegrity['contentRootHash']?.toString().trim() ?? '',
    );
    if (expectedManifestHash.isEmpty || expectedContentRootHash.isEmpty) {
      throw const _RuntimeCommandFailure(
        'PACKAGE_INTEGRITY_AUTHORITY_MISSING',
        'authoritative Package V2 integrity hashes are missing',
      );
    }

    final installPath = installation['installPath']?.toString().trim() ?? '';
    if (installPath.isEmpty) {
      throw const _RuntimeCommandFailure(
        'INSTALL_PATH_MISSING',
        'installation does not contain a runtime install path',
      );
    }
    final overlay = await _native('system.overlay.status');
    if (overlay['permissionGranted'] != true) {
      state = state.copyWith(
        permissionGranted: false,
        phase: 'permission_required',
        updatedAt: DateTime.now(),
      );
      throw const _RuntimeCommandFailure(
        'OVERLAY_PERMISSION_REQUIRED',
        'Android overlay permission is required before enabling the desktop pet',
      );
    }

    final prefs = await SharedPreferences.getInstance();
    final alpha = (prefs.getDouble(_prefsAlpha) ?? state.alpha).clamp(0.2, 1.0).toDouble();

    // Runtime V2 desiredHash covers the complete canonical RuntimeSettings
    // object. Android must never silently project unsupported desktop-only
    // values and still acknowledge that canonical hash as applied.
    final alwaysOnTop = _int(settings['alwaysOnTop'], 1);
    final clickThroughMode =
        settings['clickThroughMode']?.toString().trim().toLowerCase() ?? 'off';
    final soundEnabled = _int(settings['soundEnabled'], 0);
    final positionMode =
        settings['positionMode']?.toString().trim().toLowerCase() ?? 'absolute';
    final screenId = settings['screenId']?.toString().trim() ?? '';
    final displayFingerprint =
        settings['displayFingerprint']?.toString().trim() ?? '';
    if (alwaysOnTop != 1) {
      throw const _RuntimeCommandFailure(
        'DESIRED_SETTINGS_UNSUPPORTED',
        'Android desktop pet overlays are always-on-top; alwaysOnTop=false cannot be applied exactly',
      );
    }
    if (clickThroughMode != 'off') {
      throw _RuntimeCommandFailure(
        'DESIRED_SETTINGS_UNSUPPORTED',
        'Android desktop pet renderer does not support clickThroughMode=$clickThroughMode',
      );
    }
    if (soundEnabled != 0) {
      throw const _RuntimeCommandFailure(
        'DESIRED_SETTINGS_UNSUPPORTED',
        'Android desktop pet renderer does not support package sound playback',
      );
    }
    if (positionMode != 'absolute') {
      throw _RuntimeCommandFailure(
        'DESIRED_SETTINGS_UNSUPPORTED',
        'Android desktop pet renderer requires positionMode=absolute; requested=$positionMode',
      );
    }
    if (screenId.isNotEmpty && screenId != 'android-primary') {
      throw _RuntimeCommandFailure(
        'DESIRED_SETTINGS_UNSUPPORTED',
        'Android desktop pet renderer exposes only android-primary; requested screenId=$screenId',
      );
    }
    if (displayFingerprint.isNotEmpty) {
      throw const _RuntimeCommandFailure(
        'DESIRED_SETTINGS_UNSUPPORTED',
        'Android desktop pet renderer cannot apply desktop displayFingerprint affinity exactly',
      );
    }

    final scale = _double(settings['scale'], 1.0);
    if (!scale.isFinite || scale < 0.1 || scale > 5.0) {
      throw const _RuntimeCommandFailure(
        'DESIRED_SCALE_INVALID',
        'desktop pet scale must be between 0.1 and 5.0',
      );
    }
    final canvasWidth = _positiveInt(installation['canvasWidth'], 180);
    final canvasHeight = _positiveInt(installation['canvasHeight'], 180);
    final width = (canvasWidth * scale).round();
    final height = (canvasHeight * scale).round();
    if (width < 64 || width > 420 || height < 64 || height > 420) {
      throw _RuntimeCommandFailure(
        'DESIRED_SETTINGS_UNSUPPORTED',
        'Android desktop pet window ${width}x${height}dp is outside the supported 64..420dp range; desired state was not applied',
      );
    }
    final x = _int(settings['positionX'], 16);
    final y = _int(settings['positionY'], 120);
    final defaultAction =
        (inner['defaultActionKey'] ?? installation['defaultActionKey'])?.toString().trim() ?? '';

    await _interruptPlayback('package_switch');
    await _native('desktop.pet.renderer.unload');
    final loaded = await _native('desktop.pet.renderer.load', <String, dynamic>{
      'installationId': installationId,
      'characterId': authoritativeCharacterId,
      'petId': authoritativePetId,
      'releaseId': authoritativeReleaseId,
      'releaseVersion': expectedReleaseVersion,
      'manifestHash': expectedManifestHash,
      'contentRootHash': expectedContentRootHash,
      'authoritativeManifestJson': jsonEncode(manifest),
      'runtimeVersion': _runtimeVersion,
      'installPath': installPath,
      'actionKey': defaultAction,
      'alpha': alpha,
      'width': width,
      'height': height,
      'x': x,
      'y': y,
    });
    final appliedWidth = _positiveInt(loaded['width'], -1);
    final appliedHeight = _positiveInt(loaded['height'], -1);
    final appliedX = _int(loaded['x'], x - 1);
    final appliedY = _int(loaded['y'], y - 1);
    final appliedScale = _double(loaded['scale'], double.nan);
    final scaleTolerance = 0.500001 / canvasWidth.toDouble();
    final scaleMatches =
        appliedScale.isFinite && (appliedScale - scale).abs() <= scaleTolerance;
    if (appliedWidth != width ||
        appliedHeight != height ||
        appliedX != x ||
        appliedY != y ||
        !scaleMatches) {
      await _native('desktop.pet.renderer.unload');
      throw _RuntimeCommandFailure(
        'DESIRED_SETTINGS_NOT_APPLIED',
        'Android renderer applied ${appliedWidth}x${appliedHeight}dp at ($appliedX,$appliedY) scale=$appliedScale instead of requested ${width}x${height}dp at ($x,$y) scale=$scale',
      );
    }
    await _native('desktop.pet.renderer.show');
    state = state.copyWith(
      permissionGranted: true,
      rendererLoaded: true,
      visible: true,
      paused: false,
      installationId: installationId,
      petName: installation['name']?.toString() ?? '',
      currentActionKey: loaded['currentActionKey']?.toString() ?? defaultAction,
      playbackId: loaded['playbackId']?.toString() ?? '',
      alpha: alpha,
      phase: 'ready',
      error: '',
      updatedAt: DateTime.now(),
    );
  }

  Future<void> _playAction(
    String commandId,
    int commandSequence,
    Map<String, dynamic> outer,
    Map<String, dynamic> inner,
  ) async {
    final actionKey = inner['actionKey']?.toString().trim() ?? '';
    if (actionKey.isEmpty) {
      throw const _RuntimeCommandFailure('MISSING_ACTION_KEY', 'actionKey is required');
    }
    if (!state.rendererLoaded) {
      throw const _RuntimeCommandFailure('PET_NOT_READY', 'desktop pet renderer is not ready');
    }

    final requestedRuntimeId = inner['runtimeId']?.toString().trim() ?? '';
    final requestedInstance = inner['petInstanceId']?.toString().trim() ?? '';
    final installationId =
        (inner['installationId'] ?? outer['installationId'])?.toString().trim() ?? '';
    final characterId = inner['characterId']?.toString().trim() ?? '';
    if (requestedRuntimeId.isEmpty || requestedRuntimeId != _runtimeId) {
      throw const _RuntimeCommandFailure(
        'RUNTIME_ID_MISMATCH',
        'play action must target the current runtime identity',
      );
    }
    if (requestedInstance.isEmpty || requestedInstance != _runtimeId) {
      throw const _RuntimeCommandFailure(
        'PET_INSTANCE_MISMATCH',
        'play action must target the current pet instance',
      );
    }
    if (installationId.isEmpty || installationId != state.installationId) {
      throw const _RuntimeCommandFailure(
        'INSTALLATION_MISMATCH',
        'play action must target the active installation',
      );
    }
    final nativeStatus = await _native('desktop.pet.renderer.status');
    final activeCharacter = nativeStatus['characterId']?.toString().trim() ?? '';
    if (characterId.isEmpty || activeCharacter.isEmpty || characterId != activeCharacter) {
      throw const _RuntimeCommandFailure(
        'CHARACTER_MISMATCH',
        'play action must target the active installation character',
      );
    }

    final queuePolicy = inner['queuePolicy']?.toString().trim() ?? 'replace_current';
    if (queuePolicy != 'replace_current' && queuePolicy != 'enqueue') {
      throw const _RuntimeCommandFailure(
        'QUEUE_POLICY_INVALID',
        'play action queuePolicy is invalid',
      );
    }
    if (_playback != null && queuePolicy == 'enqueue') {
      // Runtime V2 currently exposes one physical Android lane. Until an action
      // has a renderer-owned playback identity, claiming queue admission would
      // create an unverifiable lifecycle. Reject truthfully so the scheduler can
      // retry/re-plan rather than leaving a command stuck in runtime_accepted.
      throw const _RuntimeCommandFailure(
        'RENDERER_QUEUE_BUSY',
        'Android desktop pet renderer is busy; enqueue admission is unavailable',
      );
    }
    final current = _playback;
    if (current != null) {
      final currentPlayedMs = _nonNegativeInt(nativeStatus['playedMs']);
      if (!current.interruptible || nativeStatus['interruptible'] == false) {
        throw const _RuntimeCommandFailure(
          'ACTION_NOT_INTERRUPTIBLE',
          'current desktop pet action cannot be interrupted',
        );
      }
      final interruptFloorMs = max(current.minimumPlayMs, current.interruptAfterMs);
      if (currentPlayedMs < interruptFloorMs) {
        throw _RuntimeCommandFailure(
          'ACTION_INTERRUPT_WINDOW_NOT_REACHED',
          'current action interrupt floor ${interruptFloorMs}ms has not been reached',
        );
      }
    }

    await _interruptPlayback('replaced_by_command', replacedByCommandId: commandId);
    final commandInterruptible = inner['interruptible'] != false;
    final playResult = await _native('desktop.pet.renderer.play', <String, dynamic>{
      'actionKey': actionKey,
      'playbackRate': _double(inner['playbackRate'], 1.0).clamp(0.25, 4.0),
      'interruptible': commandInterruptible,
    });
    final playbackId = playResult['playbackId']?.toString().trim() ?? '';
    if (playbackId.isEmpty) {
      throw const _RuntimeCommandFailure(
        'RENDERER_PLAYBACK_ID_MISSING',
        'renderer did not return a playback identity',
      );
    }
    final interruptAfterMs = _nonNegativeInt(playResult['interruptAfterMs']);
    final nativeMinimumPlayMs = _nonNegativeInt(playResult['minimumPlayMs']);
    final minimumPlayMs = inner.containsKey('minimumPlayMs')
        ? _nonNegativeInt(inner['minimumPlayMs'])
        : nativeMinimumPlayMs;
    int? maximumPlayMs;
    if (inner.containsKey('maximumPlayMs') && inner['maximumPlayMs'] != null) {
      maximumPlayMs = _nonNegativeInt(inner['maximumPlayMs']);
    } else if (playResult['maximumPlayMs'] != null) {
      maximumPlayMs = _nonNegativeInt(playResult['maximumPlayMs']);
    }
    if (maximumPlayMs != null && maximumPlayMs < minimumPlayMs) {
      await _native('desktop.pet.renderer.stop');
      throw const _RuntimeCommandFailure(
        'ACTION_PLAY_WINDOW_INVALID',
        'maximumPlayMs must be greater than or equal to minimumPlayMs',
      );
    }
    final tracked = _TrackedPlayback(
      commandId: commandId,
      commandSequence: commandSequence,
      playbackId: playbackId,
      actionKey: actionKey,
      installationId: state.installationId,
      characterId: characterId,
      decisionId: inner['decisionId']?.toString() ?? '',
      completionPolicy: inner['completionPolicy']?.toString() ?? '',
      interruptAfterMs: interruptAfterMs,
      minimumPlayMs: minimumPlayMs,
      maximumPlayMs: maximumPlayMs,
      interruptible: commandInterruptible && playResult['interruptible'] != false,
    );
    _playback = tracked;
    await _sendCommandAck(commandId, commandSequence, 'runtime_accepted');
    await _sendPlaybackEvent('runtime.playback.command_accepted', tracked);
    await _sendPlaybackEvent('runtime.playback.action_started', tracked);
    state = state.copyWith(
      currentActionKey: actionKey,
      playbackId: playbackId,
      paused: false,
      updatedAt: DateTime.now(),
    );
    _startPlaybackPoll(tracked);
    await _sendStateSnapshot(currentCommandId: commandId);
  }

  void _startPlaybackPoll(_TrackedPlayback tracked) {
    tracked.pollTimer?.cancel();
    tracked.pollTimer = Timer.periodic(const Duration(milliseconds: 120), (_) {
      unawaited(_pollPlayback(tracked));
    });
  }

  Future<void> _pollPlayback(_TrackedPlayback tracked) async {
    if (_playback != tracked || _disposed || !state.connected) {
      tracked.cancel();
      return;
    }
    try {
      final native = await _native('desktop.pet.renderer.status');
      _lastConfirmedNativeStatus = Map<String, dynamic>.from(native);
      _nativeStatusAvailable = true;
      _nativeStatusLastError = '';
      if (_playback != tracked) return;
      final nativeCycle = _nonNegativeInt(native['cycleIndex']);
      final completedPlaybackId = native['lastCompletedPlaybackId']?.toString() ?? '';
      final completedCycle = _nonNegativeInt(native['lastCompletedCycleIndex']);
      if ((nativeCycle >= 1 || completedCycle >= 1) && !tracked.firstCycleSent) {
        tracked.firstCycleSent = true;
        await _sendPlaybackEvent(
          'runtime.playback.action_first_cycle',
          tracked,
          extra: <String, dynamic>{'cycleIndex': max(1, max(nativeCycle, completedCycle))},
        );
      }
      final nativePlayedMs = _nonNegativeInt(native['playedMs']);
      if (completedPlaybackId != tracked.playbackId &&
          tracked.maximumPlayMs != null &&
          nativePlayedMs >= tracked.maximumPlayMs!) {
        await _interruptPlayback('max_duration_reached');
        await refreshStatus();
        await _sendStateSnapshot();
        return;
      }
      if (completedPlaybackId == tracked.playbackId) {
        tracked.cancel();
        final playedMs = _nonNegativeInt(native['lastCompletedPlayedMs']);
        await _sendPlaybackEvent(
          'runtime.playback.action_completed',
          tracked,
          extra: <String, dynamic>{
            'playedMs': playedMs,
            'completionReason': native['lastCompletionReason']?.toString().trim().isNotEmpty == true
                ? native['lastCompletionReason'].toString()
                : 'natural_end',
          },
        );
        _cursor.lastProcessedCommandSequence =
            max(_cursor.lastProcessedCommandSequence, tracked.commandSequence);
        _playback = null;
        await _persistCursor();
        await refreshStatus();
        await _sendStateSnapshot();
      }
    } catch (error) {
      if (_playback != tracked) return;
      tracked.cancel();
      await _sendPlaybackEvent(
        'runtime.playback.action_failed',
        tracked,
        extra: <String, dynamic>{
          'errorCode': 'RENDERER_STATUS_FAILED',
          'errorMessage': error.toString(),
          'recoverable': true,
        },
      );
      _cursor.lastProcessedCommandSequence =
          max(_cursor.lastProcessedCommandSequence, tracked.commandSequence);
      _playback = null;
      await _persistCursor();
      await _sendStateSnapshot();
    }
  }

  Future<void> _interruptPlayback(
    String reason, {
    String replacedByCommandId = '',
  }) async {
    final tracked = _playback;
    tracked?.cancel();
    _playback = null;

    Map<String, dynamic> rendererStop = <String, dynamic>{};
    if (state.rendererLoaded) {
      try {
        rendererStop = await _native('desktop.pet.renderer.stop');
      } catch (_) {
        if (tracked != null) {
          _playback = tracked;
          _startPlaybackPoll(tracked);
        }
        rethrow;
      }
    }
    if (tracked == null) return;

    final stoppedPlaybackId = rendererStop['stoppedPlaybackId']?.toString().trim() ?? '';
    if (stoppedPlaybackId != tracked.playbackId) {
      if (state.connected) {
        await _sendPlaybackEvent(
          'runtime.playback.action_failed',
          tracked,
          extra: <String, dynamic>{
            'errorCode': 'RENDERER_PLAYBACK_ID_MISMATCH',
            'errorMessage': 'renderer stopped a different playback instance',
            'recoverable': false,
          },
        );
        _cursor.lastProcessedCommandSequence =
            max(_cursor.lastProcessedCommandSequence, tracked.commandSequence);
        await _persistCursor();
      }
      throw const _RuntimeCommandFailure(
        'RENDERER_PLAYBACK_ID_MISMATCH',
        'renderer stopped a different playback instance',
      );
    }

    final playedMs = _nonNegativeInt(rendererStop['stoppedPlayedMs']);
    if (state.connected) {
      await _sendPlaybackEvent(
        'runtime.playback.action_interrupted',
        tracked,
        extra: <String, dynamic>{
          'playedMs': playedMs,
          'interruptReason': reason,
          if (replacedByCommandId.isNotEmpty)
            'replacedByCommandId': replacedByCommandId,
        },
      );
      _cursor.lastProcessedCommandSequence =
          max(_cursor.lastProcessedCommandSequence, tracked.commandSequence);
      await _persistCursor();
    }
  }

  Future<void> _sendPlaybackEvent(
    String name,
    _TrackedPlayback tracked, {
    Map<String, dynamic> extra = const <String, dynamic>{},
  }) async {
    final now = DateTime.now().toUtc().toIso8601String();
    final payload = <String, dynamic>{
      'type': name,
      'playbackInstanceId': tracked.playbackId,
      'commandId': tracked.commandId,
      'actionKey': tracked.actionKey,
      'triggerSource': 'runtime_command',
      'installationId': tracked.installationId,
      if (tracked.characterId.isNotEmpty) 'characterId': tracked.characterId,
      'petInstanceId': _runtimeId,
      if (tracked.decisionId.isNotEmpty) 'decisionId': tracked.decisionId,
      if (name == 'runtime.playback.action_started') 'startedAt': now,
      if (name == 'runtime.playback.action_completed') 'completedAt': now,
      if (name == 'runtime.playback.action_interrupted') 'interruptedAt': now,
      if (name == 'runtime.playback.action_failed') 'failedAt': now,
      'occurredAt': now,
      ...extra,
    };
    await _sendEnvelope('runtime_event', name, payload);
  }

  Future<void> _sendDesiredApplied(
    String commandId,
    int desiredRevision,
    String desiredHash,
  ) async {
    final now = DateTime.now().toUtc().toIso8601String();
    await _sendEnvelope('runtime_event', 'runtime.state.desired_applied', <String, dynamic>{
      'commandId': commandId,
      'desiredRevision': desiredRevision,
      'desiredHash': desiredHash,
      'appliedAt': now,
      'occurredAt': now,
    });
  }

  Future<void> _sendDesiredRejected(
    String commandId,
    int desiredRevision,
    String desiredHash,
    String errorCode,
    String errorMessage,
  ) async {
    final now = DateTime.now().toUtc().toIso8601String();
    await _sendEnvelope('runtime_event', 'runtime.state.desired_rejected', <String, dynamic>{
      'commandId': commandId,
      'desiredRevision': desiredRevision,
      'desiredHash': desiredHash,
      'errorCode': errorCode,
      'errorMessage': errorMessage,
      'rejectedAt': now,
      'occurredAt': now,
    });
  }

  Future<void> _sendHello() async {
    final payload = <String, dynamic>{
      'runtimeVersion': _runtimeVersion,
      'runtimeContractVersion': _runtimeContractVersion,
      'deviceId': _deviceId,
      'runtimeId': _runtimeId,
      'runtimeCapabilities': _mandatoryCapabilities,
      'lastAppliedDesiredRevision': _cursor.lastAppliedDesiredRevision,
      'lastProcessedCommandSequence': _cursor.lastProcessedCommandSequence,
      'lastEventSequence': _cursor.lastEventSequence,
      if (_cursor.actualStateHash.isNotEmpty) 'actualStateHash': _cursor.actualStateHash,
    };
    await _sendEnvelope('hello', 'hello', payload, allowHandshake: true);
  }

  Future<void> _sendCommandAck(
    String commandId,
    int commandSequence,
    String status, {
    String errorCode = '',
    String errorMessage = '',
  }) async {
    await _sendEnvelope('command_ack', 'command_ack', <String, dynamic>{
      'commandId': commandId,
      'commandSequence': commandSequence,
      'status': status,
      if (errorMessage.isNotEmpty) 'rejectReason': errorMessage,
      if (errorCode.isNotEmpty) 'rejectErrorCode': errorCode,
      'runtimeSessionId': _sessionId,
      'receivedAt': DateTime.now().toUtc().toIso8601String(),
    });
  }

  Future<void> _sendStateSnapshot({String currentCommandId = ''}) async {
    if (!state.connected || _sessionId.isEmpty || _socket == null) return;
    Map<String, dynamic> native = <String, dynamic>{};
    bool nativeSuccess = false;
    try {
      native = await _native('desktop.pet.renderer.status');
      _lastConfirmedNativeStatus = Map<String, dynamic>.from(native);
      _nativeStatusAvailable = true;
      _nativeStatusLastError = '';
      nativeSuccess = true;
    } catch (e) {
      _nativeStatusAvailable = false;
      _nativeStatusLastError = e.toString();
    }
    if (!nativeSuccess) {
      return;
    }
    final loaded = native['loaded'] == true;
    final visible = native['visible'] == true;
    final paused = native['paused'] == true;
    final currentAction = native['currentActionKey']?.toString() ?? '';
    final playbackId = _playback?.playbackId ?? native['playbackId']?.toString() ?? '';
    final playbackActive = native['playbackActive'] == true;
    final playbackMode = native['playbackMode']?.toString() ?? '';
    final actualFacts = <String, dynamic>{
      'installationId': loaded ? (native['installationId']?.toString() ?? state.installationId) : '',
      'petId': loaded ? (native['petId']?.toString() ?? '') : '',
      'releaseId': loaded ? (native['releaseId']?.toString() ?? '') : '',
      'instanceStatus': loaded ? 'ready' : 'absent',
      'windowStatus': loaded ? (visible ? 'visible' : 'hidden') : 'absent',
      'rendererStatus': loaded ? 'runtime_ready' : 'absent',
      'playbackStatus': !loaded
          ? 'stopped'
          : paused
              ? 'paused'
              : playbackMode == 'hold' && !playbackActive
                  ? 'holding'
                  : playbackActive || _playback != null
                      ? 'playing'
                      : 'idle',
      'stableActionKey': loaded ? (native['defaultActionKey']?.toString() ?? '') : '',
      'currentActionKey': currentAction,
      'playbackInstanceId': playbackId,
      'currentCommandId': currentCommandId,
      'visible': visible,
      'positionX': _int(native['x'], 0),
      'positionY': _int(native['y'], 0),
      'screenId': 'android-primary',
      'windowWidth': _nonNegativeInt(native['width']),
      'windowHeight': _nonNegativeInt(native['height']),
      'scale': _double(native['scale'], 1.0),
      'appliedDesiredRevision': _cursor.lastAppliedDesiredRevision,
      'appliedDesiredHash': _cursor.appliedDesiredHash,
      'appliedSettingsRevision': _cursor.appliedSettingsRevision,
    };
    final actualHash = _payloadHash(actualFacts);
    _cursor.actualStateHash = actualHash;
    final snapshot = <String, dynamic>{
      'connectionGeneration': max(1, _connectionGeneration),
      'eventSequence': _outboundSequence + 1,
      'actualStateHash': actualHash,
      'instanceStatus': actualFacts['instanceStatus'],
      'windowStatus': actualFacts['windowStatus'],
      'rendererStatus': actualFacts['rendererStatus'],
      'playbackStatus': actualFacts['playbackStatus'],
      'appliedDesiredRevision': _cursor.lastAppliedDesiredRevision,
      if (_cursor.lastAppliedDesiredRevision > 0)
        'appliedDesiredHash': _cursor.appliedDesiredHash,
      'appliedSettingsRevision': _cursor.appliedSettingsRevision,
      'installationId': actualFacts['installationId'],
      'petId': actualFacts['petId'],
      'releaseId': actualFacts['releaseId'],
      'visible': actualFacts['visible'],
      'positionX': actualFacts['positionX'],
      'positionY': actualFacts['positionY'],
      'screenId': actualFacts['screenId'],
      'windowWidth': actualFacts['windowWidth'],
      'windowHeight': actualFacts['windowHeight'],
      'scale': actualFacts['scale'],
      'stableActionKey': actualFacts['stableActionKey'],
      'currentActionKey': actualFacts['currentActionKey'],
      if (playbackId.isNotEmpty) 'playbackInstanceId': playbackId,
      if (currentCommandId.isNotEmpty) 'currentCommandId': currentCommandId,
      'lastProcessedCommandSequence': _cursor.lastProcessedCommandSequence,
      'capturedAt': DateTime.now().toUtc().toIso8601String(),
    };
    await _sendEnvelope('runtime_event', 'runtime.state.snapshot', snapshot);
    await _persistCursor();
  }

  Future<void> _sendEnvelope(
    String messageType,
    String messageName,
    Object? payload, {
    bool allowHandshake = false,
  }) async {
    final socket = _socket;
    if (socket == null || socket.readyState != WebSocket.open) {
      throw StateError('runtime socket is not open');
    }
    if (!allowHandshake && (_sessionId.isEmpty || !state.connected)) {
      throw StateError('runtime session is not ready');
    }
    _outboundSequence += 1;
    final envelope = <String, dynamic>{
      'envelopeVersion': 2,
      'protocol': _runtimeProtocol,
      'messageType': messageType,
      'messageName': messageName,
      'messageId': 'msg_${DateTime.now().microsecondsSinceEpoch}_${_randomToken(8)}',
      'userId': _userId,
      'deviceId': _deviceId,
      'runtimeId': _runtimeId,
      'runtimeSessionId': _sessionId,
      'connectionGeneration': max(1, _connectionGeneration),
      'sequence': _outboundSequence,
      'payloadSchemaVersion': 1,
      'payloadHash': _payloadHash(payload),
      'sentAt': DateTime.now().toUtc().toIso8601String(),
      'payload': payload,
    };
    final serialized = jsonEncode(envelope);
    if (utf8.encode(serialized).length > _maxMessageBytes) {
      throw StateError('runtime envelope exceeds maxMessageBytes');
    }
    socket.add(serialized);
    if (messageType == 'runtime_event' || messageType == 'command_ack') {
      _cursor.lastEventSequence = _outboundSequence;
      await _persistCursor();
    }
  }

  void _startHeartbeat() {
    _heartbeatTimer?.cancel();
    _watchdogTimer?.cancel();
    _snapshotTimer?.cancel();
    _interactionTimer?.cancel();
    _heartbeatTimer = Timer.periodic(
      Duration(milliseconds: _heartbeatIntervalMs),
      (_) => unawaited(_sendHeartbeat()),
    );
    final watchdogMs = max(1000, min(_heartbeatIntervalMs, _heartbeatTimeoutMs ~/ 3));
    _watchdogTimer = Timer.periodic(Duration(milliseconds: watchdogMs), (_) {
      if (!state.connected) return;
      if (DateTime.now().difference(_lastServerMessageAt).inMilliseconds <=
          _heartbeatTimeoutMs) {
        return;
      }
      final socket = _socket;
      if (socket != null) {
        unawaited(socket.close(4002, 'heartbeat_timeout'));
      }
    });
    _snapshotTimer = Timer.periodic(const Duration(seconds: 20), (_) {
      unawaited(_sendStateSnapshot());
    });
    _interactionTimer = Timer.periodic(const Duration(milliseconds: 120), (_) {
      if (state.connected && state.rendererLoaded) {
        unawaited(_drainRendererInteractions());
      }
    });
  }

  Future<void> _drainRendererInteractions() async {
    if (_drainingInteractions || !state.connected || !state.rendererLoaded) return;
    _drainingInteractions = true;
    try {
      final drained = await _native('desktop.pet.renderer.events.drain');
      final rawEvents = drained['events'];
      if (rawEvents is! List || rawEvents.isEmpty) return;
      Map<String, dynamic> native = <String, dynamic>{};
      try {
        native = await _native('desktop.pet.renderer.status');
        _lastConfirmedNativeStatus = Map<String, dynamic>.from(native);
        _nativeStatusAvailable = true;
        _nativeStatusLastError = '';
      } catch (e) {
        _nativeStatusAvailable = false;
        _nativeStatusLastError = e.toString();
        native = Map<String, dynamic>.from(_lastConfirmedNativeStatus);
      }
      final installationId = native['installationId']?.toString() ?? state.installationId;
      final releaseId = native['releaseId']?.toString() ?? '';
      final actionKey = native['currentActionKey']?.toString() ?? state.currentActionKey;
      final playbackId = native['playbackId']?.toString() ?? state.playbackId;
      final frameIndex = _nonNegativeInt(native['frameIndex']);

      for (final raw in rawEvents) {
        if (raw is! Map) continue;
        final event = Map<String, dynamic>.from(raw);
        final type = event['type']?.toString().trim() ?? '';
        final payload = _map(event['payload']);
        if (type.isEmpty) continue;
        final occurredMs = _positiveInt(payload['occurredAtMs']);
        final occurredAt = occurredMs > 0
            ? DateTime.fromMillisecondsSinceEpoch(occurredMs, isUtc: true)
            : DateTime.now().toUtc();
        final base = <String, dynamic>{
          'installationId': installationId,
          'releaseId': releaseId,
          'actionKey': actionKey,
          if (playbackId.isNotEmpty) 'playbackInstanceId': playbackId,
          'gestureId': payload['dragId']?.toString().trim().isNotEmpty == true
              ? payload['dragId'].toString()
              : 'gesture_${_randomToken(18)}',
          'sequence': occurredAt.millisecondsSinceEpoch,
          'occurredAt': occurredAt.toIso8601String(),
        };
        switch (type) {
          case 'runtime.pointer.clicked':
            await _sendEnvelope('runtime_event', type, <String, dynamic>{
              ...base,
              'button': 'left',
              'clickCount': 1,
              'canvasX': _nonNegativeInt(payload['canvasX']),
              'canvasY': _nonNegativeInt(payload['canvasY']),
              'screenX': _nonNegativeInt(payload['screenX']),
              'screenY': _nonNegativeInt(payload['screenY']),
              'frameIndex': frameIndex,
            });
            break;
          case 'runtime.drag.started':
            await _sendEnvelope('runtime_event', type, <String, dynamic>{
              ...base,
              'dragId': payload['dragId']?.toString() ?? '',
              'startX': _nonNegativeInt(payload['startX']),
              'startY': _nonNegativeInt(payload['startY']),
              'currentX': _nonNegativeInt(payload['currentX']),
              'currentY': _nonNegativeInt(payload['currentY']),
              'displayId': 'android-primary',
            });
            break;
          case 'runtime.drag.completed':
            await _sendEnvelope('runtime_event', type, <String, dynamic>{
              ...base,
              'dragId': payload['dragId']?.toString() ?? '',
              'startX': _nonNegativeInt(payload['startX']),
              'startY': _nonNegativeInt(payload['startY']),
              'currentX': _nonNegativeInt(payload['currentX']),
              'currentY': _nonNegativeInt(payload['currentY']),
              'displayId': 'android-primary',
            });
            unawaited(_persistDraggedPosition(
              installationId,
              _nonNegativeInt(payload['currentX']),
              _nonNegativeInt(payload['currentY']),
            ));
            break;
          case 'runtime.drag.cancelled':
            await _sendEnvelope('runtime_event', type, <String, dynamic>{
              ...base,
              'dragId': payload['dragId']?.toString() ?? '',
              'displayId': 'android-primary',
            });
            break;
        }
      }
      await refreshStatus();
      await _sendStateSnapshot();
    } catch (error) {
      state = state.copyWith(error: error.toString(), updatedAt: DateTime.now());
    } finally {
      _drainingInteractions = false;
    }
  }

  Future<void> _persistDraggedPosition(
    String installationId,
    int x,
    int y,
  ) async {
    if (installationId.isEmpty || _deviceId.isEmpty) return;
    _pendingPosition = _PendingPosition(installationId, x, y);
    if (_persistingPosition) return;

    _persistingPosition = true;
    try {
      while (_pendingPosition != null && !_disposed) {
        final pending = _pendingPosition!;
        _pendingPosition = null;
        try {
          await _persistRuntimePosition(
            pending.installationId,
            pending.x,
            pending.y,
            preservePositionMode: false,
          );
        } catch (error) {
          state = state.copyWith(
            error: '桌宠位置保存失败: $error',
            updatedAt: DateTime.now(),
          );
        }
      }
    } finally {
      _persistingPosition = false;
    }
  }

  Future<void> _persistRuntimePosition(
    String installationId,
    int x,
    int y, {
    required bool preservePositionMode,
  }) async {
    if (installationId.isEmpty || _deviceId.isEmpty) return;
    final api = _localApiRequired();
    final updated = await api.patch<Map<String, dynamic>>(
      '/api/desktop-pets/installations/${Uri.encodeComponent(installationId)}/settings',
      data: <String, dynamic>{
        'positionX': x,
        'positionY': y,
        if (!preservePositionMode) 'positionMode': 'absolute',
      },
      headers: <String, String>{
        'X-Amitia-Device-ID': _deviceId,
        'X-Amitia-Client-Type': 'mobile',
        'Idempotency-Key':
            'mobile-position-${DateTime.now().microsecondsSinceEpoch}-${_randomToken(12)}',
      },
      fromJson: (dynamic value) => Map<String, dynamic>.from(value as Map),
    );
    if (updated != null) {
      final settings = _map(updated['settings']);
      _cursor.appliedSettingsRevision = max(
        _cursor.appliedSettingsRevision,
        _nonNegativeInt(settings['settingsRevision'] ?? updated['settingsRevision']),
      );
      await _persistCursor();
    }
  }

  Future<void> _sendHeartbeat() async {
    if (!state.connected) return;
    try {
      await _sendEnvelope('ping', 'ping', <String, dynamic>{
        't': DateTime.now().millisecondsSinceEpoch,
      });
    } catch (_) {}
  }

  Future<Map<String, dynamic>> _loadInstallationDetail(String installationId) async {
    final api = _localApiRequired();
    final result = await api.get<Map<String, dynamic>>(
      '/api/desktop-pets/installations/${Uri.encodeComponent(installationId)}',
      headers: <String, String>{
        'X-Amitia-Device-ID': _deviceId,
        'X-Amitia-Client-Type': 'mobile',
      },
      fromJson: (dynamic value) => Map<String, dynamic>.from(value as Map),
    );
    if (result == null) throw StateError('installation detail is empty');
    return result;
  }

  Future<void> _loadActiveInstallationName() async {
    if (_deviceId.isEmpty) return;
    try {
      final api = _localApiRequired();
      final response = await api.get<Map<String, dynamic>>(
        '/api/desktop-pets/installations',
        headers: <String, String>{
          'X-Amitia-Device-ID': _deviceId,
          'X-Amitia-Client-Type': 'mobile',
        },
        fromJson: (dynamic value) => Map<String, dynamic>.from(value as Map),
      );
      final rawItems = response?['items'];
      if (rawItems is! List) return;
      Map<String, dynamic>? active;
      for (final raw in rawItems) {
        if (raw is! Map) continue;
        final item = Map<String, dynamic>.from(raw);
        final enabled = item['isActive'] == 1 ||
            item['desiredState']?.toString() == 'enabled' ||
            item['status']?.toString() == 'enabled';
        if (enabled) {
          active = item;
          break;
        }
      }
      if (active != null) {
        state = state.copyWith(
          installationId: active['id']?.toString() ?? state.installationId,
          petName: active['name']?.toString() ?? state.petName,
          updatedAt: DateTime.now(),
        );
      } else if (!state.rendererLoaded) {
        state = state.copyWith(
          installationId: '',
          petName: '',
          updatedAt: DateTime.now(),
        );
      }
    } catch (_) {}
  }

  BackendServiceApi _localApiRequired() {
    final api = ref.read(rawDeviceLocalBackendServiceApiProvider);
    if (api == null) throw StateError('device-local backend is unavailable');
    return api;
  }

  NativeBridgePlatformDispatcher get _dispatcher =>
      ref.read(nativeBridgePlatformDispatcherProvider);

  Future<Map<String, dynamic>> _native(
    String operation, [
    Map<String, dynamic> payload = const <String, dynamic>{},
  ]) async {
    final response = await _dispatcher.execute(<String, dynamic>{
      'protocolVersion': 1,
      'requestId': 'pet_${DateTime.now().microsecondsSinceEpoch}_${_randomToken(8)}',
      'platform': 'android',
      'operation': operation,
      'payload': payload,
    });
    if (response['status']?.toString() != 'success') {
      final error = _map(response['error']);
      throw _RuntimeCommandFailure(
        error['domainCode']?.toString().trim().isNotEmpty == true
            ? error['domainCode'].toString()
            : (error['code']?.toString() ?? 'NATIVE_ERROR'),
        error['message']?.toString() ?? 'native operation failed: $operation',
      );
    }
    return _map(response['result']);
  }


  Future<void> _onSocketClosed(int epoch, String reason) async {
    if (_disposed || epoch != _attachEpoch) return;
    _clearSocketOnly();
    _sessionId = '';
    _connectionGeneration = 0;
    _lastServerSequence = 0;
    _localPlaybackQuiesce = _localPlaybackQuiesce.then<void>((_) => _stopPlaybackLocally());
    await _localPlaybackQuiesce;
    if (_disposed || epoch != _attachEpoch) return;
    state = state.copyWith(
      connected: false,
      phase: 'reconnecting',
      playbackId: '',
      paused: false,
      error: reason == 'socket_closed' ? '' : reason,
      updatedAt: DateTime.now(),
    );
    _scheduleReconnect(epoch);
  }

  void _scheduleReconnect(int epoch) {
    if (_disposed || epoch != _attachEpoch || _config == null) return;
    _reconnectTimer?.cancel();
    _reconnectTimer = Timer(const Duration(seconds: 3), () {
      if (!_disposed && epoch == _attachEpoch && _config != null) {
        unawaited(_connect(epoch));
      }
    });
  }

  void _disconnect(String reason) {
    _reconnectTimer?.cancel();
    _heartbeatTimer?.cancel();
    _watchdogTimer?.cancel();
    _snapshotTimer?.cancel();
    _interactionTimer?.cancel();
    _clearSocketOnly();
    _sessionId = '';
    _connectionGeneration = 0;
    _lastServerSequence = 0;
    _localPlaybackQuiesce = _localPlaybackQuiesce.then<void>((_) => _stopPlaybackLocally());
    state = state.copyWith(
      connected: false,
      playbackId: '',
      paused: false,
      phase: reason == 'device_runtime_unavailable' ? 'waiting_runtime' : 'disconnected',
      updatedAt: DateTime.now(),
    );
  }

  Future<void> _stopPlaybackLocally() async {
    final tracked = _playback;
    tracked?.cancel();
    _playback = null;
    if (!Platform.isAndroid) return;
    try {
      // Stop is deliberately attempted without consulting StateNotifier.state:
      // this quiesce may finish after provider disposal, where reading state is
      // invalid. The native renderer treats an already-unloaded surface as a
      // harmless failure and the catch below keeps disconnect idempotent.
      await _native('desktop.pet.renderer.stop');
    } catch (_) {
      // The authoritative websocket session is already gone. Do not synthesize
      // lifecycle events from an uncertain transport; the next desired-state
      // reconciliation re-establishes the renderer from physical state.
    }
  }

  void _clearSocketOnly() {
    _socketSubscription?.cancel();
    _socketSubscription = null;
    final socket = _socket;
    _socket = null;
    if (socket != null) {
      unawaited(socket.close(1000, 'runtime_reset'));
    }
    _heartbeatTimer?.cancel();
    _watchdogTimer?.cancel();
    _snapshotTimer?.cancel();
    _interactionTimer?.cancel();
  }

  Future<void> _persistCursor() async {
    // Intentionally in-memory only. The runtime cursor describes renderer state
    // owned by this Android process incarnation and must never survive it.
  }

  void _trimReplay() {
    while (_durableReplay.length > 256) {
      _durableReplay.remove(_durableReplay.keys.first);
    }
  }

  String _randomToken(int length) {
    const alphabet = 'abcdefghijklmnopqrstuvwxyz0123456789';
    return List<String>.generate(
      length,
      (_) => alphabet[_random.nextInt(alphabet.length)],
      growable: false,
    ).join();
  }

  @override
  void dispose() {
    if (_disposed) return;
    _disposed = true;
    _attachEpoch++;
    _config = null;
    _disconnect('disposed');
    super.dispose();
  }
}

class _RuntimeCommandFailure implements Exception {
  final String code;
  final String message;

  const _RuntimeCommandFailure(this.code, this.message);

  @override
  String toString() => '$code: $message';
}

bool _isDurable(String type) =>
    type == 'runtime.command.sync_desired_state' ||
    type == 'runtime.command.ensure_absent' ||
    type == 'runtime.command.reload_release';

bool _isEphemeral(String type) =>
    type == 'runtime.command.play_action' ||
    type == 'runtime.command.stop_action' ||
    type == 'runtime.command.pause_action' ||
    type == 'runtime.command.resume_action' ||
    type == 'runtime.command.recenter_once';

Map<String, dynamic> _map(Object? value) {
  if (value is! Map) return <String, dynamic>{};
  return Map<String, dynamic>.from(value);
}

int _int(Object? value, [int fallback = 0]) {
  if (value is num) return value.toInt();
  return int.tryParse(value?.toString() ?? '') ?? fallback;
}

int _nonNegativeInt(Object? value, [int fallback = 0]) =>
    max(0, _int(value, fallback));

int _positiveInt(Object? value, [int fallback = 0]) {
  final parsed = _int(value, fallback);
  return parsed > 0 ? parsed : fallback;
}

double _double(Object? value, [double fallback = 0]) {
  if (value is num) return value.toDouble();
  return double.tryParse(value?.toString() ?? '') ?? fallback;
}

String _goJsonScalar(Object? value) {
  if (value is double) {
    if (!value.isFinite) {
      throw const FormatException('runtime payload contains a non-finite number');
    }
    if (value == 0 && value.isNegative) return '-0';
    final absolute = value.abs();
    // Go encoding/json renders integral float64 values in fixed notation below
    // 1e21, while JSON.stringify/jsonEncode would otherwise preserve `.0` in
    // Dart. Runtime protocol numbers are small, but keep the wire hash exact.
    if (absolute < 1e21 && value == value.truncateToDouble()) {
      return value.toInt().toString();
    }
  }
  final encoded = jsonEncode(value);
  if (value is! String) return encoded;
  return encoded
      .replaceAll('<', r'\u003c')
      .replaceAll('>', r'\u003e')
      .replaceAll('&', r'\u0026')
      .replaceAll('\u2028', r'\u2028')
      .replaceAll('\u2029', r'\u2029');
}

String _backendCanonicalJson(Object? value) {
  if (value is List) {
    return '[${value.map(_backendCanonicalJson).join(',')}]';
  }
  if (value is Map) {
    if (value.keys.any((Object? key) => key is! String)) {
      throw const FormatException('runtime payload object keys must be strings');
    }
    final keys = value.keys.cast<String>().toList()..sort();
    return '{${keys.map((String key) => '"$key":${_backendCanonicalJson(value[key])}').join(',')}}';
  }
  return _goJsonScalar(value);
}

String _payloadHash(Object? payload) {
  final canonical = _backendCanonicalJson(payload);
  return 'sha256:${sha256.convert(utf8.encode(canonical))}';
}

import 'dart:async';
import 'dart:convert';
import 'dart:math';

import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../artifact/artifact_providers.dart' show createAuthenticatedDio;
import '../backend_connection/backend_connection_availability.dart';
import '../backend_connection/backend_connection_config.dart';
import '../backend_connection/providers/backend_connection_providers.dart';
import '../native_bridge/native_bridge_platform_dispatcher.dart';
import '../native_bridge/providers/native_bridge_relay_provider.dart';
import '../services/extension_service.dart';
import '../services/providers.dart';
import '../services/system_service.dart';
import 'ui_device_identity.dart';
import 'ui_runtime_invalidation.dart';

const Set<String> _uiRuntimeChangeEvents = <String>{
  'extension_installed',
  'extension_uninstalled',
  'extension_enabled',
  'extension_disabled',
  'extension_paused',
  'extension_resumed',
  'extension_updated',
  'extension_rolled_back',
  'extension_rollback_failed',
  'extension_generation_changed',
  'extension_contributions_changed',
  'ui_provider_changed',
  'ui_profile_changed',
  'ui_slot_changed',
};

class MobileUIHostCommand {
  const MobileUIHostCommand({
    required this.eventType,
    required this.envelope,
  });

  final String eventType;
  final Map<String, dynamic> envelope;

  Map<String, dynamic> get payload {
    final raw = envelope['payload'];
    if (raw is Map<String, dynamic>) return raw;
    if (raw is Map) return raw.cast<String, dynamic>();
    return const <String, dynamic>{};
  }
}

/// In-process delivery of visual UI Host commands to the root MaterialApp.
/// Transport, deduplication and backend responses stay outside individual pages.
abstract final class MobileUIHostCommandBus {
  static final StreamController<MobileUIHostCommand> _controller =
      StreamController<MobileUIHostCommand>.broadcast(sync: true);

  static Stream<MobileUIHostCommand> get commands => _controller.stream;

  static void emit(MobileUIHostCommand command) {
    if (!_controller.isClosed) _controller.add(command);
  }
}

final mobileUIHostEventClientProvider = Provider<MobileUIHostEventClient?>((ref) {
  final availability = ref.watch(backendConnectionProvider).valueOrNull;
  if (availability is! BackendConnectionAvailable) return null;

  final client = MobileUIHostEventClient(
    config: availability.config,
    extensionService: ref.read(extensionServiceProvider),
    systemService: ref.read(systemServiceProvider),
    nativeDispatcher: ref.read(nativeBridgePlatformDispatcherProvider),
    activeConversationId: () => ref.read(activeConversationIdProvider),
    applyClientRuntimeSnapshot: (conversationId, state) {
      ref
          .read(clientRuntimePushedSessionStateProvider(conversationId).notifier)
          .state = Map<String, dynamic>.unmodifiable(state);
    },
    invalidateClientRuntime: (conversationId) {
      ref.invalidate(clientRuntimeSessionStateProvider(conversationId));
    },
  );
  unawaited(client.start());
  ref.onDispose(client.dispose);
  return client;
});

class MobileUIHostEventClient {
  MobileUIHostEventClient({
    required this.config,
    required ExtensionService extensionService,
    required SystemService systemService,
    required NativeBridgePlatformDispatcher nativeDispatcher,
    required String Function() activeConversationId,
    required void Function(
      String conversationId,
      Map<String, dynamic> sessionState,
    ) applyClientRuntimeSnapshot,
    required void Function(String conversationId) invalidateClientRuntime,
  }) : _extensionService = extensionService,
       _systemService = systemService,
       _nativeDispatcher = nativeDispatcher,
       _activeConversationId = activeConversationId,
       _applyClientRuntimeSnapshot = applyClientRuntimeSnapshot,
       _invalidateClientRuntime = invalidateClientRuntime;

  final BackendConnectionConfig config;
  final ExtensionService _extensionService;
  final SystemService _systemService;
  final NativeBridgePlatformDispatcher _nativeDispatcher;
  final String Function() _activeConversationId;
  final void Function(
    String conversationId,
    Map<String, dynamic> sessionState,
  ) _applyClientRuntimeSnapshot;
  final void Function(String conversationId) _invalidateClientRuntime;
  final UIDeviceIdentity _deviceIdentity = UIDeviceIdentity();

  static String? _processHostClientId;

  final Set<String> _processedRequestIds = <String>{};
  final Set<String> _processedProactiveMessageIds = <String>{};
  Dio? _dio;
  CancelToken? _cancelToken;
  Timer? _heartbeatTimer;
  Timer? _reconnectTimer;
  String _deviceId = '';
  String _hostClientId = '';
  String _hostSessionId = '';
  int _reconnectAttempts = 0;
  int _connectionVersion = 0;
  bool _disposed = false;
  bool _starting = false;
  DateTime? _notificationSettingsCheckedAt;
  bool _notificationsEnabled = false;

  Future<void> start() async {
    if (_disposed || _starting) return;
    _starting = true;
    try {
      _deviceId = await _deviceIdentity.getOrCreate();
      _hostClientId = _processHostClientId ??= _buildHostClientId(_deviceId);
      await _connect();
    } finally {
      _starting = false;
    }
  }

  Future<void> _connect() async {
    if (_disposed) return;
    final version = ++_connectionVersion;
    _cancelCurrentTransport();

    final dio = createAuthenticatedDio(config);
    dio.options.receiveTimeout = Duration.zero;
    final cancelToken = CancelToken();
    _dio = dio;
    _cancelToken = cancelToken;

    try {
      final response = await dio.get<ResponseBody>(
        '/api/proactive-sse',
        queryParameters: <String, dynamic>{'clientId': _hostClientId},
        options: Options(
          responseType: ResponseType.stream,
          headers: const <String, dynamic>{'Accept': 'text/event-stream'},
          receiveTimeout: Duration.zero,
        ),
        cancelToken: cancelToken,
      );
      if (_disposed || version != _connectionVersion) return;
      final body = response.data;
      if (response.statusCode != 200 || body == null) {
        throw StateError('UI Host SSE connection was not established');
      }

      await _registerHostSession();
      if (_disposed || version != _connectionVersion) return;
      _reconnectAttempts = 0;
      UIRuntimeInvalidationBus.notifyChanged();

      var buffer = '';
      await for (final chunk in body.stream.transform(utf8.decoder)) {
        if (_disposed || version != _connectionVersion) return;
        buffer += chunk.replaceAll('\r', '');
        final boundary = buffer.lastIndexOf('\n\n');
        if (boundary < 0) continue;
        _processSSEBlocks(buffer.substring(0, boundary));
        buffer = buffer.substring(boundary + 2);
      }
      if (!_disposed && version == _connectionVersion) {
        _scheduleReconnect();
      }
    } on DioException catch (error) {
      if (!_disposed && version == _connectionVersion && !CancelToken.isCancel(error)) {
        _scheduleReconnect();
      }
    } catch (_) {
      if (!_disposed && version == _connectionVersion) {
        _scheduleReconnect();
      }
    }
  }

  Future<void> _registerHostSession() async {
    final response = await _extensionService.registerUIHostSession(
      hostClientId: _hostClientId,
      deviceId: _deviceId,
      platform: _platformName(),
    );
    final sessionId = (response['hostSessionId'] ?? '').toString().trim();
    if (sessionId.isEmpty) {
      throw StateError('UI Host registration returned no host session');
    }
    _hostSessionId = sessionId;
    final rawInterval = (response['heartbeatIntervalSeconds'] as num?)?.toInt() ?? 60;
    final interval = Duration(seconds: max(20, rawInterval));
    _heartbeatTimer?.cancel();
    _heartbeatTimer = Timer.periodic(interval, (_) {
      if (_disposed || _hostSessionId.isEmpty) return;
      unawaited(
        _extensionService
            .heartbeatUIHostSession(
              hostClientId: _hostClientId,
              hostSessionId: _hostSessionId,
            )
            .catchError((_) {}),
      );
    });
  }

  void _processSSEBlocks(String data) {
    for (final rawBlock in data.split('\n\n')) {
      final block = rawBlock.trim();
      if (block.isEmpty) continue;
      var eventType = 'message';
      final dataLines = <String>[];
      for (final line in block.split('\n')) {
        if (line.startsWith('event:')) {
          eventType = line.substring(6).trim();
        } else if (line.startsWith('data:')) {
          dataLines.add(line.substring(5).trimLeft());
        }
      }
      if (dataLines.isEmpty) continue;
      try {
        final decoded = jsonDecode(dataLines.join('\n'));
        if (decoded is Map) {
          _dispatchEvent(eventType, decoded.cast<String, dynamic>());
        }
      } catch (_) {
        // Malformed event payloads are isolated from the long-lived stream.
      }
    }
  }

  void _dispatchEvent(String eventType, Map<String, dynamic> envelope) {
    if (_uiRuntimeChangeEvents.contains(eventType)) {
      if (eventType != 'extension_rollback_failed') {
        UIRuntimeInvalidationBus.notifyChanged();
      }
      return;
    }

    if (eventType == 'proactive_message') {
      unawaited(_handleProactiveMessage(envelope));
      return;
    }

    if (eventType == 'ui_client_runtime_command') {
      if (!_shouldProcessEnvelope(envelope)) return;
      unawaited(_handleClientRuntimeCommand(envelope));
      return;
    }

    if (eventType == 'ui_notify' ||
        eventType == 'ui_navigate' ||
        eventType == 'ui_dialog') {
      if (!_shouldProcessEnvelope(envelope)) return;
      MobileUIHostCommandBus.emit(
        MobileUIHostCommand(eventType: eventType, envelope: envelope),
      );
    }
  }

  bool _shouldProcessEnvelope(Map<String, dynamic> envelope) {
    final requestId = (envelope['requestId'] ?? '').toString().trim();
    if (requestId.isEmpty || _processedRequestIds.contains(requestId)) {
      return false;
    }
    final expiresAtText = (envelope['expiresAt'] ?? '').toString().trim();
    if (expiresAtText.isNotEmpty) {
      final expiresAt = DateTime.tryParse(expiresAtText);
      if (expiresAt != null && expiresAt.toUtc().isBefore(DateTime.now().toUtc())) {
        return false;
      }
    }
    _processedRequestIds.add(requestId);
    if (_processedRequestIds.length > 200) {
      _processedRequestIds.remove(_processedRequestIds.first);
    }
    return true;
  }

  Future<void> _handleClientRuntimeCommand(Map<String, dynamic> envelope) async {
    final rawPayload = envelope['payload'];
    if (rawPayload is! Map) return;
    final payload = rawPayload.cast<String, dynamic>();
    final commandId = (payload['commandId'] ?? '').toString().trim();
    final conversationId = (payload['conversationId'] ?? '').toString().trim();
    final hostClientId = (payload['hostClientId'] ?? envelope['hostClientId'] ?? _hostClientId)
        .toString()
        .trim();
    final hostSessionId = (payload['hostSessionId'] ?? envelope['hostSessionId'] ?? _hostSessionId)
        .toString()
        .trim();
    final expectResponse = payload['expectResponse'] != false;
    final rawState = payload['sessionState'];
    final sessionState = rawState is Map ? rawState.cast<String, dynamic>() : null;
    final revision = (sessionState?['revision'] as num?)?.toInt() ?? 0;

    Future<void> respond(Map<String, dynamic> result, [String error = '']) async {
      if (!expectResponse || commandId.isEmpty) return;
      await _extensionService.sendClientRuntimeResponse(
        commandId: commandId,
        hostClientId: hostClientId,
        hostSessionId: hostSessionId,
        result: result,
        error: error,
      );
    }

    try {
      if (conversationId.isEmpty || sessionState == null) {
        throw StateError('client runtime command is missing authoritative session state');
      }
      if (_activeConversationId().trim() != conversationId) {
        await respond(<String, dynamic>{
          'ok': true,
          'state': 'deferred',
          'revision': revision,
        });
        return;
      }

      // Flutter renders the authoritative declarative session rather than
      // executing a second browser-side package engine. Apply the command's
      // authoritative snapshot before acknowledging it so the server never
      // commits a transition that this host has not reconciled locally yet.
      _applyClientRuntimeSnapshot(conversationId, sessionState);
      await Future<void>.delayed(Duration.zero);
      await respond(<String, dynamic>{
        'ok': true,
        'state': 'reconciled',
        'applied': true,
        'revision': revision,
      });
      _invalidateClientRuntime(conversationId);
    } catch (error) {
      await respond(const <String, dynamic>{}, error.toString());
    }
  }

  Future<void> _handleProactiveMessage(Map<String, dynamic> payload) async {
    final messageId = (payload['messageId'] ?? '').toString().trim();
    if (messageId.isNotEmpty) {
      if (!_processedProactiveMessageIds.add(messageId)) return;
      if (_processedProactiveMessageIds.length > 200) {
        _processedProactiveMessageIds.remove(_processedProactiveMessageIds.first);
      }
    }
    final content = (payload['content'] ?? '').toString().trim();
    if (content.isEmpty || !await _notificationAllowed()) return;
    final platform = _nativeNotificationPlatform();
    if (platform == null) return;
    try {
      await _nativeDispatcher.execute(<String, dynamic>{
        'protocolVersion': 1,
        'requestId': 'proactive-${messageId.isEmpty ? DateTime.now().microsecondsSinceEpoch : messageId}',
        'platform': platform,
        'operation': 'notification.post',
        'payload': <String, dynamic>{
          'title': 'Amitia',
          'body': content,
          'channel': 'amitia_agent',
          'silent': false,
          if ((payload['conversationId'] ?? '').toString().trim().isNotEmpty)
            'conversationId': payload['conversationId'].toString().trim(),
        },
      });
    } catch (_) {
      // Notification delivery must not destabilize the global SSE connection.
    }
  }

  Future<bool> _notificationAllowed() async {
    final now = DateTime.now();
    final last = _notificationSettingsCheckedAt;
    if (last != null && now.difference(last) < const Duration(seconds: 30)) {
      return _notificationsEnabled;
    }
    try {
      final settings = await _systemService.notificationSettings(deviceId: _deviceId);
      _notificationsEnabled =
          settings?['enabled'] == true && settings?['subscribed'] == true;
    } catch (_) {
      _notificationsEnabled = false;
    }
    _notificationSettingsCheckedAt = now;
    return _notificationsEnabled;
  }

  void _scheduleReconnect() {
    if (_disposed || _reconnectTimer != null) return;
    _heartbeatTimer?.cancel();
    _heartbeatTimer = null;
    final exponent = min(_reconnectAttempts, 4);
    final baseSeconds = min(30, 2 * (1 << exponent));
    _reconnectAttempts++;
    final jitter = Random().nextInt(500);
    _reconnectTimer = Timer(
      Duration(seconds: baseSeconds, milliseconds: jitter),
      () {
        _reconnectTimer = null;
        if (!_disposed) unawaited(_connect());
      },
    );
  }

  void _cancelCurrentTransport() {
    _heartbeatTimer?.cancel();
    _heartbeatTimer = null;
    _cancelToken?.cancel('UI Host reconnect');
    _cancelToken = null;
    _dio?.close(force: true);
    _dio = null;
  }

  void dispose() {
    if (_disposed) return;
    _disposed = true;
    _connectionVersion++;
    _reconnectTimer?.cancel();
    _reconnectTimer = null;
    final sessionId = _hostSessionId;
    _hostSessionId = '';
    if (sessionId.isNotEmpty) {
      unawaited(
        _extensionService
            .disconnectUIHostSession(
              hostClientId: _hostClientId,
              hostSessionId: sessionId,
            )
            .catchError((_) {}),
      );
    }
    _cancelCurrentTransport();
  }

  static String _buildHostClientId(String deviceId) {
    final random = Random.secure();
    final suffix = List<int>.generate(6, (_) => random.nextInt(256))
        .map((value) => value.toRadixString(16).padLeft(2, '0'))
        .join();
    return 'flutter-ui-host-$deviceId-$suffix';
  }

  static String _platformName() {
    if (kIsWeb) return 'web';
    return switch (defaultTargetPlatform) {
      TargetPlatform.android => 'android',
      TargetPlatform.iOS => 'ios',
      TargetPlatform.windows => 'windows',
      TargetPlatform.macOS => 'darwin',
      TargetPlatform.linux => 'linux',
      TargetPlatform.fuchsia => 'web',
    };
  }

  static String? _nativeNotificationPlatform() {
    if (kIsWeb) return null;
    return switch (defaultTargetPlatform) {
      TargetPlatform.android => 'android',
      TargetPlatform.iOS => 'ios',
      TargetPlatform.windows => 'windows',
      _ => null,
    };
  }
}

import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'native_bridge_platform_dispatcher.dart';

class RelayEnvelope {
  final String type;
  final String requestId;
  final String platform;
  final int generation;
  final Map<String, dynamic>? payload;

  RelayEnvelope({
    required this.type,
    this.requestId = '',
    this.platform = '',
    this.generation = 0,
    this.payload,
  });

  factory RelayEnvelope.fromMap(Map<String, dynamic> map) {
    return RelayEnvelope(
      type: map['type'] as String? ?? '',
      requestId: map['requestId'] as String? ?? '',
      platform: map['platform'] as String? ?? '',
      generation: (map['generation'] as int?) ?? 0,
      payload: map['payload'] as Map<String, dynamic>?,
    );
  }

  Map<String, dynamic> toMap() {
    final map = <String, dynamic>{
      'type': type,
      'generation': generation,
    };
    if (requestId.isNotEmpty) map['requestId'] = requestId;
    if (platform.isNotEmpty) map['platform'] = platform;
    if (payload != null) map['payload'] = payload;
    return map;
  }

  String json() => jsonEncode(toMap());
}

class NativeBridgeRelayClient {
  final String baseUrl;
  final String platform;
  final Duration reconnectDelay;
  final NativeBridgePlatformDispatcher dispatcher;

  WebSocket? _socket;
  final StreamController<RelayEnvelope> _eventController =
      StreamController<RelayEnvelope>.broadcast();
  final StreamController<bool> _connectionController =
      StreamController<bool>.broadcast();

  Timer? _reconnectTimer;
  bool _disposed = false;
  bool _connecting = false;
  int _currentGeneration = 0;
  StreamSubscription<dynamic>? _nativeEventSub;

  NativeBridgeRelayClient({
    required this.baseUrl,
    required this.platform,
    required this.dispatcher,
    this.reconnectDelay = const Duration(seconds: 3),
  });

  Stream<RelayEnvelope> get events => _eventController.stream;
  Stream<bool> get connectionState => _connectionController.stream;
  bool get isConnected => _socket != null && !_disposed;
  int get currentGeneration => _currentGeneration;

  Future<void> connect() async {
    if (_disposed || _connecting) return;
    _connecting = true;

    try {
      final uri = Uri.parse('$baseUrl/api/native-bridge/relay?platform=$platform');
      final socket = await WebSocket.connect(
        uri.toString(),
        protocols: ['native-bridge-relay'],
      );
      _socket = socket;
      _connecting = false;
      _connectionController.add(true);
      _nativeEventSub = dispatcher.eventStream.listen(_onNativeEvent);

      socket.listen(
        _handleMessage,
        onError: (_) => _handleDisconnect(),
        onDone: () => _handleDisconnect(),
        cancelOnError: false,
      );
    } catch (e) {
      _connecting = false;
      _scheduleReconnect();
    }
  }

  void _handleMessage(dynamic raw) {
    if (raw is! String) return;
    try {
      final data = jsonDecode(raw) as Map<String, dynamic>;
      final envelope = RelayEnvelope.fromMap(data);
      if (envelope.type == 'native_bridge.request' &&
          envelope.requestId.isNotEmpty &&
          envelope.payload != null) {
        _handlePlatformRequest(envelope);
      } else {
        _eventController.add(envelope);
      }
    } catch (_) {}
  }

  Future<void> _handlePlatformRequest(RelayEnvelope request) async {
    final socket = _socket;
    if (socket == null) return;
    try {
      final nativeResponse = await dispatcher.execute(request.payload!);
      final responseEnvelope = RelayEnvelope(
        type: 'native_bridge.response',
        requestId: request.requestId,
        platform: platform,
        generation: _currentGeneration,
        payload: nativeResponse,
      );
      socket.add(responseEnvelope.json());
    } catch (e) {
      final errorEnvelope = RelayEnvelope(
        type: 'native_bridge.response',
        requestId: request.requestId,
        platform: platform,
        generation: _currentGeneration,
        payload: {
          'protocolVersion': 1,
          'requestID': request.requestId,
          'status': 'error',
          'error': {
            'code': 'NATIVE_ERROR',
            'message': e.toString(),
          },
        },
      );
      socket.add(errorEnvelope.json());
    }
  }

  void _onNativeEvent(Map<String, dynamic> nativeEvent) {
    final socket = _socket;
    if (socket == null) return;
    final envelope = RelayEnvelope(
      type: 'native_bridge.event',
      platform: platform,
      generation: _currentGeneration,
      payload: nativeEvent,
    );
    socket.add(envelope.json());
  }

  void updateGeneration(int generation) {
    _currentGeneration = generation;
  }

  void sendHealthUpdate(Map<String, dynamic> healthData) {
    final socket = _socket;
    if (socket == null) return;
    final envelope = RelayEnvelope(
      type: 'native_bridge.health',
      platform: platform,
      generation: _currentGeneration,
      payload: healthData,
    );
    socket.add(envelope.json());
  }

  void _handleDisconnect() {
    if (_disposed) return;
    _cancelNativeEventSub();
    _currentGeneration = 0;
    _socket = null;
    _connectionController.add(false);
    _scheduleReconnect();
  }

  void _cancelNativeEventSub() {
    _nativeEventSub?.cancel();
    _nativeEventSub = null;
  }

  void _scheduleReconnect() {
    if (_disposed) return;
    _reconnectTimer?.cancel();
    _reconnectTimer = Timer(reconnectDelay, () {
      if (!_disposed && _socket == null) {
        connect();
      }
    });
  }

  void reconnect() {
    _reconnectTimer?.cancel();
    _socket?.close();
    _socket = null;
    connect();
  }

  Future<void> dispose() async {
    _disposed = true;
    _reconnectTimer?.cancel();
    _cancelNativeEventSub();
    await _socket?.close();
    _socket = null;
    await _eventController.close();
    await _connectionController.close();
  }
}

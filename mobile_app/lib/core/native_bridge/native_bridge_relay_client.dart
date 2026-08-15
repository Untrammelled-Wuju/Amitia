import 'dart:async';
import 'dart:convert';
import 'dart:io';

class RelayEnvelope {
  final String type;
  final String requestId;
  final String platform;
  final String operation;
  final Map<String, dynamic>? payload;
  final Map<String, dynamic>? result;
  final Map<String, dynamic>? error;

  RelayEnvelope({
    required this.type,
    this.requestId = '',
    this.platform = '',
    this.operation = '',
    this.payload,
    this.result,
    this.error,
  });

  factory RelayEnvelope.fromMap(Map<String, dynamic> map) {
    return RelayEnvelope(
      type: map['type'] as String? ?? '',
      requestId: map['requestId'] as String? ?? '',
      platform: map['platform'] as String? ?? '',
      operation: map['operation'] as String? ?? '',
      payload: map['payload'] as Map<String, dynamic>?,
      result: map['result'] as Map<String, dynamic>?,
      error: map['error'] as Map<String, dynamic>?,
    );
  }

  Map<String, dynamic> toMap() {
    final map = <String, dynamic>{
      'type': type,
    };
    if (requestId.isNotEmpty) map['requestId'] = requestId;
    if (platform.isNotEmpty) map['platform'] = platform;
    if (operation.isNotEmpty) map['operation'] = operation;
    if (payload != null) map['payload'] = payload;
    if (result != null) map['result'] = result;
    if (error != null) map['error'] = error;
    return map;
  }

  String json() => jsonEncode(toMap());
}

class NativeBridgeRelayClient {
  final String baseUrl;
  final String platform;
  final Duration heartbeatInterval;
  final Duration reconnectDelay;

  WebSocket? _socket;
  final Map<String, Completer<RelayEnvelope>> _pendingRequests = {};
  final StreamController<RelayEnvelope> _eventController =
      StreamController<RelayEnvelope>.broadcast();
  final StreamController<bool> _connectionController =
      StreamController<bool>.broadcast();

  Timer? _heartbeatTimer;
  Timer? _reconnectTimer;
  bool _disposed = false;
  bool _connecting = false;
  int _requestCounter = 0;

  NativeBridgeRelayClient({
    required this.baseUrl,
    this.platform = 'ios',
    this.heartbeatInterval = const Duration(seconds: 30),
    this.reconnectDelay = const Duration(seconds: 3),
  });

  Stream<RelayEnvelope> get events => _eventController.stream;
  Stream<bool> get connectionState => _connectionController.stream;
  bool get isConnected => _socket != null && !_disposed;

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
      _startHeartbeat();

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
      if (envelope.type == 'native_bridge.response' &&
          envelope.requestId.isNotEmpty) {
        final completer = _pendingRequests.remove(envelope.requestId);
        if (completer != null && !completer.isCompleted) {
          completer.complete(envelope);
        }
      } else {
        _eventController.add(envelope);
      }
    } catch (_) {}
  }

  Future<RelayEnvelope> sendRequest({
    required String operation,
    Map<String, dynamic>? payload,
    Duration timeout = const Duration(seconds: 30),
  }) async {
    final socket = _socket;
    if (socket == null) {
      throw StateError('relay client not connected');
    }

    final requestId = 'req_${++_requestCounter}_${DateTime.now().millisecondsSinceEpoch}';
    final envelope = RelayEnvelope(
      type: 'native_bridge.request',
      requestId: requestId,
      platform: platform,
      operation: operation,
      payload: payload,
    );

    final completer = Completer<RelayEnvelope>();
    _pendingRequests[requestId] = completer;

    socket.add(envelope.json());

    Timer(timeout, () {
      if (_pendingRequests.remove(requestId) != null &&
          !completer.isCompleted) {
        completer.complete(RelayEnvelope(
          type: 'native_bridge.response',
          requestId: requestId,
          error: {'code': 'TIMEOUT', message: 'request timed out'},
        ));
      }
    });

    return completer.future;
  }

  void sendEvent({
    required String operation,
    Map<String, dynamic>? payload,
  }) {
    final socket = _socket;
    if (socket == null) return;

    final envelope = RelayEnvelope(
      type: 'native_bridge.event',
      platform: platform,
      operation: operation,
      payload: payload,
    );
    socket.add(envelope.json());
  }

  void _startHeartbeat() {
    _heartbeatTimer?.cancel();
    _heartbeatTimer = Timer.periodic(heartbeatInterval, (_) {
      final socket = _socket;
      if (socket == null) return;
      socket.add(jsonEncode({'type': 'ping'}));
    });
  }

  void _handleDisconnect() {
    if (_disposed) return;
    _heartbeatTimer?.cancel();
    _socket = null;
    _connectionController.add(false);

    for (final completer in _pendingRequests.values) {
      if (!completer.isCompleted) {
        completer.complete(RelayEnvelope(
          type: 'native_bridge.response',
          error: {'code': 'DISCONNECTED', message: 'relay disconnected'},
        ));
      }
    }
    _pendingRequests.clear();
    _scheduleReconnect();
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
    _heartbeatTimer?.cancel();
    _reconnectTimer?.cancel();
    _socket?.close();
    _socket = null;
    connect();
  }

  Future<void> dispose() async {
    _disposed = true;
    _heartbeatTimer?.cancel();
    _reconnectTimer?.cancel();
    await _socket?.close();
    _socket = null;
    await _eventController.close();
    await _connectionController.close();
  }
}

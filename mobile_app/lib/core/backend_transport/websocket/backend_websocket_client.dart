import 'dart:async';
import 'dart:io';

import 'package:web_socket_channel/io.dart';
import 'package:web_socket_channel/web_socket_channel.dart';

import '../../backend_connection/backend_connection_config.dart';
import '../../backend_connection/backend_uri_builder.dart';
import '../auth/backend_auth_header.dart';
import '../errors/backend_transport_error.dart';
import '../errors/backend_transport_error_code.dart';
import '../state/backend_websocket_state.dart';
import 'backend_websocket_message.dart';
import 'backend_websocket_session.dart';
import 'backend_websocket_transport.dart';

class BackendWebSocketSessionImpl implements BackendWebSocketSession {
  @override
  final int generation;
  final WebSocketChannel _channel;
  final StreamController<BackendWebSocketMessage> _controller =
      StreamController<BackendWebSocketMessage>.broadcast();
  final Set<StreamSubscription> _subscriptions = {};
  BackendWebSocketState _state = BackendWebSocketState.connecting;
  bool _closed = false;

  BackendWebSocketSessionImpl({
    required this.generation,
    required WebSocketChannel channel,
  }) : _channel = channel {
    _subscriptions.add(
      _channel.stream.listen(
        (data) {
          if (data is String) {
            _controller.add(WebSocketTextMessage(data));
          } else if (data is List<int>) {
            _controller.add(WebSocketBinaryMessage(data));
          }
        },
        onError: (Object error) {
          if (!_closed) {
            _controller.add(WebSocketErrorMessage(error));
            _state = BackendWebSocketState.disconnected;
          }
        },
        onDone: () {
          if (!_closed) {
            _state = BackendWebSocketState.disconnected;
          }
        },
      ),
    );
    _state = BackendWebSocketState.connected;
  }

  @override
  BackendWebSocketState get state => _closed ? BackendWebSocketState.closed : _state;

  @override
  Stream<BackendWebSocketMessage> get messages => _controller.stream;

  @override
  Future<void> send(BackendWebSocketMessage message) async {
    if (_closed) {
      throw BackendTransportError(
        code: BackendTransportErrorCode.transportClosed,
        generation: generation,
      );
    }
    switch (message) {
      case WebSocketTextMessage(:final data):
        _channel.sink.add(data);
      case WebSocketBinaryMessage(:final data):
        _channel.sink.add(data);
      case WebSocketCloseMessage(:final code, :final reason):
        await _channel.sink.close(code, reason);
      case WebSocketErrorMessage():
        break;
    }
  }

  @override
  Future<void> close() async {
    if (_closed) return;
    _closed = true;
    await _channel.sink.close();
    for (final sub in _subscriptions) {
      await sub.cancel();
    }
    _subscriptions.clear();
    await _controller.close();
    _state = BackendWebSocketState.closed;
  }
}

class BackendWebSocketClient implements BackendWebSocketTransport {
  final BackendConnectionConfig _config;
  final BackendUriBuilder _uriBuilder;
  final Set<BackendWebSocketSessionImpl> _sessions = {};
  BackendWebSocketState _state = BackendWebSocketState.idle;
  bool _closed = false;

  BackendWebSocketClient(
    this._config, {
    BackendUriBuilder? uriBuilder,
  }) : _uriBuilder = uriBuilder ?? BackendUriBuilder();

  @override
  BackendWebSocketState get state => _closed ? BackendWebSocketState.closed : _state;

  @override
  Future<BackendWebSocketSession> connect(
    String path, {
    Map<String, dynamic>? queryParameters,
  }) async {
    if (_closed) {
      throw BackendTransportError(
        code: BackendTransportErrorCode.transportClosed,
        path: path,
        generation: _config.generation,
      );
    }

    final uri = _uriBuilder.webSocket(
      _config,
      path,
      queryParameters: queryParameters,
    );

    final token = _config.credential.revealForTransport();
    final headers = <String, String>{
      BackendAuthHeader.localToken: token,
      'User-Agent': 'Amitia-Mobile',
    };

    final webSocket = await WebSocket.connect(
      uri.toString(),
      headers: headers,
    );
    final channel = IOWebSocketChannel(webSocket);

    final session = BackendWebSocketSessionImpl(
      generation: _config.generation,
      channel: channel,
    );

    _sessions.add(session);
    _state = BackendWebSocketState.connected;
    return session;
  }

  @override
  Future<void> close() async {
    if (_closed) return;
    _closed = true;
    for (final session in _sessions) {
      await session.close();
    }
    _sessions.clear();
    _state = BackendWebSocketState.closed;
  }
}

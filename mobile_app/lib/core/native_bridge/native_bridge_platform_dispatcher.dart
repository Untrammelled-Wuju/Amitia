import 'dart:async';
import 'dart:io' show Platform;
import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';

typedef NativeBackendActionHandler = Future<Map<String, dynamic>> Function(
  String actionId,
  Map<String, dynamic>? payload,
);

abstract interface class NativeBridgePlatformDispatcher {
  Future<Map<String, dynamic>> execute(Map<String, dynamic> nativeRequest);

  Future<Map<String, dynamic>> health();

  Stream<Map<String, dynamic>> get eventStream;

  void setBackendActionHandler(NativeBackendActionHandler? handler);
}

class AndroidNativeBridgeDispatcher implements NativeBridgePlatformDispatcher {
  static const MethodChannel _channel =
      MethodChannel('com.amitia.android_native/bridge');

  final StreamController<Map<String, dynamic>> _eventController =
      StreamController<Map<String, dynamic>>.broadcast();

  AndroidNativeBridgeDispatcher() {
    _channel.setMethodCallHandler(_handleNativeCall);
  }

  Future<dynamic> _handleNativeCall(MethodCall call) async {
    if (call.method == 'nativeEvent' && call.arguments is Map) {
      _eventController.add(Map<String, dynamic>.from(call.arguments as Map));
    }
    return null;
  }

  @override
  Future<Map<String, dynamic>> execute(Map<String, dynamic> nativeRequest) async {
    final result = await _channel.invokeMapMethod<String, dynamic>(
      'nativeBridge.execute',
      nativeRequest,
    );
    return result ?? {};
  }

  @override
  Future<Map<String, dynamic>> health() async {
    final result = await _channel.invokeMapMethod<String, dynamic>(
      'nativeBridge.health',
      {},
    );
    return result ?? {};
  }

  @override
  Stream<Map<String, dynamic>> get eventStream => _eventController.stream;

  @override
  void setBackendActionHandler(NativeBackendActionHandler? handler) {}
}

class IOSNativeBridgeDispatcher implements NativeBridgePlatformDispatcher {
  static const MethodChannel _channel =
      MethodChannel('com.amitia.ios_native/bridge');
  static const MethodChannel _backendActionChannel =
      MethodChannel('com.amitia.ios_native/backend_action');

  NativeBackendActionHandler? _backendActionHandler;
  final StreamController<Map<String, dynamic>> _eventController =
      StreamController<Map<String, dynamic>>.broadcast();

  IOSNativeBridgeDispatcher() {
    _channel.setMethodCallHandler(_handleNativeCall);
    _backendActionChannel.setMethodCallHandler(_handleBackendActionCall);
  }

  Future<dynamic> _handleNativeCall(MethodCall call) async {
    if (call.method == 'nativeEvent' && call.arguments is Map) {
      _eventController.add(Map<String, dynamic>.from(call.arguments as Map));
    }
    return null;
  }


  Future<dynamic> _handleBackendActionCall(MethodCall call) async {
    if (call.method != 'backendAction.execute') {
      throw MissingPluginException('unsupported backend action method: ${call.method}');
    }
    final args = call.arguments;
    if (args is! Map) {
      return {
        'status': 'error',
        'error': {'code': 'INVALID_ARGUMENT', 'message': 'backend action arguments must be a map'},
      };
    }
    final actionId = args['actionId'] as String? ?? '';
    final rawPayload = args['payload'];
    final payload = rawPayload is Map
        ? Map<String, dynamic>.from(rawPayload)
        : null;
    final handler = _backendActionHandler;
    if (handler == null || actionId.isEmpty) {
      return {
        'status': 'error',
        'error': {
          'code': handler == null ? 'BACKEND_DISPATCHER_NOT_READY' : 'INVALID_ARGUMENT',
          'message': handler == null ? 'native relay is not ready' : 'missing actionId',
        },
      };
    }
    return handler(actionId, payload);
  }

  @override
  Future<Map<String, dynamic>> execute(Map<String, dynamic> nativeRequest) async {
    final result = await _channel.invokeMapMethod<String, dynamic>(
      'nativeBridge.execute',
      nativeRequest,
    );
    return result ?? {};
  }

  @override
  Future<Map<String, dynamic>> health() async {
    final result = await _channel.invokeMapMethod<String, dynamic>(
      'nativeBridge.health',
      {},
    );
    return result ?? {};
  }

  @override
  Stream<Map<String, dynamic>> get eventStream => _eventController.stream;

  @override
  void setBackendActionHandler(NativeBackendActionHandler? handler) {
    _backendActionHandler = handler;
  }
}

class FallbackNativeBridgeDispatcher implements NativeBridgePlatformDispatcher {
  @override
  Future<Map<String, dynamic>> execute(Map<String, dynamic> nativeRequest) async {
    return {
      'protocolVersion': 1,
      'status': 'error',
      'error': {
        'code': 'PLATFORM_NOT_SUPPORTED',
        'message': 'native bridge not supported on this platform',
      },
    };
  }

  @override
  Future<Map<String, dynamic>> health() async {
    return {'ready': false, 'foreground': false};
  }

  @override
  Stream<Map<String, dynamic>> get eventStream => const Stream.empty();

  @override
  void setBackendActionHandler(NativeBackendActionHandler? handler) {}
}

NativeBridgePlatformDispatcher createPlatformDispatcher() {
  if (kIsWeb) return FallbackNativeBridgeDispatcher();
  if (Platform.isIOS) return IOSNativeBridgeDispatcher();
  if (Platform.isAndroid) return AndroidNativeBridgeDispatcher();
  return FallbackNativeBridgeDispatcher();
}

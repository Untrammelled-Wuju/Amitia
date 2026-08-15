import 'dart:async';
import 'dart:io' show Platform;
import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';

abstract interface class NativeBridgePlatformDispatcher {
  Future<Map<String, dynamic>> execute(Map<String, dynamic> nativeRequest);

  Future<Map<String, dynamic>> health();

  Stream<Map<String, dynamic>> get eventStream;
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
}

class IOSNativeBridgeDispatcher implements NativeBridgePlatformDispatcher {
  static const MethodChannel _channel =
      MethodChannel('com.amitia.ios_native/bridge');

  final StreamController<Map<String, dynamic>> _eventController =
      StreamController<Map<String, dynamic>>.broadcast();

  IOSNativeBridgeDispatcher() {
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
}

NativeBridgePlatformDispatcher createPlatformDispatcher() {
  if (kIsWeb) return FallbackNativeBridgeDispatcher();
  if (Platform.isIOS) return IOSNativeBridgeDispatcher();
  if (Platform.isAndroid) return AndroidNativeBridgeDispatcher();
  return FallbackNativeBridgeDispatcher();
}

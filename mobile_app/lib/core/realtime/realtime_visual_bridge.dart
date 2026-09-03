import 'dart:typed_data';

import 'package:flutter/services.dart';

class RealtimeVisualFrame {
  final String source;
  final int sequence;
  final DateTime capturedAt;
  final String mime;
  final int width;
  final int height;
  final Uint8List bytes;

  const RealtimeVisualFrame({
    required this.source,
    required this.sequence,
    required this.capturedAt,
    required this.mime,
    required this.width,
    required this.height,
    required this.bytes,
  });

  factory RealtimeVisualFrame.fromEvent(dynamic event) {
    if (event is! Map) {
      throw StateError('Unsupported realtime visual event: ${event.runtimeType}');
    }
    final raw = event['data'];
    final bytes = raw is Uint8List
        ? raw
        : raw is List<int>
            ? Uint8List.fromList(raw)
            : Uint8List(0);
    return RealtimeVisualFrame(
      source: event['source']?.toString() ?? '',
      sequence: int.tryParse(event['sequence']?.toString() ?? '') ?? 0,
      capturedAt: event['capturedAtMs'] is num
          ? DateTime.fromMillisecondsSinceEpoch((event['capturedAtMs'] as num).toInt(), isUtc: true)
          : DateTime.tryParse(event['capturedAt']?.toString() ?? '')?.toUtc() ?? DateTime.now().toUtc(),
      mime: event['mime']?.toString() ?? 'image/jpeg',
      width: int.tryParse(event['width']?.toString() ?? '') ?? 0,
      height: int.tryParse(event['height']?.toString() ?? '') ?? 0,
      bytes: bytes,
    );
  }
}

class RealtimeVisualStatus {
  final bool cameraActive;
  final bool screenActive;
  final bool cameraSupported;
  final bool screenSupported;
  final bool crossAppScreenSupported;

  const RealtimeVisualStatus({
    required this.cameraActive,
    required this.screenActive,
    required this.cameraSupported,
    required this.screenSupported,
    required this.crossAppScreenSupported,
  });

  factory RealtimeVisualStatus.fromMap(Map<dynamic, dynamic> map) {
    return RealtimeVisualStatus(
      cameraActive: map['cameraActive'] == true,
      screenActive: map['screenActive'] == true,
      cameraSupported: map['cameraSupported'] != false,
      screenSupported: map['screenSupported'] != false,
      crossAppScreenSupported: map['crossAppScreenSupported'] == true,
    );
  }
}

class RealtimeVisualBridge {
  static const MethodChannel _control = MethodChannel('com.amitia.realtime_visual/control');
  static const EventChannel _frames = EventChannel('com.amitia.realtime_visual/frames');

  Stream<RealtimeVisualFrame>? _cachedFrames;

  Stream<RealtimeVisualFrame> get frames {
    return _cachedFrames ??= _frames
        .receiveBroadcastStream()
        .map(RealtimeVisualFrame.fromEvent)
        .where((frame) => frame.source.isNotEmpty && frame.bytes.isNotEmpty)
        .asBroadcastStream();
  }

  Future<void> startCamera({String facing = 'front'}) async {
    await _control.invokeMethod<void>('startCamera', <String, dynamic>{'facing': facing});
  }

  Future<void> stopCamera() async {
    await _control.invokeMethod<void>('stopCamera');
  }

  Future<void> switchCamera() async {
    await _control.invokeMethod<void>('switchCamera');
  }

  Future<void> startScreen() async {
    await _control.invokeMethod<void>('startScreen');
  }

  Future<void> stopScreen() async {
    await _control.invokeMethod<void>('stopScreen');
  }

  Future<void> requestImmediateFrame(String source) async {
    await _control.invokeMethod<void>('requestImmediateFrame', <String, dynamic>{'source': source});
  }

  Future<RealtimeVisualStatus> status() async {
    final raw = await _control.invokeMapMethod<dynamic, dynamic>('status') ?? const <dynamic, dynamic>{};
    return RealtimeVisualStatus.fromMap(raw);
  }

  Future<void> reset() async {
    await _control.invokeMethod<void>('reset');
  }
}

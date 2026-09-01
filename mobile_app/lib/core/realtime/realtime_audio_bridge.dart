import 'dart:typed_data';

import 'package:flutter/services.dart';

/// Platform audio bridge used by realtime voice calls.
///
/// Capture is mono PCM16 at 16 kHz. Playback accepts mono PCM16 at 24 kHz,
/// matching backend/internal/realtime/proxy.go and the desktop client.
class RealtimeAudioBridge {
  static const MethodChannel _control =
      MethodChannel('com.amitia.realtime_audio/control');
  static const EventChannel _input =
      EventChannel('com.amitia.realtime_audio/input');

  Stream<Uint8List>? _cachedInput;

  Stream<Uint8List> get inputPcm {
    return _cachedInput ??= _input.receiveBroadcastStream().map((event) {
      if (event is Uint8List) return event;
      if (event is List<int>) return Uint8List.fromList(event);
      throw StateError('Unsupported realtime audio payload: ${event.runtimeType}');
    }).asBroadcastStream();
  }

  Future<void> startCapture() async {
    await _control.invokeMethod<void>('startCapture');
  }

  Future<void> stopCapture() async {
    await _control.invokeMethod<void>('stopCapture');
  }

  Future<void> playPcm(Uint8List pcm) async {
    if (pcm.isEmpty) return;
    await _control.invokeMethod<void>('playPcm', pcm);
  }

  Future<void> reset() async {
    await _control.invokeMethod<void>('reset');
  }

  /// Delivers a finalized realtime-ASR transcript to the Android local
  /// workflow ingress. The native side posts directly to the Device Agent,
  /// so cloud-mode realtime voice never executes a local trigger in Cloud Core.
  Future<void> emitWorkflowAsrFinal({
    required String transcript,
    required String eventId,
    String? sessionId,
    String? conversationId,
  }) async {
    final normalized = transcript.trim();
    if (normalized.isEmpty || eventId.trim().isEmpty) return;
    try {
      await _control.invokeMethod<void>('emitWorkflowASRFinal', <String, dynamic>{
        'transcript': normalized,
        'eventId': eventId.trim(),
        if ((sessionId ?? '').trim().isNotEmpty) 'sessionId': sessionId!.trim(),
        if ((conversationId ?? '').trim().isNotEmpty)
          'conversationId': conversationId!.trim(),
      });
    } on MissingPluginException {
      // Device workflow triggers are Android-only for this bridge.
    } on PlatformException {
      // Realtime voice must stay usable if the local Device Agent is restarting.
    }
  }
}

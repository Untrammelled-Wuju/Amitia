import 'dart:typed_data';

import 'package:amitia_app/core/realtime/realtime_voice_activity_detector.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('emits speech start and end around sustained voice activity', () {
    final detector = RealtimeVoiceActivityDetector(
      minRms: 0.01,
      noiseMultiplier: 2,
      speechStartFrames: 2,
      speechEndFrames: 2,
    );

    expect(detector.process(_pcm(0.002)), isNull);
    expect(detector.process(_pcm(0.04)), isNull);
    expect(detector.process(_pcm(0.04)), RealtimeVadEvent.speechStart);
    expect(detector.isSpeechActive, isTrue);
    expect(detector.process(_pcm(0.001)), isNull);
    expect(detector.process(_pcm(0.001)), RealtimeVadEvent.speechEnd);
    expect(detector.isSpeechActive, isFalse);
  });

  test('reset clears active speech state', () {
    final detector = RealtimeVoiceActivityDetector(
      minRms: 0.01,
      speechStartFrames: 1,
      speechEndFrames: 1,
    );

    expect(detector.process(_pcm(0.04)), RealtimeVadEvent.speechStart);
    detector.reset();
    expect(detector.isSpeechActive, isFalse);
    expect(detector.process(_pcm(0.001)), isNull);
  });
}

Uint8List _pcm(double amplitude) {
  final data = ByteData(320);
  final value = (amplitude.clamp(0, 1) * 32767).round();
  for (var offset = 0; offset + 1 < data.lengthInBytes; offset += 2) {
    data.setInt16(offset, value, Endian.little);
  }
  return data.buffer.asUint8List();
}

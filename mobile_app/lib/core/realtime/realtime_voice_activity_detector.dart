import 'dart:typed_data';

enum RealtimeVadEvent { speechStart, speechEnd }

class RealtimeVoiceActivityDetector {
  RealtimeVoiceActivityDetector({
    this.minRms = 0.018,
    this.noiseMultiplier = 3.2,
    this.speechStartFrames = 2,
    this.speechEndFrames = 2,
  });

  final double minRms;
  final double noiseMultiplier;
  final int speechStartFrames;
  final int speechEndFrames;

  double _noiseFloor = 0.004;
  double _rms = 0;
  bool _active = false;
  int _speechFrames = 0;
  int _silenceFrames = 0;

  bool get isSpeechActive => _active;
  double get rms => _rms;

  double get threshold {
    final adaptive = _noiseFloor * noiseMultiplier;
    if (adaptive < minRms) return minRms;
    if (adaptive > 0.12) return 0.12;
    return adaptive;
  }

  void reset() {
    _noiseFloor = 0.004;
    _rms = 0;
    _active = false;
    _speechFrames = 0;
    _silenceFrames = 0;
  }

  RealtimeVadEvent? process(Uint8List pcm) {
    if (pcm.length < 2) return null;
    final data = ByteData.sublistView(pcm);
    var sumSquares = 0.0;
    var samples = 0;
    for (var offset = 0; offset + 1 < pcm.length; offset += 2) {
      final sample = data.getInt16(offset, Endian.little) / 32768.0;
      sumSquares += sample * sample;
      samples++;
    }
    if (samples == 0) return null;
    _rms = sumSquares <= 0 ? 0 : _sqrt(sumSquares / samples);
    final speech = _rms >= threshold;

    if (!_active) {
      if (speech) {
        _speechFrames++;
        _silenceFrames = 0;
      } else {
        _speechFrames = 0;
        final floor = _rms > 0.05 ? 0.05 : _rms;
        _noiseFloor = _noiseFloor * 0.95 + floor * 0.05;
      }
      if (_speechFrames >= speechStartFrames) {
        _active = true;
        _speechFrames = 0;
        _silenceFrames = 0;
        return RealtimeVadEvent.speechStart;
      }
      return null;
    }

    if (speech) {
      _silenceFrames = 0;
      return null;
    }
    _silenceFrames++;
    final floor = _rms > 0.05 ? 0.05 : _rms;
    _noiseFloor = _noiseFloor * 0.98 + floor * 0.02;
    if (_silenceFrames >= speechEndFrames) {
      _active = false;
      _silenceFrames = 0;
      return RealtimeVadEvent.speechEnd;
    }
    return null;
  }

  double _sqrt(double value) {
    if (value <= 0) return 0;
    var estimate = value;
    for (var index = 0; index < 20; index++) {
      estimate = (estimate + value / estimate) / 2;
    }
    return estimate;
  }
}

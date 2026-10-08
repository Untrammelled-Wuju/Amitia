import 'dart:async';
import 'dart:io';

import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend_transport/providers/backend_transport_providers.dart';
import '../native_bridge/providers/native_bridge_relay_provider.dart';
import '../runtime/backend/mobile_backend_providers.dart';
import 'device_owned_speech_service.dart';

final ownedSpeechPlayerProvider = Provider<OwnedSpeechPlayer>((ref) {
  final platform = switch (defaultTargetPlatform) {
    TargetPlatform.android => 'android',
    TargetPlatform.iOS => 'ios',
    TargetPlatform.windows => 'windows',
    _ => null,
  };
  final player = OwnedSpeechPlayer(
    platform: kIsWeb ? null : platform,
    execute: (request) =>
        ref.read(nativeBridgePlatformDispatcherProvider).execute(request),
  );
  ref.listen(
    mobileDeploymentConfigProvider,
    (_, __) => unawaited(player.stop()),
  );
  ref.listen(rawBackendServiceApiProvider, (_, __) => unawaited(player.stop()));
  ref.onDispose(() => unawaited(player.close()));
  return player;
});

class OwnedSpeechPlayer {
  final String? platform;
  final Future<Map<String, dynamic>> Function(Map<String, dynamic>) execute;
  bool _starting = false;
  bool _playing = false;
  bool _closed = false;
  bool _checking = false;
  int _generation = 0;
  Timer? _watch;
  Timer? _cleanup;
  Directory? _directory;

  OwnedSpeechPlayer({required this.platform, required this.execute});

  Future<void> play(OwnedSpeechAudio audio) async {
    if (_closed || _starting || platform == null) {
      throw StateError('语音播放器不可用或正在启动');
    }
    _starting = true;
    await stop();
    final generation = ++_generation;
    Directory? directory;
    try {
      await audio.assertCurrent();
      if (_closed || generation != _generation) throw StateError('旧语音播放已取消');
      directory = await Directory.systemTemp.createTemp('amitia_owned_speech_');
      _directory = directory;
      final file = File('${directory.path}/speech.mp3');
      await file.writeAsBytes(audio.bytes, flush: true);
      await audio.assertCurrent();
      if (_closed || generation != _generation) {
        throw StateError('Core 已切换，旧语音播放已取消');
      }
      _playing = true;
      final result = await execute({
        'protocolVersion': 1,
        'requestId': 'owned-speech-${audio.requestId}',
        'platform': platform,
        'operation': 'media.audio.play_file',
        'payload': {'path': file.path},
      });
      if (!const ['success', 'ok'].contains(result['status'])) {
        throw StateError('设备未确认语音播放成功');
      }
      if (_closed || generation != _generation) throw StateError('旧语音播放已取消');
      _watch = Timer.periodic(const Duration(seconds: 1), (_) {
        if (_checking || generation != _generation) return;
        _checking = true;
        unawaited(
          audio
              .assertCurrent()
              .catchError((Object _) async {
                if (generation == _generation) await stop();
              })
              .whenComplete(() => _checking = false),
        );
      });
      final rawDuration = result['result'] is Map
          ? result['result']['durationMs']
          : null;
      final duration = rawDuration is int && rawDuration >= 0
          ? (rawDuration + 1000).clamp(1000, 301000)
          : 301000;
      _cleanup = Timer(
        Duration(milliseconds: duration),
        () => unawaited(stop()),
      );
    } catch (_) {
      if (generation == _generation) await stop();
      if (directory != null && await directory.exists()) {
        await directory.delete(recursive: true);
      }
      rethrow;
    } finally {
      _starting = false;
    }
  }

  Future<void> stop() async {
    ++_generation;
    _watch?.cancel();
    _watch = null;
    _cleanup?.cancel();
    _cleanup = null;
    final directory = _directory;
    _directory = null;
    final shouldStop = _playing || _starting;
    _playing = false;
    if (shouldStop && platform != null) {
      try {
        await execute({
          'protocolVersion': 1,
          'requestId': 'owned-speech-stop-$_generation',
          'platform': platform,
          'operation': 'media.audio.stop',
          'payload': <String, dynamic>{},
        });
      } catch (_) {}
    }
    if (directory != null) {
      try {
        if (await directory.exists()) await directory.delete(recursive: true);
      } catch (_) {}
    }
  }

  Future<void> close() async {
    _closed = true;
    await stop();
  }
}

import 'dart:io';

import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../artifact/artifact_providers.dart' show createAuthenticatedDio;
import '../backend_connection/backend_connection_availability.dart';
import '../backend_connection/providers/backend_connection_providers.dart';
import '../native_bridge/providers/native_bridge_relay_provider.dart';

String _voicePreviewExtension(String url) {
  final lower = Uri.tryParse(url)?.path.toLowerCase() ?? url.toLowerCase();
  for (final extension in const <String>['.mp3', '.wav', '.m4a', '.aac', '.ogg', '.flac']) {
    if (lower.endsWith(extension)) return extension;
  }
  return '.mp3';
}

/// Downloads a backend-authenticated TTS result and delegates playback to the
/// device-native media bridge. Keeping this in one place prevents character
/// voice previews and global cloned-voice previews from drifting apart.
Future<void> playBackendVoicePreview(
  WidgetRef ref,
  String audioUrl, {
  String requestIdPrefix = 'voice-preview',
}) async {
  final url = audioUrl.trim();
  if (url.isEmpty) throw StateError('后端未返回试听音频地址');

  final platform = switch (defaultTargetPlatform) {
    TargetPlatform.android => 'android',
    TargetPlatform.iOS => 'ios',
    TargetPlatform.windows => 'windows',
    _ => null,
  };
  if (kIsWeb || platform == null) {
    throw UnsupportedError('当前平台尚未接入 Flutter 语音本地播放桥');
  }

  final availability = await ref.read(backendConnectionProvider.future);
  if (availability is! BackendConnectionAvailable) {
    throw StateError('后端当前不可用');
  }

  Dio? dio;
  Directory? tempDir;
  var playbackStarted = false;
  try {
    dio = createAuthenticatedDio(availability.config);
    tempDir = await Directory.systemTemp.createTemp('amitia_voice_preview_');
    final tempFile = File('${tempDir.path}/preview_audio${_voicePreviewExtension(url)}');
    await dio.download(url, tempFile.path);
    if (!await tempFile.exists() || await tempFile.length() == 0) {
      throw StateError('试听音频下载失败');
    }

    final dispatcher = ref.read(nativeBridgePlatformDispatcherProvider);
    final nativeResult = await dispatcher.execute(<String, dynamic>{
      'protocolVersion': 1,
      'requestId': '$requestIdPrefix-${DateTime.now().microsecondsSinceEpoch}',
      'platform': platform,
      'operation': 'media.audio.play_file',
      'payload': <String, dynamic>{'path': tempFile.path},
    });
    if (!const <String>{'success', 'ok'}.contains((nativeResult['status'] ?? '').toString())) {
      final error = nativeResult['error'];
      final message = error is Map ? (error['message'] ?? error['code'])?.toString() : null;
      throw StateError(message?.isNotEmpty == true ? message! : '系统音频播放失败');
    }
    playbackStarted = true;

    // Native players open the file before start(). Delay cleanup to avoid
    // retaining stale previews while not racing the player file descriptor.
    final cleanupDir = tempDir;
    Future<void>.delayed(const Duration(seconds: 30), () async {
      try {
        if (await cleanupDir!.exists()) await cleanupDir.delete(recursive: true);
      } catch (_) {}
    });
    tempDir = null;
  } finally {
    dio?.close(force: true);
    if (!playbackStarted && tempDir != null) {
      try {
        if (await tempDir.exists()) await tempDir.delete(recursive: true);
      } catch (_) {}
    }
  }
}

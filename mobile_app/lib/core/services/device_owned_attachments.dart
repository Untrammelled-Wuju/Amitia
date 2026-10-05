import 'dart:convert';
import 'package:crypto/crypto.dart';

Map<String, dynamic> ownedAudioAttachment(
  String uri, {
  String name = 'voice.wav',
}) {
  final match = RegExp(
    r'^data:(audio/(?:wav|webm));base64,([A-Za-z0-9+/]+={0,2})$',
  ).firstMatch(uri);
  if (match == null ||
      match.group(2)!.length > ((1048576 + 2) ~/ 3) * 4 ||
      name.isEmpty ||
      name.length > 256 ||
      name.contains(RegExp(r'[\x00\r\n]')))
    throw StateError('请选择 WAV 或 WebM 音频，单段不超过 1 MiB');
  final bytes = base64Decode(match.group(2)!);
  if (bytes.isEmpty || bytes.length > 1048576) throw StateError('音频大小无效');
  return {
    'kind': 'audio',
    'name': name,
    'mimeType': match.group(1),
    'data': match.group(2),
    'sha256': sha256.convert(bytes).toString(),
  };
}

String? ownedAudioURL(dynamic attachments) {
  if (attachments is! List) return null;
  for (final item in attachments.whereType<Map>()) {
    if (item['kind'] != 'audio' ||
        !const ['audio/wav', 'audio/webm'].contains(item['mimeType']) ||
        item['data'] is! String)
      continue;
    final data = item['data'] as String;
    if (data.length <= ((1048576 + 2) ~/ 3) * 4 &&
        RegExp(r'^[A-Za-z0-9+/]+={0,2}$').hasMatch(data))
      return 'data:${item['mimeType']};base64,$data';
  }
  return null;
}

String ownedMessageText(Map row) {
  return row['transcription'] is String &&
          (row['transcription'] as String).isNotEmpty &&
          row['content'] == row['transcriptionSourceContent']
      ? row['transcription'] as String
      : (row['content'] ?? '').toString();
}

Map<String, dynamic> ownedImageAttachment(
  String uri, {
  String name = 'image.png',
}) {
  final match = RegExp(
    r'^data:(image/(?:png|jpeg|gif));base64,([A-Za-z0-9+/]+={0,2})$',
  ).firstMatch(uri);
  if (match == null || match.group(2)!.length > ((1048576 + 2) ~/ 3) * 4) {
    throw StateError('请选择 PNG、JPEG 或 GIF 图片，单张不超过 1 MiB');
  }
  final bytes = base64Decode(match.group(2)!);
  if (bytes.isEmpty ||
      bytes.length > 1048576 ||
      name.isEmpty ||
      name.length > 256 ||
      name.contains(RegExp(r'[\x00\r\n]'))) {
    throw StateError('图片大小或名称无效');
  }
  return {
    'kind': 'image',
    'name': name,
    'mimeType': match.group(1),
    'data': match.group(2),
    'sha256': sha256.convert(bytes).toString(),
  };
}

String? ownedImageURL(dynamic attachments) {
  if (attachments is! List) return null;
  for (final item in attachments.whereType<Map>()) {
    if (item['kind'] != 'image' ||
        !const [
          'image/png',
          'image/jpeg',
          'image/gif',
        ].contains(item['mimeType']) ||
        item['data'] is! String)
      continue;
    final data = item['data'] as String;
    if (data.length <= ((1048576 + 2) ~/ 3) * 4 &&
        RegExp(r'^[A-Za-z0-9+/]+={0,2}$').hasMatch(data))
      return 'data:${item['mimeType']};base64,$data';
  }
  return null;
}

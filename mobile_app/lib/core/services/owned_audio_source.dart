import 'dart:typed_data';
import 'package:just_audio/just_audio.dart';
import 'device_owned_attachments.dart';
import 'dart:convert';

class OwnedAudioSource extends StreamAudioSource {
  final Uint8List _bytes;
  final String mimeType;

  OwnedAudioSource._(this._bytes, this.mimeType);

  factory OwnedAudioSource.fromDataURI(String uri) {
    final attachment = ownedAudioAttachment(uri);
    return OwnedAudioSource._(
      base64Decode(attachment['data'] as String),
      attachment['mimeType'] as String,
    );
  }

  @override
  Future<StreamAudioResponse> request([int? start, int? end]) async {
    final offset = start ?? 0;
    final limit = end ?? _bytes.length;
    if (offset < 0 || limit < offset || limit > _bytes.length)
      throw RangeError('音频读取范围无效');
    return StreamAudioResponse(
      sourceLength: _bytes.length,
      contentLength: limit - offset,
      offset: offset,
      stream: Stream.value(_bytes.sublist(offset, limit)),
      contentType: mimeType,
    );
  }
}

import 'dart:convert';
import 'package:crypto/crypto.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/services/device_owned_attachments.dart';
import 'package:amitia_app/core/services/owned_audio_source.dart';

void main() {
  test(
    'audio remains in owner bytes and range playback never writes a second file',
    () async {
      const uri = 'data:audio/wav;base64,AQIDBA==';
      final attachment = ownedAudioAttachment(uri);
      expect(attachment['sha256'], sha256.convert([1, 2, 3, 4]).toString());
      expect(ownedAudioURL([attachment]), uri);
      final source = OwnedAudioSource.fromDataURI(uri);
      final range = await source.request(1, 3);
      expect(range.sourceLength, 4);
      expect(range.contentLength, 2);
      expect(range.offset, 1);
      expect(range.contentType, 'audio/wav');
      expect(await range.stream.first, [2, 3]);
      await expectLater(source.request(-1, 2), throwsRangeError);
      await expectLater(source.request(2, 10), throwsRangeError);
      expect(
        ownedMessageText({
          'content': '[语音]',
          'transcriptionSourceContent': '[语音]',
          'transcription': '喝茶',
        }),
        '喝茶',
      );
      expect(
        ownedMessageText({
          'content': '修改后的文字',
          'transcriptionSourceContent': '[语音]',
          'transcription': '喝茶',
        }),
        '修改后的文字',
      );
    },
  );
  test('image bytes have a stable integrity hash and owner message image', () {
    const data =
        'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVQIHWP4z8DwHwAFgAI/ScLttAAAAABJRU5ErkJggg==';
    const uri = 'data:image/png;base64,$data';
    final result = ownedImageAttachment(uri, name: 'picture.png');
    expect(result['sha256'], sha256.convert(base64Decode(data)).toString());
    expect(result['name'], 'picture.png');
    expect(ownedImageURL([result]), uri);
  });
  test(
    'remote references, unsafe types, names and oversized content are rejected',
    () {
      for (final uri in [
        'https://internal.example/private',
        'amitia://artifacts/image',
        'data:image/svg+xml;base64,AAAA',
        'data:image/png;base64,${'A' * 1398108}',
      ]) {
        expect(() => ownedImageAttachment(uri), throwsStateError);
      }
      expect(
        () => ownedImageAttachment(
          'data:image/png;base64,AAAA',
          name: 'image\n.png',
        ),
        throwsStateError,
      );
      expect(
        ownedImageURL([
          {'kind': 'image', 'mimeType': 'image/svg+xml', 'data': 'AAAA'},
        ]),
        isNull,
      );
    },
  );
}

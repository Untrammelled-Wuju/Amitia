import 'dart:convert';
import 'package:crypto/crypto.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/services/device_owned_attachments.dart';
import 'package:amitia_app/core/services/owned_audio_source.dart';

void main() {
  test('file and video bytes retain their owner integrity and metadata', () {
    for (final example in [
      ['file', 'text/plain', 'notes.txt'],
      ['file', 'application/pdf', 'report.pdf'],
      ['file', ownedFileMimes['docx']!, 'report.docx'],
      ['video', 'video/mp4', 'clip.mp4'],
    ]) {
      final uri = 'data:${example[1]};base64,AQIDBA==';
      final item = ownedFileAttachment(uri, name: example[2], kind: example[0]);
      expect(item['sha256'], sha256.convert([1, 2, 3, 4]).toString());
      final restored = ownedFileMetadata([item])!;
      expect(restored['uri'], uri);
      expect(restored['name'], example[2]);
      expect(restored['sizeBytes'], 4);
      expect(
        ownedFileMetadata([
          {...item, 'sha256': 'invalid'},
        ]),
        isNull,
      );
    }
  });
  test(
    'file and video reject remote sources, invalid bytes and unsafe names',
    () {
      for (final uri in [
        'https://internal.example/private',
        'amitia://artifacts/file',
        'data:application/octet-stream;base64,AQID',
        'data:text/plain;base64,AB==',
        'data:text/plain;base64,${'A' * 1398108}',
      ]) {
        expect(
          () => ownedFileAttachment(uri, name: 'file.txt'),
          throwsStateError,
        );
      }
      expect(
        () => ownedFileAttachment(
          'data:text/plain;base64,AQID',
          name: '${'文' * 86}.txt',
        ),
        throwsStateError,
      );
      expect(
        () => ownedFileAttachment(
          'data:video/mp4;base64,AQID',
          name: 'video\n.mp4',
          kind: 'video',
        ),
        throwsStateError,
      );
    },
  );
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

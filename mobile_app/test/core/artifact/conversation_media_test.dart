import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:amitia_app/core/artifact/artifact_service.dart';
import 'package:amitia_app/core/artifact/artifact_providers.dart';
import 'package:amitia_app/core/widgets/conversation_media_image.dart';
import 'package:amitia_app/features/conversation/rendering/preview/html_document.dart';
import 'package:html/parser.dart' as html;

class MediaAdapter implements HttpClientAdapter {
  final List<RequestOptions> requests = [];
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    requests.add(options);
    if (options.path.endsWith('/media-ticket')) {
      return ResponseBody.fromString(
        jsonEncode({'url': '/media/artifacts/a/ticket'}),
        200,
        headers: {
          Headers.contentTypeHeader: ['application/json'],
        },
      );
    }
    return ResponseBody.fromString(
      '<h1>Loaded file</h1>',
      200,
      headers: {
        Headers.contentTypeHeader: ['text/html; charset=utf-8'],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  late MediaAdapter adapter;
  late HttpArtifactService service;
  setUp(() {
    adapter = MediaAdapter();
    final dio = Dio()..httpClientAdapter = adapter;
    service = HttpArtifactService(dio: dio, baseUrl: 'http://core.example');
  });

  test('resource URI receives a media ticket', () async {
    expect(
      (await service.resolveMediaUri('amitia://artifacts/a')).toString(),
      'http://core.example/media/artifacts/a/ticket',
    );
    expect(
      adapter.requests.single.path,
      'http://core.example/api/artifacts/v1/a/media-ticket',
    );
  });

  test('absolute and relative artifact content URLs receive tickets', () async {
    for (final url in [
      'http://core.example/api/artifacts/v1/a/content',
      '/api/artifacts/v1/a/content',
    ]) {
      expect(
        (await service.resolveMediaUri(url)).path,
        '/media/artifacts/a/ticket',
      );
    }
    expect(adapter.requests.length, 2);
  });

  test('external media does not call the authenticated client', () async {
    expect(
      (await service.resolveMediaUri('https://images.example/a.png')).host,
      'images.example',
    );
    expect(adapter.requests, isEmpty);
  });

  test(
    'provider relay reads authenticated bytes instead of local media tickets',
    () async {
      final dio = Dio()..httpClientAdapter = adapter;
      final relay = HttpArtifactService(
        dio: dio,
        baseUrl: 'http://localhost/internal/device-mesh/provider',
      );
      final uri = await relay.resolveMediaUri('amitia://artifacts/a');
      expect(uri.scheme, 'data');
      expect(uri.data!.contentAsString(), '<h1>Loaded file</h1>');
      expect(
        await relay.readText('amitia://artifacts/a'),
        '<h1>Loaded file</h1>',
      );
      expect(
        adapter.requests.first.path,
        'http://localhost/internal/device-mesh/provider/api/artifacts/v1/a/content',
      );
    },
  );

  test('data HTML is decoded without network or query mutation', () async {
    final uri = Uri.dataFromString('<h1>中文</h1>', encoding: utf8).toString();
    expect((await service.resolveDownloadUri(uri)).toString(), uri);
    expect(await service.readText(uri), '<h1>中文</h1>');
  });

  test('full HTML retains head and body with a relative resource base', () async {
    final source = await buildHtmlPreviewDocument(
      '<html><head><title>Report</title><link href="style.css" rel="stylesheet"></head><body><h1>Report</h1><img src="photo.png"></body></html>',
      baseUrl: 'https://site.example/reports/index.html',
    );
    final document = html.parse(source);
    expect(document.querySelectorAll('html'), hasLength(1));
    expect(document.querySelector('title')!.text, 'Report');
    expect(
      document.querySelector('base')!.attributes['href'],
      'https://site.example/reports/index.html',
    );
    expect(source, contains("connect-src 'none'"));
    expect(source, contains('img-src https: http:'));
  });

  test(
    'embedded artifact references are resolved before loading HTML',
    () async {
      final source = await buildHtmlPreviewDocument(
        '<img src="amitia://artifacts/a">',
        resolveMedia: service.resolveMediaUri,
      );
      expect(
        source,
        contains('src="http://core.example/media/artifacts/a/ticket"'),
      );
    },
  );
  testWidgets(
    'conversation image decodes data instead of treating it as an asset',
    (tester) async {
      final png = base64Decode(
        'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/l9sAAAAASUVORK5CYII=',
      );
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            artifactServiceProvider.overrideWith((ref) async => service),
          ],
          child: MaterialApp(
            home: Scaffold(
              body: ConversationMediaImage(
                url: Uri.dataFromBytes(png, mimeType: 'image/png').toString(),
              ),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(
        tester.widget<Image>(find.byType(Image)).image,
        isA<MemoryImage>(),
      );
      expect(tester.takeException(), isNull);
    },
  );
}

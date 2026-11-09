import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_config.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_credential.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_endpoint.dart';
import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/backend_transport/http/backend_http_client.dart';
import 'package:amitia_app/features/desktop_pet/infrastructure/desktop_pet_plugin_api.dart';

class _RecordingServer {
  HttpServer? _server;
  int port = 0;
  final List<_Recorded> requests = [];

  Future<void> start() async {
    _server = await HttpServer.bind('127.0.0.1', 0);
    port = _server!.port;
    _server!.listen(_handle);
  }

  Future<void> _handle(HttpRequest request) async {
    requests.add(
      _Recorded(
        request.uri.path,
        request.method,
        request.uri.queryParameters,
        request.headers.value('X-Amitia-Management-Target'),
        request.headers.value('Idempotency-Key'),
      ),
    );
    request.response.headers.contentType = ContentType.json;
    request.response.statusCode = 200;

    if (request.uri.path == '/api/extensions/pet/plugins') {
      request.response.write(
        jsonEncode({
          'code': 200,
          'message': 'ok',
          'data': {
            'plugins': [
              {
                'extensionId': 'ext-1',
                'pluginId': 'plg-1',
                'name': 'Pet Walker',
              },
            ],
            'total': 1,
            'page': 1,
            'pageSize': 20,
          },
        }),
      );
    } else if (request.uri.path == '/api/extensions/pet/plugins/plg-1' &&
        request.method == 'GET') {
      request.response.write(
        jsonEncode({
          'code': 200,
          'message': 'ok',
          'data': {
            'extensionId': 'ext-1',
            'pluginId': 'plg-1',
            'name': 'Pet Walker',
            'description': '',
            'version': '1.0.0',
            'enabled': true,
            'installState': 'installed',
            'requiredPermissions': ['window'],
          },
        }),
      );
    } else if (request.uri.path == '/api/extensions/pet/plugins/install') {
      request.response.write(
        jsonEncode({
          'code': 200,
          'message': 'ok',
          'data': {
            'extensionId': 'ext-n',
            'version': '1.0.0',
            'installState': 'installed',
          },
        }),
      );
    } else if (request.uri.path.endsWith('/update') &&
        request.method == 'POST') {
      request.response.write(
        jsonEncode({
          'code': 200,
          'message': 'ok',
          'data': {
            'extensionId': 'ext-1',
            'version': '1.1.0',
            'installState': 'installed',
          },
        }),
      );
    } else if (request.uri.path.endsWith('/enable') &&
        request.method == 'POST') {
      request.response.write(
        jsonEncode({
          'code': 200,
          'message': 'ok',
          'data': {'extensionId': 'ext-1', 'success': true},
        }),
      );
    } else if (request.uri.path.endsWith('/disable') &&
        request.method == 'POST') {
      request.response.write(
        jsonEncode({
          'code': 200,
          'message': 'ok',
          'data': {'extensionId': 'ext-1', 'success': true},
        }),
      );
    } else if (request.uri.path.endsWith('/actions/uninstall') &&
        request.method == 'POST') {
      request.response.write(
        jsonEncode({
          'code': 200,
          'message': 'ok',
          'data': {'extensionId': 'ext-1', 'success': true},
        }),
      );
    } else if (request.uri.path.endsWith('/ext-1') &&
        request.method == 'DELETE') {
      request.response.write(
        jsonEncode({
          'code': 200,
          'message': 'ok',
          'data': {'extensionId': 'ext-1', 'success': true},
        }),
      );
    } else if (request.uri.path == '/api/extensions/packages/artifacts') {
      request.response.statusCode = 201;
      request.response.write(
        jsonEncode({
          'artifactId': 'artifact-1',
          'preview': {
            'sessionId': 'preview-1',
            'id': 'com.amitia.qa/desktop-pet-plugin',
            'name': 'QA Desktop Pet',
            'version': '1.0.0',
            'scopeType': 'global',
            'scopeId': '',
            'managementTarget': 'pet_center',
            'contributionKinds': ['petx'],
            'compatible': true,
            'installable': true,
            'signature': {'status': 'unsigned'},
            'highRiskCapabilities': [],
            'capabilityConfirmations': [],
            'warnings': [],
            'errors': [],
            'scripts': 0,
            'fileCount': 3,
            'assets': 1,
          },
        }),
      );
    } else if (request.uri.path ==
        '/api/extensions/packages/previews/preview-1/confirm') {
      request.response.write(jsonEncode({'confirmationToken': 'token-1'}));
    } else if (request.uri.path ==
        '/api/extensions/packages/operations/install') {
      request.response.write(jsonEncode({'operationId': 'op-1'}));
    } else if (request.uri.path == '/api/extensions/packages/operations/op-1') {
      request.response.write(jsonEncode({'status': 'completed'}));
    } else {
      request.response.write(jsonEncode({'code': 200, 'message': 'ok'}));
    }
    await request.response.close();
  }

  Future<void> stop() async {
    await _server?.close(force: true);
    requests.clear();
  }
}

class _Recorded {
  final String path;
  final String method;
  final Map<String, String> query;
  final String? managementTarget;
  final String? idempotencyKey;
  _Recorded(
    this.path,
    this.method,
    this.query,
    this.managementTarget,
    this.idempotencyKey,
  );
}

BackendConnectionConfig _cfg(int p) => BackendConnectionConfig(
  schemaVersion: 1,
  generation: 1,
  endpoint: BackendConnectionEndpoint(
    host: '127.0.0.1',
    port: p,
    httpScheme: 'http',
    webSocketScheme: 'ws',
    livenessPath: '/livez',
    readinessPath: '/readyz',
  ),
  authStrategy: BackendAuthStrategy.localToken,
  credential: BackendConnectionCredential.tryCreate(
    'test_token_32characters_abcdefghij',
  )!,
);

void main() {
  group('DesktopPetPluginApi parse + path', () {
    late _RecordingServer s;
    late DesktopPetPluginApi api;

    setUp(() async {
      s = _RecordingServer();
      await s.start();
      api = DesktopPetPluginApi(
        BackendServiceApi(BackendHttpClient(_cfg(s.port)), 1),
      );
    });

    tearDown(() async {
      await s.stop();
    });

    test('list GET with query params and parses result', () async {
      final result = await api.list();

      expect(result.plugins.length, 1);
      expect(result.total, 1);
      expect(result.plugins.first.extensionId, 'ext-1');
      expect(result.plugins.first.name, 'Pet Walker');

      final req = s.requests.first;
      expect(req.method, 'GET');
      expect(req.path, '/api/extensions/pet/plugins');
      expect(req.query['page'], '1');
      expect(req.query['pageSize'], '20');
    });

    test('list omits search when empty', () async {
      await api.list();
      expect(s.requests.first.query.containsKey('search'), false);
    });

    test('list includes search when provided', () async {
      await api.list(search: 'pet');
      expect(s.requests.first.query['search'], 'pet');
    });

    test('list trims search before sending', () async {
      await api.list(search: '  pet  ');
      expect(s.requests.first.query['search'], 'pet');
    });

    test('detail GET and parses result', () async {
      final d = await api.detail('plg-1');
      expect(d.pluginId, 'plg-1');
      expect(d.requiredPermissions, ['window']);
      expect(s.requests.first.path, '/api/extensions/pet/plugins/plg-1');
    });

    test('install POST path', () async {
      await api.install('/pkgs/foo.zip');
      final req = s.requests.first;
      expect(req.method, 'POST');
      expect(req.path, '/api/extensions/pet/plugins/install');
    });

    test('update POST carries extensionId in the body route', () async {
      final r = await api.update('ext-1', '/pkgs/bar.zip');
      expect(r.version, '1.1.0');
      expect(
        s.requests.first.path,
        '/api/extensions/pet/plugins/actions/update',
      );
    });

    test(
      'petx package uses preview confirmation and package operation',
      () async {
        final directory = await Directory.systemTemp.createTemp('petx-api-');
        addTearDown(() => directory.delete(recursive: true));
        final package = File('${directory.path}/qa.petx')
          ..writeAsBytesSync([1, 2, 3, 4]);

        final preview = await api.previewPackage(package.path);
        final operationId = await api.commitPackagePreview(preview);

        expect(operationId, 'op-1');
        expect(s.requests.map((request) => request.path), [
          '/api/extensions/packages/artifacts',
          '/api/extensions/packages/previews/preview-1/confirm',
          '/api/extensions/packages/operations/install',
          '/api/extensions/packages/operations/op-1',
        ]);
        for (final request in s.requests) {
          expect(request.managementTarget, 'pet-center');
        }
        expect(s.requests[2].idempotencyKey, isNotNull);
      },
    );

    test('enable POST with extensionId', () async {
      final r = await api.enable('ext-1');
      expect(r.success, true);
      expect(
        s.requests.first.path,
        '/api/extensions/pet/plugins/actions/enable',
      );
    });

    test('disable POST with extensionId', () async {
      final r = await api.disable('ext-1');
      expect(r.success, true);
      expect(
        s.requests.first.path,
        '/api/extensions/pet/plugins/actions/disable',
      );
    });

    test('uninstall POST carries extensionId in the body route', () async {
      final r = await api.uninstall('ext-1');
      expect(r.extensionId, 'ext-1');
      expect(r.success, true);
      final req = s.requests.first;
      expect(req.method, 'POST');
      expect(req.path, '/api/extensions/pet/plugins/actions/uninstall');
    });

    test('list throws when data is null', () async {
      await s.stop();
      final nullServer = _NullDataRecordingServer();
      await nullServer.start();
      final nullApi = DesktopPetPluginApi(
        BackendServiceApi(BackendHttpClient(_cfg(nullServer.port)), 1),
      );
      await expectLater(() => nullApi.list(), throwsA(isA<StateError>()));
      await nullServer.stop();
    });

    test('install throws when data is null', () async {
      await s.stop();
      final nullServer = _NullDataRecordingServer();
      await nullServer.start();
      final nullApi = DesktopPetPluginApi(
        BackendServiceApi(BackendHttpClient(_cfg(nullServer.port)), 1),
      );
      await expectLater(
        () => nullApi.install('/pkgs/x.zip'),
        throwsA(isA<StateError>()),
      );
      await nullServer.stop();
    });

    test('extensionId with special chars is URL encoded', () async {
      await s.stop();
      final nullServer = _NullDataRecordingServer();
      await nullServer.start();
      final nullApi = DesktopPetPluginApi(
        BackendServiceApi(BackendHttpClient(_cfg(nullServer.port)), 1),
      );
      await expectLater(
        () => nullApi.detail('plg/1+2'),
        throwsA(isA<StateError>()),
      );
      expect(nullServer.path, '/api/extensions/pet/plugins/plg%2F1%2B2');
      await nullServer.stop();
    });
  });
}

class _NullDataRecordingServer {
  HttpServer? _server;
  int port = 0;
  String path = '';

  Future<void> start() async {
    _server = await HttpServer.bind('127.0.0.1', 0);
    port = _server!.port;
    _server!.listen(_handle);
  }

  Future<void> _handle(HttpRequest request) async {
    path = request.uri.path;
    request.response.headers.contentType = ContentType.json;
    request.response.statusCode = 200;
    request.response.write(
      jsonEncode({'code': 200, 'message': 'ok', 'data': null}),
    );
    await request.response.close();
  }

  Future<void> stop() async {
    await _server?.close(force: true);
    path = '';
  }
}

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
    requests.add(_Recorded(request.uri.path, request.method, request.uri.queryParameters));
    request.response.headers.contentType = ContentType.json;
    request.response.statusCode = 200;

    if (request.uri.path == '/api/extensions/desktop-pet/plugins') {
      request.response.write(jsonEncode({
        'code': 200, 'message': 'ok',
        'data': {
          'plugins': [
            {'extensionId': 'ext-1', 'pluginId': 'plg-1', 'name': 'Pet Walker'},
          ],
          'total': 1, 'page': 1, 'pageSize': 20,
        },
      }));
    } else if (request.uri.path == '/api/extensions/desktop-pet/plugins/plg-1' && request.method == 'GET') {
      request.response.write(jsonEncode({
        'code': 200, 'message': 'ok',
        'data': {
          'extensionId': 'ext-1', 'pluginId': 'plg-1', 'name': 'Pet Walker',
          'description': '', 'version': '1.0.0', 'enabled': true, 'installState': 'installed',
          'requiredPermissions': ['window'],
        },
      }));
    } else if (request.uri.path == '/api/extensions/desktop-pet/plugins/install') {
      request.response.write(jsonEncode({
        'code': 200, 'message': 'ok',
        'data': {'extensionId': 'ext-n', 'version': '1.0.0', 'installState': 'installed'},
      }));
    } else if (request.uri.path.endsWith('/update') && request.method == 'POST') {
      request.response.write(jsonEncode({
        'code': 200, 'message': 'ok',
        'data': {'extensionId': 'ext-1', 'version': '1.1.0', 'installState': 'installed'},
      }));
    } else if (request.uri.path.endsWith('/enable') && request.method == 'POST') {
      request.response.write(jsonEncode({
        'code': 200, 'message': 'ok',
        'data': {'extensionId': 'ext-1', 'success': true},
      }));
    } else if (request.uri.path.endsWith('/disable') && request.method == 'POST') {
      request.response.write(jsonEncode({
        'code': 200, 'message': 'ok',
        'data': {'extensionId': 'ext-1', 'success': true},
      }));
    } else if (request.uri.path.endsWith('/ext-1') && request.method == 'DELETE') {
      request.response.write(jsonEncode({
        'code': 200, 'message': 'ok',
        'data': {'extensionId': 'ext-1', 'success': true},
      }));
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
  _Recorded(this.path, this.method, this.query);
}

BackendConnectionConfig _cfg(int p) => BackendConnectionConfig(
  schemaVersion: 1, generation: 1,
  endpoint: BackendConnectionEndpoint(
    host: '127.0.0.1', port: p, httpScheme: 'http', webSocketScheme: 'ws',
    livenessPath: '/livez', readinessPath: '/readyz',
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
      api = DesktopPetPluginApi(BackendServiceApi(BackendHttpClient(_cfg(s.port)), 1));
    });

    tearDown(() async { await s.stop(); });

    test('list GET with query params and parses result', () async {
      final result = await api.list();

      expect(result.plugins.length, 1);
      expect(result.total, 1);
      expect(result.plugins.first.extensionId, 'ext-1');
      expect(result.plugins.first.name, 'Pet Walker');

      final req = s.requests.first;
      expect(req.method, 'GET');
      expect(req.path, '/api/extensions/desktop-pet/plugins');
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
      expect(s.requests.first.path, '/api/extensions/desktop-pet/plugins/plg-1');
    });

    test('install POST path', () async {
      await api.install('/pkgs/foo.zip');
      final req = s.requests.first;
      expect(req.method, 'POST');
      expect(req.path, '/api/extensions/desktop-pet/plugins/install');
    });

    test('update POST with extensionId in path', () async {
      final r = await api.update('ext-1', '/pkgs/bar.zip');
      expect(r.version, '1.1.0');
      expect(s.requests.first.path, '/api/extensions/desktop-pet/plugins/ext-1/update');
    });

    test('enable POST with extensionId', () async {
      final r = await api.enable('ext-1');
      expect(r.success, true);
      expect(s.requests.first.path, '/api/extensions/desktop-pet/plugins/ext-1/enable');
    });

    test('disable POST with extensionId', () async {
      final r = await api.disable('ext-1');
      expect(r.success, true);
      expect(s.requests.first.path, '/api/extensions/desktop-pet/plugins/ext-1/disable');
    });

    test('uninstall DELETE with extensionId and parses response', () async {
      final r = await api.uninstall('ext-1');
      expect(r.extensionId, 'ext-1');
      expect(r.success, true);
      final req = s.requests.first;
      expect(req.method, 'DELETE');
      expect(req.path, '/api/extensions/desktop-pet/plugins/ext-1');
    });

    test('list throws when data is null', () async {
      await s.stop();
      final nullServer = _NullDataRecordingServer();
      await nullServer.start();
      final nullApi = DesktopPetPluginApi(BackendServiceApi(BackendHttpClient(_cfg(nullServer.port)), 1));
      await expectLater(() => nullApi.list(), throwsA(isA<StateError>()));
      await nullServer.stop();
    });

    test('install throws when data is null', () async {
      await s.stop();
      final nullServer = _NullDataRecordingServer();
      await nullServer.start();
      final nullApi = DesktopPetPluginApi(BackendServiceApi(BackendHttpClient(_cfg(nullServer.port)), 1));
      await expectLater(() => nullApi.install('/pkgs/x.zip'), throwsA(isA<StateError>()));
      await nullServer.stop();
    });

    test('extensionId with special chars is URL encoded', () async {
      await s.stop();
      final nullServer = _NullDataRecordingServer();
      await nullServer.start();
      final nullApi = DesktopPetPluginApi(BackendServiceApi(BackendHttpClient(_cfg(nullServer.port)), 1));
      await expectLater(() => nullApi.detail('plg/1+2'), throwsA(isA<StateError>()));
      expect(nullServer.path, '/api/extensions/desktop-pet/plugins/plg%2F1%2B2');
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
    request.response.write(jsonEncode({
      'code': 200, 'message': 'ok',
      'data': null,
    }));
    await request.response.close();
  }

  Future<void> stop() async {
    await _server?.close(force: true);
    path = '';
  }
}

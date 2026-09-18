import 'dart:convert';

import 'package:amitia_app/core/app_update/app_update_service.dart';
import 'package:dio/dio.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  const channel = MethodChannel('com.amitia.app_update/control');
  late List<MethodCall> calls;

  setUp(() async {
    calls = <MethodCall>[];
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(channel, (call) async {
          calls.add(call);
          switch (call.method) {
            case 'getInstalledInfo':
              return <String, Object>{
                'packageName': 'com.amitia.amitia_app',
                'versionName': '1.0.0',
                'versionCode': 1,
                'canInstallPackages': true,
              };
            case 'verifyManifest':
              return true;
            case 'download':
              return 42;
            case 'install':
              return null;
            case 'getPendingInstallResult':
              return null;
          }
          return null;
        });
  });

  tearDown(() async {
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(channel, null);
  });

  test('checks update through simulated server and native bridge', () async {
    final service = _service();
    final result = await service.check();

    expect(result.installed.versionCode, 1);
    expect(result.available?.versionCode, 2);
    expect(result.available?.versionName, '1.0.1');
    expect(
      calls.map((call) => call.method),
      containsAll(<String>['getInstalledInfo', 'verifyManifest']),
    );
    service.dispose();
  });

  test('downloads and installs through native bridge', () async {
    final service = _service();
    final result = await service.check();
    final downloadId = await service.download(result.available!);
    await service.install(downloadId, result.available!);

    expect(downloadId, 42);
    final downloadCall = calls.firstWhere((call) => call.method == 'download');
    expect(
      (downloadCall.arguments as Map<Object?, Object?>)['url'],
      'https://example.com/app.apk',
    );
    final installCall = calls.firstWhere((call) => call.method == 'install');
    expect((installCall.arguments as Map<Object?, Object?>)['downloadId'], 42);
    service.dispose();
  });
}

AppUpdateService _service() {
  final dio = Dio(BaseOptions())..httpClientAdapter = _UpdateServerAdapter();
  return AppUpdateService(dio: dio, baseUrl: 'https://updates.test/android');
}

class _UpdateServerAdapter implements HttpClientAdapter {
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    if (options.path.endsWith('.sig')) {
      return ResponseBody.fromString(
        'test-signature',
        200,
        headers: <String, List<String>>{
          Headers.contentTypeHeader: <String>['text/plain'],
        },
      );
    }
    return ResponseBody.fromString(
      jsonEncode(<String, dynamic>{
        'schemaVersion': 1,
        'product': 'android',
        'packageName': 'com.amitia.amitia_app',
        'channel': 'stable',
        'versionCode': 2,
        'versionName': '1.0.1',
        'minSupportedVersionCode': 1,
        'mandatory': false,
        'rolloutPercentage': 100,
        'publishedAt': '2026-09-16T00:00:00Z',
        'apk': <String, dynamic>{
          'url': 'https://example.com/app.apk',
          'size': 1234,
          'sha256': 'a' * 64,
          'abi': 'arm64-v8a',
        },
        'releaseNotes': 'test',
      }),
      200,
      headers: <String, List<String>>{
        Headers.contentTypeHeader: <String>['application/json'],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

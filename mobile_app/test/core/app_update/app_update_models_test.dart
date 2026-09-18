import 'package:amitia_app/core/app_update/app_update_models.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('parses signed update manifest and computes mandatory state', () {
    final manifest = AppUpdateManifest.fromMap(<String, dynamic>{
      'schemaVersion': 1,
      'product': 'android',
      'packageName': 'com.amitia.amitia_app',
      'channel': 'stable',
      'versionCode': 12,
      'versionName': '1.0.3',
      'minSupportedVersionCode': 10,
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
    });

    expect(manifest.versionCode, 12);
    expect(manifest.apk.abi, 'arm64-v8a');
    expect(manifest.requiresVersion(9), isTrue);
    expect(manifest.requiresVersion(10), isFalse);
  });

  test('parses install result and download progress', () {
    final status = AppUpdateDownloadStatus.fromMap(<Object?, Object?>{
      'statusCode': 8,
      'status': 'successful',
      'downloadedBytes': 50,
      'totalBytes': 100,
    });
    final result = AppUpdateInstallResult.fromMap(<Object?, Object?>{
      'statusCode': 0,
      'status': 'success',
      'message': '',
      'sessionId': 7,
      'timestamp': 1000,
    });

    expect(status.successful, isTrue);
    expect(status.progress, 0.5);
    expect(result.successful, isTrue);
  });
}

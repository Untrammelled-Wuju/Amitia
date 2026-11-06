import 'package:amitia_app/app/theme/app_theme.dart';
import 'package:amitia_app/core/app_update/app_update_models.dart';
import 'package:amitia_app/core/app_update/app_update_prompt_sheet.dart';
import 'package:amitia_app/core/app_update/app_update_service.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('renders admin update title highlights and markdown content', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(390, 844));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    final service = _NoCurrentDownloadUpdateService();
    final update = _manifest();
    final controller = AppUpdateDownloadController(
      service: service,
      update: update,
    );
    addTearDown(() {
      controller.dispose();
      service.dispose();
    });

    await tester.pumpWidget(
      MaterialApp(
        theme: AppTheme.lightTheme(),
        home: Scaffold(
          body: AppUpdatePromptSheet(
            update: update,
            currentVersion: '26.2.0-beta.19',
            controller: controller,
            mandatory: false,
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('发现新版本'), findsOneWidget);
    expect(find.text('Amitia 可以更新了！'), findsOneWidget);
    expect(find.text('当前 v26.2.0-beta.19'), findsNothing);
    final currentVersionStyle = tester
        .widget<Text>(find.byKey(const ValueKey('app-update-current-version')))
        .style;
    final latestVersionStyle = tester
        .widget<Text>(find.byKey(const ValueKey('app-update-latest-version')))
        .style;
    expect(latestVersionStyle?.color, currentVersionStyle?.color);
    expect(find.text('本次更新'), findsOneWidget);
    expect(find.text('完整更新内容'), findsOneWidget);
    expect(find.text('本版亮点'), findsOneWidget);
    expect(find.text('新增本地模型库'), findsWidgets);
    expect(find.text('主要更新'), findsOneWidget);
    expect(find.text('下载并安装'), findsOneWidget);
    expect(find.text('稍后提醒'), findsOneWidget);
    expect(find.text('跳过此版本'), findsOneWidget);
  });

  test('download controller continues after the sheet is dismissed', () async {
    final service = _SuccessfulUpdateService();
    final update = _manifest();
    final controller = AppUpdateDownloadController(
      service: service,
      update: update,
    );
    addTearDown(controller.dispose);

    await controller.start();
    final completed = await controller.terminal;

    expect(completed, isTrue);
    expect(controller.completed, isTrue);
    expect(controller.downloadId, 7);
    expect(controller.progress, 1);
  });

  test('download controller restores the active system download', () async {
    final service = _RestoredUpdateService();
    final update = _manifest();
    final controller = AppUpdateDownloadController(
      service: service,
      update: update,
    );
    addTearDown(controller.dispose);

    await controller.restore();

    expect(controller.completed, isTrue);
    expect(controller.downloadId, 9);
    expect(controller.progress, 1);
  });
}

AppUpdateManifest _manifest() {
  return AppUpdateManifest.fromMap(<String, dynamic>{
    'schemaVersion': 1,
    'product': 'android',
    'packageName': 'com.amitia.amitia_app',
    'channel': 'stable',
    'versionCode': 20,
    'versionName': '26.2.0-beta.20',
    'minSupportedVersionCode': 0,
    'mandatory': false,
    'rolloutPercentage': 100,
    'publishedAt': '2026-11-02T00:00:00Z',
    'apk': <String, dynamic>{
      'url': 'https://example.com/app.apk',
      'size': 1024,
      'sha256': 'a' * 64,
      'abi': 'arm64-v8a',
    },
    'releaseNotes': '## 主要更新\n\n- 新增本地模型库',
    'updateTitle': '本次更新集中完善更新体验',
    'highlights': <String>['新增本地模型库', '完善后台下载'],
  });
}

class _SuccessfulUpdateService extends AppUpdateService {
  _SuccessfulUpdateService()
    : super(dio: Dio(), baseUrl: 'https://updates.test');

  @override
  Future<int> download(AppUpdateManifest manifest) async => 7;

  @override
  Future<AppUpdateDownloadStatus?> getCurrentDownload(
    AppUpdateManifest manifest,
  ) async => null;

  @override
  Future<AppUpdateDownloadStatus> getDownloadStatus(int downloadId) async {
    return const AppUpdateDownloadStatus(
      downloadId: 7,
      statusCode: 8,
      status: 'successful',
      downloadedBytes: 1024,
      totalBytes: 1024,
      reasonCode: 0,
      reason: '',
      localUri: '/tmp/update.apk',
      message: '',
    );
  }
}

class _NoCurrentDownloadUpdateService extends AppUpdateService {
  _NoCurrentDownloadUpdateService()
    : super(dio: Dio(), baseUrl: 'https://updates.test');

  @override
  Future<AppUpdateDownloadStatus?> getCurrentDownload(
    AppUpdateManifest manifest,
  ) async => null;
}

class _RestoredUpdateService extends AppUpdateService {
  _RestoredUpdateService() : super(dio: Dio(), baseUrl: 'https://updates.test');

  @override
  Future<AppUpdateDownloadStatus?> getCurrentDownload(
    AppUpdateManifest manifest,
  ) async {
    return AppUpdateDownloadStatus.fromMap(<Object?, Object?>{
      'downloadId': 9,
      'statusCode': 8,
      'status': 'successful',
      'downloadedBytes': 1024,
      'totalBytes': 1024,
      'reasonCode': 0,
      'reason': '',
      'localUri': 'content://downloads/9',
      'message': '',
    });
  }
}

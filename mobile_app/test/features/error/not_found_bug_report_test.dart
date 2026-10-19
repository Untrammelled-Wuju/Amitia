import 'package:amitia_app/app/app_routes.dart';
import 'package:amitia_app/core/app_update/app_update_models.dart';
import 'package:amitia_app/core/app_update/app_update_providers.dart';
import 'package:amitia_app/core/app_update/app_update_service.dart';
import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/backend_transport/http/backend_http_request.dart';
import 'package:amitia_app/core/backend_transport/http/backend_http_response.dart';
import 'package:amitia_app/core/backend_transport/http/backend_http_transport.dart';
import 'package:amitia_app/core/backend_transport/state/backend_http_state.dart';
import 'package:amitia_app/core/services/providers.dart';
import 'package:amitia_app/core/services/user_feedback_service.dart';
import 'package:amitia_app/core/widgets/amitia_button.dart';
import 'package:amitia_app/features/error/presentation/pages/not_found_page.dart';
import 'package:amitia_app/features/settings/presentation/widgets/bug_report_dialog.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:shared_preferences/shared_preferences.dart';

final class _NoopTransport implements BackendHttpTransport {
  @override
  BackendHttpState get state => BackendHttpState.idle;

  @override
  Future<void> close() async {}

  @override
  Future<BackendHttpResponse> send(BackendHttpRequest request) async {
    return BackendHttpResponse(statusCode: 200, headers: const <String, String>{}, data: <String, dynamic>{});
  }
}

final class _FakeFeedbackService extends UserFeedbackService {
  _FakeFeedbackService() : super(BackendServiceApi(_NoopTransport(), 1));

  Map<String, dynamic>? submitted;

  @override
  Future<Map<String, dynamic>> settings() async {
    return <String, dynamic>{
      'uploadUrl': 'https://amitia.example.com/api/v1/feedbacks',
      'defaultUploadUrl': 'https://amitia.example.com/api/v1/feedbacks',
      'logLineLimit': 120,
      'errorLineLimit': 40,
      'maxScreenshotMB': 6,
    };
  }

  @override
  Future<Map<String, dynamic>> submit(Map<String, dynamic> payload) async {
    submitted = payload;
    return const <String, dynamic>{};
  }

  @override
  Future<Map<String, dynamic>> preview(Map<String, dynamic> payload) async {
    return <String, dynamic>{'text': '预览内容', 'payload': payload};
  }
}

final class _FakeAppUpdateService extends AppUpdateService {
  @override
  Future<InstalledAppInfo> getInstalledInfo() async {
    return const InstalledAppInfo(
      packageName: 'com.amitia.amitia_app',
      versionName: '26.2.0-beta.7',
      versionCode: 11,
      canInstallPackages: true,
    );
  }
}

void main() {
  setUp(() {
    SharedPreferences.setMockInitialValues(<String, Object>{});
  });

  testWidgets('404 页面提供提交报错信息入口', (tester) async {
    final router = GoRouter(
      initialLocation: '/404',
      routes: [
        GoRoute(path: AppRoutes.chat, builder: (context, state) => const Scaffold(body: Text('chat'))),
        GoRoute(path: '/404', builder: (context, state) => const NotFoundPage(attemptedPath: '/missing-page')),
      ],
    );

    await tester.pumpWidget(
      ProviderScope(
        child: MaterialApp.router(routerConfig: router),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('提交报错信息'), findsOneWidget);
  });

  testWidgets('404 报错弹窗只保留功能异常反馈', (tester) async {
    final service = _FakeFeedbackService();
    bool? submitted;

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          userFeedbackServiceProvider.overrideWithValue(service),
          appUpdateServiceProvider.overrideWithValue(_FakeAppUpdateService()),
        ],
        child: MaterialApp(
          home: Builder(
            builder: (context) => Scaffold(
              body: Center(
                child: ElevatedButton(
                  onPressed: () async {
                    submitted = await showBugReportDialog(context, attemptedPath: '/missing-page');
                  },
                  child: const Text('打开报错弹窗'),
                ),
              ),
            ),
          ),
        ),
      ),
    );

    await tester.tap(find.text('打开报错弹窗'));
    await tester.pumpAndSettle();

    expect(find.text('反馈类型'), findsOneWidget);
    expect(find.text('功能异常'), findsOneWidget);
    expect(find.text('上传报错日志文件'), findsOneWidget);
    expect(find.text('当前页面：/missing-page'), findsOneWidget);
    expect(find.text('体验问题'), findsNothing);
    expect(find.text('功能建议'), findsNothing);

    await tester.tap(find.widgetWithText(AmitiaButton, '取消'));
    await tester.pumpAndSettle();
    expect(submitted, isFalse);
  });

  testWidgets('404 报错弹窗提交时固定使用 bug 类型', (tester) async {
    final service = _FakeFeedbackService();

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          userFeedbackServiceProvider.overrideWithValue(service),
          appUpdateServiceProvider.overrideWithValue(_FakeAppUpdateService()),
        ],
        child: MaterialApp(
          home: Builder(
            builder: (context) => Scaffold(
              body: Center(
                child: ElevatedButton(
                  onPressed: () => showBugReportDialog(context, attemptedPath: '/missing-page'),
                  child: const Text('打开报错弹窗'),
                ),
              ),
            ),
          ),
        ),
      ),
    );

    await tester.tap(find.text('打开报错弹窗'));
    await tester.pumpAndSettle();
    await tester.enterText(find.byType(TextField).at(0), '404 页面无法打开');
    await tester.enterText(find.byType(TextField).at(1), '进入不存在的页面后只看到 404');
    await tester.pump();
    await tester.tap(find.widgetWithText(AmitiaButton, '提交报错信息'));
    await tester.runAsync(() => Future<void>.delayed(const Duration(milliseconds: 100)));
    await tester.pumpAndSettle();

    expect(service.submitted?['category'], 'bug');
    expect(service.submitted?['title'], '404 页面无法打开');
    expect(service.submitted?['description'], '进入不存在的页面后只看到 404');
    expect(service.submitted?['uploadUrl'], 'https://amitia.example.com/api/v1/feedbacks');
  });
}

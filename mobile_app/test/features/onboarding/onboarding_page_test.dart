import 'package:amitia_app/core/services/onboarding_completion_store.dart';
import 'package:amitia_app/core/services/providers.dart';
import 'package:amitia_app/features/onboarding/presentation/pages/onboarding_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  Future<void> pumpOnboarding(
    WidgetTester tester, {
    String gateError = '',
  }) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: <Override>[
          onboardingGateProvider.overrideWith(
            (ref) async =>
                OnboardingGateResult(completed: false, errorText: gateError),
          ),
        ],
        child: const MaterialApp(home: OnboardingPage()),
      ),
    );
    await tester.pumpAndSettle();
  }

  group('OnboardingPage', () {
    testWidgets('首次进入先展示隐私与使用边界确认门', (tester) async {
      SharedPreferences.setMockInitialValues({});
      await pumpOnboarding(tester);

      expect(find.text('欢迎使用阿米提亚'), findsOneWidget);
      expect(find.text('数据由你自己掌控'), findsOneWidget);
      expect(find.text('模型 API 数据提醒'), findsOneWidget);
      expect(find.text('AI 是虚拟角色'), findsOneWidget);
      expect(find.text('你可以随时删除数据'), findsOneWidget);
      expect(find.text('查看完整隐私说明与使用边界'), findsOneWidget);
      expect(find.text('开始使用'), findsOneWidget);
      expect(find.text('步骤 1/3'), findsNothing);
    });

    testWidgets('确认边界后进入三步引导的第一步', (tester) async {
      SharedPreferences.setMockInitialValues({});
      await pumpOnboarding(tester);

      await tester.tap(find.text('我已阅读并同意隐私说明和使用边界'));
      await tester.pump();
      await tester.tap(find.text('开始使用'));
      await tester.pumpAndSettle();

      expect(find.text('步骤 1/3'), findsOneWidget);
      expect(find.text('统筹中心'), findsWidgets);
      expect(find.text('连接统筹中心'), findsOneWidget);
      expect(find.text('以本机作为统筹中心'), findsOneWidget);
      expect(find.text('使用远程统筹中心'), findsOneWidget);
      expect(find.text('下一步'), findsOneWidget);
      expect(find.text('上一步'), findsNothing);
    });

    testWidgets('选择远程统筹中心后展示地址输入与检测连接', (tester) async {
      SharedPreferences.setMockInitialValues({});
      await pumpOnboarding(tester);

      await tester.tap(find.text('我已阅读并同意隐私说明和使用边界'));
      await tester.pump();
      await tester.tap(find.text('开始使用'));
      await tester.pumpAndSettle();

      await tester.tap(find.text('使用远程统筹中心'));
      await tester.pumpAndSettle();

      expect(find.text('统筹中心地址'), findsOneWidget);
      expect(find.text('检测连接'), findsOneWidget);
    });

    testWidgets('已确认过边界时直接进入引导步骤', (tester) async {
      SharedPreferences.setMockInitialValues({
        'onboarding_consent_accepted': true,
      });
      await pumpOnboarding(tester);

      expect(find.text('步骤 1/3'), findsOneWidget);
      expect(find.text('欢迎使用阿米提亚'), findsNothing);
    });

    testWidgets('引导状态读取失败时展示原始报错', (tester) async {
      SharedPreferences.setMockInitialValues({
        'onboarding_consent_accepted': true,
      });
      await pumpOnboarding(tester, gateError: 'connection refused: 127.0.0.1:18899');

      expect(find.text('步骤 1/3'), findsOneWidget);
      expect(
        find.textContaining('connection refused: 127.0.0.1:18899'),
        findsOneWidget,
      );
    });
  });

  group('onboardingGateProvider', () {
    test('未写入完成标记时引导未完成', () async {
      SharedPreferences.setMockInitialValues({});
      final container = ProviderContainer();
      addTearDown(container.dispose);

      final gate = await container.read(onboardingGateProvider.future);

      expect(gate.completed, isFalse);
      expect(gate.errorText, isEmpty);
    });

    test('写入完成标记后引导已完成', () async {
      SharedPreferences.setMockInitialValues({
        onboardingCompletedPreferenceKey: true,
      });
      final container = ProviderContainer();
      addTearDown(container.dispose);

      final gate = await container.read(onboardingGateProvider.future);

      expect(gate.completed, isTrue);
    });

    test('markOnboardingCompleted 写入持久化标记', () async {
      SharedPreferences.setMockInitialValues({});

      await markOnboardingCompleted();

      expect(await readOnboardingCompleted(), isTrue);
      expect(
        SharedPreferences.getInstance().then(
          (preferences) =>
              preferences.getBool(onboardingCompletedPreferenceKey),
        ),
        completion(isTrue),
      );
    });
  });
}

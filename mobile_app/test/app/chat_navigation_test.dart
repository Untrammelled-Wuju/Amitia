import 'package:amitia_app/app/app.dart';
import 'package:amitia_app/core/models/character.dart';
import 'package:amitia_app/core/services/providers.dart';
import 'package:amitia_app/core/widgets/amitia_drawer.dart';
import 'package:amitia_app/features/agent/presentation/pages/agent_page.dart';
import 'package:amitia_app/features/chat/presentation/pages/chat_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

ProviderContainer _createTestContainer() {
  final container = ProviderContainer(
    overrides: [
      characterListProvider.overrideWith((ref) async {
        return [
          CharacterDto(
            id: 'c1',
            name: 'TestCharacter',
            avatar: '',
            identity: '',
            personality: '',
            speakingStyle: '',
            description: '',
            status: '',
            isActive: 1,
            createdAt: '',
          ),
        ];
      }),
    ],
  );
  return container;
}

void main() {
  testWidgets(
    'chat composer uses one scaffold inset owner without a final jump',
    (tester) async {
      tester.view.devicePixelRatio = 1;
      tester.view.physicalSize = const Size(400, 800);
      tester.view.viewPadding = const FakeViewPadding(bottom: 24);
      tester.view.padding = const FakeViewPadding(bottom: 24);
      tester.view.viewInsets = FakeViewPadding.zero;
      addTearDown(tester.view.resetDevicePixelRatio);
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetViewPadding);
      addTearDown(tester.view.resetPadding);
      addTearDown(tester.view.resetViewInsets);

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            startupStageProvider.overrideWith((ref) async => 'ready'),
            characterListProvider.overrideWith((ref) async {
              return [
                CharacterDto(
                  id: 'c1',
                  name: 'TestCharacter',
                  avatar: '',
                  identity: '',
                  personality: '',
                  speakingStyle: '',
                  description: '',
                  status: '',
                  isActive: 1,
                  createdAt: '',
                ),
              ];
            }),
          ],
          child: const AmitiaApp(),
        ),
      );
      await tester.pumpAndSettle();

      final composerSurface = find.byKey(
        const ValueKey('chat-composer-surface'),
      );
      final scaffolds = tester.widgetList<Scaffold>(find.byType(Scaffold));
      expect(
        scaffolds.where((widget) => widget.resizeToAvoidBottomInset == true),
        hasLength(1),
      );
      expect(
        scaffolds.where((widget) => widget.resizeToAvoidBottomInset == false),
        hasLength(1),
      );
      expect(tester.getBottomRight(composerSurface).dy, closeTo(764, 0.01));

      tester.view.viewInsets = const FakeViewPadding(bottom: 8);
      tester.view.padding = const FakeViewPadding(bottom: 16);
      await tester.pump();
      expect(tester.getBottomRight(composerSurface).dy, closeTo(764, 0.01));

      tester.view.viewInsets = const FakeViewPadding(bottom: 20);
      tester.view.padding = const FakeViewPadding(bottom: 4);
      await tester.pump();
      expect(tester.getBottomRight(composerSurface).dy, closeTo(764, 0.01));

      tester.view.viewInsets = const FakeViewPadding(bottom: 60);
      tester.view.padding = FakeViewPadding.zero;
      await tester.pump();
      expect(tester.getBottomRight(composerSurface).dy, closeTo(728, 0.01));

      tester.view.viewInsets = const FakeViewPadding(bottom: 120);
      await tester.pump();
      expect(tester.getBottomRight(composerSurface).dy, closeTo(668, 0.01));

      tester.view.viewInsets = const FakeViewPadding(bottom: 260);
      await tester.pump();
      expect(tester.getBottomRight(composerSurface).dy, closeTo(528, 0.01));

      await tester.pump(const Duration(milliseconds: 400));
      expect(tester.getBottomRight(composerSurface).dy, closeTo(528, 0.01));

      tester.view.viewInsets = const FakeViewPadding(bottom: 120);
      await tester.pump();
      expect(tester.getBottomRight(composerSurface).dy, closeTo(668, 0.01));

      tester.view.viewInsets = const FakeViewPadding(bottom: 20);
      tester.view.padding = const FakeViewPadding(bottom: 4);
      await tester.pump();
      expect(tester.getBottomRight(composerSurface).dy, closeTo(764, 0.01));

      tester.view.viewInsets = FakeViewPadding.zero;
      tester.view.padding = const FakeViewPadding(bottom: 24);
      await tester.pump();
      expect(tester.getBottomRight(composerSurface).dy, closeTo(764, 0.01));

      await tester.pump(const Duration(milliseconds: 300));
      expect(tester.getBottomRight(composerSurface).dy, closeTo(764, 0.01));
    },
  );

  testWidgets('drawer destination overlays chat and back reveals chat root', (
    tester,
  ) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          startupStageProvider.overrideWith((ref) async => 'ready'),
          characterListProvider.overrideWith((ref) async {
            return [
              CharacterDto(
                id: 'c1',
                name: 'TestCharacter',
                avatar: '',
                identity: '',
                personality: '',
                speakingStyle: '',
                description: '',
                status: '',
                isActive: 1,
                createdAt: '',
              ),
            ];
          }),
        ],
        child: const AmitiaApp(),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byType(ChatPage), findsOneWidget);

    await tester.tap(find.byIcon(Icons.menu_rounded));
    await tester.pumpAndSettle();
    await tester.tap(find.text('任务'));
    await tester.pumpAndSettle();

    expect(find.byType(AgentPage), findsOneWidget);
    expect(find.byIcon(Icons.menu_rounded), findsNothing);
    expect(find.byType(AmitiaDrawer), findsNothing);

    final exitingRoute = ModalRoute.of(tester.element(find.byType(AgentPage)))!;
    final targetRoute = ModalRoute.of(
      tester.element(find.byType(ChatPage, skipOffstage: false)),
    )!;

    await tester.tap(find.byIcon(Icons.arrow_back_ios_new));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 110));

    expect(exitingRoute.animation!.value, inExclusiveRange(0, 1));
    expect(targetRoute.secondaryAnimation!.value, inExclusiveRange(0, 1));
    expect(tester.getTopLeft(find.byType(AgentPage)).dx, greaterThan(0));
    expect(
      tester.getTopLeft(find.byType(ChatPage, skipOffstage: false)).dx,
      lessThan(0),
    );

    await tester.pumpAndSettle();

    expect(find.byType(ChatPage), findsOneWidget);
    expect(find.byType(AgentPage), findsNothing);
    expect(find.byIcon(Icons.menu_rounded), findsOneWidget);
    final shellScaffold = find.byWidgetPredicate(
      (widget) => widget is Scaffold && widget.drawer is AmitiaDrawer,
    );
    expect(shellScaffold, findsOneWidget);
    expect(tester.state<ScaffoldState>(shellScaffold).isDrawerOpen, isFalse);
  });

  testWidgets('system back from drawer destination reveals chat root', (
    tester,
  ) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          startupStageProvider.overrideWith((ref) async => 'ready'),
          characterListProvider.overrideWith((ref) async {
            return [
              CharacterDto(
                id: 'c1',
                name: 'TestCharacter',
                avatar: '',
                identity: '',
                personality: '',
                speakingStyle: '',
                description: '',
                status: '',
                isActive: 1,
                createdAt: '',
              ),
            ];
          }),
        ],
        child: const AmitiaApp(),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byIcon(Icons.menu_rounded));
    await tester.pumpAndSettle();
    await tester.tap(find.text('任务'));
    await tester.pumpAndSettle();

    expect(find.byType(AgentPage), findsOneWidget);

    await tester.binding.handlePopRoute();
    await tester.pumpAndSettle();

    expect(find.byType(ChatPage), findsOneWidget);
    expect(find.byType(AgentPage), findsNothing);
  });

  testWidgets('chat root exits only after two consecutive system backs', (
    tester,
  ) async {
    var exitCalls = 0;
    tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
      SystemChannels.platform,
      (call) async {
        if (call.method == 'SystemNavigator.pop') {
          exitCalls++;
        }
        return null;
      },
    );
    addTearDown(() {
      tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
        SystemChannels.platform,
        null,
      );
    });

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          startupStageProvider.overrideWith((ref) async => 'ready'),
          characterListProvider.overrideWith((ref) async {
            return [
              CharacterDto(
                id: 'c1',
                name: 'TestCharacter',
                avatar: '',
                identity: '',
                personality: '',
                speakingStyle: '',
                description: '',
                status: '',
                isActive: 1,
                createdAt: '',
              ),
            ];
          }),
        ],
        child: const AmitiaApp(),
      ),
    );
    await tester.pumpAndSettle();

    await tester.binding.handlePopRoute();
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.byType(ChatPage), findsOneWidget);
    expect(exitCalls, 0);
    expect(find.text('再按一次退出应用'), findsOneWidget);

    await tester.binding.handlePopRoute();
    await tester.pump();

    expect(exitCalls, 1);
  });
}

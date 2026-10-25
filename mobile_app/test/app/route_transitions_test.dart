import 'package:amitia_app/app/amitia_page_route.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

Widget _app(
  GlobalKey<NavigatorState> navigatorKey, {
  bool disableAnimations = false,
  TargetPlatform platform = TargetPlatform.iOS,
}) {
  return MaterialApp(
    theme: ThemeData(platform: platform),
    builder: (context, child) {
      if (!disableAnimations) return child!;
      return MediaQuery(
        data: MediaQuery.of(context).copyWith(disableAnimations: true),
        child: child!,
      );
    },
    home: Navigator(
      key: navigatorKey,
      onGenerateRoute: (settings) => amitiaSlideRoute<void>(
        settings: settings,
        rootLevel: true,
        builder: (_) => const Scaffold(body: Center(child: Text('root'))),
      ),
    ),
  );
}

AmitiaSlideRoute<void> _detailRoute() {
  return amitiaSlideRoute<void>(
    builder: (_) => const Scaffold(body: Center(child: Text('detail'))),
  );
}

Finder _detailSlideFinder() {
  return find
      .ancestor(
        of: find.text('detail', skipOffstage: false),
        matching: find.byType(SlideTransition, skipOffstage: false),
      )
      .first;
}

SlideTransition _detailSlide(WidgetTester tester) {
  return tester.widget<SlideTransition>(_detailSlideFinder());
}

SlideTransition _rootSlide(WidgetTester tester) {
  return tester.widget<SlideTransition>(
    find
        .ancestor(
          of: find.text('root', skipOffstage: false),
          matching: find.byType(SlideTransition, skipOffstage: false),
        )
        .first,
  );
}

void main() {
  testWidgets('推入二级页时根页等距左移', (tester) async {
    final navigatorKey = GlobalKey<NavigatorState>();
    await tester.pumpWidget(_app(navigatorKey));
    await tester.pumpAndSettle();

    expect(_rootSlide(tester).position.value.dx, 0);

    navigatorKey.currentState!.push(_detailRoute());
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 120));

    final during = _rootSlide(tester).position.value.dx;
    expect(during, lessThan(0));
    expect(during, greaterThanOrEqualTo(-amitiaSlideRatio));

    await tester.pumpAndSettle();
    expect(
      _rootSlide(tester).position.value.dx,
      moreOrLessEquals(-amitiaSlideRatio, epsilon: 0.002),
    );
    expect(find.text('detail'), findsOneWidget);
  });

  testWidgets('返回时根页从左侧回到原位', (tester) async {
    final navigatorKey = GlobalKey<NavigatorState>();
    await tester.pumpWidget(_app(navigatorKey));
    await tester.pumpAndSettle();

    navigatorKey.currentState!.push(_detailRoute());
    await tester.pumpAndSettle();
    expect(
      _rootSlide(tester).position.value.dx,
      moreOrLessEquals(-amitiaSlideRatio, epsilon: 0.002),
    );

    navigatorKey.currentState!.pop();
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 120));

    final during = _rootSlide(tester).position.value.dx;
    expect(during, greaterThan(-amitiaSlideRatio));
    expect(during, lessThanOrEqualTo(0));

    await tester.pumpAndSettle();
    expect(
      _rootSlide(tester).position.value.dx,
      moreOrLessEquals(0, epsilon: 0.002),
    );
    expect(find.text('detail'), findsNothing);
  });

  testWidgets('左边缘右滑手势可返回上一页', (tester) async {
    final navigatorKey = GlobalKey<NavigatorState>();
    await tester.pumpWidget(_app(navigatorKey));
    await tester.pumpAndSettle();

    navigatorKey.currentState!.push(_detailRoute());
    await tester.pumpAndSettle();

    final gesture = await tester.startGesture(const Offset(6, 300));
    await gesture.moveBy(const Offset(520, 0));
    await tester.pump();
    expect(
      _rootSlide(tester).position.value.dx,
      greaterThan(-amitiaSlideRatio),
    );

    await gesture.up();
    await tester.pumpAndSettle();

    expect(find.text('detail'), findsNothing);
    expect(
      _rootSlide(tester).position.value.dx,
      moreOrLessEquals(0, epsilon: 0.002),
    );
  });

  testWidgets('拖动距离不足时手势回弹，不返回', (tester) async {
    final navigatorKey = GlobalKey<NavigatorState>();
    await tester.pumpWidget(_app(navigatorKey));
    await tester.pumpAndSettle();

    navigatorKey.currentState!.push(_detailRoute());
    await tester.pumpAndSettle();

    final gesture = await tester.startGesture(const Offset(6, 300));
    await gesture.moveBy(const Offset(90, 0));
    await tester.pump();
    await gesture.up();
    await tester.pumpAndSettle();

    expect(find.text('detail'), findsOneWidget);
    expect(
      _rootSlide(tester).position.value.dx,
      moreOrLessEquals(-amitiaSlideRatio, epsilon: 0.002),
    );
  });

  testWidgets('页面中部起手不触发返回手势', (tester) async {
    final navigatorKey = GlobalKey<NavigatorState>();
    await tester.pumpWidget(_app(navigatorKey));
    await tester.pumpAndSettle();

    navigatorKey.currentState!.push(_detailRoute());
    await tester.pumpAndSettle();

    final gesture = await tester.startGesture(const Offset(400, 300));
    await gesture.moveBy(const Offset(520, 0));
    await tester.pump();
    await gesture.up();
    await tester.pumpAndSettle();

    expect(find.text('detail'), findsOneWidget);
  });

  test('根层页面自身不播放进入动画', () {
    final root = amitiaSlideRoute<void>(
      rootLevel: true,
      builder: (_) => const SizedBox(),
    );
    final pushed = amitiaSlideRoute<void>(builder: (_) => const SizedBox());
    expect(root.transitionDuration, Duration.zero);
    expect(pushed.transitionDuration, greaterThan(Duration.zero));
  });

  testWidgets('底页与顶页共享同一进度，位移比例恒定', (tester) async {
    final navigatorKey = GlobalKey<NavigatorState>();
    await tester.pumpWidget(_app(navigatorKey));
    await tester.pumpAndSettle();

    final detail = _detailRoute();
    navigatorKey.currentState!.push(detail);
    await tester.pumpAndSettle();

    detail.handleStartBackGesture(progress: 1);
    await tester.pump();

    var previousShared = 1.0;
    for (final progress in <double>[0.8, 0.6, 0.4, 0.2]) {
      detail.handleUpdateBackGestureProgress(progress: progress);
      await tester.pump();
      final topDx = _detailSlide(tester).position.value.dx;
      final rootDx = _rootSlide(tester).position.value.dx;
      final shared = 1 - topDx;
      expect(shared, lessThan(previousShared));
      expect(
        rootDx / shared,
        moreOrLessEquals(-amitiaSlideRatio, epsilon: 0.005),
      );
      previousShared = shared;
    }
    expect(previousShared, lessThan(0.8));
  });

  testWidgets('预测返回取消后回弹到位且不返回', (tester) async {
    final navigatorKey = GlobalKey<NavigatorState>();
    await tester.pumpWidget(
      _app(navigatorKey, platform: TargetPlatform.android),
    );
    await tester.pumpAndSettle();

    final detail = _detailRoute();
    navigatorKey.currentState!.push(detail);
    await tester.pumpAndSettle();

    detail.handleStartBackGesture(progress: 1);
    await tester.pump();
    detail.handleUpdateBackGestureProgress(progress: 0.4);
    await tester.pump();

    expect(detail.backGestureActive, isTrue);
    expect(_detailSlide(tester).position.value.dx, greaterThan(0.05));
    expect(
      _rootSlide(tester).position.value.dx,
      greaterThan(-amitiaSlideRatio),
    );

    detail.handleCancelBackGesture();
    await tester.pumpAndSettle();

    expect(detail.backGestureActive, isFalse);
    expect(find.text('detail'), findsOneWidget);
    expect(
      _detailSlide(tester).position.value.dx,
      moreOrLessEquals(0, epsilon: 0.002),
    );
    expect(
      _rootSlide(tester).position.value.dx,
      moreOrLessEquals(-amitiaSlideRatio, epsilon: 0.002),
    );
  });

  testWidgets('预测返回提交后退出当前页且底页归位', (tester) async {
    final navigatorKey = GlobalKey<NavigatorState>();
    await tester.pumpWidget(
      _app(navigatorKey, platform: TargetPlatform.android),
    );
    await tester.pumpAndSettle();

    final detail = _detailRoute();
    navigatorKey.currentState!.push(detail);
    await tester.pumpAndSettle();

    detail.handleStartBackGesture(progress: 1);
    await tester.pump();
    detail.handleUpdateBackGestureProgress(progress: 0.3);
    await tester.pump();
    detail.handleCommitBackGesture();
    await tester.pumpAndSettle();

    expect(find.text('detail'), findsNothing);
    expect(detail.backGestureActive, isFalse);
    expect(
      _rootSlide(tester).position.value.dx,
      moreOrLessEquals(0, epsilon: 0.002),
    );
  });

  testWidgets('位移不影响页面命中测试', (tester) async {
    final navigatorKey = GlobalKey<NavigatorState>();
    await tester.pumpWidget(_app(navigatorKey));
    await tester.pumpAndSettle();

    final detail = _detailRoute();
    navigatorKey.currentState!.push(detail);
    await tester.pumpAndSettle();

    expect(
      tester.widget<SlideTransition>(_detailSlideFinder()).transformHitTests,
      isFalse,
    );
  });

  testWidgets('Android 上边缘滑动交由系统返回处理，不由页面重复响应', (tester) async {
    final navigatorKey = GlobalKey<NavigatorState>();
    await tester.pumpWidget(
      _app(navigatorKey, platform: TargetPlatform.android),
    );
    await tester.pumpAndSettle();

    navigatorKey.currentState!.push(_detailRoute());
    await tester.pumpAndSettle();

    final gesture = await tester.startGesture(const Offset(6, 300));
    await gesture.moveBy(const Offset(520, 0));
    await tester.pump();
    await gesture.up();
    await tester.pumpAndSettle();

    expect(find.text('detail'), findsOneWidget);
  });

  testWidgets('系统关闭动画时直接展示页面', (tester) async {
    final navigatorKey = GlobalKey<NavigatorState>();
    await tester.pumpWidget(_app(navigatorKey, disableAnimations: true));
    await tester.pumpAndSettle();

    navigatorKey.currentState!.push(_detailRoute());
    await tester.pump();

    expect(find.text('detail'), findsOneWidget);
  });

  testWidgets('进入和返回逐帧保持两页边缘贴合', (tester) async {
    final navigatorKey = GlobalKey<NavigatorState>();
    await tester.pumpWidget(_app(navigatorKey));
    await tester.pumpAndSettle();
    navigatorKey.currentState!.push(_detailRoute());
    await tester.pump();
    void expectJoinedEdges() {
      final root = tester.getRect(
        find
            .ancestor(
              of: find.text('root', skipOffstage: false),
              matching: find.byType(Scaffold, skipOffstage: false),
            )
            .first,
      );
      final detail = tester.getRect(
        find
            .ancestor(
              of: find.text('detail', skipOffstage: false),
              matching: find.byType(Scaffold, skipOffstage: false),
            )
            .first,
      );
      expect(root.right, closeTo(detail.left, 0.01));
    }

    for (var frame = 0; frame < 25; frame++) {
      await tester.pump(const Duration(milliseconds: 16));
      expectJoinedEdges();
    }
    await tester.pumpAndSettle();
    navigatorKey.currentState!.pop();
    await tester.pump();
    for (var frame = 0; frame < 22; frame++) {
      await tester.pump(const Duration(milliseconds: 16));
      expectJoinedEdges();
    }
    await tester.pumpAndSettle();
  });

  testWidgets('预测返回线性跟手且停手不自行移动，收尾不瞬间完成', (tester) async {
    final navigatorKey = GlobalKey<NavigatorState>();
    await tester.pumpWidget(
      _app(navigatorKey, platform: TargetPlatform.android),
    );
    await tester.pumpAndSettle();
    final detail = _detailRoute();
    navigatorKey.currentState!.push(detail);
    await tester.pumpAndSettle();
    detail.handleStartBackGesture(progress: 1);
    detail.handleUpdateBackGestureProgress(progress: 0.4);
    await tester.pump();
    expect(_detailSlide(tester).position.value.dx, closeTo(0.6, 0.001));
    expect(_rootSlide(tester).position.value.dx, closeTo(-0.4, 0.001));
    await tester.pump(const Duration(milliseconds: 120));
    expect(_detailSlide(tester).position.value.dx, closeTo(0.6, 0.001));
    detail.handleCommitBackGesture();
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));
    expect(find.text('detail'), findsOneWidget);
    expect(detail.backGestureActive, isTrue);
    final top = _detailSlide(tester).position.value.dx;
    expect(top, inExclusiveRange(0.6, 1));
    expect(_rootSlide(tester).position.value.dx, closeTo(top - 1, 0.001));
    await tester.pumpAndSettle();
    expect(find.text('detail'), findsNothing);
    expect(detail.backGestureActive, isFalse);
  });

  testWidgets('多级页面推入与手势取消全过程边缘贴合', (tester) async {
    final navigatorKey = GlobalKey<NavigatorState>();
    await tester.pumpWidget(_app(navigatorKey));
    await tester.pumpAndSettle();
    navigatorKey.currentState!.push(_detailRoute());
    await tester.pumpAndSettle();
    final third = amitiaSlideRoute<void>(
      builder: (_) => const Scaffold(body: Center(child: Text('third'))),
    );
    Rect pageRect(String label) => tester.getRect(
      find
          .ancestor(
            of: find.text(label, skipOffstage: false),
            matching: find.byType(Scaffold, skipOffstage: false),
          )
          .first,
    );
    void expectJoinedEdges() {
      expect(pageRect('detail').right, closeTo(pageRect('third').left, 0.01));
    }

    navigatorKey.currentState!.push(third);
    await tester.pump();
    for (var frame = 0; frame < 25; frame++) {
      await tester.pump(const Duration(milliseconds: 16));
      expectJoinedEdges();
    }
    await tester.pumpAndSettle();
    third.handleStartBackGesture(progress: 1);
    for (final progress in [0.8, 0.6, 0.4]) {
      third.handleUpdateBackGestureProgress(progress: progress);
      await tester.pump();
      expectJoinedEdges();
    }
    third.handleCancelBackGesture();
    await tester.pump();
    for (var frame = 0; frame < 13; frame++) {
      await tester.pump(const Duration(milliseconds: 16));
      expectJoinedEdges();
    }
    await tester.pumpAndSettle();
    expect(find.text('third'), findsOneWidget);
    expect(third.backGestureActive, isFalse);
  });
}

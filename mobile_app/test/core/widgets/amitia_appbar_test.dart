import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:amitia_app/core/widgets/amitia_icons.dart';
import 'package:amitia_app/core/widgets/amitia_scaffold.dart';

Finder _hasGlyph(AmitiaIconGlyph glyph) {
  return find.byWidgetPredicate(
    (widget) => widget is AmitiaIcon && widget.glyph == glyph,
  );
}

void main() {
  group('AmitiaAppBar back button', () {
    testWidgets('shows back button when navigation is back', (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            appBar: AmitiaAppBar(
              title: 'Test',
              navigation: AmitiaAppBarNavigation.back,
            ),
          ),
        ),
      );

      expect(_hasGlyph(AmitiaIconGlyph.back), findsOneWidget);
    });

    testWidgets('shows back button when showBackButton is true', (
      tester,
    ) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            appBar: AmitiaAppBar(title: 'Test', showBackButton: true),
          ),
        ),
      );

      expect(_hasGlyph(AmitiaIconGlyph.back), findsOneWidget);
    });

    testWidgets('does not show leading when navigation is none', (
      tester,
    ) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            appBar: AmitiaAppBar(
              title: 'Test',
              navigation: AmitiaAppBarNavigation.none,
            ),
          ),
        ),
      );

      expect(_hasGlyph(AmitiaIconGlyph.back), findsNothing);
      expect(find.byIcon(Icons.menu), findsNothing);
    });

    testWidgets('back button pops when there is a route to pop', (
      tester,
    ) async {
      final goRouter = GoRouter(
        initialLocation: '/second',
        routes: [
          GoRoute(
            path: '/first',
            builder: (context, state) => Scaffold(
              appBar: AmitiaAppBar(title: 'First'),
              body: const Center(child: Text('First Page')),
            ),
          ),
          GoRoute(
            path: '/second',
            builder: (context, state) => Scaffold(
              appBar: AmitiaAppBar(
                title: 'Second',
                navigation: AmitiaAppBarNavigation.back,
              ),
              body: const Center(child: Text('Second Page')),
            ),
          ),
        ],
      );

      await tester.pumpWidget(MaterialApp.router(routerConfig: goRouter));
      await tester.pumpAndSettle();

      expect(find.text('Second Page'), findsOneWidget);

      await tester.tap(_hasGlyph(AmitiaIconGlyph.back));
      await tester.pumpAndSettle();
    });

    testWidgets('custom leading widget overrides navigation', (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            appBar: AmitiaAppBar(
              title: 'Test',
              navigation: AmitiaAppBarNavigation.back,
              leading: const Icon(Icons.close),
            ),
          ),
        ),
      );

      expect(find.byIcon(Icons.close), findsOneWidget);
      expect(_hasGlyph(AmitiaIconGlyph.back), findsNothing);
    });
  });

  group('AmitiaAppBar title', () {
    testWidgets('displays text title', (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(appBar: AmitiaAppBar(title: 'My Title')),
        ),
      );

      expect(find.text('My Title'), findsOneWidget);
    });

    testWidgets('displays title widget when provided', (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            appBar: AmitiaAppBar(titleWidget: const Text('Custom Widget')),
          ),
        ),
      );

      expect(find.text('Custom Widget'), findsOneWidget);
    });

    testWidgets('title widget takes precedence over title string', (
      tester,
    ) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            appBar: AmitiaAppBar(
              title: 'String Title',
              titleWidget: const Text('Widget Title'),
            ),
          ),
        ),
      );

      expect(find.text('Widget Title'), findsOneWidget);
      expect(find.text('String Title'), findsNothing);
    });
  });

  group('AmitiaAppBar actions', () {
    testWidgets('displays action widgets', (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            appBar: AmitiaAppBar(
              title: 'Test',
              actions: [
                IconButton(icon: const Icon(Icons.search), onPressed: () {}),
                IconButton(icon: const Icon(Icons.more_vert), onPressed: () {}),
              ],
            ),
          ),
        ),
      );

      expect(find.byIcon(Icons.search), findsOneWidget);
      expect(find.byIcon(Icons.more_vert), findsOneWidget);
    });
  });
}

import 'package:amitia_app/app/route_transitions.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('back target transition keeps a fully covered route visible', (
    tester,
  ) async {
    await tester.pumpWidget(
      MaterialApp(
        home: backTargetTransition(
          secondaryAnimation: const AlwaysStoppedAnimation<double>(1),
          child: const Center(child: Text('chat')),
        ),
      ),
    );

    expect(find.text('chat'), findsOneWidget);
    final slideFinder = find.byWidgetPredicate(
      (widget) => widget is SlideTransition && widget.position.value.dx < 0,
    );
    expect(slideFinder, findsOneWidget);
    expect(
      find.descendant(of: slideFinder, matching: find.byType(FadeTransition)),
      findsNothing,
    );
    final slide = tester.widget<SlideTransition>(slideFinder);
    expect(slide.position.value.dx, lessThan(0));
  });
}

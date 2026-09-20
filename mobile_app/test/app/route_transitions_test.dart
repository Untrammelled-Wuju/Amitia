import 'package:amitia_app/app/route_transitions.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('back target transition restores the covered route', (
    tester,
  ) async {
    await tester.pumpWidget(
      MaterialApp(
        home: backTargetTransition(
          secondaryAnimation: const AlwaysStoppedAnimation<double>(0),
          child: const Center(child: Text('chat')),
        ),
      ),
    );

    expect(find.text('chat'), findsOneWidget);
    final slide = tester.widget<SlideTransition>(
      find
          .ancestor(
            of: find.text('chat'),
            matching: find.byType(SlideTransition),
          )
          .first,
    );
    final fade = tester.widget<FadeTransition>(
      find
          .ancestor(
            of: find.text('chat'),
            matching: find.byType(FadeTransition),
          )
          .first,
    );
    expect(slide.position.value, Offset.zero);
    expect(fade.opacity.value, 1);
  });
}

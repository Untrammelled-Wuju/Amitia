import 'package:amitia_app/core/widgets/amitia_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  for (final glyph in AmitiaIconGlyph.values) {
    testWidgets('renders ${glyph.name} at supported sizes', (tester) async {
      for (final size in const [16.0, 20.0, 24.0]) {
        await tester.pumpWidget(
          MaterialApp(
            home: Scaffold(
              body: Center(
                child: AmitiaIcon(
                  glyph,
                  size: size,
                  color: const Color(0xFF353535),
                ),
              ),
            ),
          ),
        );

        expect(find.byType(AmitiaIcon), findsOneWidget);
        expect(tester.getSize(find.byType(AmitiaIcon)), Size.square(size));
        expect(tester.takeException(), isNull);
      }
    });
  }

  testWidgets('maps common material icons to the Amitia glyph set', (
    tester,
  ) async {
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: Column(
            children: [
              AmitiaIcon.adaptive(Icons.cleaning_services_outlined),
              AmitiaIcon.adaptive(Icons.accessibility_new),
            ],
          ),
        ),
      ),
    );

    expect(
      AmitiaIcon.tryFromMaterial(Icons.cleaning_services_outlined),
      AmitiaIconGlyph.cleanup,
    );
    expect(AmitiaIcon.tryFromMaterial(Icons.accessibility_new), isNull);
    expect(find.byType(AmitiaIcon), findsOneWidget);
    expect(find.byType(Icon), findsOneWidget);
  });
}

import 'dart:io';

import 'package:amitia_app/core/widgets/amitia_popup_menu.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('popup menu placement follows a left trigger', () {
    final placement = AmitiaPopupMenuPlacement.resolve(
      anchorRect: const Rect.fromLTWH(24, 80, 48, 48),
      overlayRect: const Rect.fromLTWH(0, 0, 400, 800),
      estimatedMenuHeight: 180,
    );

    expect(placement.horizontalAnchor, AmitiaPopupMenuHorizontalAnchor.start);
    expect(placement.opensAbove, isFalse);
    expect(placement.scaleAlignment, Alignment.topLeft);
  });

  test('popup menu placement follows a right and bottom trigger', () {
    final placement = AmitiaPopupMenuPlacement.resolve(
      anchorRect: const Rect.fromLTWH(320, 680, 48, 48),
      overlayRect: const Rect.fromLTWH(0, 0, 400, 800),
      estimatedMenuHeight: 180,
    );

    expect(placement.horizontalAnchor, AmitiaPopupMenuHorizontalAnchor.end);
    expect(placement.opensAbove, isTrue);
    expect(placement.scaleAlignment, Alignment.bottomRight);
  });

  test('popup menu placement centers a middle trigger', () {
    final placement = AmitiaPopupMenuPlacement.resolve(
      anchorRect: const Rect.fromLTWH(176, 100, 48, 48),
      overlayRect: const Rect.fromLTWH(0, 0, 400, 800),
      estimatedMenuHeight: 180,
    );

    expect(placement.horizontalAnchor, AmitiaPopupMenuHorizontalAnchor.center);
    expect(placement.scaleAlignment, Alignment.topCenter);
  });

  testWidgets('popup menu scales from the trigger corner and selects', (
    tester,
  ) async {
    String? selected;
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: Align(
            alignment: Alignment.bottomRight,
            child: AmitiaPopupMenuButton<String>(
              tooltip: '操作',
              onSelected: (value) => selected = value,
              itemBuilder: (_) => const [
                PopupMenuItem(value: 'first', child: Text('第一项')),
              ],
            ),
          ),
        ),
      ),
    );

    await tester.tap(find.byType(IconButton));
    await tester.pumpAndSettle();

    final scale = tester.widget<ScaleTransition>(
      find.ancestor(
        of: find.text('第一项'),
        matching: find.byType(ScaleTransition),
      ),
    );
    expect(scale.alignment, Alignment.bottomRight);

    await tester.tap(find.text('第一项'));
    await tester.pumpAndSettle();
    expect(selected, 'first');
  });

  test('built-in popup menu buttons are fully replaced', () {
    final pattern = RegExp(r'\bPopupMenuButton\s*<');
    final matches = <String>[];
    for (final entity in Directory('lib').listSync(recursive: true)) {
      if (entity is! File || !entity.path.endsWith('.dart')) continue;
      final source = entity.readAsStringSync();
      if (pattern.hasMatch(source)) matches.add(entity.path);
    }

    expect(matches, isEmpty);
  });
}

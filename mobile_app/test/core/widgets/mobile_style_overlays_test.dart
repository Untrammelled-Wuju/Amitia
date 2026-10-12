import 'package:amitia_app/core/ui_runtime/mobile_style_runtime.dart';
import 'package:amitia_app/core/widgets/amitia_dialogs.dart';
import 'package:amitia_app/core/widgets/amitia_popup_menu.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

Widget _styledApp({
  required Widget home,
  required AmitiaStyleDocument document,
}) {
  return MaterialApp(
    builder: (context, child) {
      return AmitiaStyleScopeHost(
        scope: const AmitiaStyleScopeData(platform: 'mobile', page: 'chat'),
        child: AmitiaStyleDocumentHost(
          document: document,
          child: child ?? const SizedBox.shrink(),
        ),
      );
    },
    home: home,
  );
}

void main() {
  test('overlay component targets are registered', () {
    for (final target in const [
      'component.dialog',
      'component.bottom_sheet',
      'component.popover',
      'component.list_item',
    ]) {
      final descriptor = AmitiaStyleTargetRegistry.descriptor(target);
      expect(descriptor, isNotNull);
      expect(descriptor?.properties['backgroundColor'], 'color');
      expect(descriptor?.properties['borderRadius'], 'length');
      expect(descriptor?.properties['boxShadow'], 'shadow');
      expect(descriptor?.properties['padding'], 'insets');
    }
  });

  testWidgets('dialog styles apply to the shared alert dialog', (tester) async {
    const padding = EdgeInsets.fromLTRB(22, 12, 24, 16);
    final document = AmitiaStyleDocument(
      revision: 1,
      rules: <AmitiaStyleRule>[
        AmitiaStyleRule(
          id: 'dialog',
          target: 'component.dialog',
          properties: const <String, dynamic>{
            'backgroundColor': '#112233',
            'foregroundColor': '#f0f1f2',
            'borderColor': '#334455',
            'borderWidth': 2,
            'borderRadius': 18,
            'fontSize': 22,
            'padding': <String, dynamic>{
              'left': 22,
              'top': 12,
              'right': 24,
              'bottom': 16,
            },
            'boxShadow': <Map<String, dynamic>>[
              <String, dynamic>{
                'color': '#66000000',
                'blurRadius': 12,
                'offsetX': 2,
                'offsetY': 4,
              },
            ],
          },
        ),
      ],
    );
    await tester.pumpWidget(
      _styledApp(
        document: document,
        home: Scaffold(
          body: Builder(
            builder: (context) => ElevatedButton(
              onPressed: () => showAmitiaConfirmDialog(
                context,
                title: 'Styled title',
                message: 'Styled message',
              ),
              child: const Text('Open'),
            ),
          ),
        ),
      ),
    );

    await tester.tap(find.text('Open'));
    await tester.pumpAndSettle();

    final alert = tester.widget<AlertDialog>(find.byType(AlertDialog));
    final shape = alert.shape! as RoundedRectangleBorder;
    expect(alert.backgroundColor, const Color(0xFF112233));
    expect(alert.titlePadding, padding);
    expect(alert.contentPadding, padding);
    expect(alert.actionsPadding, padding);
    expect(shape.borderRadius, BorderRadius.circular(18));
    expect(shape.side.color, const Color(0xFF334455));
    expect(shape.side.width, 2);
    expect(
      tester.widget<Text>(find.text('Styled title')).style?.color,
      const Color(0xFFF0F1F2),
    );
    expect(tester.widget<Text>(find.text('Styled title')).style?.fontSize, 22);
    final shadowSurface = tester.widget<DecoratedBox>(
      find
          .ancestor(
            of: find.byType(AlertDialog),
            matching: find.byWidgetPredicate(
              (widget) =>
                  widget is DecoratedBox &&
                  widget.decoration is BoxDecoration &&
                  (widget.decoration as BoxDecoration).boxShadow?.isNotEmpty ==
                      true,
            ),
          )
          .first,
    );
    expect(
      (shadowSurface.decoration as BoxDecoration).boxShadow?.single.color,
      const Color(0x66000000),
    );
  });

  testWidgets('bottom sheet and list item styles apply to action sheets', (
    tester,
  ) async {
    final document = AmitiaStyleDocument(
      revision: 1,
      rules: <AmitiaStyleRule>[
        AmitiaStyleRule(
          id: 'sheet',
          target: 'component.bottom_sheet',
          properties: const <String, dynamic>{
            'backgroundColor': '#223344',
            'foregroundColor': '#f4f5f6',
            'borderRadius': 20,
            'padding': <String, dynamic>{
              'left': 18,
              'top': 8,
              'right': 18,
              'bottom': 28,
            },
          },
        ),
        AmitiaStyleRule(
          id: 'item',
          target: 'component.list_item',
          properties: const <String, dynamic>{
            'backgroundColor': '#556677',
            'foregroundColor': '#ffffff',
            'fontSize': 17,
            'borderRadius': 12,
            'padding': <String, dynamic>{
              'left': 10,
              'top': 8,
              'right': 10,
              'bottom': 8,
            },
          },
        ),
      ],
    );
    await tester.pumpWidget(
      _styledApp(
        document: document,
        home: Scaffold(
          body: Builder(
            builder: (context) => ElevatedButton(
              onPressed: () => showAmitiaActionSheet<int>(
                context,
                title: 'Actions',
                actions: const <AmitiaActionSheetItem<int>>[
                  AmitiaActionSheetItem<int>(
                    icon: Icons.edit,
                    label: 'Edit',
                    value: 0,
                  ),
                  AmitiaActionSheetItem<int>(
                    icon: Icons.delete,
                    label: 'Delete',
                    value: 1,
                    isDestructive: true,
                  ),
                ],
              ),
              child: const Text('Open'),
            ),
          ),
        ),
      ),
    );

    await tester.tap(find.text('Open'));
    await tester.pumpAndSettle();

    final sheetSurface = tester.widget<DecoratedBox>(
      find.byWidgetPredicate(
        (widget) =>
            widget is DecoratedBox &&
            widget.decoration is BoxDecoration &&
            (widget.decoration as BoxDecoration).color ==
                const Color(0xFF223344),
      ),
    );
    expect(
      (sheetSurface.decoration as BoxDecoration).borderRadius,
      BorderRadius.circular(20),
    );
    expect(
      tester.widget<Text>(find.text('Actions')).style?.color,
      const Color(0xFFF4F5F6),
    );
    expect(
      find.byWidgetPredicate(
        (widget) =>
            widget is DecoratedBox &&
            widget.decoration is BoxDecoration &&
            (widget.decoration as BoxDecoration).color ==
                const Color(0xFF556677),
      ),
      findsNWidgets(2),
    );
    expect(tester.widget<Text>(find.text('Edit')).style?.fontSize, 17);
  });

  testWidgets('popover and list item styles apply to popup menus', (
    tester,
  ) async {
    final document = AmitiaStyleDocument(
      revision: 1,
      rules: <AmitiaStyleRule>[
        AmitiaStyleRule(
          id: 'popover',
          target: 'component.popover',
          properties: const <String, dynamic>{
            'backgroundColor': '#123456',
            'foregroundColor': '#ffffff',
            'borderColor': '#abcdef',
            'borderWidth': 2,
            'borderRadius': 20,
            'fontSize': 16,
            'padding': 10,
          },
        ),
        AmitiaStyleRule(
          id: 'item',
          target: 'component.list_item',
          properties: const <String, dynamic>{
            'backgroundColor': '#654321',
            'foregroundColor': '#fedcba',
            'borderRadius': 12,
            'fontSize': 18,
          },
        ),
      ],
    );
    await tester.pumpWidget(
      _styledApp(
        document: document,
        home: Scaffold(
          body: AmitiaPopupMenuButton<String>(
            itemBuilder: (_) => const <PopupMenuEntry<String>>[
              PopupMenuItem<String>(value: 'item', child: Text('Styled item')),
            ],
          ),
        ),
      ),
    );

    await tester.tap(find.byType(IconButton));
    await tester.pumpAndSettle();

    final popupMaterial = tester.widget<Material>(
      find.byWidgetPredicate(
        (widget) =>
            widget is Material && widget.color == const Color(0xFF123456),
      ),
    );
    final popupShape = popupMaterial.shape! as RoundedRectangleBorder;
    expect(popupShape.borderRadius, BorderRadius.circular(20));
    expect(popupShape.side.color, const Color(0xFFABCDEF));
    expect(popupShape.side.width, 2);
    expect(
      tester
          .widget<SingleChildScrollView>(find.byType(SingleChildScrollView))
          .padding,
      const EdgeInsets.all(10),
    );
    expect(
      find.byWidgetPredicate(
        (widget) =>
            widget is Material && widget.color == const Color(0xFF654321),
      ),
      findsOneWidget,
    );
    final itemTextStyle = tester
        .widget<DefaultTextStyle>(
          find
              .ancestor(
                of: find.text('Styled item'),
                matching: find.byType(DefaultTextStyle),
              )
              .first,
        )
        .style;
    expect(itemTextStyle.fontSize, 18);
    expect(itemTextStyle.color, const Color(0xFFFEDCBA));
  });

  testWidgets('popup items do not inherit underlined bold text', (
    tester,
  ) async {
    await tester.pumpWidget(
      _styledApp(
        document: const AmitiaStyleDocument(),
        home: DefaultTextStyle(
          style: const TextStyle(
            fontWeight: FontWeight.w700,
            decoration: TextDecoration.underline,
            decorationColor: Colors.yellow,
          ),
          child: Scaffold(
            body: AmitiaPopupMenuButton<String>(
              itemBuilder: (_) => const <PopupMenuEntry<String>>[
                PopupMenuItem<String>(value: 'item', child: Text('Plain item')),
              ],
            ),
          ),
        ),
      ),
    );

    await tester.tap(find.byType(IconButton));
    await tester.pumpAndSettle();

    final itemTextStyle = DefaultTextStyle.of(
      tester.element(find.text('Plain item')),
    ).style;
    expect(itemTextStyle.fontWeight, FontWeight.w400);
    expect(itemTextStyle.decoration, TextDecoration.none);
  });
}

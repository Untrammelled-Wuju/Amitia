import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/widgets/amitia_dialogs.dart';

void main() {
  group('showAmitiaConfirmDialog', () {
    testWidgets('displays title and message', (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: Builder(
              builder: (context) => ElevatedButton(
                onPressed: () => showAmitiaConfirmDialog(
                  context,
                  title: 'Test Title',
                  message: 'Test Message',
                ),
                child: const Text('Open'),
              ),
            ),
          ),
        ),
      );

      await tester.tap(find.text('Open'));
      await tester.pumpAndSettle();

      expect(find.text('Test Title'), findsOneWidget);
      expect(find.text('Test Message'), findsOneWidget);
      expect(find.text('确认'), findsOneWidget);
      expect(find.text('取消'), findsOneWidget);
    });

    testWidgets('returns true when confirm is tapped', (tester) async {
      bool? result;

      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: Builder(
              builder: (context) => ElevatedButton(
                onPressed: () async {
                  result = await showAmitiaConfirmDialog(
                    context,
                    title: 'Confirm',
                    message: 'Are you sure?',
                  );
                },
                child: const Text('Open'),
              ),
            ),
          ),
        ),
      );

      await tester.tap(find.text('Open'));
      await tester.pumpAndSettle();

      await tester.tap(find.text('确认'));
      await tester.pumpAndSettle();

      expect(result, isTrue);
    });

    testWidgets('returns false when cancel is tapped', (tester) async {
      bool? result;

      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: Builder(
              builder: (context) => ElevatedButton(
                onPressed: () async {
                  result = await showAmitiaConfirmDialog(
                    context,
                    title: 'Confirm',
                    message: 'Are you sure?',
                  );
                },
                child: const Text('Open'),
              ),
            ),
          ),
        ),
      );

      await tester.tap(find.text('Open'));
      await tester.pumpAndSettle();

      await tester.tap(find.text('取消'));
      await tester.pumpAndSettle();

      expect(result, isFalse);
    });

    testWidgets('displays custom confirm label', (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: Builder(
              builder: (context) => ElevatedButton(
                onPressed: () => showAmitiaConfirmDialog(
                  context,
                  title: 'Delete',
                  message: 'Delete this item?',
                  confirmLabel: '删除',
                  isDestructive: true,
                ),
                child: const Text('Open'),
              ),
            ),
          ),
        ),
      );

      await tester.tap(find.text('Open'));
      await tester.pumpAndSettle();

      expect(find.text('删除'), findsOneWidget);
    });
  });

  group('showAmitiaInfoDialog', () {
    testWidgets('displays info dialog with OK button', (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: Builder(
              builder: (context) => ElevatedButton(
                onPressed: () => showAmitiaInfoDialog(
                  context,
                  title: 'Info',
                  message: 'This is info',
                ),
                child: const Text('Open'),
              ),
            ),
          ),
        ),
      );

      await tester.tap(find.text('Open'));
      await tester.pumpAndSettle();

      expect(find.text('Info'), findsOneWidget);
      expect(find.text('This is info'), findsOneWidget);
      expect(find.text('知道了'), findsOneWidget);
    });

    testWidgets('can be dismissed by tapping OK', (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: Builder(
              builder: (context) => ElevatedButton(
                onPressed: () => showAmitiaInfoDialog(
                  context,
                  title: 'Info',
                  message: 'Dismiss me',
                ),
                child: const Text('Open'),
              ),
            ),
          ),
        ),
      );

      await tester.tap(find.text('Open'));
      await tester.pumpAndSettle();

      await tester.tap(find.text('知道了'));
      await tester.pumpAndSettle();

      expect(find.text('Info'), findsNothing);
    });
  });

  group('showAmitiaActionSheet', () {
    testWidgets('displays all action items', (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: Builder(
              builder: (context) => ElevatedButton(
                onPressed: () => showAmitiaActionSheet<int>(
                  context,
                  title: 'Actions',
                  actions: const [
                    AmitiaActionSheetItem(icon: Icons.edit, label: 'Edit', value: 0),
                    AmitiaActionSheetItem(icon: Icons.delete, label: 'Delete', value: 1, isDestructive: true),
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

      expect(find.text('Actions'), findsOneWidget);
      expect(find.text('Edit'), findsOneWidget);
      expect(find.text('Delete'), findsOneWidget);
    });

    testWidgets('returns selected value when tapped', (tester) async {
      int? result;

      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: Builder(
              builder: (context) => ElevatedButton(
                onPressed: () async {
                  result = await showAmitiaActionSheet<int>(
                    context,
                    title: 'Actions',
                    actions: const [
                      AmitiaActionSheetItem(icon: Icons.edit, label: 'Edit', value: 0),
                      AmitiaActionSheetItem(icon: Icons.delete, label: 'Delete', value: 1),
                    ],
                  );
                },
                child: const Text('Open'),
              ),
            ),
          ),
        ),
      );

      await tester.tap(find.text('Open'));
      await tester.pumpAndSettle();

      await tester.tap(find.text('Delete'));
      await tester.pumpAndSettle();

      expect(result, 1);
    });
  });

  group('amitiaSnackBar', () {
    testWidgets('displays snackbar with message', (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: Builder(
              builder: (context) => ElevatedButton(
                onPressed: () => amitiaSnackBar(context, 'Test Snackbar'),
                child: const Text('Show'),
              ),
            ),
          ),
        ),
      );

      await tester.tap(find.text('Show'));
      await tester.pump();

      expect(find.text('Test Snackbar'), findsOneWidget);
    });
  });

  group('AmitiaActionSheetItem', () {
    test('creates with required parameters', () {
      const item = AmitiaActionSheetItem<int>(
        icon: Icons.edit,
        label: 'Edit',
        value: 1,
      );
      expect(item.icon, Icons.edit);
      expect(item.label, 'Edit');
      expect(item.value, 1);
      expect(item.isDestructive, isFalse);
    });

    test('creates with isDestructive flag', () {
      const item = AmitiaActionSheetItem<int>(
        icon: Icons.delete,
        label: 'Delete',
        value: 2,
        isDestructive: true,
      );
      expect(item.isDestructive, isTrue);
    });
  });
}

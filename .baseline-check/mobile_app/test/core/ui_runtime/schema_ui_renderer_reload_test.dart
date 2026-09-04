import 'package:amitia_app/features/extensions/schema_ui/models/schema_ui_types.dart';
import 'package:amitia_app/features/extensions/schema_ui/renderer/schema_ui_renderer.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('reload_schema result invokes the host schema reloader', (tester) async {
    var reloadCount = 0;
    final document = SchemaUIDocument(
      children: const <SchemaUINode>[
        SchemaUINode(
          id: 'reload-button',
          type: SchemaUI.nodeButton,
          props: <String, dynamic>{'label': 'Reload schema'},
          actions: <SchemaUIActionBinding>[
            SchemaUIActionBinding(
              actionId: 'reload.action',
              target: 'invoke_capability',
            ),
          ],
        ),
      ],
    );

    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: SchemaUIRenderer(
            document: document,
            extensionId: 'extension.test',
            contributionId: 'contribution.test',
            embedded: true,
            onActionDispatch: (_) async => <String, dynamic>{
              'reload_schema': true,
            },
            onReloadSchema: () {
              reloadCount++;
            },
          ),
        ),
      ),
    );

    await tester.tap(find.text('Reload schema'));
    await tester.pumpAndSettle();

    expect(reloadCount, 1);
  });
}

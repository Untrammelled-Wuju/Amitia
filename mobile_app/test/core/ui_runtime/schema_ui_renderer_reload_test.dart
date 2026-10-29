import 'package:amitia_app/core/widgets/amitia_misc.dart';
import 'package:amitia_app/features/extensions/schema_ui/models/schema_ui_types.dart';
import 'package:amitia_app/features/extensions/schema_ui/renderer/schema_ui_renderer.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('refresh lifecycle polls and stops when disposed', (
    tester,
  ) async {
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
    var calls = 0;
    final document = SchemaUIDocument.fromJson({
      'schemaVersion': 'schema-ui/1',
      'type': 'document',
      'lifecycle': {
        'refreshIntervalMs': 1000,
        'onMount': [
          {'action_id': 'status', 'target': 'host_command'},
        ],
        'onRefresh': [
          {'action_id': 'status', 'target': 'host_command'},
        ],
      },
      'children': <dynamic>[],
    });
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: SchemaUIRenderer(
            document: document,
            extensionId: 'test',
            contributionId: 'test',
            embedded: true,
            onActionDispatch: (_) async {
              calls++;
              return <String, dynamic>{};
            },
          ),
        ),
      ),
    );
    await tester.pump();
    final initial = calls;
    await tester.pump(const Duration(seconds: 1));
    expect(calls, greaterThan(initial));
    await tester.pumpWidget(const SizedBox());
    final disposed = calls;
    await tester.pump(const Duration(seconds: 2));
    expect(calls, disposed);
  });

  testWidgets('reload_schema result invokes the host schema reloader', (
    tester,
  ) async {
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

  testWidgets('button_group resolves child button bindings', (tester) async {
    final document = SchemaUIDocument(
      children: const <SchemaUINode>[
        SchemaUINode(
          id: 'actions',
          type: SchemaUI.nodeButtonGroup,
          children: <SchemaUINode>[
            SchemaUINode(
              id: 'install',
              type: SchemaUI.nodeButton,
              props: <String, dynamic>{'text': '开始安装'},
              bindings: <SchemaUIBinding>[
                SchemaUIBinding(
                  path: 'text',
                  source: 'state',
                  sourcePath: 'status.installLabel',
                ),
                SchemaUIBinding(
                  path: 'disabled',
                  source: 'state',
                  sourcePath: 'status.installing',
                ),
              ],
              actions: <SchemaUIActionBinding>[
                SchemaUIActionBinding(
                  actionId: 'install',
                  target: 'host_command',
                ),
              ],
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
            initialContext: <String, dynamic>{
              'localState': <String, dynamic>{
                'status': <String, dynamic>{
                  'installLabel': '重新安装 QQ 与 SnowLuma',
                  'installing': true,
                },
              },
            },
            onActionDispatch: (_) async => <String, dynamic>{},
          ),
        ),
      ),
    );
    await tester.pump();

    expect(find.text('重新安装 QQ 与 SnowLuma'), findsOneWidget);
    final button = tester.widget<AmitiaButton>(find.byType(AmitiaButton));
    expect(button.onPressed, isNull);
  });
}

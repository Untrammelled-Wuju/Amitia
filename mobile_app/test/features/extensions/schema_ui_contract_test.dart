import 'package:flutter_test/flutter_test.dart';

import 'package:amitia_app/features/extensions/schema_ui/engine/binding_engine.dart';
import 'package:amitia_app/features/extensions/schema_ui/models/schema_ui_types.dart';

void main() {
  test('schema binding resolves sourcePath into the target property', () {
    const binding = SchemaUIBinding(
      path: 'text',
      sourcePath: 'dashboard.status.running',
      source: 'state',
    );
    final value = const BindingEngine().resolveBinding(
      binding,
      const BindingContext(
        localState: {
          'dashboard': {
            'status': {'running': true},
          },
        },
      ),
    );
    expect(value, isTrue);
  });

  test('schema action parses statePath and lifecycle actions', () {
    final document = SchemaUIDocument.fromJson({
      'schemaVersion': 'schema-ui/1',
      'children': [
        {'id': 'page', 'type': 'page'},
      ],
      'lifecycle': {
        'onMount': [
          {
            'action_id': 'command',
            'target': 'tool',
            'statePath': 'dashboard',
          },
        ],
      },
    });
    expect(document.lifecycle?.onMount.single.statePath, 'dashboard');
  });
}

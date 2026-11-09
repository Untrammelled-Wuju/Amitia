import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

Map<String, dynamic> _findNode(List<dynamic> nodes, String id) {
  for (final raw in nodes) {
    final node = Map<String, dynamic>.from(raw as Map);
    if (node['id'] == id) return node;
    final children = node['children'];
    if (children is List) {
      final nested = _findNode(children, id);
      if (nested.isNotEmpty) return nested;
    }
  }
  return <String, dynamic>{};
}

void main() {
  const pluginRoot = '../plugins-builtin/runtime-insights';

  test('long-running maintenance is provided by the runtime-insights plugin', () {
    final manifest = jsonDecode(
      File('$pluginRoot/amitia-extension.json').readAsStringSync(),
    ) as Map<String, dynamic>;
    final contributions =
        (manifest['modules'][0]['contributions'] as List).cast<Map>();
    final routeRegistry = contributions.firstWhere(
      (item) => item['id'] == 'runtime-insights-routes',
    );
    final routes =
        routeRegistry['spec']['metadata']['routes'] as List<dynamic>;
    expect(
      routes,
      contains(
        predicate(
          (value) =>
              value is Map &&
              value['id'] == 'long-running' &&
              value['path'] == '/long-running',
        ),
      ),
    );
  });

  test('maintenance schema loads overview and preserves revision-bound saves', () {
    final document = jsonDecode(
      File('$pluginRoot/ui/schema/long-running-mobile.json').readAsStringSync(),
    ) as Map<String, dynamic>;
    final mountActions =
        (document['lifecycle']['onMount'] as List<dynamic>)
            .map((item) => item['input']['action'])
            .toList();
    expect(
      mountActions,
      ['maintenance.overview', 'maintenance.config'],
    );
    final save = _findNode(
      document['children'] as List<dynamic>,
      'long-running-mobile-save',
    );
    final input = save['actions'][0]['input'] as Map<String, dynamic>;
    expect(input['action'], 'maintenance.update');
    expect(input['payload']['expectedRevision'], r'$state.maintenanceConfig.revision');
    expect(
      input['payload']['expectedTimeoutRevision'],
      r'$state.maintenanceConfig.timeoutRevision',
    );
  });
}

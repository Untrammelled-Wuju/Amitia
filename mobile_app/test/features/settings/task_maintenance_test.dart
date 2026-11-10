import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  test('runtime maintenance pages remain in the core route catalog', () {
    final catalog = File(
      'lib/core/ui_runtime/routing/builtin_route_catalog.dart',
    ).readAsStringSync();
    expect(catalog, contains("path: '/settings/runtime'"));
    expect(catalog, contains("path: '/settings/decision-viz'"));
    expect(catalog, contains("path: '/settings/long-running'"));
    expect(catalog, contains('LongRunningPage'));
  });
}

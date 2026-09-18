import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  test('extension center keeps a single-column list layout', () {
    final source = File(
      'lib/features/extensions/presentation/pages/extension_center_page.dart',
    ).readAsStringSync();

    expect(source, contains("'扩展能力'"));
    expect(source, contains('Column('));
    expect(source, isNot(contains('Wrap(')));
  });

  test('workshop home stays simple and single-column', () {
    final source = File(
      'lib/features/workshop/presentation/pages/workshop_home_page.dart',
    ).readAsStringSync();

    expect(source, contains("'制作工具'"));
    expect(source, contains('Border('));
    expect(source, isNot(contains('LinearGradient')));
    expect(source, isNot(contains('_buildTipCard')));
  });
}

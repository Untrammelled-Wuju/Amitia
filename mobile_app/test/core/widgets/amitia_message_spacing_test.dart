import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  test('chat message bubbles use compact vertical spacing', () {
    final source = File(
      'lib/core/widgets/amitia_message.dart',
    ).readAsStringSync();

    expect(
      source,
      contains(
        'padding: EdgeInsets.only(\n'
        '        left: AppSpacing.lg,\n'
        '        right: AppSpacing.lg,\n'
        '        bottom: 8,\n'
        '      ),',
      ),
    );
  });
}

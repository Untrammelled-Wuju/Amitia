import 'package:amitia_app/core/platform/device_form_factor.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('phone sizes lock to portrait only', () {
    const size = Size(599, 900);

    expect(isTabletSize(size), isFalse);
    expect(preferredOrientationsForSize(size), const <DeviceOrientation>[
      DeviceOrientation.portraitUp,
    ]);
  });

  test('tablet sizes keep all orientations', () {
    const size = Size(600, 900);

    expect(isTabletSize(size), isTrue);
    expect(preferredOrientationsForSize(size), DeviceOrientation.values);
  });
}

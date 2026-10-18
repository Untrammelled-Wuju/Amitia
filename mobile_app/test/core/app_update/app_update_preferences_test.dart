import 'package:amitia_app/core/app_update/app_update_preferences.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() {
    SharedPreferences.setMockInitialValues(<String, Object>{});
  });

  test(
    'returns the version-derived default without stored preference',
    () async {
      expect(
        await AppUpdatePreferences.loadIncludeTest(defaultValue: true),
        isTrue,
      );
      expect(
        await AppUpdatePreferences.loadIncludeTest(defaultValue: false),
        isFalse,
      );
    },
  );

  test('preserves an explicit disable for a beta build', () async {
    await AppUpdatePreferences.saveIncludeTest(false);

    expect(
      await AppUpdatePreferences.loadIncludeTest(defaultValue: true),
      isFalse,
    );
  });

  test('preserves an explicit enable for a stable build', () async {
    await AppUpdatePreferences.saveIncludeTest(true);

    expect(
      await AppUpdatePreferences.loadIncludeTest(defaultValue: false),
      isTrue,
    );
  });
}

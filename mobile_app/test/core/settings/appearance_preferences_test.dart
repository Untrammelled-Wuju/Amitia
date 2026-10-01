import 'package:amitia_app/core/settings/appearance_preferences.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('missing appearance defaults to blue before and after initialization', () async {
    SharedPreferences.setMockInitialValues(<String, Object>{});
    final preferences = AppearancePreferencesNotifier();
    expect(preferences.state.accentColorIndex, 1);
    await preferences.init();
    expect(preferences.state.accentColorIndex, 1);
    preferences.dispose();
  });

  test('saved warm accent survives initialization', () async {
    SharedPreferences.setMockInitialValues(<String, Object>{
      'appearance.accentColorIndex': 0,
    });
    final preferences = AppearancePreferencesNotifier();
    await preferences.init();
    expect(preferences.state.accentColorIndex, 0);
    preferences.dispose();
  });
}

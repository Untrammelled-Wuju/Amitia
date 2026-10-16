import 'package:shared_preferences/shared_preferences.dart';

class AppUpdatePreferences {
  static const String includeTestKey = 'app_update.include_test';
  static const String skippedVersionCodeKey = 'app_update.skipped_version_code';

  static Future<bool> loadIncludeTest() async {
    final preferences = await SharedPreferences.getInstance();
    return preferences.getBool(includeTestKey) ?? false;
  }

  static Future<void> saveIncludeTest(bool value) async {
    final preferences = await SharedPreferences.getInstance();
    await preferences.setBool(includeTestKey, value);
  }

  static Future<int> loadSkippedVersionCode() async {
    final preferences = await SharedPreferences.getInstance();
    return preferences.getInt(skippedVersionCodeKey) ?? 0;
  }

  static Future<void> saveSkippedVersionCode(int value) async {
    final preferences = await SharedPreferences.getInstance();
    await preferences.setInt(skippedVersionCodeKey, value);
  }
}

import 'package:shared_preferences/shared_preferences.dart';

abstract interface class AuthTokenStore {
  Future<String?> getToken();
  Future<void> setToken(String token);
  Future<void> clear();
}

class SharedPreferencesAuthTokenStore implements AuthTokenStore {
  static const String _key = 'ai_companion_token';

  const SharedPreferencesAuthTokenStore();

  @override
  Future<String?> getToken() async {
    final prefs = await SharedPreferences.getInstance();
    return prefs.getString(_key);
  }

  @override
  Future<void> setToken(String token) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_key, token);
  }

  @override
  Future<void> clear() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove(_key);
  }
}

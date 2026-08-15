import 'package:shared_preferences/shared_preferences.dart';
import '../auth/auth_token_store.dart';
import '../backend_transport/backend_service_api.dart';

class AuthResult {
  final String token;
  final UserInfo user;

  AuthResult({required this.token, required this.user});
}

class UserInfo {
  final String id;
  final String username;
  final String role;

  UserInfo({required this.id, required this.username, required this.role});

  factory UserInfo.fromJson(Map<String, dynamic> json) {
    return UserInfo(
      id: (json['userId'] ?? json['id'] ?? '').toString(),
      username: json['username'] as String? ?? '',
      role: json['role'] as String? ?? 'user',
    );
  }
}

class AuthService {
  static const String _userIdKey = 'user_id';
  static const String _usernameKey = 'username';

  final BackendServiceApi _api;
  final AuthTokenStore _tokenStore;

  AuthService(this._api, {AuthTokenStore? tokenStore})
      : _tokenStore = tokenStore ?? const SharedPreferencesAuthTokenStore();

  Future<bool> get isLoggedIn async {
    final token = await _tokenStore.getToken();
    return token != null && token.isNotEmpty;
  }

  Future<String?> get token => _tokenStore.getToken();

  Future<UserInfo?> get currentUser async {
    final prefs = await SharedPreferences.getInstance();
    final id = prefs.getString(_userIdKey);
    final username = prefs.getString(_usernameKey);
    if (id == null || username == null) return null;
    return UserInfo(id: id, username: username, role: 'user');
  }

  Future<AuthResult> login(String username, String password) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/public/auth/login',
      data: {'username': username, 'password': password},
    );

    if (resp == null) {
      throw ServiceApiException(code: 10000, message: '登录响应为空');
    }

    final token = resp['token'] as String? ?? '';
    final userInfo = UserInfo.fromJson(resp);

    await _tokenStore.setToken(token);
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_userIdKey, userInfo.id);
    await prefs.setString(_usernameKey, userInfo.username);

    return AuthResult(token: token, user: userInfo);
  }

  Future<void> logout() async {
    try {
      await _api.post('/api/auth/logout');
    } catch (_) {}
    await _tokenStore.clear();
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove(_userIdKey);
    await prefs.remove(_usernameKey);
  }

  Future<Map<String, dynamic>?> setup(String username, String password) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/public/auth/setup',
      data: {'username': username, 'password': password},
    );
    return resp;
  }
}

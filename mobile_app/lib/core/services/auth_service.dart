import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import '../auth/account_session_store.dart';
import '../backend_transport/backend_service_api.dart';

class AuthResult {
  final String accessToken;
  final String? refreshToken;
  final String? sessionId;
  final String? accessTokenExpiresAt;
  final UserInfo user;

  AuthResult({
    required this.accessToken,
    this.refreshToken,
    this.sessionId,
    this.accessTokenExpiresAt,
    required this.user,
  });
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
  final BackendServiceApi _api;
  final AccountSessionStore _sessionStore;

  AuthService(this._api, {AccountSessionStore? sessionStore})
      : _sessionStore = sessionStore ?? FlutterSecureAccountSessionStore();

  Future<bool> get isLoggedIn async {
    final token = await _sessionStore.getAccessToken();
    return token != null && token.isNotEmpty;
  }

  Future<String?> get accessToken => _sessionStore.getAccessToken();

  Future<String?> get refreshToken => _sessionStore.getRefreshToken();

  Future<UserInfo?> get currentUser async {
    final userId = await _sessionStore.getUserId();
    final username = await _sessionStore.getUsername();
    if (userId == null || username == null) return null;
    return UserInfo(id: userId, username: username, role: 'user');
  }

  Future<void> saveSession({
    required String accessToken,
    String? refreshToken,
    String? sessionId,
    String? accessTokenExpiresAt,
    required String userId,
    required String username,
    required String role,
  }) async {
    await _sessionStore.setFullSession(
      accessToken: accessToken,
      refreshToken: refreshToken,
      sessionId: sessionId,
      userId: userId,
      username: username,
      role: role,
      expiresAt: accessTokenExpiresAt,
    );
  }

  Future<AuthResult> login(String username, String password) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/public/auth/login',
      data: {'username': username, 'password': password},
    );

    if (resp == null) {
      throw ServiceApiException(code: 10000, message: '登录响应为空');
    }

    final token = resp['token'] as String? ?? resp['accessToken'] as String? ?? '';
    final refreshToken = resp['refreshToken'] as String?;
    final accessTokenExpiresAt = resp['accessTokenExpiresAt'] as String?;
    final sessionData = resp['session'] as Map<String, dynamic>?;
    final sessionId = sessionData?['sessionId'] as String? ?? resp['sessionId'] as String?;
    final userInfo = UserInfo.fromJson(resp);

    await saveSession(
      accessToken: token,
      refreshToken: refreshToken,
      sessionId: sessionId,
      accessTokenExpiresAt: accessTokenExpiresAt,
      userId: userInfo.id,
      username: userInfo.username,
      role: userInfo.role,
    );

    return AuthResult(
      accessToken: token,
      refreshToken: refreshToken,
      sessionId: sessionId,
      accessTokenExpiresAt: accessTokenExpiresAt,
      user: userInfo,
    );
  }

  Future<void> logout() async {
    try {
      await _api.post('/api/auth/logout');
    } catch (_) {}
    await _sessionStore.clear();
  }

  Future<Map<String, dynamic>?> setup(String username, String password) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/public/auth/setup',
      data: {'username': username, 'password': password},
    );
    return resp;
  }
}

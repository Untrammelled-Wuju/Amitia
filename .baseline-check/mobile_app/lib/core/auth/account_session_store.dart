import 'package:flutter_secure_storage/flutter_secure_storage.dart';

abstract interface class AccountSessionStore {
  Future<String?> getAccessToken();
  Future<void> setAccessToken(String? token);
  Future<String?> getRefreshToken();
  Future<void> setRefreshToken(String? token);
  Future<String?> getSessionId();
  Future<void> setSessionId(String? sessionId);
  Future<String?> getUserId();
  Future<void> setUserId(String? userId);
  Future<String?> getUsername();
  Future<void> setUsername(String? username);
  Future<String?> getRole();
  Future<void> setRole(String? role);
  Future<String?> getAccessTokenExpiresAt();
  Future<void> setAccessTokenExpiresAt(String? expiresAt);
  Future<void> setFullSession({
    required String accessToken,
    String? refreshToken,
    String? sessionId,
    String? userId,
    String? username,
    String? role,
    String? expiresAt,
  });
  Future<void> clear();
}

class FlutterSecureAccountSessionStore implements AccountSessionStore {
  static const _accessKey = 'amitia_access_token';
  static const _refreshKey = 'amitia_refresh_token';
  static const _sessionKey = 'amitia_session_id';
  static const _userIdKey = 'amitia_user_id';
  static const _usernameKey = 'amitia_username';
  static const _roleKey = 'amitia_role';
  static const _expiresAtKey = 'amitia_access_expires';

  final FlutterSecureStorage _storage;

  FlutterSecureAccountSessionStore([FlutterSecureStorage? storage])
      : _storage = storage ?? const FlutterSecureStorage();

  @override
  Future<String?> getAccessToken() => _storage.read(key: _accessKey);

  @override
  Future<void> setAccessToken(String? token) async {
    if (token == null) {
      await _storage.delete(key: _accessKey);
    } else {
      await _storage.write(key: _accessKey, value: token);
    }
  }

  @override
  Future<String?> getRefreshToken() => _storage.read(key: _refreshKey);

  @override
  Future<void> setRefreshToken(String? token) async {
    if (token == null) {
      await _storage.delete(key: _refreshKey);
    } else {
      await _storage.write(key: _refreshKey, value: token);
    }
  }

  @override
  Future<String?> getSessionId() => _storage.read(key: _sessionKey);

  @override
  Future<void> setSessionId(String? sessionId) async {
    if (sessionId == null) {
      await _storage.delete(key: _sessionKey);
    } else {
      await _storage.write(key: _sessionKey, value: sessionId);
    }
  }

  @override
  Future<String?> getUserId() => _storage.read(key: _userIdKey);

  @override
  Future<void> setUserId(String? userId) async {
    if (userId == null) {
      await _storage.delete(key: _userIdKey);
    } else {
      await _storage.write(key: _userIdKey, value: userId);
    }
  }

  @override
  Future<String?> getUsername() => _storage.read(key: _usernameKey);

  @override
  Future<void> setUsername(String? username) async {
    if (username == null) {
      await _storage.delete(key: _usernameKey);
    } else {
      await _storage.write(key: _usernameKey, value: username);
    }
  }

  @override
  Future<String?> getRole() => _storage.read(key: _roleKey);

  @override
  Future<void> setRole(String? role) async {
    if (role == null) {
      await _storage.delete(key: _roleKey);
    } else {
      await _storage.write(key: _roleKey, value: role);
    }
  }

  @override
  Future<String?> getAccessTokenExpiresAt() => _storage.read(key: _expiresAtKey);

  @override
  Future<void> setAccessTokenExpiresAt(String? expiresAt) async {
    if (expiresAt == null) {
      await _storage.delete(key: _expiresAtKey);
    } else {
      await _storage.write(key: _expiresAtKey, value: expiresAt);
    }
  }

  @override
  Future<void> setFullSession({
    required String accessToken,
    String? refreshToken,
    String? sessionId,
    String? userId,
    String? username,
    String? role,
    String? expiresAt,
  }) async {
    await setAccessToken(accessToken);
    if (refreshToken != null) await setRefreshToken(refreshToken);
    if (sessionId != null) await setSessionId(sessionId);
    if (userId != null) await setUserId(userId);
    if (username != null) await setUsername(username);
    if (role != null) await setRole(role);
    if (expiresAt != null) await setAccessTokenExpiresAt(expiresAt);
  }

  @override
  Future<void> clear() async {
    await _storage.deleteAll();
  }
}

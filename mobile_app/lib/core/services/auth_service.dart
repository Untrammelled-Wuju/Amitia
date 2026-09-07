import 'package:dio/dio.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import '../auth/account_session_store.dart';
import '../backend_transport/backend_service_api.dart';
import '../runtime/backend/backend_topology_resolver.dart';

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
  final String nickname;
  final String userLabel;
  final String bio;

  UserInfo({
    required this.id,
    required this.username,
    required this.role,
    this.nickname = '',
    this.userLabel = '',
    this.bio = '',
  });

  factory UserInfo.fromJson(Map<String, dynamic> json) {
    return UserInfo(
      id: (json['userId'] ?? json['id'] ?? '').toString(),
      username: json['username'] as String? ?? '',
      role: json['role'] as String? ?? 'user',
      nickname: json['nickname'] as String? ?? '',
      userLabel: json['userLabel'] as String? ?? '',
      bio: json['bio'] as String? ?? '',
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
    final role = await _sessionStore.getRole();
    if (userId == null || username == null) return null;
    return UserInfo(id: userId, username: username, role: role ?? 'user');
  }

  Future<UserInfo> fetchProfile() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/auth/me');
    if (resp == null) {
      throw ServiceApiException(code: 10000, message: '用户资料响应为空');
    }
    return UserInfo.fromJson(resp);
  }

  Future<UserInfo> updateProfile({
    required String nickname,
    required String userLabel,
    required String bio,
  }) async {
    final resp = await _api.put<Map<String, dynamic>>(
      '/api/auth/me',
      data: {
        'nickname': nickname,
        'userLabel': userLabel,
        'bio': bio,
      },
    );
    if (resp == null) {
      throw ServiceApiException(code: 10000, message: '更新用户资料响应为空');
    }
    return UserInfo.fromJson(resp);
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
    return _persistAuthResponse(resp, fallbackUsername: username);
  }

  Future<void> logout() async {
    try {
      await _api.post('/api/auth/logout');
    } catch (_) {}
    await _sessionStore.clear();
  }

  Future<void> logoutAll() async {
    await _api.post('/api/auth/logout-all');
    await _sessionStore.clear();
  }

  Future<List<Map<String, dynamic>>> sessions() async {
    final response = await _api.get<List<dynamic>>('/api/auth/sessions');
    if (response == null) return const [];
    return response.whereType<Map>().map((item) => Map<String, dynamic>.from(item)).toList();
  }

  Future<void> revokeSession(String sessionId) async {
    await _api.delete('/api/auth/sessions/${Uri.encodeComponent(sessionId)}');
  }

  Future<int> revokeOtherSessions() async {
    final response = await _api.deleteWithResponse<Map<String, dynamic>>('/api/auth/sessions');
    return (response?['revokedCount'] as num?)?.toInt() ?? 0;
  }

  Future<void> changePassword(String oldPassword, String newPassword) async {
    final response = await _api.post<Map<String, dynamic>>(
      '/api/auth/change-password',
      data: {'oldPassword': oldPassword, 'newPassword': newPassword},
    );
    if (response == null) throw ServiceApiException(code: 10000, message: '修改密码响应为空');
    final user = response['user'] is Map
        ? UserInfo.fromJson(Map<String, dynamic>.from(response['user'] as Map))
        : await currentUser;
    if (user == null) throw ServiceApiException(code: 10000, message: '用户会话信息缺失');
    final session = response['session'] is Map ? Map<String, dynamic>.from(response['session'] as Map) : const <String, dynamic>{};
    final accessToken = (response['accessToken'] ?? response['token'] ?? '').toString();
    if (accessToken.isEmpty) throw ServiceApiException(code: 10000, message: '新访问令牌缺失');
    await saveSession(
      accessToken: accessToken,
      refreshToken: response['refreshToken']?.toString(),
      sessionId: session['sessionId']?.toString(),
      accessTokenExpiresAt: response['accessTokenExpiresAt']?.toString(),
      userId: user.id,
      username: user.username,
      role: user.role,
    );
  }

  Dio _remoteBootstrapClient(String remoteCoreUri) {
    final baseUri = normalizeRemoteCoreUri(remoteCoreUri);
    return Dio(
      BaseOptions(
        baseUrl: baseUri.toString().replaceAll(RegExp(r'/+$'), ''),
        connectTimeout: const Duration(seconds: 8),
        receiveTimeout: const Duration(seconds: 30),
        headers: const <String, String>{
          'Accept': 'application/json',
          'X-Amitia-Client-Type': 'mobile',
        },
      ),
    );
  }

  Map<String, dynamic> _unwrapBootstrapResponse(dynamic raw) {
    if (raw is! Map) {
      throw ServiceApiException(code: 10000, message: 'Cloud Core 返回了无效响应');
    }
    final outer = Map<String, dynamic>.from(raw);
    final rawCode = outer['code'];
    if (rawCode is num && rawCode.toInt() != 200) {
      final detailData = outer['data'];
      final errorCode = detailData is Map ? detailData['errorCode']?.toString() : null;
      final message = (outer['message'] ?? outer['msg'] ?? '').toString().trim();
      throw ServiceApiException(
        code: rawCode.toInt(),
        message: message.isEmpty ? 'Cloud Core 请求失败' : message,
        detail: errorCode,
      );
    }
    final data = outer['data'];
    if (data is Map) return Map<String, dynamic>.from(data);
    return outer;
  }

  Future<AuthResult> _persistAuthResponse(
    Map<String, dynamic> resp, {
    required String fallbackUsername,
  }) async {
    final token = (resp['token'] ?? resp['accessToken'] ?? '').toString();
    if (token.isEmpty) {
      throw ServiceApiException(code: 10000, message: '认证响应未返回访问令牌');
    }
    final userRaw = resp['user'];
    final userMap = userRaw is Map
        ? Map<String, dynamic>.from(userRaw)
        : <String, dynamic>{...resp, if ((resp['username'] ?? '').toString().isEmpty) 'username': fallbackUsername};
    final parsedUser = UserInfo.fromJson(userMap);
    final user = parsedUser.username.isEmpty
        ? UserInfo(
            id: parsedUser.id,
            username: fallbackUsername,
            role: parsedUser.role,
            nickname: parsedUser.nickname,
            userLabel: parsedUser.userLabel,
            bio: parsedUser.bio,
          )
        : parsedUser;
    final sessionRaw = resp['session'];
    final session = sessionRaw is Map
        ? Map<String, dynamic>.from(sessionRaw)
        : const <String, dynamic>{};
    final refreshToken = resp['refreshToken']?.toString();
    final sessionId = session['sessionId']?.toString() ?? resp['sessionId']?.toString();
    final accessTokenExpiresAt = resp['accessTokenExpiresAt']?.toString();
    await saveSession(
      accessToken: token,
      refreshToken: refreshToken,
      sessionId: sessionId,
      accessTokenExpiresAt: accessTokenExpiresAt,
      userId: user.id,
      username: user.username,
      role: user.role,
    );
    return AuthResult(
      accessToken: token,
      refreshToken: refreshToken,
      sessionId: sessionId,
      accessTokenExpiresAt: accessTokenExpiresAt,
      user: user,
    );
  }

  Future<bool> hasAdminAt(String remoteCoreUri) async {
    final dio = _remoteBootstrapClient(remoteCoreUri);
    try {
      final response = await dio.get<dynamic>('/api/public/auth/status');
      final data = _unwrapBootstrapResponse(response.data);
      return data['hasAdmin'] == true;
    } finally {
      dio.close(force: true);
    }
  }

  Future<AuthResult> loginAt(
    String remoteCoreUri,
    String username,
    String password,
  ) async {
    final dio = _remoteBootstrapClient(remoteCoreUri);
    try {
      final response = await dio.post<dynamic>(
        '/api/public/auth/login',
        data: <String, dynamic>{'username': username, 'password': password},
      );
      return _persistAuthResponse(
        _unwrapBootstrapResponse(response.data),
        fallbackUsername: username,
      );
    } on ServiceApiException {
      rethrow;
    } on DioException catch (error) {
      throw ServiceApiException(
        code: error.response?.statusCode ?? 10000,
        message: 'Cloud Core 登录失败',
      );
    } finally {
      dio.close(force: true);
    }
  }

  Future<AuthResult> setupAndLoginAt(
    String remoteCoreUri,
    String username,
    String password, {
    required String setupToken,
  }) async {
    final token = setupToken.trim();
    if (token.length < 32) {
      throw ServiceApiException(code: 400, message: 'Cloud 初始化令牌至少 32 位');
    }
    final dio = _remoteBootstrapClient(remoteCoreUri);
    try {
      final response = await dio.post<dynamic>(
        '/api/public/auth/setup',
        data: <String, dynamic>{'username': username, 'password': password},
        options: Options(headers: <String, String>{'X-Amitia-Setup-Token': token}),
      );
      return _persistAuthResponse(
        _unwrapBootstrapResponse(response.data),
        fallbackUsername: username,
      );
    } on ServiceApiException {
      rethrow;
    } on DioException catch (error) {
      throw ServiceApiException(
        code: error.response?.statusCode ?? 10000,
        message: 'Cloud Core 首管理员初始化失败',
      );
    } finally {
      dio.close(force: true);
    }
  }

  Future<bool> hasAdmin() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/public/auth/status');
    return resp?['hasAdmin'] == true;
  }

  Future<AuthResult> setupAndLogin(
    String username,
    String password, {
    String? setupToken,
  }) async {
    final token = setupToken?.trim() ?? '';
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/public/auth/setup',
      data: {'username': username, 'password': password},
      headers: token.isEmpty ? null : {'X-Amitia-Setup-Token': token},
    );
    if (resp == null) {
      throw ServiceApiException(code: 10000, message: '初始化响应为空');
    }
    return _persistAuthResponse(resp, fallbackUsername: username);
  }

  Future<Map<String, dynamic>?> setup(String username, String password) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/public/auth/setup',
      data: {'username': username, 'password': password},
    );
    return resp;
  }
}

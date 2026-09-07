import 'dart:async';

import '../../ui_runtime/ui_device_identity.dart';
import '../errors/backend_transport_error.dart';
import '../errors/backend_transport_error_code.dart';
import '../http/backend_http_method.dart';
import '../http/backend_http_request.dart';
import '../http/backend_http_response.dart';
import '../http/backend_http_transport.dart';
import '../state/backend_http_state.dart';

const Duration _sessionRenewSkew = Duration(minutes: 2);

/// Short-lived management authority for the loopback Device Agent.
///
/// The embedded runtime intentionally exposes only the root local token through
/// the native bridge. Builder/package management routes require a Desktop
/// Session, so mobile must exchange that root token through `/api/local/sessions`
/// instead of weakening the server-side management boundary.
final class DeviceLocalManagementSession {
  DeviceLocalManagementSession(this._rootHttp, {UIDeviceIdentity? identity})
      : _identity = identity ?? UIDeviceIdentity();

  final BackendHttpTransport _rootHttp;
  final UIDeviceIdentity _identity;

  String? _sessionToken;
  String? _instanceId;
  DateTime? _expiresAt;
  Future<void>? _createInFlight;

  Future<Map<String, String>> authHeaders() async {
    await _ensureSession();
    final token = _sessionToken;
    final instanceId = _instanceId;
    if (token == null || token.isEmpty || instanceId == null || instanceId.isEmpty) {
      throw StateError('device-local management session is unavailable');
    }
    return <String, String>{
      'X-Amitia-Desktop-Session': token,
      'X-Amitia-Desktop-Instance': instanceId,
    };
  }

  void invalidate() {
    _sessionToken = null;
    _expiresAt = null;
  }

  Future<void> _ensureSession() async {
    final expiresAt = _expiresAt;
    if (_sessionToken != null &&
        expiresAt != null &&
        DateTime.now().add(_sessionRenewSkew).isBefore(expiresAt)) {
      return;
    }

    final inFlight = _createInFlight;
    if (inFlight != null) {
      await inFlight;
      return;
    }

    final create = _createSession();
    _createInFlight = create;
    try {
      await create;
    } finally {
      if (identical(_createInFlight, create)) {
        _createInFlight = null;
      }
    }
  }

  Future<void> _createSession() async {
    final instanceId = _instanceId ??= 'mobile:${await _identity.getOrCreate()}';
    final response = await _rootHttp.send(
      BackendHttpRequest(
        method: BackendHttpMethod.post,
        path: '/api/local/sessions',
        headers: <String, String>{
          'X-Amitia-Desktop-Instance': instanceId,
          'X-Request-ID': 'mobile-session-${DateTime.now().microsecondsSinceEpoch}',
        },
        body: <String, dynamic>{'desktopInstanceId': instanceId},
        timeout: const Duration(seconds: 5),
      ),
    );

    final payload = _unwrapPayload(response.data);
    final token = (payload['sessionToken'] ?? '').toString().trim();
    final expiresAt = DateTime.tryParse((payload['expiresAt'] ?? '').toString());
    if (token.isEmpty || expiresAt == null) {
      throw StateError('device-local backend returned an invalid management session');
    }

    _sessionToken = token;
    _expiresAt = expiresAt.toLocal();
  }

  Map<String, dynamic> _unwrapPayload(dynamic raw) {
    if (raw is! Map) {
      throw StateError('device-local backend returned an invalid session response');
    }
    final map = raw.cast<dynamic, dynamic>();
    final data = map['data'];
    if (data is Map) {
      return data.map((key, value) => MapEntry(key.toString(), value));
    }
    return map.map((key, value) => MapEntry(key.toString(), value));
  }
}

/// Adds a short-lived management session only to Device Agent routes that need
/// management authority. The underlying transport keeps injecting the root
/// LocalToken as a fallback credential; server authentication evaluates the
/// Desktop Session first, so no root-token permission is widened.
final class DeviceLocalSessionHttpTransport implements BackendHttpTransport {
  DeviceLocalSessionHttpTransport(this._rootHttp)
      : _session = DeviceLocalManagementSession(_rootHttp);

  final BackendHttpTransport _rootHttp;
  final DeviceLocalManagementSession _session;
  bool _closed = false;

  @override
  BackendHttpState get state => _rootHttp.state;

  @override
  Future<BackendHttpResponse> send(BackendHttpRequest request) async {
    if (_closed) {
      throw BackendTransportError(
        code: BackendTransportErrorCode.transportClosed,
        method: request.method.value,
        path: request.path,
      );
    }
    if (!_requiresManagementSession(request.path)) {
      return _rootHttp.send(request);
    }

    try {
      return await _sendWithSession(request);
    } on BackendTransportError catch (error) {
      if (error.code != BackendTransportErrorCode.authenticationFailed) rethrow;
      _session.invalidate();
      return _sendWithSession(request);
    }
  }

  Future<BackendHttpResponse> _sendWithSession(BackendHttpRequest request) async {
    final authHeaders = await _session.authHeaders();
    return _rootHttp.send(
      BackendHttpRequest(
        method: request.method,
        path: request.path,
        queryParameters: request.queryParameters,
        headers: <String, String>{...?request.headers, ...authHeaders},
        body: request.body,
        timeout: request.timeout,
        streamResponse: request.streamResponse,
        cancelToken: request.cancelToken,
      ),
    );
  }

  bool _requiresManagementSession(String path) {
    final normalized = path.split('?').first;
    if (normalized == '/api/local/sessions') return false;

    // Native runtime producer endpoints are deliberately LocalToken-only.
    // Do not attach a Desktop Session there because server authentication
    // intentionally prefers the session credential when both are present.
    const runtimeProducerPaths = <String>{
      '/api/local/workflows/trigger-capabilities/status',
      '/api/local/workflows/trigger-app-catalog/status',
      '/api/local/workflows/wake-runtime/status',
      '/api/local/workflows/wake-runtime/audio',
      '/api/local/workflows/wake-runtime/device-status',
      '/api/local/workflows/android-runtime-health/status',
    };
    if (runtimeProducerPaths.contains(normalized)) return false;

    return normalized == '/api/local/workflows' ||
        normalized.startsWith('/api/local/workflows/') ||
        normalized == '/api/local/workflow-runs' ||
        normalized.startsWith('/api/local/workflow-runs/') ||
        normalized == '/api/extensions/packages' ||
        normalized.startsWith('/api/extensions/packages/') ||
        normalized == '/api/extensions/kernel/extensions' ||
        normalized.startsWith('/api/extensions/kernel/extensions/');
  }

  @override
  Future<void> close() async {
    _closed = true;
    _session.invalidate();
    await _rootHttp.close();
  }
}

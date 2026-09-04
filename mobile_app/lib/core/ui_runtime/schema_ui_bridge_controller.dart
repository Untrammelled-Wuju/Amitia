import 'dart:async';

import '../services/extension_service.dart';
import '../../features/extensions/schema_ui/engine/action_dispatcher.dart';

typedef SchemaUIHostAction = FutureOr<dynamic> Function(dynamic input);

/// Owns the lifecycle of a Schema UI bridge session.
///
/// Both provider-hosted Schema UI and slot-hosted Schema UI must use this
/// controller so local host actions and extension-backed `ui.action.invoke`
/// actions follow the same contract on Flutter.
class SchemaUIBridgeController {
  SchemaUIBridgeController(this._service);

  final ExtensionService _service;

  String _sessionId = '';
  String _sessionOrigin = '';
  int _sessionContractVersion = 0;
  String _sessionToken = '';
  int _sessionGeneration = 0;
  int _nonceSequence = 0;
  String _sessionIdentity = '';
  Future<void>? _creatingSession;
  int _epoch = 0;
  bool _disposed = false;

  Future<dynamic> dispatch(
    ActionInvocation invocation, {
    required String contributionId,
    int contractVersion = 1,
    String characterId = '',
    String conversationId = '',
    Map<String, SchemaUIHostAction> localActions =
        const <String, SchemaUIHostAction>{},
  }) async {
    if (_disposed) throw StateError('Schema UI bridge controller is disposed');
    final local = localActions[invocation.actionId];
    if (local != null) {
      return Future.sync(
        () => local(invocation.input ?? const <String, dynamic>{}),
      );
    }

    final sourceContributionId = contributionId.trim();
    if (sourceContributionId.isEmpty) {
      throw StateError('Schema UI action does not reference a contribution');
    }

    await _ensureSession(
      contributionId: sourceContributionId,
      contractVersion: contractVersion,
      characterId: characterId.trim(),
      conversationId: conversationId.trim(),
    );
    if (_disposed || _sessionId.isEmpty) {
      throw StateError('UI session unavailable');
    }

    return _service.invokeSchemaUIBridge(
      _sessionId,
      contributionId: sourceContributionId,
      origin: _sessionOrigin,
      contractVersion: _sessionContractVersion,
      token: _sessionToken,
      generation: _sessionGeneration,
      nonce: _nextNonce(),
      actionId: invocation.actionId,
      input: invocation.input ?? const <String, dynamic>{},
    );
  }

  Future<dynamic> requestData(
    String key, {
    required String contributionId,
    int contractVersion = 1,
    String characterId = '',
    String conversationId = '',
    Map<String, dynamic> params = const <String, dynamic>{},
  }) async {
    if (_disposed) throw StateError('Schema UI bridge controller is disposed');
    final normalizedKey = key.trim();
    if (normalizedKey.isEmpty) throw StateError('Schema UI data source key is empty');
    final sourceContributionId = contributionId.trim();
    if (sourceContributionId.isEmpty) throw StateError('Schema UI data source does not reference a contribution');
    await _ensureSession(
      contributionId: sourceContributionId,
      contractVersion: contractVersion,
      characterId: characterId.trim(),
      conversationId: conversationId.trim(),
    );
    if (_disposed || _sessionId.isEmpty) throw StateError('UI session unavailable');
    return _service.invokeSchemaUIBridgeMessage(
      _sessionId,
      contributionId: sourceContributionId,
      origin: _sessionOrigin,
      contractVersion: _sessionContractVersion,
      token: _sessionToken,
      generation: _sessionGeneration,
      nonce: _nextNonce(),
      method: 'ui.data.request',
      payload: {'key': normalizedKey, 'params': params},
    );
  }

  Future<void> _ensureSession({
    required String contributionId,
    required int contractVersion,
    required String characterId,
    required String conversationId,
  }) async {
    if (_disposed) throw StateError('Schema UI bridge controller is disposed');
    final identity = '$contributionId\u0000$contractVersion\u0000$characterId\u0000$conversationId';
    if (_sessionId.isNotEmpty && _sessionIdentity == identity) return;

    final pending = _creatingSession;
    if (pending != null) {
      await pending;
      if (_sessionId.isNotEmpty && _sessionIdentity == identity) return;
      if (_disposed) throw StateError('Schema UI bridge controller is disposed');
    }

    if (_sessionIdentity.isNotEmpty && _sessionIdentity != identity) {
      await reset();
    }

    final completer = Completer<void>();
    _creatingSession = completer.future;
    final createEpoch = ++_epoch;
    _sessionIdentity = identity;
    try {
      final response = await _service.createSchemaUISession(
        contributionId: contributionId,
        characterId: characterId,
        conversationId: conversationId,
      );
      final id = (response['sessionId'] ?? response['session_id'] ?? '')
          .toString()
          .trim();
      if (id.isEmpty) {
        throw StateError('UI session response missing sessionId');
      }

      if (_disposed || createEpoch != _epoch || _sessionIdentity != identity) {
        await _safeRevoke(id);
        return;
      }

      _sessionId = id;
      _sessionOrigin = (response['origin'] ?? '').toString();
      _sessionContractVersion =
          (response['contractVersion'] as num?)?.toInt() ??
          (response['contract_version'] as num?)?.toInt() ??
          contractVersion;
      _sessionToken = (response['token'] ?? '').toString().trim();
      _sessionGeneration =
          (response['generation'] as num?)?.toInt() ?? 0;
      if (_sessionToken.isEmpty) {
        _sessionId = '';
        _sessionOrigin = '';
        _sessionContractVersion = 0;
        _sessionToken = '';
        _sessionGeneration = 0;
        _sessionIdentity = '';
        await _safeRevoke(id);
        throw StateError('UI session response missing token');
      }
    } finally {
      completer.complete();
      _creatingSession = null;
    }
  }

  /// Invalidates the current session. Any in-flight create that completes after
  /// this call is revoked instead of being published as the active session.
  Future<void> reset() async {
    _epoch++;
    _sessionIdentity = '';
    final id = _sessionId;
    _sessionId = '';
    _sessionOrigin = '';
    _sessionContractVersion = 0;
    _sessionToken = '';
    _sessionGeneration = 0;
    if (id.isNotEmpty) await _safeRevoke(id);
  }

  String _nextNonce() {
    _nonceSequence++;
    final now = DateTime.now().microsecondsSinceEpoch.toRadixString(36);
    return '$now-${_nonceSequence.toRadixString(36)}-${identityHashCode(this).toRadixString(36)}';
  }

  Future<void> _safeRevoke(String id) async {
    try {
      await _service.revokeSchemaUISession(id);
    } catch (_) {
      // Revocation is best effort. The server also expires abandoned UI sessions.
    }
  }

  Future<void> dispose() async {
    if (_disposed) return;
    _disposed = true;
    await reset();
  }
}

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/artifact/artifact_providers.dart';
import '../../../core/backend_connection/backend_connection_availability.dart';
import '../../../core/backend_connection/providers/backend_connection_providers.dart';

/// Uses the canonical Extension Package lifecycle for Game Center packages.
///
/// Game Center intentionally does not own a second installer. A game plugin is
/// an .amitiax extension whose preview targets `game_center` (or contributes a
/// `game_plugin`). All package writes flow through preview -> confirmation ->
/// package operation, exactly like the Extension Center.
class GameCenterPackageLifecycleClient {
  final Ref _ref;

  const GameCenterPackageLifecycleClient(this._ref);

  Future<Dio> _dio() async {
    final availability = await _ref.read(backendConnectionProvider.future);
    if (availability is! BackendConnectionAvailable) {
      throw StateError('后端当前不可用');
    }
    return createAuthenticatedDio(availability.config);
  }

  Future<Map<String, dynamic>> previewPackage(
    String archivePath, {
    String expectedExtensionId = '',
  }) async {
    Dio? dio;
    try {
      dio = await _dio();
      final fileName = archivePath.split(RegExp(r'[/\\]')).last;
      if (!fileName.toLowerCase().endsWith('.amitiax')) {
        throw StateError('请选择 .amitiax 游戏扩展包');
      }
      final response = await dio.post(
        '/api/extensions/packages/artifacts',
        data: FormData.fromMap({
          'scopeType': 'global',
          'scopeId': '',
          'file': await MultipartFile.fromFile(archivePath, filename: fileName),
        }),
      );
      final raw = response.data;
      if (raw is! Map || raw['preview'] is! Map) {
        throw StateError('后端未返回扩展包预览');
      }
      final preview = Map<String, dynamic>.from(raw['preview'] as Map);
      _validateGamePreview(preview, expectedExtensionId: expectedExtensionId);
      return preview;
    } finally {
      dio?.close(force: true);
    }
  }

  Future<String> commitPackage(
    Map<String, dynamic> preview, {
    String expectedExtensionId = '',
  }) async {
    _validateGamePreview(preview, expectedExtensionId: expectedExtensionId);
    final sessionId = (preview['sessionId'] ?? '').toString().trim();
    if (sessionId.isEmpty) throw StateError('预览会话无效');

    Dio? dio;
    try {
      dio = await _dio();
      final confirmResponse = await dio.post(
        '/api/extensions/packages/previews/${Uri.encodeComponent(sessionId)}/confirm',
        data: {
          'scopeType': (preview['scopeType'] ?? 'global').toString(),
          'scopeId': (preview['scopeId'] ?? '').toString(),
          'confirmations': buildConfirmations(preview),
        },
      );
      final confirmed = _unwrapData(confirmResponse.data);
      if (confirmed is! Map) throw StateError('安装确认失败');
      final token = (confirmed['confirmationToken'] ?? '').toString().trim();
      if (token.isEmpty) throw StateError('安装确认令牌缺失');

      final extensionId = (preview['id'] ?? '').toString().trim();
      final isUpdate = expectedExtensionId.trim().isNotEmpty ||
          (preview['currentVersion'] ?? '').toString().trim().isNotEmpty;
      final operationResponse = await dio.post(
        isUpdate
            ? '/api/extensions/packages/operations/update'
            : '/api/extensions/packages/operations/install',
        data: {
          'sessionId': sessionId,
          'scopeType': (preview['scopeType'] ?? 'global').toString(),
          'scopeId': (preview['scopeId'] ?? '').toString(),
          'confirmationToken': token,
          if (isUpdate) 'expectedExtensionId': expectedExtensionId.trim().isNotEmpty ? expectedExtensionId.trim() : extensionId,
          'idempotencyKey': 'mobile-game-package-${DateTime.now().microsecondsSinceEpoch}',
        },
      );
      final operation = _unwrapData(operationResponse.data);
      final operationId = operation is Map ? (operation['operationId'] ?? '').toString().trim() : '';
      if (operationId.isNotEmpty) {
        await _waitForOperation(dio, operationId);
      }
      return operationId;
    } finally {
      dio?.close(force: true);
    }
  }

  Future<Map<String, dynamic>> previewUninstall(String extensionId) async {
    final id = extensionId.trim();
    if (id.isEmpty) throw ArgumentError.value(extensionId, 'extensionId', 'must not be empty');
    Dio? dio;
    try {
      dio = await _dio();
      final response = await dio.post(
        '/api/extensions/kernel/extensions/uninstall/preview',
        data: {'extensionId': id, 'scopeType': 'global', 'scopeId': ''},
      );
      final raw = _unwrapData(response.data);
      if (raw is! Map) throw StateError('后端未返回卸载预览');
      return Map<String, dynamic>.from(raw);
    } finally {
      dio?.close(force: true);
    }
  }

  Future<String> commitUninstall(
    String extensionId,
    Map<String, dynamic> preview,
  ) async {
    final id = extensionId.trim();
    if (id.isEmpty) throw ArgumentError.value(extensionId, 'extensionId', 'must not be empty');
    if (preview['uninstallable'] == false) throw StateError('后端判定当前游戏扩展不可卸载');

    final confirmations = <String, bool>{};
    for (final value in (preview['requiredConfirmations'] as List?) ?? const []) {
      final key = value.toString().trim();
      if (key.isNotEmpty) confirmations[key] = true;
    }

    Dio? dio;
    try {
      dio = await _dio();
      final confirmResponse = await dio.post(
        '/api/extensions/kernel/extensions/uninstall/confirm',
        data: {
          'extensionId': id,
          'scopeType': 'global',
          'scopeId': '',
          'confirmations': confirmations,
        },
      );
      final confirmation = _unwrapData(confirmResponse.data);
      if (confirmation is! Map) throw StateError('卸载确认失败');
      final token = (confirmation['confirmationToken'] ?? '').toString().trim();
      if (token.isEmpty) throw StateError('卸载确认令牌缺失');

      final uninstallResponse = await dio.post(
        '/api/extensions/kernel/extensions/uninstall',
        data: {
          'extensionId': id,
          'scopeType': 'global',
          'scopeId': '',
          'confirmationToken': token,
        },
      );
      final operation = _unwrapData(uninstallResponse.data);
      final operationId = operation is Map ? (operation['operationId'] ?? '').toString().trim() : '';
      if (operationId.isNotEmpty) {
        await _waitForOperation(dio, operationId);
      }
      return operationId;
    } finally {
      dio?.close(force: true);
    }
  }

  Map<String, bool> buildConfirmations(Map<String, dynamic> preview) {
    final result = <String, bool>{};
    for (final value in (preview['capabilityConfirmations'] as List?) ?? const []) {
      final key = value.toString().trim();
      if (key.isNotEmpty) result[key] = true;
    }
    final signature = preview['signature'];
    final signatureStatus = signature is Map ? (signature['status'] ?? '').toString() : '';
    if (signatureStatus == 'unsigned') result['confirm.unsigned_dev'] = true;
    final scriptCount = (preview['scripts'] as num?)?.toInt() ?? 0;
    if (scriptCount > 0) result['confirm.scripts'] = true;
    if ((preview['currentVersion'] ?? '').toString().trim().isNotEmpty) {
      result['confirm.version_change'] = true;
    }
    if (((preview['highRiskCapabilities'] as List?) ?? const []).isNotEmpty) {
      result['confirm.permission_escalation'] = true;
    }
    if (preview['upgradeDiff'] is Map) {
      final diff = preview['upgradeDiff'] as Map;
      if (diff['signerChanged'] == true) result['confirm.signer_change'] = true;
      if (diff['configMigrationRequired'] == true) result['confirm.config_migration'] = true;
    }
    return result;
  }

  void _validateGamePreview(
    Map<String, dynamic> preview, {
    required String expectedExtensionId,
  }) {
    final target = (preview['managementTarget'] ?? '').toString();
    final kinds = ((preview['contributionKinds'] as List?) ?? const [])
        .map((e) => e.toString())
        .toSet();
    if (target != 'game_center' && !kinds.contains('game_plugin')) {
      throw StateError('该 .amitiax 包不是游戏扩展，已阻止从游戏中心安装');
    }
    if (preview['installable'] == false || preview['compatible'] == false) {
      throw StateError('该游戏扩展当前不可安装或与当前 Amitia 不兼容');
    }
    final expected = expectedExtensionId.trim();
    if (expected.isNotEmpty) {
      final actual = (preview['id'] ?? '').toString().trim();
      if (actual != expected) {
        throw StateError('更新包 ID 不匹配：需要 $expected，实际为 $actual');
      }
    }
  }

  dynamic _unwrapData(dynamic raw) {
    if (raw is Map && raw['data'] is Map) return raw['data'];
    return raw;
  }

  Future<void> _waitForOperation(Dio dio, String operationId) async {
    for (var attempt = 0; attempt < 120; attempt++) {
      final response = await dio.get(
        '/api/extensions/packages/operations/${Uri.encodeComponent(operationId)}',
      );
      final raw = _unwrapData(response.data);
      if (raw is Map) {
        final status = (raw['status'] ?? '').toString().toLowerCase();
        if (status == 'completed') return;
        if (status == 'failed' || status == 'requires_recovery') {
          final code = (raw['errorCode'] ?? raw['error'] ?? '扩展包操作失败').toString();
          throw StateError(code);
        }
      }
      await Future<void>.delayed(const Duration(milliseconds: 500));
    }
    throw StateError('扩展包操作等待超时，请刷新游戏中心检查最终状态');
  }
}

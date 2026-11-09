import 'dart:convert';

import '../../../core/backend_transport/backend_service_api.dart';
import 'desktop_pet_plugin_dto.dart';

class DesktopPetPluginApi {
  static const _basePath = '/api/extensions/pet/plugins';
  static const _packagesPath = '/api/extensions/packages';
  static const _managementTarget = 'pet-center';
  static const _managementTargetHeader = {
    'X-Amitia-Management-Target': _managementTarget,
  };

  final BackendServiceApi _api;

  DesktopPetPluginApi(this._api);

  String _id(String value) => Uri.encodeComponent(value.trim());

  T _requireData<T>(T? value, String operation) {
    if (value == null) {
      throw StateError('Desktop Pet Plugin $operation returned empty data');
    }
    return value;
  }

  Future<DesktopPetPluginList> list({
    int page = 1,
    int pageSize = 20,
    String? search,
  }) async {
    final query = <String, String>{'page': '$page', 'pageSize': '$pageSize'};
    final normalized = search?.trim();
    if (normalized != null && normalized.isNotEmpty) {
      query['search'] = normalized;
    }
    final resp = await _api.get<Map<String, dynamic>>(
      _basePath,
      queryParameters: query,
    );
    return DesktopPetPluginList.fromJson(_requireData(resp, 'list'));
  }

  Future<DesktopPetPluginDetail> detail(String pluginId) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '$_basePath/${_id(pluginId)}',
    );
    return DesktopPetPluginDetail.fromJson(_requireData(resp, 'detail'));
  }

  Future<DesktopPetPluginInstallResult> install(String packagePath) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '$_basePath/install',
      data: {'packagePath': packagePath},
    );
    return DesktopPetPluginInstallResult.fromJson(
      _requireData(resp, 'install'),
    );
  }

  Future<DesktopPetPluginInstallResult> update(
    String extensionId,
    String packagePath,
  ) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '$_basePath/actions/update',
      data: {'extensionId': extensionId, 'packagePath': packagePath},
    );
    return DesktopPetPluginInstallResult.fromJson(_requireData(resp, 'update'));
  }

  Future<DesktopPetPluginMutationResult> enable(String extensionId) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '$_basePath/actions/enable',
      data: {'extensionId': extensionId},
    );
    return DesktopPetPluginMutationResult.fromJson(
      _requireData(resp, 'enable'),
    );
  }

  Future<DesktopPetPluginMutationResult> disable(String extensionId) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '$_basePath/actions/disable',
      data: {'extensionId': extensionId},
    );
    return DesktopPetPluginMutationResult.fromJson(
      _requireData(resp, 'disable'),
    );
  }

  Future<DesktopPetPluginMutationResult> uninstall(String extensionId) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '$_basePath/actions/uninstall',
      data: {'extensionId': extensionId},
    );
    return DesktopPetPluginMutationResult.fromJson(
      _requireData(resp, 'uninstall'),
    );
  }

  Future<Map<String, dynamic>> previewPackage(
    String packagePath, {
    String expectedExtensionId = '',
  }) async {
    final path = packagePath.trim();
    if (!path.toLowerCase().endsWith('.petx')) {
      throw StateError('请选择 .petx 桌宠插件包: $path');
    }
    final response = await _api.postMultipart<Map<String, dynamic>>(
      '$_packagesPath/artifacts',
      fields: const {'scopeType': 'global', 'scopeId': ''},
      files: {
        'file': [path],
      },
      headers: _managementTargetHeader,
      fromJson: (dynamic value) => Map<String, dynamic>.from(value as Map),
    );
    final rawPreview = response?['preview'];
    if (rawPreview is! Map) {
      throw StateError('桌宠插件包预览响应缺少 preview: ${jsonEncode(response)}');
    }
    final preview = Map<String, dynamic>.from(rawPreview);
    final target = (preview['managementTarget'] ?? '').toString().trim();
    final kinds = ((preview['contributionKinds'] as List?) ?? const [])
        .map((item) => item.toString())
        .toSet();
    if (target != 'pet_center' && !kinds.contains('petx')) {
      throw StateError(
        '该扩展包不是桌宠插件，已阻止从桌宠中心安装；原始预览: '
        '${jsonEncode(preview)}',
      );
    }
    final errors = ((preview['errors'] as List?) ?? const [])
        .map((item) => item.toString())
        .where((item) => item.trim().isNotEmpty)
        .toList();
    if (preview['installable'] == false ||
        preview['compatible'] == false ||
        errors.isNotEmpty) {
      throw StateError('桌宠插件包当前不可安装；原始预览: ${jsonEncode(preview)}');
    }
    final expected = expectedExtensionId.trim();
    if (expected.isNotEmpty && (preview['id'] ?? '').toString() != expected) {
      throw StateError(
        '更新包 ID 不匹配：需要 $expected，实际为 ${preview['id']}；原始预览: '
        '${jsonEncode(preview)}',
      );
    }
    return preview;
  }

  Future<String> commitPackagePreview(
    Map<String, dynamic> preview, {
    String expectedExtensionId = '',
  }) async {
    final sessionId = (preview['sessionId'] ?? '').toString().trim();
    if (sessionId.isEmpty) {
      throw StateError('桌宠插件包预览会话无效: ${jsonEncode(preview)}');
    }
    final previewExtensionId = (preview['id'] ?? '').toString().trim();
    final expected = expectedExtensionId.trim();
    final isUpdate =
        expected.isNotEmpty ||
        (preview['currentVersion'] ?? '').toString().trim().isNotEmpty;
    final targetExtensionId = expected.isNotEmpty
        ? expected
        : previewExtensionId;
    final idempotencyKey =
        'mobile-pet-package-${isUpdate ? 'update' : 'install'}-'
        '${DateTime.now().microsecondsSinceEpoch}';
    final confirmed = await _api.post<Map<String, dynamic>>(
      '$_packagesPath/previews/${Uri.encodeComponent(sessionId)}/confirm',
      data: {
        'scopeType': (preview['scopeType'] ?? 'global').toString(),
        'scopeId': (preview['scopeId'] ?? '').toString(),
        'confirmations': _buildConfirmations(preview),
      },
      headers: _managementTargetHeader,
      fromJson: (dynamic value) => Map<String, dynamic>.from(value as Map),
    );
    final token = (confirmed?['confirmationToken'] ?? '').toString().trim();
    if (token.isEmpty) {
      throw StateError('桌宠插件安装确认令牌缺失: ${jsonEncode(confirmed)}');
    }
    final operation = await _api.post<Map<String, dynamic>>(
      isUpdate
          ? '$_packagesPath/operations/update'
          : '$_packagesPath/operations/install',
      data: {
        'sessionId': sessionId,
        'scopeType': (preview['scopeType'] ?? 'global').toString(),
        'scopeId': (preview['scopeId'] ?? '').toString(),
        'confirmationToken': token,
        if (isUpdate && targetExtensionId.isNotEmpty)
          'expectedExtensionId': targetExtensionId,
        'idempotencyKey': idempotencyKey,
      },
      headers: {..._managementTargetHeader, 'Idempotency-Key': idempotencyKey},
      fromJson: (dynamic value) => Map<String, dynamic>.from(value as Map),
    );
    final operationId = (operation?['operationId'] ?? '').toString().trim();
    if (operationId.isEmpty) {
      throw StateError('桌宠插件包操作缺少 operationId: ${jsonEncode(operation)}');
    }
    await _waitForOperation(operationId);
    return operationId;
  }

  Map<String, bool> _buildConfirmations(Map<String, dynamic> preview) {
    final result = <String, bool>{};
    for (final value
        in (preview['capabilityConfirmations'] as List?) ?? const []) {
      final key = value.toString().trim();
      if (key.isNotEmpty) result[key] = true;
    }
    final signature = preview['signature'];
    if (signature is Map &&
        (signature['status'] ?? '').toString() == 'unsigned') {
      result['confirm.unsigned_dev'] = true;
    }
    if (((preview['scripts'] as num?)?.toInt() ?? 0) > 0) {
      result['confirm.scripts'] = true;
    }
    if ((preview['currentVersion'] ?? '').toString().trim().isNotEmpty) {
      result['confirm.version_change'] = true;
    }
    if (((preview['highRiskCapabilities'] as List?) ?? const []).isNotEmpty) {
      result['confirm.permission_escalation'] = true;
    }
    final diff = preview['upgradeDiff'];
    if (diff is Map) {
      if (diff['signerChanged'] == true) {
        result['confirm.signer_change'] = true;
      }
      if (diff['configMigrationRequired'] == true) {
        result['confirm.config_migration'] = true;
      }
    }
    return result;
  }

  Future<void> _waitForOperation(String operationId) async {
    for (var attempt = 0; attempt < 60; attempt++) {
      final operation = await _api.get<Map<String, dynamic>>(
        '$_packagesPath/operations/${Uri.encodeComponent(operationId)}',
        headers: _managementTargetHeader,
        fromJson: (dynamic value) => Map<String, dynamic>.from(value as Map),
      );
      final status = (operation?['status'] ?? '').toString().toLowerCase();
      if (status == 'completed') return;
      if (status == 'failed' || status == 'requires_recovery') {
        throw StateError('桌宠插件包操作失败: ${jsonEncode(operation)}');
      }
      final delayMs = switch (attempt) {
        < 3 => 250,
        < 8 => 500,
        < 15 => 1000,
        _ => 2000,
      };
      await Future<void>.delayed(Duration(milliseconds: delayMs));
    }
    throw StateError('桌宠插件包操作等待超时，operationId=$operationId');
  }
}

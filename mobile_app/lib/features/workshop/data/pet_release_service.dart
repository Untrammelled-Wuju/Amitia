import 'dart:async';

import '../../../core/backend_transport/backend_service_api.dart';

class PetRelease {
  const PetRelease({
    required this.id,
    required this.petId,
    required this.version,
    required this.lifecycle,
    required this.defaultActionKey,
    required this.actionCount,
    required this.contentRootHash,
    required this.manifestHash,
  });

  final String id;
  final String petId;
  final String version;
  final String lifecycle;
  final String defaultActionKey;
  final int actionCount;
  final String contentRootHash;
  final String manifestHash;

  factory PetRelease.fromJson(Map<String, dynamic> json) {
    final id = (json['id'] ?? '').toString();
    final petId = (json['petId'] ?? '').toString();
    if (id.isEmpty || petId.isEmpty) {
      throw StateError('桌宠发布记录缺少编号');
    }
    return PetRelease(
      id: id,
      petId: petId,
      version: (json['version'] ?? '').toString(),
      lifecycle: (json['lifecycle'] ?? json['status'] ?? '').toString(),
      defaultActionKey: (json['defaultActionKey'] ?? '').toString(),
      actionCount: (json['actionCount'] as num?)?.toInt() ?? 0,
      contentRootHash: (json['contentRootHash'] ?? '').toString(),
      manifestHash: (json['manifestHash'] ?? '').toString(),
    );
  }
}

class PetInstallationOperation {
  const PetInstallationOperation({
    required this.id,
    required this.installationId,
    required this.status,
    required this.stage,
    required this.errorCode,
    required this.errorMessage,
  });

  final String id;
  final String installationId;
  final String status;
  final String stage;
  final String errorCode;
  final String errorMessage;

  bool get terminal =>
      const {'completed', 'failed_terminal', 'cancelled'}.contains(status);

  bool get failed => const {'failed_terminal', 'cancelled'}.contains(status);

  factory PetInstallationOperation.fromJson(Map<String, dynamic> json) {
    final id = (json['id'] ?? json['operationId'] ?? '').toString();
    final status = (json['status'] ?? '').toString();
    if (id.isEmpty || status.isEmpty) {
      throw StateError('桌宠安装操作响应无效');
    }
    return PetInstallationOperation(
      id: id,
      installationId: (json['installationId'] ?? '').toString(),
      status: status,
      stage: (json['stage'] ?? '').toString(),
      errorCode: (json['errorCode'] ?? '').toString(),
      errorMessage: (json['errorMessage'] ?? '').toString(),
    );
  }
}

class PetReleaseService {
  PetReleaseService(this._api);

  final BackendServiceApi _api;
  int _keyCounter = 0;

  Future<PetRelease> buildRelease({
    required String processingTaskId,
    required String defaultAction,
    required List<String> includedActions,
    String petId = '',
  }) async {
    if (processingTaskId.trim().isEmpty) {
      throw ArgumentError.value(processingTaskId, 'processingTaskId');
    }
    if (defaultAction.trim().isEmpty || includedActions.isEmpty) {
      throw ArgumentError('桌宠发布必须包含默认动作和至少一个动作');
    }
    final response = await _api.post<Map<String, dynamic>>(
      '/api/desktop-pets/releases/build',
      data: {
        'processingTaskId': processingTaskId,
        'petId': petId,
        'defaultAction': defaultAction,
        'includedActions': includedActions,
        'idempotencyKey': _newKey('release'),
      },
    );
    final release = response?['release'];
    if (release is! Map) throw StateError('桌宠发布响应缺少 release');
    return PetRelease.fromJson(Map<String, dynamic>.from(release));
  }

  Future<List<PetRelease>> listReleases() async {
    final response = await _api.get<Map<String, dynamic>>(
      '/api/desktop-pets/releases',
    );
    final items = response?['items'];
    if (items is! List) throw StateError('桌宠发布列表响应无效');
    return items
        .whereType<Map>()
        .map((item) => PetRelease.fromJson(Map<String, dynamic>.from(item)))
        .where((item) => item.lifecycle == 'ready')
        .toList(growable: false);
  }

  Future<List<Map<String, dynamic>>> listInstallations(String deviceId) async {
    _requireDevice(deviceId);
    final response = await _api.get<Map<String, dynamic>>(
      '/api/desktop-pets/installations',
      headers: _deviceHeaders(deviceId),
    );
    final items = response?['items'];
    if (items is! List) throw StateError('桌宠安装列表响应无效');
    return items
        .whereType<Map>()
        .map((item) => Map<String, dynamic>.from(item))
        .toList(growable: false);
  }

  Future<PetInstallationOperation> install(
    String deviceId,
    PetRelease release,
  ) {
    return _startOperation(
      deviceId,
      '/api/desktop-pets/pets/${Uri.encodeComponent(release.petId)}/releases/${Uri.encodeComponent(release.id)}/install',
      const {},
    );
  }

  Future<PetInstallationOperation> upgrade(
    String deviceId,
    String installationId,
    String releaseId,
  ) {
    return _startOperation(
      deviceId,
      '/api/desktop-pets/installations/${Uri.encodeComponent(installationId)}/upgrade',
      {'targetReleaseId': releaseId},
    );
  }

  Future<PetInstallationOperation> switchRelease(
    String deviceId,
    String installationId,
    String releaseId,
  ) {
    return _startOperation(
      deviceId,
      '/api/desktop-pets/installations/${Uri.encodeComponent(installationId)}/switch',
      {'targetReleaseId': releaseId},
    );
  }

  Future<PetInstallationOperation> downgrade(
    String deviceId,
    String installationId,
    String releaseId, {
    required bool safetyConfirm,
  }) {
    return _startOperation(
      deviceId,
      '/api/desktop-pets/installations/${Uri.encodeComponent(installationId)}/downgrade',
      {'targetReleaseId': releaseId, 'safetyConfirm': safetyConfirm},
    );
  }

  Future<PetInstallationOperation> repair(
    String deviceId,
    String installationId,
  ) {
    return _startOperation(
      deviceId,
      '/api/desktop-pets/installations/${Uri.encodeComponent(installationId)}/repair',
      const {},
    );
  }

  Future<PetInstallationOperation> setEnabled(
    String deviceId,
    String installationId,
    bool enabled,
  ) {
    return _startOperation(
      deviceId,
      '/api/desktop-pets/installations/${Uri.encodeComponent(installationId)}/${enabled ? 'enable' : 'disable'}',
      const {},
    );
  }

  Future<PetInstallationOperation> changeDefaultAction(
    String deviceId,
    String installationId,
    String actionKey,
  ) {
    return _startPatchOperation(
      deviceId,
      '/api/desktop-pets/installations/${Uri.encodeComponent(installationId)}/default-action',
      {'actionKey': actionKey},
    );
  }

  Future<PetInstallationOperation> recenter(
    String deviceId,
    String installationId,
  ) {
    return _startOperation(
      deviceId,
      '/api/desktop-pets/installations/${Uri.encodeComponent(installationId)}/recenter',
      const {},
    );
  }

  Future<PetInstallationOperation> uninstall(
    String deviceId,
    String installationId,
  ) {
    return _startDeleteOperation(
      deviceId,
      '/api/desktop-pets/installations/${Uri.encodeComponent(installationId)}',
    );
  }

  Future<PetInstallationOperation> waitForOperation(
    String deviceId,
    PetInstallationOperation initial, {
    int attempts = 40,
    Duration interval = const Duration(milliseconds: 500),
  }) async {
    var current = initial;
    for (var attempt = 0; attempt < attempts && !current.terminal; attempt++) {
      await Future<void>.delayed(interval);
      current = await _readOperation(deviceId, current.id);
    }
    return current;
  }

  Future<PetInstallationOperation> _startOperation(
    String deviceId,
    String path,
    Map<String, dynamic> payload,
  ) async {
    _requireDevice(deviceId);
    final idempotencyKey = _newKey('operation');
    final response = await _api.post<Map<String, dynamic>>(
      path,
      data: payload,
      headers: {..._deviceHeaders(deviceId), 'Idempotency-Key': idempotencyKey},
    );
    final value = response;
    if (value == null) throw StateError('桌宠安装操作响应为空');
    return PetInstallationOperation.fromJson(value);
  }

  Future<PetInstallationOperation> _startPatchOperation(
    String deviceId,
    String path,
    Map<String, dynamic> payload,
  ) async {
    _requireDevice(deviceId);
    final response = await _api.patch<Map<String, dynamic>>(
      path,
      data: payload,
      headers: {
        ..._deviceHeaders(deviceId),
        'Idempotency-Key': _newKey('operation'),
      },
    );
    if (response == null) throw StateError('桌宠安装操作响应为空');
    return PetInstallationOperation.fromJson(response);
  }

  Future<PetInstallationOperation> _startDeleteOperation(
    String deviceId,
    String path,
  ) async {
    _requireDevice(deviceId);
    final response = await _api.deleteWithResponse<Map<String, dynamic>>(
      path,
      headers: {
        ..._deviceHeaders(deviceId),
        'Idempotency-Key': _newKey('operation'),
      },
    );
    if (response == null) throw StateError('桌宠安装操作响应为空');
    return PetInstallationOperation.fromJson(response);
  }

  Future<PetInstallationOperation> _readOperation(
    String deviceId,
    String operationId,
  ) async {
    final response = await _api.get<Map<String, dynamic>>(
      '/api/desktop-pets/operations/${Uri.encodeComponent(operationId)}',
      headers: _deviceHeaders(deviceId),
    );
    final operation = response?['operation'];
    if (operation is! Map) throw StateError('桌宠安装操作状态响应无效');
    return PetInstallationOperation.fromJson(
      Map<String, dynamic>.from(operation),
    );
  }

  Map<String, String> _deviceHeaders(String deviceId) => {
    'X-Amitia-Device-ID': deviceId,
  };

  void _requireDevice(String deviceId) {
    if (deviceId.trim().isEmpty) throw StateError('缺少桌宠运行设备编号');
  }

  String _newKey(String action) {
    _keyCounter++;
    return 'mobile-pet-$action-${DateTime.now().microsecondsSinceEpoch}-$_keyCounter';
  }
}

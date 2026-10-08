// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only

import 'dart:math';
import 'dart:convert';

import '../backend_transport/backend_service_api.dart';

class ExtensionTaskService {
  final BackendServiceApi _api;
  final bool Function()? _sourceIsCurrent;
  final Object _sourceAuthority = Object();
  final bool Function()? _isBound;
  final bool Function()? _taskIsCurrent;
  final Object _taskAuthority = Object();

  ExtensionTaskService(
    this._api, {
    bool Function()? sourceIsCurrent,
    bool Function()? isBound,
    bool Function()? taskIsCurrent,
  }) : _sourceIsCurrent = sourceIsCurrent,
       _isBound = isBound,
       _taskIsCurrent = taskIsCurrent;

  bool get bound => _isBound?.call() == true;
  static String _taskStamp(Map scope) => jsonEncode(
    (scope.keys
            .where(
              (key) =>
                  !const ['requestId', 'turnId', 'executionId'].contains(key),
            )
            .toList()
          ..sort())
        .map((key) => [key, scope[key]])
        .toList(),
  );

  Future<Map<String, dynamic>?> _checkTask([
    Map<String, dynamic>? expected,
  ]) async {
    if (!bound) {
      if (expected?['_taskAuthority'] != null)
        throw StateError('原 Core 任务不能操作到本机');
      return null;
    }
    if (_taskIsCurrent?.call() != true ||
        (expected != null &&
            !identical(expected['_taskAuthority'], _taskAuthority)))
      throw StateError('任务服务归属已变化，请重新打开原任务列表');
    final state = await _api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/coordination/me',
    );
    if (_taskIsCurrent?.call() != true ||
        state?['coordinationAvailable'] != true ||
        state?['coreId'] is! String ||
        (state!['coreId'] as String).isEmpty ||
        state['policy'] is! Map ||
        ['providerEpoch', 'modeRevision', 'permissionRevision'].any(
          (key) => state['policy'][key] is! int || state['policy'][key] < 1,
        ) ||
        state['policy']['coordinated'] is! bool)
      throw StateError('任务 Core 服务尚未就绪');
    if (expected != null) {
      final management = expected['managementExecutionScope'];
      if (expected['executionScope'] is! Map ||
          management is! Map ||
          management['coreId'] != state['coreId'] ||
          [
            'providerEpoch',
            'modeRevision',
            'permissionRevision',
            'coordinated',
          ].any((key) => management[key] != state['policy'][key]))
        throw StateError('任务原权限范围已变化，请重新打开列表');
    }
    return state;
  }

  Map<String, dynamic> _taskRow(Map value, Map? policy) {
    if (policy == null) return Map<String, dynamic>.from(value);
    final scope = value['executionScope'];
    final management = value['managementExecutionScope'];
    if (scope is! Map ||
        management is! Map ||
        scope['coreId'] != policy['coreId'] ||
        management['coreId'] != policy['coreId'] ||
        value['ownerId'] != scope['resourceOwnerId'] ||
        value['readOnly'] is! bool ||
        [
          'providerEpoch',
          'modeRevision',
          'permissionRevision',
          'coordinated',
        ].any((key) => management[key] != policy['policy'][key]))
      throw StateError('任务响应缺少有效的原所有者与管理范围');
    return Map<String, dynamic>.unmodifiable({
      ...Map<String, dynamic>.from(value),
      'executionScope': Map<String, dynamic>.unmodifiable(scope),
      'managementExecutionScope': Map<String, dynamic>.unmodifiable(management),
      '_taskAuthority': _taskAuthority,
    });
  }

  void _requireTaskId(String id, Map<String, dynamic>? expected) {
    if (bound && (expected == null || expected['taskRunId'] != id)) {
      throw StateError('任务 ID 与原始范围不一致');
    }
  }

  Future<void> _taskControl(
    String id,
    String action,
    Map<String, dynamic> data,
    Map<String, dynamic>? expected,
  ) async {
    _requireTaskId(id, expected);
    final policy = await _checkTask(expected);
    if (bound && expected?['readOnly'] != false) {
      throw StateError('历史任务只读或缺少原始范围');
    }
    final result = await _api.post<Map<String, dynamic>>(
      '/api/extensions/tasks/${Uri.encodeComponent(id)}/$action',
      data: {
        ...data,
        if (expected != null)
          'expectedExecutionScope': expected['executionScope'],
      },
    );
    await _checkTask(expected);
    if (policy != null) {
      if (result == null ||
          (action == 'retry'
              ? result['taskRunId'] is! String ||
                    (result['taskRunId'] as String).isEmpty
              : result['taskRunId'] != id) ||
          result['executionScope'] is! Map ||
          _taskStamp(result['executionScope'] as Map) !=
              _taskStamp(expected!['executionScope'] as Map)) {
        throw StateError('任务操作响应归属已变化，请重新打开列表确认结果');
      }
      _taskRow(result, policy);
    }
  }

  void _requireSource([Map<String, dynamic>? approval]) {
    if (_sourceIsCurrent == null) return;
    if (_api.generation <= 0 ||
        !_sourceIsCurrent() ||
        (approval != null &&
            !identical(approval['_sourceAuthority'], _sourceAuthority))) {
      throw StateError('本机 Runtime 或审批归属已变化，请重新打开本机审批');
    }
  }

  Future<void> _validateSourceApproval(Map<String, dynamic> approval) async {
    _requireSource(approval);
    if (_sourceIsCurrent == null) return;
    final values = await listSourceTaskApprovals();
    final current = values
        .where((item) => item['id'] == approval['id'])
        .firstOrNull;
    if (current == null ||
        current['revision'] != approval['revision'] ||
        current['status'] != approval['status'] ||
        jsonEncode(current['binding']) != jsonEncode(approval['binding'])) {
      throw StateError('审批的配对、角色、权限或执行状态已变化，请重新打开');
    }
    _requireSource(approval);
  }

  Future<List<Map<String, dynamic>>> listSourceTaskApprovals() async {
    _requireSource();
    final values = await _api.get<List<dynamic>>(
      '/internal/device-mesh/task-approvals',
    );
    _requireSource();
    if (values == null || values.length > 64) throw StateError('本机审批列表无效');
    final result = <Map<String, dynamic>>[];
    for (final value in values) {
      if (value is! Map) throw StateError('本机审批身份无效');
      final item = Map<String, dynamic>.from(value);
      final binding = item['binding'];
      final permissions = item['permissions'];
      if (item['id'] is! String ||
          (item['id'] as String).isEmpty ||
          item['revision'] is! int ||
          item['revision'] < 1 ||
          ![
            'pending',
            'approved',
            'denied',
            'claimed',
            'revoked',
          ].contains(item['status']) ||
          binding is! Map ||
          binding['executionScope'] is! Map ||
          binding['executionScope']['coreId'] is! String ||
          binding['executionScope']['targetDeviceId'] is! String ||
          binding['taskRunId'] is! String ||
          (binding['taskRunId'] as String).isEmpty ||
          binding['taskGeneration'] is! int ||
          binding['taskGeneration'] < 1 ||
          binding['executionTarget'] is! Map ||
          binding['executionTarget']['spaceId'] !=
              binding['executionScope']['coreId'] ||
          binding['executionTarget']['deviceId'] !=
              binding['executionScope']['targetDeviceId'] ||
          binding['executionTarget']['runtimeId'] is! String ||
          (binding['executionTarget']['runtimeId'] as String).isEmpty ||
          binding['executionTarget']['runtimeSessionId'] is! String ||
          (binding['executionTarget']['runtimeSessionId'] as String).isEmpty ||
          binding['executionTarget']['connectionGeneration'] is! int ||
          binding['executionTarget']['connectionGeneration'] < 1 ||
          binding['target'] is! Map ||
          binding['target']['taskId'] is! String ||
          binding['target']['installedGeneration'] is! int ||
          binding['target']['installedGeneration'] < 1 ||
          binding['inputHash'] is! String ||
          !RegExp(r'^[a-f0-9]{64}$').hasMatch(binding['inputHash']) ||
          permissions is! List ||
          permissions.length > 64 ||
          permissions.any(
            (permission) =>
                permission is! Map ||
                permission['permissionId'] is! String ||
                (permission['permissionId'] as String).isEmpty,
          )) {
        throw StateError('目标设备返回的审批身份或资源声明无效');
      }
      result.add({
        ...item,
        if (_sourceIsCurrent != null) '_sourceAuthority': _sourceAuthority,
      });
    }
    return result;
  }

  Future<void> decideSourceTaskApproval(
    Map<String, dynamic> approval,
    bool approved,
  ) async {
    await _validateSourceApproval(approval);
    if (approval['status'] != 'pending' ||
        approval['revision'] is! int ||
        approval['revision'] < 1) {
      throw StateError('审批已处理，请刷新列表');
    }
    final value = await _api.post<Map<String, dynamic>>(
      '/internal/device-mesh/task-approvals/${Uri.encodeComponent(approval['id'])}/decision',
      data: {'expectedRevision': approval['revision'], 'approved': approved},
    );
    _requireSource(approval);
    if (value == null ||
        value['id'] != approval['id'] ||
        value['revision'] != approval['revision'] + 1 ||
        value['status'] != (approved ? 'approved' : 'denied') ||
        jsonEncode(value['binding']) != jsonEncode(approval['binding'])) {
      throw StateError('设备未确认本次审批决定，请刷新列表');
    }
  }

  Future<List<Map<String, dynamic>>> listDefinitions({int limit = 200}) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/extensions/task-definitions',
      queryParameters: {'limit': limit},
    );
    return _items(resp);
  }

  Future<void> revokeSourceTaskApproval(Map<String, dynamic> approval) async {
    await _validateSourceApproval(approval);
    if (!['approved', 'claimed'].contains(approval['status']) ||
        approval['revision'] is! int ||
        approval['revision'] < 1) {
      throw StateError('审批状态已变化，请刷新列表');
    }
    final value = await _api.post<Map<String, dynamic>>(
      '/internal/device-mesh/task-approvals/${Uri.encodeComponent(approval['id'])}/revoke',
      data: {'expectedRevision': approval['revision']},
    );
    _requireSource(approval);
    if (value == null ||
        value['id'] != approval['id'] ||
        value['revision'] != approval['revision'] + 1 ||
        value['status'] != 'revoked' ||
        jsonEncode(value['binding']) != jsonEncode(approval['binding'])) {
      throw StateError('设备未确认本次撤销，请刷新列表');
    }
  }

  Future<List<Map<String, dynamic>>> listRuns({int limit = 200}) async {
    final policy = await _checkTask();
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/extensions/tasks',
      queryParameters: {'limit': limit},
    );
    final current = await _checkTask();
    if (jsonEncode([policy?['coreId'], policy?['policy']]) !=
        jsonEncode([current?['coreId'], current?['policy']])) {
      throw StateError('任务列表加载期间服务范围已变化');
    }
    return _items(resp).map((row) => _taskRow(row, policy)).toList();
  }

  Future<Map<String, dynamic>?> enqueue({
    required String taskDefinitionId,
    required String extensionId,
    required String moduleId,
    required Map<String, dynamic> input,
    int priority = 0,
    String source = 'mobile',
    String? deviceId,
  }) {
    return _api.post<Map<String, dynamic>>(
      '/api/extensions/tasks',
      data: {
        'taskDefinitionId': taskDefinitionId,
        'extensionId': extensionId,
        'moduleId': moduleId,
        'input': input,
        'priority': priority,
        'source': source,
        if ((deviceId ?? '').trim().isNotEmpty) 'deviceId': deviceId!.trim(),
      },
    );
  }

  Future<Map<String, dynamic>> prepareOwnedTask(String targetDeviceId) async {
    final before = await _api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/coordination/me',
    );
    if (before?['coreId'] is! String ||
        before?['coordinationAvailable'] != true) {
      throw StateError('Core 的设备任务服务尚未就绪');
    }
    final result = await _api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/tasks/roles',
      queryParameters: {'targetDeviceId': targetDeviceId},
    );
    final after = await _api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/coordination/me',
    );
    final scope = result?['executionScope'];
    final roles = result?['roles'];
    if (after?['coordinationAvailable'] != true ||
        after?['coreId'] != before!['coreId'] ||
        scope is! Map ||
        scope['coreId'] != before['coreId'] ||
        scope['targetDeviceId'] != targetDeviceId ||
        result?['roleOwnerId'] != scope['roleOwnerId'] ||
        result?['roleOwnerId'] != scope['resourceOwnerId'] ||
        roles is! List ||
        roles.any(
          (role) =>
              role is! Map ||
              role['id'] is! String ||
              (role['id'] as String).isEmpty ||
              role['revision'] is! int ||
              (role['revision'] as int) < 1,
        )) {
      throw StateError('服务提供者或目标数据归属已变化，请刷新后重新选择角色');
    }
    return result!;
  }

  Future<Map<String, dynamic>> enqueueOwnedTask(
    Map<String, dynamic> submission,
  ) async {
    final result = await _api.post<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/tasks',
      data: submission,
    );
    final task = result?['task'];
    final scope = result?['executionScope'];
    final expected = submission['expectedExecutionScope'];
    final scopeMatches =
        scope is Map &&
        expected is Map &&
        expected.entries.every(
          (entry) =>
              ['requestId', 'turnId', 'executionId'].contains(entry.key) ||
              scope[entry.key] == entry.value,
        );
    if (task is! Map ||
        task['taskRunId'] is! String ||
        (task['taskRunId'] as String).isEmpty ||
        scope is! Map ||
        !scopeMatches ||
        scope['coreId'] != submission['expectedCoreId'] ||
        scope['targetDeviceId'] != submission['targetDeviceId'] ||
        scope['roleId'] != submission['characterId'] ||
        scope['roleRevision'] != submission['expectedRoleRevision'] ||
        scope['modeRevision'] != submission['expectedModeRevision']) {
      throw StateError('任务提交结果缺少当前服务提供者的有效确认，请使用原请求重试');
    }
    return Map<String, dynamic>.from(task);
  }

  Future<Map<String, dynamic>> listOwnedDeviceTaskCatalog(
    Map<String, dynamic> options,
    String characterId, {
    String cursor = '',
  }) async {
    final roles = options['roles'];
    final proof = options['executionScope'];
    if (roles is! List || proof is! Map || cursor.length > 1024) {
      throw StateError('设备任务目录缺少有效授权');
    }
    final selected = roles
        .whereType<Map>()
        .where((role) => role['id'] == characterId)
        .toList();
    if (selected.length != 1) throw StateError('请先选择当前数据归属方提供的角色');
    final expected = <String, dynamic>{
      ...Map<String, dynamic>.from(proof),
      'roleId': characterId,
      'roleRevision': selected.single['revision'],
    };
    final random = Random.secure();
    final requestId =
        'catalog-${List.generate(16, (_) => random.nextInt(256).toRadixString(16).padLeft(2, '0')).join()}';
    final page = await _api.post<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/tasks/catalog',
      data: {
        'targetDeviceId': expected['targetDeviceId'],
        'characterId': characterId,
        'requestId': requestId,
        'expectedExecutionScope': expected,
        'cursor': cursor,
        'limit': 8,
      },
    );
    final actual = page?['executionScope'];
    final entries = page?['entries'];
    final hash = RegExp(r'^[a-f0-9]{64}$');
    if (actual is! Map ||
        !expected.entries.every(
          (entry) =>
              ['requestId', 'turnId', 'executionId'].contains(entry.key) ||
              actual[entry.key] == entry.value,
        ) ||
        page?['revision'] is! String ||
        !hash.hasMatch(page!['revision']) ||
        entries is! List ||
        entries.length > 8 ||
        (page['nextCursor'] != null &&
            (page['nextCursor'] is! String ||
                (page['nextCursor'] as String).length > 1024))) {
      throw StateError('设备任务目录缺少有效的服务提供者确认');
    }
    final seen = <String>{};
    for (final entry in entries) {
      if (entry is! Map) throw StateError('设备任务目录条目无效');
      final reference = entry['reference'];
      final pin = entry['target'];
      final definition = entry['definition'];
      if (reference is! Map ||
          pin is! Map ||
          definition is! Map ||
          reference['coreId'] != expected['coreId'] ||
          reference['deviceId'] != expected['targetDeviceId'] ||
          reference['catalogId'] is! String ||
          !RegExp(
            r'^mesh-task-[a-f0-9]{64}$',
          ).hasMatch(reference['catalogId']) ||
          !seen.add(reference['catalogId']) ||
          reference['sourceTaskId'] != definition['taskId'] ||
          reference['sourceTaskId'] != pin['taskId'] ||
          pin['deviceId'] != expected['targetDeviceId'] ||
          reference['portableFingerprint'] != pin['portableFingerprint'] ||
          pin['portableFingerprint'] is! String ||
          !hash.hasMatch(pin['portableFingerprint']) ||
          pin['definitionFingerprint'] is! String ||
          !hash.hasMatch(pin['definitionFingerprint']) ||
          pin['installedGeneration'] is! int ||
          pin['installedGeneration'] < 1) {
        throw StateError('设备任务目录来源或安装版本无效');
      }
    }
    return page;
  }

  Future<Map<String, dynamic>> runtimeDetail(
    String taskRunId, {
    Map<String, dynamic>? expectedRun,
  }) async {
    _requireTaskId(taskRunId, expectedRun);
    if (bound && expectedRun == null) throw StateError('请从原任务列表打开详情');
    final policy = await _checkTask(expectedRun);
    final id = Uri.encodeComponent(taskRunId);
    final values = await Future.wait<Map<String, dynamic>?>([
      _api.get<Map<String, dynamic>>('/api/extensions/tasks/$id'),
      _api.get<Map<String, dynamic>>('/api/extensions/tasks/$id/progress'),
      _api.get<Map<String, dynamic>>('/api/extensions/tasks/$id/result'),
      _api.get<Map<String, dynamic>>('/api/extensions/tasks/$id/checkpoint'),
    ]);
    await _checkTask(expectedRun);
    if (policy != null) {
      for (final value in values) {
        if (value == null ||
            value['taskRunId'] != taskRunId ||
            value['executionScope'] is! Map ||
            _taskStamp(value['executionScope'] as Map) !=
                _taskStamp(expectedRun!['executionScope'] as Map))
          throw StateError('任务详情的数据归属已变化，迟到结果已丢弃');
        _taskRow(value, policy);
      }
    }
    return {
      'run': values[0] == null
          ? const <String, dynamic>{}
          : _taskRow(values[0]!, policy),
      'progress': values[1] ?? const <String, dynamic>{},
      'result': values[2] ?? const <String, dynamic>{},
      'checkpoint': values[3] ?? const <String, dynamic>{},
    };
  }

  Future<void> pause(
    String taskRunId, {
    required int generation,
    String reason = 'user_requested',
    Map<String, dynamic>? expectedRun,
  }) async {
    await _taskControl(taskRunId, 'pause', {
      'generation': generation,
      'reason': reason,
    }, expectedRun);
  }

  Future<void> resume(
    String taskRunId, {
    required int generation,
    String resumeKind = 'resume',
    Map<String, dynamic>? expectedRun,
  }) async {
    await _taskControl(taskRunId, 'resume', {
      'generation': generation,
      'resumeKind': resumeKind,
    }, expectedRun);
  }

  Future<void> cancel(
    String taskRunId, {
    String reason = 'user_requested',
    Map<String, dynamic>? expectedRun,
  }) async {
    await _taskControl(taskRunId, 'cancel', {'reason': reason}, expectedRun);
  }

  Future<void> retry(
    String taskRunId, {
    Map<String, dynamic>? expectedRun,
  }) async {
    await _taskControl(taskRunId, 'retry', {}, expectedRun);
  }

  Future<void> recover(
    String taskRunId, {
    Map<String, dynamic>? expectedRun,
  }) async {
    await _taskControl(taskRunId, 'recover', {}, expectedRun);
  }

  List<Map<String, dynamic>> _items(Map<String, dynamic>? page) {
    final raw = page?['items'];
    if (raw is! List) return const [];
    return raw
        .whereType<Map>()
        .map((item) => Map<String, dynamic>.from(item))
        .toList(growable: false);
  }
}

import '../backend_transport/backend_service_api.dart';
import '../ui_runtime/ui_client_info.dart';

class ExtensionCenterCard {
  final String extensionId;
  final String displayName;
  final String description;
  final String version;
  final String status;
  final bool enabled;
  final List<String> contributionTags;
  final List<String> platforms;
  final String? installedAt;
  final String? updatedAt;

  ExtensionCenterCard({
    required this.extensionId,
    required this.displayName,
    required this.description,
    required this.version,
    required this.status,
    required this.enabled,
    required this.contributionTags,
    required this.platforms,
    this.installedAt,
    this.updatedAt,
  });

  factory ExtensionCenterCard.fromJson(Map<String, dynamic> json) {
    return ExtensionCenterCard(
      extensionId: (json['extensionId'] ?? '').toString(),
      displayName: (json['displayName'] ?? '').toString(),
      description: (json['description'] ?? '').toString(),
      version: (json['version'] ?? '').toString(),
      status: (json['status'] ?? '').toString(),
      enabled: (json['enabled'] as bool?) ?? false,
      contributionTags: ((json['contributionTags'] as List?) ?? []).map((e) => e.toString()).toList(),
      platforms: ((json['platforms'] as List?) ?? []).map((e) => e.toString()).toList(),
      installedAt: json['installedAt'] as String?,
      updatedAt: json['updatedAt'] as String?,
    );
  }
}

class ExtensionCenterView {
  final List<ExtensionCenterCard> installed;
  final List<ExtensionCenterCard> discover;
  final List<ExtensionCenterCard> updates;
  final List<ExtensionCenterCard> needsAction;

  ExtensionCenterView({
    required this.installed,
    required this.discover,
    required this.updates,
    required this.needsAction,
  });

  factory ExtensionCenterView.fromJson(Map<String, dynamic> json) {
    return ExtensionCenterView(
      installed: ((json['installed'] as List?) ?? []).map((e) => ExtensionCenterCard.fromJson(e as Map<String, dynamic>)).toList(),
      discover: ((json['discover'] as List?) ?? []).map((e) => ExtensionCenterCard.fromJson(e as Map<String, dynamic>)).toList(),
      updates: ((json['updates'] as List?) ?? []).map((e) => ExtensionCenterCard.fromJson(e as Map<String, dynamic>)).toList(),
      needsAction: ((json['needsAction'] as List?) ?? []).map((e) => ExtensionCenterCard.fromJson(e as Map<String, dynamic>)).toList(),
    );
  }

  List<ExtensionCenterCard> get all => [...installed, ...discover, ...updates, ...needsAction];
}


class WorkflowApiTarget {
  final String location;
  final String deviceId;

  const WorkflowApiTarget._(this.location, [this.deviceId = '']);
  const WorkflowApiTarget.local() : this._('local');
  const WorkflowApiTarget.cloud() : this._('cloud');
  const WorkflowApiTarget.device(String deviceId) : this._('device', deviceId);

  bool get isDevice => location == 'device';
  bool get isCloud => location == 'cloud';
  bool get isLocal => location == 'local';
}

class ExtensionService {
  final BackendServiceApi _api;

  ExtensionService(this._api);

  Future<Map<String, dynamic>> getUISnapshot(String platform, {String deviceId = ''}) async {
    final client = currentUIClientInfo();
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/extensions/ui/snapshot',
      queryParameters: {
        'platform': platform,
        if (deviceId.isNotEmpty) 'deviceId': deviceId,
        ...client.toQueryParameters(),
      },
    );
    return resp ?? <String, dynamic>{};
  }

  Future<List<Map<String, dynamic>>> getConversationUIEvents(
    String conversationId, {
    int limit = 2000,
    int offset = 0,
  }) async {
    final id = conversationId.trim();
    if (id.isEmpty) return const <Map<String, dynamic>>[];
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/extensions/events/conversation-ui/${Uri.encodeComponent(id)}',
      queryParameters: {
        'limit': limit.clamp(1, 5000),
        'offset': offset < 0 ? 0 : offset,
      },
    );
    return ((resp?['items'] as List?) ?? const <dynamic>[])
        .whereType<Map>()
        .map((item) => item.cast<String, dynamic>())
        .toList(growable: false);
  }

  Future<List<Map<String, dynamic>>> getConversationUIEventsAfterSequence(
    String conversationId, {
    int afterSequence = 0,
    int limit = 2000,
  }) async {
    final id = conversationId.trim();
    if (id.isEmpty) return const <Map<String, dynamic>>[];
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/extensions/events/conversation-ui/${Uri.encodeComponent(id)}',
      queryParameters: {
        'limit': limit.clamp(1, 5000),
        'afterSequence': afterSequence < 0 ? 0 : afterSequence,
      },
    );
    return ((resp?['items'] as List?) ?? const <dynamic>[])
        .whereType<Map>()
        .map((item) => item.cast<String, dynamic>())
        .toList(growable: false);
  }

  Future<Map<String, dynamic>> getClientRuntimeSessionState(String conversationId) async {
    final id = conversationId.trim();
    if (id.isEmpty) {
      return const <String, dynamic>{'conversationId': '', 'revision': 0, 'packages': <dynamic>[]};
    }
    return await _api.get<Map<String, dynamic>>(
          '/api/extensions/ui/client-runtime-state',
          queryParameters: {'conversationId': id},
        ) ??
        <String, dynamic>{'conversationId': id, 'revision': 0, 'packages': <dynamic>[]};
  }

  Future<Map<String, dynamic>> acknowledgeClientRuntimeSessionState(
    String conversationId,
    int revision,
  ) async {
    final id = conversationId.trim();
    if (id.isEmpty) {
      return const <String, dynamic>{'conversationId': '', 'revision': 0, 'packages': <dynamic>[]};
    }
    return await _api.post<Map<String, dynamic>>(
          '/api/extensions/ui/client-runtime-session-ack',
          data: {'conversationId': id, 'revision': revision},
        ) ??
        <String, dynamic>{'conversationId': id, 'revision': revision, 'packages': <dynamic>[]};
  }

  Future<List<Map<String, dynamic>>> getUIProviders() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/extensions/ui/providers');
    return ((resp?['providers'] as List?) ?? const [])
        .whereType<Map>()
        .map((e) => e.cast<String, dynamic>())
        .toList();
  }

  Future<Map<String, dynamic>> getUIProfile({required String platform, String deviceId = '', String scope = 'user'}) async {
    final client = currentUIClientInfo();
    return await _api.get<Map<String, dynamic>>(
      '/api/extensions/ui/profile',
      queryParameters: {
        'platform': platform,
        'scope': scope,
        if (deviceId.isNotEmpty) 'deviceId': deviceId,
        ...client.toQueryParameters(),
      },
    ) ?? <String, dynamic>{};
  }

  Future<Map<String, dynamic>> updateUIProfile(Map<String, dynamic> profile, {required String platform, String deviceId = '', String scope = 'user'}) async {
    final client = currentUIClientInfo();
    return await _api.put<Map<String, dynamic>>(
      '/api/extensions/ui/profile', data: profile,
      queryParameters: {
        'platform': platform,
        'scope': scope,
        if (deviceId.isNotEmpty) 'deviceId': deviceId,
        ...client.toQueryParameters(),
      },
    ) ?? <String, dynamic>{};
  }

  Future<void> deleteUIProfileOverride({required String platform, String deviceId = '', required String scope, int? revision}) async {
    final client = currentUIClientInfo();
    await _api.delete('/api/extensions/ui/profile', queryParameters: {
      'platform': platform,
      'scope': scope,
      if (deviceId.isNotEmpty) 'deviceId': deviceId,
      'revision': ?revision,
      ...client.toQueryParameters(),
    });
  }

  Future<Map<String, dynamic>> getUISchema(String extensionId, String contributionId) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/extensions/ui/schema/${Uri.encodeComponent(extensionId)}/${Uri.encodeComponent(contributionId)}',
    );
    final document = resp?['document'];
    if (document is Map<String, dynamic>) return document;
    if (document is Map) return document.cast<String, dynamic>();
    throw StateError('UI schema document missing');
  }

  Future<Map<String, dynamic>> createWebUISession(Map<String, dynamic> request) async {
    return await _api.post<Map<String, dynamic>>('/api/extension/webui/session', data: request) ?? <String, dynamic>{};
  }

  Future<Map<String, dynamic>> invokeWebUIBridge(String sessionId, Map<String, dynamic> request) async {
    return await _api.post<Map<String, dynamic>>('/api/extension/webui/bridge/${Uri.encodeComponent(sessionId)}', data: request) ?? <String, dynamic>{};
  }

  Future<void> revokeWebUISession(String sessionId) async {
    await _api.delete('/api/extension/webui/session/${Uri.encodeComponent(sessionId)}');
  }

  Future<Map<String, dynamic>> openExtensionPage(
    String extensionId,
    String pageId, {
    Map<String, dynamic> params = const {},
    String scopeSnapshot = '',
  }) async {
    return await _api.post<Map<String, dynamic>>(
          '/api/extensions/ui/open-page',
          data: {
            'extensionId': extensionId,
            'pageId': pageId,
            'params': params,
            if (scopeSnapshot.isNotEmpty) 'scopeSnapshot': scopeSnapshot,
          },
        ) ??
        <String, dynamic>{};
  }

  Future<Map<String, dynamic>> getExtensionPageSessionStatus(String sessionId) async {
    return await _api.get<Map<String, dynamic>>(
          '/api/extensions/ui/page-sessions/${Uri.encodeComponent(sessionId)}/status',
        ) ??
        <String, dynamic>{};
  }

  Future<void> closeExtensionPageSession(String sessionId) async {
    await _api.delete('/api/extensions/ui/page-sessions/${Uri.encodeComponent(sessionId)}');
  }

  Future<ExtensionCenterView> getExtensionCenterView() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/extension-center/view');
    if (resp == null) return ExtensionCenterView(installed: [], discover: [], updates: [], needsAction: []);
    return ExtensionCenterView.fromJson(resp);
  }

  Future<List<Map<String, dynamic>>> skills({String characterId = ''}) async {
    final resp = await _api.get<List<dynamic>>(
      '/api/extensions/skills',
      queryParameters: {if (characterId.isNotEmpty) 'characterId': characterId},
    );
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<bool> enableSkill(String id, {String characterId = ''}) async {
    await _api.post(
      '/api/extensions/skills/${Uri.encodeComponent(id)}/enable${characterId.isNotEmpty ? '?characterId=${Uri.encodeQueryComponent(characterId)}' : ''}',
    );
    return true;
  }

  Future<bool> disableSkill(String id, {String characterId = ''}) async {
    await _api.post(
      '/api/extensions/skills/${Uri.encodeComponent(id)}/disable${characterId.isNotEmpty ? '?characterId=${Uri.encodeQueryComponent(characterId)}' : ''}',
    );
    return true;
  }

  Future<Map<String, dynamic>?> getSkill(String id, {String characterId = ''}) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/extensions/skills/${Uri.encodeComponent(id)}',
      queryParameters: {if (characterId.isNotEmpty) 'characterId': characterId},
    );
    return resp;
  }

  Future<List<Map<String, dynamic>>> getPermissions(String id, {String characterId = ''}) async {
    final resp = await _api.get<List<dynamic>>(
      '/api/extensions/skills/${Uri.encodeComponent(id)}/permissions',
      queryParameters: {if (characterId.isNotEmpty) 'characterId': characterId},
    );
    return (resp ?? const []).whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList(growable: false);
  }

  Future<bool> updatePermissions(String id, Map<String, dynamic> data) async {
    await _api.put('/api/extensions/skills/${Uri.encodeComponent(id)}/permissions', data: data);
    return true;
  }

  Future<Map<String, dynamic>?> getSkillConfig(String id, {String characterId = ''}) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/extensions/skills/${Uri.encodeComponent(id)}/config',
      queryParameters: {if (characterId.isNotEmpty) 'characterId': characterId},
    );
    return resp;
  }

  Future<bool> updateSkillConfig(String id, Map<String, dynamic> data, {String characterId = ''}) async {
    await _api.put(
      '/api/extensions/skills/${Uri.encodeComponent(id)}/config',
      data: data,
      queryParameters: {if (characterId.isNotEmpty) 'characterId': characterId},
    );
    return true;
  }

  Future<bool> resetSkillConfig(String id, {String characterId = ''}) async {
    await _api.post(
      '/api/extensions/skills/${Uri.encodeComponent(id)}/config/reset${characterId.isNotEmpty ? '?characterId=${Uri.encodeQueryComponent(characterId)}' : ''}',
    );
    return true;
  }

  Future<Map<String, dynamic>?> executeSkill(String id, Map<String, dynamic> params) async {
    return _api.post<Map<String, dynamic>>(
      '/api/extensions/skills/${Uri.encodeComponent(id)}/execute',
      data: params,
    );
  }

  Future<Map<String, dynamic>?> rollbackSkill(String id, String version, {String characterId = ''}) async {
    return _api.post<Map<String, dynamic>>(
      '/api/extensions/skills/${Uri.encodeComponent(id)}/versions/${Uri.encodeComponent(version)}/rollback${characterId.isNotEmpty ? '?characterId=${Uri.encodeQueryComponent(characterId)}' : ''}',
    );
  }

  Future<Map<String, dynamic>> skillRuns(String id, String characterId, {int page = 1, int pageSize = 50}) async {
    return await _api.get<Map<String, dynamic>>(
          '/api/extensions/runs',
          queryParameters: {
            'characterId': characterId,
            'skillId': id,
            'page': page,
            'pageSize': pageSize,
          },
        ) ??
        <String, dynamic>{'items': <dynamic>[], 'total': 0, 'page': page, 'pageSize': pageSize};
  }


  Future<List<Map<String, dynamic>>> kernelExtensions() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/extensions/kernel/extensions');
    final items = resp?['extensions'];
    if (items is! List) return const [];
    return items.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList(growable: false);
  }

  Future<Map<String, dynamic>> checkKernelExtensionUpdate(String extensionId) async {
    return await _api.post<Map<String, dynamic>>(
          '/api/extensions/${Uri.encodeComponent(extensionId)}/updates/check',
        ) ??
        <String, dynamic>{};
  }

  Future<Map<String, dynamic>> downloadKernelExtensionUpdate(String extensionId, String version) async {
    return await _api.post<Map<String, dynamic>>(
          '/api/extensions/${Uri.encodeComponent(extensionId)}/updates/download',
          data: {'version': version},
        ) ??
        <String, dynamic>{};
  }

  Future<Map<String, dynamic>> installKernelExtensionUpdate(String extensionId, String operationId) async {
    return await _api.post<Map<String, dynamic>>(
          '/api/extensions/${Uri.encodeComponent(extensionId)}/updates/install',
          data: {'operationId': operationId},
        ) ??
        <String, dynamic>{};
  }

  Future<Map<String, dynamic>> cancelKernelExtensionUpdate(String extensionId, String operationId) async {
    return await _api.post<Map<String, dynamic>>(
          '/api/extensions/${Uri.encodeComponent(extensionId)}/updates/cancel',
          data: {'operationId': operationId},
        ) ??
        <String, dynamic>{};
  }

  Future<Map<String, dynamic>> retryKernelExtensionUpdate(String extensionId, String operationId) async {
    return await _api.post<Map<String, dynamic>>(
          '/api/extensions/${Uri.encodeComponent(extensionId)}/updates/retry',
          data: {'operationId': operationId},
        ) ??
        <String, dynamic>{};
  }

  Future<Map<String, dynamic>> rollbackKernelExtensionUpdate(String extensionId, String operationId) async {
    return await _api.post<Map<String, dynamic>>(
          '/api/extensions/${Uri.encodeComponent(extensionId)}/updates/rollback',
          data: {'operationId': operationId},
        ) ??
        <String, dynamic>{};
  }

  Future<Map<String, dynamic>> kernelUpdateOperation(String operationId) async {
    return await _api.get<Map<String, dynamic>>(
          '/api/extensions/updates/operations/${Uri.encodeComponent(operationId)}',
        ) ??
        <String, dynamic>{};
  }

  Future<List<Map<String, dynamic>>> kernelUpdateOperationSteps(String operationId) async {
    final resp = await _api.get<dynamic>(
      '/api/extensions/updates/operations/${Uri.encodeComponent(operationId)}/steps',
    );
    dynamic raw = resp;
    if (raw is Map && raw['items'] is List) raw = raw['items'];
    if (raw is! List) return const [];
    return raw.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList(growable: false);
  }

  Future<Map<String, dynamic>> kernelExtension(String id) async {
    return await _api.get<Map<String, dynamic>>(
          '/api/extensions/kernel/extension',
          queryParameters: {'id': id},
        ) ??
        <String, dynamic>{};
  }

  Future<void> setKernelExtensionEnabled(String id, bool enabled) async {
    await _api.post(
      enabled ? '/api/extensions/kernel/extensions/enable' : '/api/extensions/kernel/extensions/disable',
      data: {'id': id},
    );
  }

  Future<Map<String, dynamic>> previewKernelUninstall(String id, {String scopeType = 'global', String scopeId = ''}) async {
    return await _api.post<Map<String, dynamic>>(
          '/api/extensions/kernel/extensions/uninstall/preview',
          data: {'extensionId': id, 'scopeType': scopeType, 'scopeId': scopeId},
        ) ??
        <String, dynamic>{};
  }

  Future<Map<String, dynamic>> confirmKernelUninstall(
    String id,
    Map<String, bool> confirmations, {
    String scopeType = 'global',
    String scopeId = '',
  }) async {
    return await _api.post<Map<String, dynamic>>(
          '/api/extensions/kernel/extensions/uninstall/confirm',
          data: {
            'extensionId': id,
            'scopeType': scopeType,
            'scopeId': scopeId,
            'confirmations': confirmations,
          },
        ) ??
        <String, dynamic>{};
  }

  Future<Map<String, dynamic>> uninstallKernelExtension(
    String id,
    String confirmationToken, {
    String scopeType = 'global',
    String scopeId = '',
  }) async {
    return await _api.post<Map<String, dynamic>>(
          '/api/extensions/kernel/extensions/uninstall',
          data: {
            'extensionId': id,
            'scopeType': scopeType,
            'scopeId': scopeId,
            'confirmationToken': confirmationToken,
          },
        ) ??
        <String, dynamic>{};
  }

  Future<List<Map<String, dynamic>>> plugins({int page = 1, int pageSize = 100}) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/extensions/plugins',
      queryParameters: {'page': page, 'pageSize': pageSize},
    );
    final items = resp?['items'];
    if (items is! List) return const [];
    return items.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList(growable: false);
  }

  Future<bool> enablePlugin(String id) async {
    await _api.post('/api/extensions/plugins/$id/enable');
    return true;
  }

  Future<bool> disablePlugin(String id) async {
    await _api.post('/api/extensions/plugins/$id/disable');
    return true;
  }

  Future<bool> reloadPlugin(String id) async {
    await _api.post('/api/extensions/plugins/$id/reload');
    return true;
  }

  Future<Map<String, dynamic>?> getPluginConfig(String id) async {
    final resp = await _api.get<Map<String, dynamic>>('/api/extensions/plugins/$id/config');
    return resp;
  }

  Future<bool> updatePluginConfig(String id, Map<String, dynamic> data) async {
    await _api.put('/api/extensions/plugins/$id/config', data: data);
    return true;
  }

  Future<Map<String, dynamic>?> getPluginHealth(String id) async {
    final resp = await _api.get<Map<String, dynamic>>('/api/extensions/plugins/$id/health');
    return resp;
  }

  Future<List<Map<String, dynamic>>> agentSkills({int page = 1, int pageSize = 100, String characterId = ''}) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/extensions/agent-skills',
      queryParameters: {
        'page': page,
        'pageSize': pageSize,
        if (characterId.isNotEmpty) 'characterId': characterId,
      },
    );
    final items = resp?['items'];
    if (items is! List) return const [];
    return items.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList(growable: false);
  }

  Future<bool> enableAgentSkill(String id) async {
    await _api.post('/api/extensions/agent-skills/$id/enable');
    return true;
  }

  Future<bool> disableAgentSkill(String id) async {
    await _api.post('/api/extensions/agent-skills/$id/disable');
    return true;
  }

  Future<bool> removeAgentSkill(String id) async {
    await _api.delete('/api/extensions/agent-skills/$id');
    return true;
  }

  Future<List<Map<String, dynamic>>> workshopSessions() async {
    final resp = await _api.get<List<dynamic>>('/api/extensions/workshop/sessions');
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<Map<String, dynamic>?> createWorkshopSession(Map<String, dynamic> data) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/extensions/workshop/sessions', data: data);
    return resp;
  }

  Future<List<Map<String, dynamic>>> extensionRuns({String characterId = '', int page = 1, int pageSize = 100}) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/extensions/runs',
      queryParameters: {
        'page': page,
        'pageSize': pageSize,
        if (characterId.isNotEmpty) 'characterId': characterId,
      },
    );
    final items = resp?['items'];
    if (items is! List) return const [];
    return items.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList(growable: false);
  }

  Future<Map<String, dynamic>?> getExtensionRun(String runId) async {
    final resp = await _api.get<Map<String, dynamic>>('/api/extensions/runs/$runId');
    return resp;
  }

  // ---- Extension Kernel Workflow V2 ----
  String _workflowBase(WorkflowApiTarget target) {
    if (target.isLocal) return '/api/local/workflows';
    if (target.isDevice) {
      final deviceId = target.deviceId.trim();
      if (deviceId.isEmpty) throw StateError('device workflow target requires deviceId');
      return '/api/extensions/workflow-devices/${Uri.encodeComponent(deviceId)}/workflows';
    }
    return '/api/extensions/workflows';
  }

  String _workflowRunBase(WorkflowApiTarget target) {
    if (target.isLocal) return '/api/local/workflow-runs';
    if (target.isDevice) throw StateError('remote device run details are not exposed through the control plane');
    return '/api/extensions/workflow-runs';
  }

  String _workflowSyncBase(WorkflowApiTarget target) {
    // Remote device mutations are mirrored as account-level workflow events in
    // Cloud Core; the local target reads the device-local durable outbox.
    return target.isLocal ? '/api/local/workflows/sync-events' : '/api/extensions/workflows/sync-events';
  }

  WorkflowApiTarget _kernelWorkflowTarget(WorkflowApiTarget target, String feature) {
    if (target.isDevice) throw StateError('$feature is not exposed through the remote device control plane');
    return target;
  }

  int? _workflowRevision(Map<String, dynamic>? definition) {
    final installation = definition?['installation'];
    if (installation is! Map) return null;
    final value = installation['revision'];
    if (value is int && value > 0) return value;
    return int.tryParse(value?.toString() ?? '');
  }

  Future<List<Map<String, dynamic>>> workflowDevices() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/extensions/workflow-devices');
    final items = resp?['items'];
    if (items is! List) return const <Map<String, dynamic>>[];
    return items.whereType<Map>().map((e) => e.map((k, v) => MapEntry(k.toString(), v))).toList(growable: false);
  }

  Future<Map<String, dynamic>> workflowSyncEvents({
    WorkflowApiTarget target = const WorkflowApiTarget.cloud(),
    int? afterCursor,
    int limit = 200,
  }) async {
    final resp = await _api.get<Map<String, dynamic>>(
      _workflowSyncBase(target),
      queryParameters: <String, dynamic>{
        if (afterCursor != null && afterCursor >= 0) 'afterCursor': afterCursor,
        if (afterCursor != null && afterCursor >= 0) 'limit': limit.clamp(1, 1000),
      },
    );
    final items = ((resp?['items'] as List?) ?? const <dynamic>[])
        .whereType<Map>()
        .map((item) => item.map((key, value) => MapEntry(key.toString(), value)))
        .toList(growable: false);
    final rawCursor = resp?['cursor'];
    final cursor = rawCursor is int ? rawCursor : int.tryParse(rawCursor?.toString() ?? '') ?? (afterCursor ?? 0);
    return <String, dynamic>{'cursor': cursor, 'items': items};
  }

  Future<List<Map<String, dynamic>>> workflows({
    int limit = 100,
    int offset = 0,
    WorkflowApiTarget target = const WorkflowApiTarget.cloud(),
  }) async {
    final resp = await _api.get<Map<String, dynamic>>(
      _workflowBase(target),
      queryParameters: target.isDevice ? null : {'limit': limit.clamp(1, 200), 'offset': offset < 0 ? 0 : offset},
    );
    final items = resp?['items'];
    if (items is! List) return const <Map<String, dynamic>>[];
    final cached = resp?['cached'] == true;
    final offline = resp?['offline'] == true;
    return items.whereType<Map>().map((raw) {
      final item = raw.map((k, v) => MapEntry(k.toString(), v));
      item['id'] ??= item['workflowId'];
      item['schemaVersion'] ??= 'workflow-v2';
      item['nodes'] ??= <dynamic>[];
      item['edges'] ??= <dynamic>[];
      item['triggers'] ??= <dynamic>[];
      item['callableByAgent'] ??= false;
      item['agentTool'] ??= <String, dynamic>{};
      item['cached'] = cached || item['cached'] == true;
      item['offline'] = offline || item['offline'] == true;
      return item;
    }).toList(growable: false);
  }

  Future<List<Map<String, dynamic>>> workflowCatalog({WorkflowApiTarget target = const WorkflowApiTarget.cloud()}) async {
    final value = _kernelWorkflowTarget(target, 'workflow catalog');
    final resp = await _api.get<Map<String, dynamic>>('${_workflowBase(value)}/catalog');
    final items = resp?['items'];
    if (items is! List) return const <Map<String, dynamic>>[];
    return items.whereType<Map>().map((e) => e.cast<String, dynamic>()).toList(growable: false);
  }

  Future<Map<String, dynamic>> generateWorkflowWithAI(String instruction, {WorkflowApiTarget target = const WorkflowApiTarget.cloud()}) async {
    final value = _kernelWorkflowTarget(target, 'AI workflow generation');
    return await _api.post<Map<String, dynamic>>('${_workflowBase(value)}/ai/generate', data: {'instruction': instruction}) ?? <String, dynamic>{};
  }

  Future<Map<String, dynamic>> editWorkflowWithAI(String id, String instruction, {WorkflowApiTarget target = const WorkflowApiTarget.cloud()}) async {
    final value = _kernelWorkflowTarget(target, 'AI workflow editing');
    return await _api.post<Map<String, dynamic>>('${_workflowBase(value)}/${Uri.encodeComponent(id)}/ai/edit', data: {'instruction': instruction}) ?? <String, dynamic>{};
  }

  Future<Map<String, dynamic>> repairWorkflowWithAI(String id, {String instruction = '', WorkflowApiTarget target = const WorkflowApiTarget.cloud()}) async {
    final value = _kernelWorkflowTarget(target, 'AI workflow repair');
    return await _api.post<Map<String, dynamic>>('${_workflowBase(value)}/${Uri.encodeComponent(id)}/ai/repair', data: {'instruction': instruction}) ?? <String, dynamic>{};
  }

  Future<Map<String, dynamic>> explainWorkflowWithAI(String id, {String instruction = '', WorkflowApiTarget target = const WorkflowApiTarget.cloud()}) async {
    final value = _kernelWorkflowTarget(target, 'AI workflow explanation');
    return await _api.post<Map<String, dynamic>>('${_workflowBase(value)}/${Uri.encodeComponent(id)}/ai/explain', data: {'instruction': instruction}) ?? <String, dynamic>{};
  }

  Future<Map<String, dynamic>> createWorkflow(Map<String, dynamic> definition, {WorkflowApiTarget target = const WorkflowApiTarget.cloud()}) async {
    return await _api.post<Map<String, dynamic>>(_workflowBase(target), data: definition) ?? <String, dynamic>{};
  }

  Future<Map<String, dynamic>> getWorkflow(String id, {WorkflowApiTarget target = const WorkflowApiTarget.cloud()}) async {
    return await _api.get<Map<String, dynamic>>('${_workflowBase(target)}/${Uri.encodeComponent(id)}') ?? <String, dynamic>{};
  }

  Future<Map<String, dynamic>> workflowAnalysis(String id, {WorkflowApiTarget target = const WorkflowApiTarget.cloud()}) async {
    final value = _kernelWorkflowTarget(target, 'workflow analysis');
    return await _api.get<Map<String, dynamic>>('${_workflowBase(value)}/${Uri.encodeComponent(id)}/analysis') ?? <String, dynamic>{};
  }

  Future<Map<String, dynamic>> workflowStats(String id, {WorkflowApiTarget target = const WorkflowApiTarget.cloud()}) async {
    final value = _kernelWorkflowTarget(target, 'workflow statistics');
    return await _api.get<Map<String, dynamic>>('${_workflowBase(value)}/${Uri.encodeComponent(id)}/stats') ?? <String, dynamic>{};
  }

  Future<Map<String, dynamic>> updateWorkflow(
    String id,
    Map<String, dynamic> definition, {
    WorkflowApiTarget target = const WorkflowApiTarget.cloud(),
    int? expectedRevision,
  }) async {
    final revision = expectedRevision ?? _workflowRevision(definition);
    return await _api.put<Map<String, dynamic>>(
          '${_workflowBase(target)}/${Uri.encodeComponent(id)}',
          data: definition,
          queryParameters: revision != null && revision > 0 ? {'expectedRevision': revision} : null,
        ) ??
        <String, dynamic>{};
  }

  Future<Map<String, dynamic>> patchWorkflow(
    String id,
    Map<String, dynamic> patch, {
    WorkflowApiTarget target = const WorkflowApiTarget.cloud(),
    int? expectedRevision,
  }) async {
    final value = _kernelWorkflowTarget(target, 'workflow patch');
    return await _api.patch<Map<String, dynamic>>(
          '${_workflowBase(value)}/${Uri.encodeComponent(id)}',
          data: patch,
          queryParameters: expectedRevision != null && expectedRevision > 0 ? {'expectedRevision': expectedRevision} : null,
        ) ??
        <String, dynamic>{};
  }

  Future<Map<String, dynamic>> exportWorkflow(String id, {WorkflowApiTarget target = const WorkflowApiTarget.cloud()}) async {
    final value = _kernelWorkflowTarget(target, 'workflow export');
    return await _api.get<Map<String, dynamic>>('${_workflowBase(value)}/${Uri.encodeComponent(id)}/export') ?? <String, dynamic>{};
  }

  Future<Map<String, dynamic>> importWorkflow(Map<String, dynamic> payload, {WorkflowApiTarget target = const WorkflowApiTarget.cloud()}) async {
    final value = _kernelWorkflowTarget(target, 'workflow import');
    return await _api.post<Map<String, dynamic>>('${_workflowBase(value)}/import', data: payload) ?? <String, dynamic>{};
  }

  Future<List<Map<String, dynamic>>> workflowTemplates({WorkflowApiTarget target = const WorkflowApiTarget.cloud()}) async {
    final value = _kernelWorkflowTarget(target, 'workflow templates');
    final resp = await _api.get<Map<String, dynamic>>('${_workflowBase(value)}/templates');
    final items = resp?['items'];
    if (items is! List) return const <Map<String, dynamic>>[];
    return items.whereType<Map>().map((e) => e.cast<String, dynamic>()).toList(growable: false);
  }

  Future<void> saveWorkflowTemplate(String id, {String name = '', String description = '', WorkflowApiTarget target = const WorkflowApiTarget.cloud()}) async {
    final value = _kernelWorkflowTarget(target, 'workflow templates');
    await _api.post('${_workflowBase(value)}/${Uri.encodeComponent(id)}/templates', data: {'name': name, 'description': description});
  }

  Future<Map<String, dynamic>> instantiateWorkflowTemplate(String templateId, {String name = '', WorkflowApiTarget target = const WorkflowApiTarget.cloud()}) async {
    final value = _kernelWorkflowTarget(target, 'workflow templates');
    return await _api.post<Map<String, dynamic>>('${_workflowBase(value)}/templates/${Uri.encodeComponent(templateId)}/instantiate', data: {'name': name}) ?? <String, dynamic>{};
  }

  Future<void> deleteWorkflowTemplate(String templateId, {WorkflowApiTarget target = const WorkflowApiTarget.cloud()}) async {
    final value = _kernelWorkflowTarget(target, 'workflow templates');
    await _api.delete('${_workflowBase(value)}/templates/${Uri.encodeComponent(templateId)}');
  }

  Future<List<Map<String, dynamic>>> workflowRevisions(String id, {int limit = 50, WorkflowApiTarget target = const WorkflowApiTarget.cloud()}) async {
    final value = _kernelWorkflowTarget(target, 'workflow revisions');
    final resp = await _api.get<Map<String, dynamic>>('${_workflowBase(value)}/${Uri.encodeComponent(id)}/revisions', queryParameters: {'limit': limit.clamp(1, 100)});
    final items = resp?['items'];
    if (items is! List) return const <Map<String, dynamic>>[];
    return items.whereType<Map>().map((e) => e.cast<String, dynamic>()).toList(growable: false);
  }

  Future<Map<String, dynamic>> createWorkflowRevision(String id, {String note = '', WorkflowApiTarget target = const WorkflowApiTarget.cloud()}) async {
    final value = _kernelWorkflowTarget(target, 'workflow revisions');
    return await _api.post<Map<String, dynamic>>('${_workflowBase(value)}/${Uri.encodeComponent(id)}/revisions', data: {'note': note}) ?? <String, dynamic>{};
  }

  Future<Map<String, dynamic>> rollbackWorkflowRevision(String id, String revisionId, {WorkflowApiTarget target = const WorkflowApiTarget.cloud(), int? expectedRevision}) async {
    final value = _kernelWorkflowTarget(target, 'workflow revisions');
    return await _api.post<Map<String, dynamic>>(
          '${_workflowBase(value)}/${Uri.encodeComponent(id)}/revisions/${Uri.encodeComponent(revisionId)}/rollback',
          queryParameters: expectedRevision != null && expectedRevision > 0 ? {'expectedRevision': expectedRevision} : null,
        ) ??
        <String, dynamic>{};
  }

  Future<Map<String, dynamic>> duplicateWorkflow(String id, {WorkflowApiTarget target = const WorkflowApiTarget.cloud()}) async {
    final value = _kernelWorkflowTarget(target, 'workflow duplication');
    return await _api.post<Map<String, dynamic>>('${_workflowBase(value)}/${Uri.encodeComponent(id)}/duplicate') ?? <String, dynamic>{};
  }

  Future<void> deleteWorkflow(String id, {WorkflowApiTarget target = const WorkflowApiTarget.cloud()}) async {
    await _api.delete('${_workflowBase(target)}/${Uri.encodeComponent(id)}');
  }

  Future<Map<String, dynamic>> setWorkflowEnabled(
    String id,
    bool enabled, {
    WorkflowApiTarget target = const WorkflowApiTarget.cloud(),
    int? expectedRevision,
  }) async {
    return await _api.post<Map<String, dynamic>>(
          '${_workflowBase(target)}/${Uri.encodeComponent(id)}/${enabled ? 'enable' : 'disable'}',
          queryParameters: expectedRevision != null && expectedRevision > 0 ? {'expectedRevision': expectedRevision} : null,
        ) ??
        <String, dynamic>{};
  }

  Future<Map<String, dynamic>> validateWorkflow(Map<String, dynamic> definition, {WorkflowApiTarget target = const WorkflowApiTarget.cloud()}) async {
    final value = _kernelWorkflowTarget(target, 'workflow validation');
    return await _api.post<Map<String, dynamic>>('${_workflowBase(value)}/validate', data: definition) ?? <String, dynamic>{};
  }

  Future<Map<String, dynamic>> runWorkflow(String id, {Map<String, dynamic> input = const {}, bool wait = false, WorkflowApiTarget target = const WorkflowApiTarget.cloud()}) async {
    return await _api.post<Map<String, dynamic>>(
          '${_workflowBase(target)}/${Uri.encodeComponent(id)}/run',
          data: target.isDevice ? {'input': input} : {'input': input, 'wait': wait},
        ) ??
        <String, dynamic>{};
  }

  Future<Map<String, dynamic>> workflowRuns(String id, {int limit = 50, int offset = 0, String status = '', WorkflowApiTarget target = const WorkflowApiTarget.cloud()}) async {
    final value = _kernelWorkflowTarget(target, 'workflow run history');
    return await _api.get<Map<String, dynamic>>(
          '${_workflowBase(value)}/${Uri.encodeComponent(id)}/runs',
          queryParameters: {'limit': limit.clamp(1, 200), 'offset': offset < 0 ? 0 : offset, if (status.isNotEmpty) 'status': status},
        ) ??
        <String, dynamic>{'items': <dynamic>[], 'total': 0};
  }

  Future<Map<String, dynamic>> getWorkflowRun(String runId, {WorkflowApiTarget target = const WorkflowApiTarget.cloud()}) async {
    return await _api.get<Map<String, dynamic>>('${_workflowRunBase(target)}/${Uri.encodeComponent(runId)}') ?? <String, dynamic>{};
  }

  Future<void> cancelWorkflowRun(String runId, {WorkflowApiTarget target = const WorkflowApiTarget.cloud()}) async {
    await _api.post('${_workflowRunBase(target)}/${Uri.encodeComponent(runId)}/cancel');
  }

  Future<void> pauseWorkflowRun(String runId, {String reason = 'Paused from Creative Workshop', WorkflowApiTarget target = const WorkflowApiTarget.cloud()}) async {
    await _api.post('${_workflowRunBase(target)}/${Uri.encodeComponent(runId)}/pause', data: {'reason': reason});
  }

  Future<void> resumeWorkflowRun(String runId, {WorkflowApiTarget target = const WorkflowApiTarget.cloud()}) async {
    await _api.post('${_workflowRunBase(target)}/${Uri.encodeComponent(runId)}/resume');
  }

  Future<Map<String, dynamic>> rerunWorkflowRun(String runId, {bool wait = false, WorkflowApiTarget target = const WorkflowApiTarget.cloud()}) async {
    return await _api.post<Map<String, dynamic>>('${_workflowRunBase(target)}/${Uri.encodeComponent(runId)}/rerun', data: <String, dynamic>{'wait': wait}) ?? <String, dynamic>{};
  }

  Future<Map<String, dynamic>> recoverWorkflowRun(String runId, {WorkflowApiTarget target = const WorkflowApiTarget.cloud()}) async {
    return await _api.post<Map<String, dynamic>>('${_workflowRunBase(target)}/${Uri.encodeComponent(runId)}/recover') ?? <String, dynamic>{};
  }

  Future<Map<String, dynamic>> dispatchWorkflowEvent(
    String eventType, {
    Map<String, dynamic> payload = const {},
    WorkflowApiTarget target = const WorkflowApiTarget.cloud(),
  }) async {
    final value = _kernelWorkflowTarget(target, 'workflow event dispatch');
    return await _api.post<Map<String, dynamic>>('${_workflowBase(value)}/events/${Uri.encodeComponent(eventType)}', data: payload) ?? <String, dynamic>{};
  }

}

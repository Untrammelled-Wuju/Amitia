import '../backend_transport/backend_service_api.dart';

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

class ExtensionService {
  final BackendServiceApi _api;

  ExtensionService(this._api);

  Future<ExtensionCenterView> getExtensionCenterView() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/extension-center/view');
    if (resp == null) return ExtensionCenterView(installed: [], discover: [], updates: [], needsAction: []);
    return ExtensionCenterView.fromJson(resp);
  }

  Future<List<Map<String, dynamic>>> skills() async {
    final resp = await _api.get<List<dynamic>>('/api/extensions/skills');
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<bool> enableSkill(String id) async {
    await _api.post('/api/extensions/skills/$id/enable');
    return true;
  }

  Future<bool> disableSkill(String id) async {
    await _api.post('/api/extensions/skills/$id/disable');
    return true;
  }

  Future<Map<String, dynamic>?> getSkill(String id) async {
    final resp = await _api.get<Map<String, dynamic>>('/api/extensions/skills/$id');
    return resp;
  }

  Future<Map<String, dynamic>?> getPermissions(String id) async {
    final resp = await _api.get<Map<String, dynamic>>('/api/extensions/skills/$id/permissions');
    return resp;
  }

  Future<bool> updatePermissions(String id, Map<String, dynamic> data) async {
    await _api.put('/api/extensions/skills/$id/permissions', data: data);
    return true;
  }

  Future<Map<String, dynamic>?> getSkillConfig(String id) async {
    final resp = await _api.get<Map<String, dynamic>>('/api/extensions/skills/$id/config');
    return resp;
  }

  Future<bool> updateSkillConfig(String id, Map<String, dynamic> data) async {
    await _api.put('/api/extensions/skills/$id/config', data: data);
    return true;
  }

  Future<bool> resetSkillConfig(String id) async {
    await _api.post('/api/extensions/skills/$id/config/reset');
    return true;
  }

  Future<Map<String, dynamic>?> executeSkill(String id, Map<String, dynamic> params) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/extensions/skills/$id/execute', data: params);
    return resp;
  }

  Future<List<Map<String, dynamic>>> plugins() async {
    final resp = await _api.get<List<dynamic>>('/api/extensions/plugins');
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
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

  Future<List<Map<String, dynamic>>> agentSkills() async {
    final resp = await _api.get<List<dynamic>>('/api/extensions/agent-skills');
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
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

  Future<List<Map<String, dynamic>>> extensionRuns() async {
    final resp = await _api.get<List<dynamic>>('/api/extensions/runs');
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<Map<String, dynamic>?> getExtensionRun(String runId) async {
    final resp = await _api.get<Map<String, dynamic>>('/api/extensions/runs/$runId');
    return resp;
  }
}

import '../api/api_client.dart';

class ExtensionService {
  final ApiClient _api = ApiClient();

  Future<List<Map<String, dynamic>>> skills() async {
    final resp = await _api.get<List<dynamic>>('/api/extensions/skills');
    if (resp.data == null) return [];
    return resp.data!.map((e) => e as Map<String, dynamic>).toList();
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
    return resp.data;
  }

  Future<Map<String, dynamic>?> getPermissions(String id) async {
    final resp = await _api.get<Map<String, dynamic>>('/api/extensions/skills/$id/permissions');
    return resp.data;
  }

  Future<bool> updatePermissions(String id, Map<String, dynamic> data) async {
    await _api.put('/api/extensions/skills/$id/permissions', data: data);
    return true;
  }

  Future<Map<String, dynamic>?> getSkillConfig(String id) async {
    final resp = await _api.get<Map<String, dynamic>>('/api/extensions/skills/$id/config');
    return resp.data;
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
    return resp.data;
  }

  Future<List<Map<String, dynamic>>> plugins() async {
    final resp = await _api.get<List<dynamic>>('/api/extensions/plugins');
    if (resp.data == null) return [];
    return resp.data!.map((e) => e as Map<String, dynamic>).toList();
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
    return resp.data;
  }

  Future<bool> updatePluginConfig(String id, Map<String, dynamic> data) async {
    await _api.put('/api/extensions/plugins/$id/config', data: data);
    return true;
  }

  Future<Map<String, dynamic>?> getPluginHealth(String id) async {
    final resp = await _api.get<Map<String, dynamic>>('/api/extensions/plugins/$id/health');
    return resp.data;
  }

  Future<List<Map<String, dynamic>>> agentSkills() async {
    final resp = await _api.get<List<dynamic>>('/api/extensions/agent-skills');
    if (resp.data == null) return [];
    return resp.data!.map((e) => e as Map<String, dynamic>).toList();
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
    if (resp.data == null) return [];
    return resp.data!.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<Map<String, dynamic>?> createWorkshopSession(Map<String, dynamic> data) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/extensions/workshop/sessions', data: data);
    return resp.data;
  }

  Future<List<Map<String, dynamic>>> extensionRuns() async {
    final resp = await _api.get<List<dynamic>>('/api/extensions/runs');
    if (resp.data == null) return [];
    return resp.data!.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<Map<String, dynamic>?> getExtensionRun(String runId) async {
    final resp = await _api.get<Map<String, dynamic>>('/api/extensions/runs/$runId');
    return resp.data;
  }
}

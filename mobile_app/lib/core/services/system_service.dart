import '../backend_transport/backend_service_api.dart';
import '../models/safety.dart';

class SystemService {
  final BackendServiceApi _api;

  SystemService(this._api);

  Future<Map<String, dynamic>?> health() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/health');
    return resp;
  }

  Future<Map<String, dynamic>?> diagnostics() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/diagnostics');
    return resp;
  }

  Future<Map<String, dynamic>?> runDiagnostics() async {
    final resp = await _api.post<Map<String, dynamic>>('/api/diagnostics/run');
    return resp;
  }

  Future<Map<String, dynamic>?> version() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/version');
    return resp;
  }

  Future<Map<String, dynamic>?> about() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/about');
    return resp;
  }

  Future<Map<String, dynamic>?> setupStatus() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/setup/status');
    return resp;
  }

  Future<Map<String, dynamic>?> setupChecks() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/setup/checks');
    return resp;
  }

  Future<bool> setupFinish() async {
    await _api.post('/api/setup/finish');
    return true;
  }

  Future<bool> setupReset() async {
    await _api.post('/api/setup/reset');
    return true;
  }

  Future<Map<String, dynamic>?> onboardingStatus() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/onboarding/status');
    return resp;
  }

  Future<bool> onboardingComplete() async {
    await _api.post('/api/onboarding/complete');
    return true;
  }

  Future<bool> onboardingReset() async {
    await _api.post('/api/onboarding/reset');
    return true;
  }

  Future<Map<String, dynamic>?> toolRoute(Map<String, dynamic> data) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/tools/route', data: data);
    return resp;
  }

  Future<Map<String, dynamic>?> chatStats() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/chats/stats');
    return resp;
  }

  Future<Map<String, dynamic>?> chatCompressionStatus(String id) async {
    final resp = await _api.get<Map<String, dynamic>>('/api/chats/conversations/$id/compression-status');
    return resp;
  }

  Future<Map<String, dynamic>?> cleanupPreview() async {
    final resp = await _api.post<Map<String, dynamic>>('/api/chats/cleanup/preview');
    return resp;
  }

  Future<Map<String, dynamic>?> cleanupConfirm(Map<String, dynamic> data) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/chats/cleanup/confirm', data: data);
    return resp;
  }

  Future<bool> cleanupVacuum() async {
    await _api.post('/api/chats/cleanup/vacuum');
    return true;
  }

  Future<Map<String, dynamic>?> export(Map<String, dynamic> data) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/chats/export', data: data);
    return resp;
  }

  Future<Map<String, dynamic>?> pipelineStatus() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/memory/pipeline/status');
    return resp;
  }

  Future<Map<String, dynamic>?> memoryStats() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/memory/retrieval/stats');
    return resp;
  }

  Future<Map<String, dynamic>?> graphStats() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/graph/stats');
    return resp;
  }

  Future<List<Map<String, dynamic>>> graphNodes() async {
    final resp = await _api.get<List<dynamic>>('/api/graph/nodes');
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<List<Map<String, dynamic>>> graphEdges() async {
    final resp = await _api.get<List<dynamic>>('/api/graph/edges');
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }
}

class SafetyService {
  final BackendServiceApi _api;

  SafetyService(this._api);

  Future<SafetyConfigDto?> config() async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/safety/config',
      fromJson: (e) => e as Map<String, dynamic>,
    );
    if (resp == null) return null;
    return SafetyConfigDto.fromJson(resp);
  }

  Future<bool> updateConfig(Map<String, dynamic> data) async {
    await _api.put('/api/safety/config', data: data);
    return true;
  }

  Future<Map<String, dynamic>?> bdiConfig() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/safety/bdi-config');
    return resp;
  }

  Future<bool> updateBdiConfig(Map<String, dynamic> data) async {
    await _api.put('/api/safety/bdi-config', data: data);
    return true;
  }

  Future<List<Map<String, dynamic>>> auditLogs() async {
    final resp = await _api.get<List<dynamic>>('/api/safety/audit-logs');
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<bool> checkInput(String text) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/safety/check-input', data: {'text': text});
    return resp?['safe'] == true;
  }

  Future<bool> checkOutput(String text) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/safety/check-output', data: {'text': text});
    return resp?['safe'] == true;
  }
}

import '../api/api_client.dart';
import '../models/safety.dart';

class SystemService {
  final ApiClient _api = ApiClient();

  Future<Map<String, dynamic>?> health() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/health');
    return resp.data;
  }

  Future<Map<String, dynamic>?> diagnostics() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/diagnostics');
    return resp.data;
  }

  Future<Map<String, dynamic>?> runDiagnostics() async {
    final resp = await _api.post<Map<String, dynamic>>('/api/diagnostics/run');
    return resp.data;
  }

  Future<Map<String, dynamic>?> version() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/version');
    return resp.data;
  }

  Future<Map<String, dynamic>?> about() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/about');
    return resp.data;
  }

  Future<Map<String, dynamic>?> setupStatus() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/setup/status');
    return resp.data;
  }

  Future<Map<String, dynamic>?> setupChecks() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/setup/checks');
    return resp.data;
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
    return resp.data;
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
    return resp.data;
  }

  Future<Map<String, dynamic>?> chatStats() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/chats/stats');
    return resp.data;
  }

  Future<Map<String, dynamic>?> chatCompressionStatus(String id) async {
    final resp = await _api.get<Map<String, dynamic>>('/api/chats/conversations/$id/compression-status');
    return resp.data;
  }

  Future<Map<String, dynamic>?> cleanupPreview() async {
    final resp = await _api.post<Map<String, dynamic>>('/api/chats/cleanup/preview');
    return resp.data;
  }

  Future<Map<String, dynamic>?> cleanupConfirm(Map<String, dynamic> data) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/chats/cleanup/confirm', data: data);
    return resp.data;
  }

  Future<bool> cleanupVacuum() async {
    await _api.post('/api/chats/cleanup/vacuum');
    return true;
  }

  Future<Map<String, dynamic>?> export(Map<String, dynamic> data) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/chats/export', data: data);
    return resp.data;
  }

  Future<Map<String, dynamic>?> pipelineStatus() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/memory/pipeline/status');
    return resp.data;
  }

  Future<Map<String, dynamic>?> memoryStats() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/memory/retrieval/stats');
    return resp.data;
  }

  Future<Map<String, dynamic>?> graphStats() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/graph/stats');
    return resp.data;
  }

  Future<List<Map<String, dynamic>>> graphNodes() async {
    final resp = await _api.get<List<dynamic>>('/api/graph/nodes');
    if (resp.data == null) return [];
    return resp.data!.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<List<Map<String, dynamic>>> graphEdges() async {
    final resp = await _api.get<List<dynamic>>('/api/graph/edges');
    if (resp.data == null) return [];
    return resp.data!.map((e) => e as Map<String, dynamic>).toList();
  }
}

class SafetyService {
  final ApiClient _api = ApiClient();

  Future<SafetyConfigDto?> config() async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/safety/config',
      fromJson: (e) => e as Map<String, dynamic>,
    );
    if (resp.data == null) return null;
    return SafetyConfigDto.fromJson(resp.data!);
  }

  Future<bool> updateConfig(Map<String, dynamic> data) async {
    await _api.put('/api/safety/config', data: data);
    return true;
  }

  Future<Map<String, dynamic>?> bdiConfig() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/safety/bdi-config');
    return resp.data;
  }

  Future<bool> updateBdiConfig(Map<String, dynamic> data) async {
    await _api.put('/api/safety/bdi-config', data: data);
    return true;
  }

  Future<List<Map<String, dynamic>>> auditLogs() async {
    final resp = await _api.get<List<dynamic>>('/api/safety/audit-logs');
    if (resp.data == null) return [];
    return resp.data!.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<bool> checkInput(String text) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/safety/check-input', data: {'text': text});
    return resp.data?['safe'] == true;
  }

  Future<bool> checkOutput(String text) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/safety/check-output', data: {'text': text});
    return resp.data?['safe'] == true;
  }
}

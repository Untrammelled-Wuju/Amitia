import '../backend_transport/backend_service_api.dart';

class MCPService {
  final BackendServiceApi _api;

  MCPService(this._api);

  Future<List<Map<String, dynamic>>> servers() async {
    final resp = await _api.get<List<dynamic>>('/api/mcp/servers');
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<Map<String, dynamic>?> createServer(Map<String, dynamic> data) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/mcp/servers', data: data);
    return resp;
  }

  Future<Map<String, dynamic>?> updateServer(String id, Map<String, dynamic> data) async {
    final resp = await _api.put<Map<String, dynamic>>('/api/mcp/servers/$id', data: data);
    return resp;
  }

  Future<bool> deleteServer(String id) async {
    await _api.delete('/api/mcp/servers/$id');
    return true;
  }

  Future<Map<String, dynamic>?> getServer(String id) async {
    final resp = await _api.get<Map<String, dynamic>>('/api/mcp/servers/$id');
    return resp;
  }

  Future<Map<String, dynamic>?> testServer(String id) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/mcp/servers/$id/test');
    return resp;
  }

  Future<Map<String, dynamic>?> connectServer(String id) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/mcp/servers/$id/connect');
    return resp;
  }

  Future<Map<String, dynamic>?> disconnectServer(String id) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/mcp/servers/$id/disconnect');
    return resp;
  }

  Future<Map<String, dynamic>?> refreshTools(String id) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/mcp/servers/$id/refresh');
    return resp;
  }

  Future<List<Map<String, dynamic>>> tools(String id) async {
    final resp = await _api.get<List<dynamic>>('/api/mcp/servers/$id/tools');
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<List<Map<String, dynamic>>> interactions() async {
    final resp = await _api.get<List<dynamic>>('/api/mcp/interactions');
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<Map<String, dynamic>?> resolveInteraction(String id, Map<String, dynamic> data) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/mcp/interactions/$id/resolve', data: data);
    return resp;
  }

  Future<List<Map<String, dynamic>>> operations() async {
    final resp = await _api.get<List<dynamic>>('/api/mcp/operations');
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }
}

class QQService {
  final BackendServiceApi _api;

  QQService(this._api);

  Future<Map<String, dynamic>?> status() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/qq/status');
    return resp;
  }

  Future<Map<String, dynamic>?> config() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/qq/config');
    return resp;
  }

  Future<Map<String, dynamic>?> connect(Map<String, dynamic> data) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/qq/connect', data: data);
    return resp;
  }

  Future<Map<String, dynamic>?> disconnect() async {
    final resp = await _api.post<Map<String, dynamic>>('/api/qq/disconnect');
    return resp;
  }
}

class ImageGenService {
  final BackendServiceApi _api;

  ImageGenService(this._api);

  Future<List<Map<String, dynamic>>> configs() async {
    final resp = await _api.get<List<dynamic>>('/api/imagegen/configs');
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<Map<String, dynamic>?> createConfig(Map<String, dynamic> data) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/imagegen/configs', data: data);
    return resp;
  }

  Future<Map<String, dynamic>?> updateConfig(String id, Map<String, dynamic> data) async {
    final resp = await _api.put<Map<String, dynamic>>('/api/imagegen/configs/$id', data: data);
    return resp;
  }

  Future<bool> deleteConfig(String id) async {
    await _api.delete('/api/imagegen/configs/$id');
    return true;
  }

  Future<bool> activate(String id) async {
    await _api.post('/api/imagegen/configs/$id/activate');
    return true;
  }

  Future<Map<String, dynamic>?> test(String id) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/imagegen/configs/$id/test');
    return resp;
  }

  Future<List<Map<String, dynamic>>> providers() async {
    final resp = await _api.get<List<dynamic>>('/api/imagegen/providers');
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }
}

class VisionService {
  final BackendServiceApi _api;

  VisionService(this._api);

  Future<List<Map<String, dynamic>>> configs() async {
    final resp = await _api.get<List<dynamic>>('/api/vision/configs');
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<Map<String, dynamic>?> createConfig(Map<String, dynamic> data) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/vision/configs', data: data);
    return resp;
  }

  Future<Map<String, dynamic>?> updateConfig(String id, Map<String, dynamic> data) async {
    final resp = await _api.put<Map<String, dynamic>>('/api/vision/configs/$id', data: data);
    return resp;
  }

  Future<bool> deleteConfig(String id) async {
    await _api.delete('/api/vision/configs/$id');
    return true;
  }

  Future<bool> activate(String id) async {
    await _api.post('/api/vision/configs/$id/activate');
    return true;
  }

  Future<Map<String, dynamic>?> test(String id) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/vision/configs/$id/test');
    return resp;
  }

  Future<List<Map<String, dynamic>>> providers() async {
    final resp = await _api.get<List<dynamic>>('/api/vision/providers');
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }
}

class EmbeddingService {
  final BackendServiceApi _api;

  EmbeddingService(this._api);

  Future<List<Map<String, dynamic>>> configs() async {
    final resp = await _api.get<List<dynamic>>('/api/embedding/configs');
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<Map<String, dynamic>?> createConfig(Map<String, dynamic> data) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/embedding/configs', data: data);
    return resp;
  }

  Future<Map<String, dynamic>?> updateConfig(String id, Map<String, dynamic> data) async {
    final resp = await _api.put<Map<String, dynamic>>('/api/embedding/configs/$id', data: data);
    return resp;
  }

  Future<bool> deleteConfig(String id) async {
    await _api.delete('/api/embedding/configs/$id');
    return true;
  }

  Future<bool> activate(String id) async {
    await _api.post('/api/embedding/configs/$id/activate');
    return true;
  }

  Future<Map<String, dynamic>?> test(String id) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/embedding/configs/$id/test');
    return resp;
  }

  Future<List<Map<String, dynamic>>> providers() async {
    final resp = await _api.get<List<dynamic>>('/api/embedding/providers');
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }
}

class EmoteService {
  final BackendServiceApi _api;

  EmoteService(this._api);

  Future<List<Map<String, dynamic>>> groups() async {
    final resp = await _api.get<List<dynamic>>('/api/emote-groups');
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<Map<String, dynamic>?> createGroup(Map<String, dynamic> data) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/emote-groups', data: data);
    return resp;
  }

  Future<Map<String, dynamic>?> updateGroup(String id, Map<String, dynamic> data) async {
    final resp = await _api.put<Map<String, dynamic>>('/api/emote-groups/$id', data: data);
    return resp;
  }

  Future<bool> deleteGroup(String id) async {
    await _api.delete('/api/emote-groups/$id');
    return true;
  }

  Future<List<Map<String, dynamic>>> listEmotes() async {
    final resp = await _api.get<List<dynamic>>('/api/emotes');
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<Map<String, dynamic>?> uploadEmote(String filePath) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/emotes/upload', data: {'filePath': filePath});
    return resp;
  }

  Future<Map<String, dynamic>?> sendEmote(String characterId, String emoteId) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/chat/send-emote',
      data: {'characterId': characterId, 'emoteId': emoteId},
    );
    return resp;
  }

  Future<Map<String, dynamic>?> getSettings(String characterId) async {
    final resp = await _api.get<Map<String, dynamic>>('/api/characters/$characterId/emote-settings');
    return resp;
  }

  Future<bool> saveSettings(String characterId, Map<String, dynamic> data) async {
    await _api.put('/api/characters/$characterId/emote-settings', data: data);
    return true;
  }
}

class ProactiveService {
  final BackendServiceApi _api;

  ProactiveService(this._api);

  Future<List<Map<String, dynamic>>> rules() async {
    final resp = await _api.get<List<dynamic>>('/api/proactive/rules');
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<Map<String, dynamic>?> createRule(Map<String, dynamic> data) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/proactive/rules', data: data);
    return resp;
  }

  Future<Map<String, dynamic>?> updateRule(String id, Map<String, dynamic> data) async {
    final resp = await _api.put<Map<String, dynamic>>('/api/proactive/rules/$id', data: data);
    return resp;
  }

  Future<bool> deleteRule(String id) async {
    await _api.delete('/api/proactive/rules/$id');
    return true;
  }

  Future<bool> toggleRule(String id, bool enabled) async {
    await _api.post('/api/proactive/rules/$id/toggle', data: {'enabled': enabled});
    return true;
  }

  Future<bool> triggerRule(String id) async {
    await _api.post('/api/proactive/rules/$id/trigger');
    return true;
  }

  Future<Map<String, dynamic>?> status() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/proactive/status');
    return resp;
  }

  Future<Map<String, dynamic>?> queueSummary() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/proactive/queue-summary');
    return resp;
  }

  Future<List<Map<String, dynamic>>> history() async {
    final resp = await _api.get<List<dynamic>>('/api/proactive/history');
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }
}

class TemporalService {
  final BackendServiceApi _api;

  TemporalService(this._api);

  Future<Map<String, dynamic>?> userProfile() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/temporal/profile');
    return resp;
  }

  Future<bool> updateUserProfile(Map<String, dynamic> data) async {
    await _api.put('/api/temporal/profile', data: data);
    return true;
  }

  Future<Map<String, dynamic>?> characterProfile(String characterId) async {
    final resp = await _api.get<Map<String, dynamic>>('/api/temporal/characters/$characterId/profile');
    return resp;
  }

  Future<bool> updateCharacterProfile(String characterId, Map<String, dynamic> data) async {
    await _api.put('/api/temporal/characters/$characterId/profile', data: data);
    return true;
  }

  Future<Map<String, dynamic>?> snapshot() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/temporal/snapshot');
    return resp;
  }

  Future<Map<String, dynamic>?> diagnostics() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/temporal/diagnostics');
    return resp;
  }

  Future<List<Map<String, dynamic>>> anchors() async {
    final resp = await _api.get<List<dynamic>>('/api/temporal/anchors');
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<Map<String, dynamic>?> createAnchor(Map<String, dynamic> data) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/temporal/anchors', data: data);
    return resp;
  }

  Future<Map<String, dynamic>?> updateAnchor(String id, Map<String, dynamic> data) async {
    final resp = await _api.put<Map<String, dynamic>>('/api/temporal/anchors/$id', data: data);
    return resp;
  }

  Future<bool> deleteAnchor(String id) async {
    await _api.delete('/api/temporal/anchors/$id');
    return true;
  }

  Future<bool> recompute() async {
    await _api.post('/api/temporal/recompute');
    return true;
  }
}

import 'dart:convert';
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

  Future<Map<String, dynamic>?> reconnectServer(String id) async {
    return _api.post<Map<String, dynamic>>('/api/mcp/servers/$id/reconnect');
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

  Future<List<Map<String, dynamic>>> prompts(String id) async {
    final resp = await _api.get<List<dynamic>>('/api/mcp/servers/$id/prompts');
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<Map<String, dynamic>> resources(String id) async {
    final resp = await _api.get<Map<String, dynamic>>('/api/mcp/servers/$id/resources');
    return resp ?? <String, dynamic>{'resources': <dynamic>[], 'resourceTemplates': <dynamic>[]};
  }

  Future<List<Map<String, dynamic>>> tasks(String id) async {
    final resp = await _api.get<List<dynamic>>('/api/mcp/servers/$id/tasks');
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<List<Map<String, dynamic>>> logs(String id, {int limit = 100}) async {
    final resp = await _api.get<List<dynamic>>(
      '/api/mcp/servers/$id/logs',
      queryParameters: {'limit': limit},
    );
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<List<Map<String, dynamic>>> capabilities(String id) async {
    final resp = await _api.get<List<dynamic>>('/api/mcp/servers/$id/capabilities');
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<void> setToolEnabled(String serverId, String toolId, bool enabled) async {
    await _api.put<Map<String, dynamic>>(
      '/api/mcp/servers/$serverId/tools/$toolId/scope',
      data: {'characterId': '', 'enabled': enabled},
    );
  }

  Future<void> setCapability(
    String serverId,
    String capability,
    bool enabled, {
    Map<String, dynamic>? configuration,
  }) async {
    await _api.put<Map<String, dynamic>>(
      '/api/mcp/servers/$serverId/capabilities/$capability',
      data: {
        'enabled': enabled,
        'configuration': configuration ?? <String, dynamic>{},
      },
    );
  }

  Future<Map<String, dynamic>?> readResource(String serverId, String uri) =>
      _api.post<Map<String, dynamic>>(
        '/api/mcp/servers/$serverId/resources/read',
        data: {'characterId': '', 'uri': uri},
      );

  Future<Map<String, dynamic>?> setResourceSubscription(
    String serverId,
    String uri,
    bool subscribed, {
    String characterId = '',
  }) =>
      _api.post<Map<String, dynamic>>(
        '/api/mcp/servers/$serverId/resources/${subscribed ? 'subscribe' : 'unsubscribe'}',
        data: {'characterId': characterId, 'uri': uri},
      );

  Future<Map<String, dynamic>?> completePromptArgument(
    String serverId, {
    required String promptName,
    required String argumentName,
    String value = '',
    Map<String, String> contextArguments = const <String, String>{},
    String characterId = '',
  }) =>
      _api.post<Map<String, dynamic>>(
        '/api/mcp/servers/$serverId/completion',
        data: {
          'characterId': characterId,
          'ref': {'type': 'ref/prompt', 'name': promptName},
          'argument': {'name': argumentName, 'value': value},
          'contextArguments': contextArguments,
        },
      );

  Future<Map<String, dynamic>?> getPrompt(
    String serverId,
    String name, {
    Map<String, String> arguments = const {},
  }) =>
      _api.post<Map<String, dynamic>>(
        '/api/mcp/servers/$serverId/prompts/get',
        data: {'characterId': '', 'name': name, 'arguments': arguments},
      );

  Future<Map<String, dynamic>?> cancelTask(String serverId, String taskId) =>
      _api.post<Map<String, dynamic>>('/api/mcp/servers/$serverId/tasks/$taskId/cancel');

  Future<Map<String, dynamic>?> startOAuth(
    String serverId, {
    required String resourceUrl,
    String redirectUri = '',
    List<String> scopes = const [],
  }) =>
      _api.post<Map<String, dynamic>>(
        '/api/mcp/servers/$serverId/oauth/start',
        data: {
          'resourceUrl': resourceUrl,
          'redirectUri': redirectUri,
          'scopes': scopes,
        },
      );

  Future<Map<String, dynamic>?> revokeOAuth(String serverId) =>
      _api.post<Map<String, dynamic>>('/api/mcp/servers/$serverId/oauth/revoke');

  Future<Map<String, dynamic>?> previewAgentSkillDependencies({
    required String agentSkillExtensionId,
    required String characterId,
    required List<dynamic> dependencies,
  }) =>
      _api.post<Map<String, dynamic>>(
        '/api/mcp/agent-skills/dependencies/preview',
        data: {
          'agentSkillExtensionId': agentSkillExtensionId,
          'characterId': characterId,
          'dependencies': dependencies,
        },
      );

  Future<Map<String, dynamic>?> installAgentSkillDependencies(
    Map<String, dynamic> plan, {
    bool installOptional = false,
    bool confirmHttp = false,
    bool confirmStdio = false,
  }) =>
      _api.post<Map<String, dynamic>>(
        '/api/mcp/agent-skills/dependencies/install',
        data: {
          'plan': plan,
          'installOptional': installOptional,
          'confirmHttp': confirmHttp,
          'confirmStdio': confirmStdio,
          'enableServers': true,
        },
      );

  Future<Map<String, dynamic>?> removeAgentSkillDependencies(String skillId) =>
      _api.delete<Map<String, dynamic>>('/api/mcp/agent-skills/$skillId/dependencies');

  Future<List<Map<String, dynamic>>> agentSkillDependencies(String skillId) async {
    final resp = await _api.get<List<dynamic>>('/api/mcp/agent-skills/$skillId/dependencies');
    if (resp == null) return const [];
    return resp.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList();
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

class WechatService {
  final BackendServiceApi _api;

  WechatService(this._api);

  Future<Map<String, dynamic>?> status() =>
      _api.get<Map<String, dynamic>>('/api/wechat/status');

  Future<Map<String, dynamic>?> bridgeStatus() =>
      _api.get<Map<String, dynamic>>('/api/wechat/bridge/status-detail');

  Future<Map<String, dynamic>?> qrCode() =>
      _api.get<Map<String, dynamic>>('/api/wechat/bridge/qrcode');

  Future<List<Map<String, dynamic>>> events() async {
    final response = await _api.get<Map<String, dynamic>>('/api/wechat/events');
    final raw = response?['events'];
    if (raw is! List) return const [];
    return raw.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList();
  }

  Future<Map<String, dynamic>?> startLogin() =>
      _api.get<Map<String, dynamic>>('/api/wechat/login/start');

  Future<Map<String, dynamic>?> rescan() =>
      _api.post<Map<String, dynamic>>('/api/wechat/login/rescan');

  Future<Map<String, dynamic>?> reconnect() =>
      _api.post<Map<String, dynamic>>('/api/wechat/login/reconnect');

  Future<Map<String, dynamic>?> waitForLogin() =>
      _api.post<Map<String, dynamic>>('/api/wechat/login/wait');

  Future<Map<String, dynamic>?> recoverBridge() =>
      _api.post<Map<String, dynamic>>('/api/wechat/bridge/recover');

  Future<Map<String, dynamic>?> riskSummary() =>
      _api.get<Map<String, dynamic>>('/api/wechat/cloud-check/risk-summary');

  Future<Map<String, dynamic>?> runCloudCheck() =>
      _api.post<Map<String, dynamic>>('/api/wechat/cloud-check/run');
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

  Future<bool> reorderGroups(List<String> ids) async {
    await _api.post('/api/emote-groups/reorder', data: {'ids': ids});
    return true;
  }

  Future<bool> addEmotesToGroup(String groupId, List<String> emoteIds) async {
    await _api.post(
      '/api/emote-groups/$groupId/emotes',
      data: {'emoteIds': emoteIds},
    );
    return true;
  }

  Future<bool> removeEmoteFromGroup(String groupId, String emoteId) async {
    await _api.delete('/api/emote-groups/$groupId/emotes/$emoteId');
    return true;
  }

  Future<List<Map<String, dynamic>>> listEmotes() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/emotes');
    final items = resp?['items'];
    if (items is! List) return [];
    return items.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList();
  }

  Future<Map<String, dynamic>?> uploadEmote(
    String filePath, {
    Map<String, dynamic> config = const {},
  }) async {
    final resp = await _api.postMultipart<Map<String, dynamic>>(
      '/api/emotes/upload',
      fields: {'config': jsonEncode(config)},
      files: {'file': [filePath]},
    );
    return resp;
  }

  Future<Map<String, dynamic>> batchUploadEmotes(
    List<String> filePaths, {
    List<Map<String, dynamic>> configs = const [],
  }) async {
    final resp = await _api.postMultipart<Map<String, dynamic>>(
      '/api/emotes/batch-upload',
      fields: {'configs': jsonEncode(configs)},
      files: {'files': filePaths},
    );
    return resp ?? <String, dynamic>{};
  }

  Future<Map<String, dynamic>?> updateEmote(String id, Map<String, dynamic> data) async {
    return _api.put<Map<String, dynamic>>('/api/emotes/$id', data: data);
  }

  Future<bool> deleteEmote(String id) async {
    await _api.delete('/api/emotes/$id');
    return true;
  }

  Future<Map<String, dynamic>> batchUpdateEmotes(
    List<String> ids,
    Map<String, dynamic> update,
  ) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/emotes/batch-update',
      data: {'ids': ids, 'update': update},
    );
    return resp ?? <String, dynamic>{};
  }

  Future<Map<String, dynamic>?> sendEmote(String conversationId, String characterId, String emoteId) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/chat/send-emote',
      data: {'conversationId': conversationId, 'characterId': characterId, 'emoteId': emoteId},
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

  Future<List<Map<String, dynamic>>> rules({String? characterId}) async {
    final resp = await _api.get<List<dynamic>>(
      '/api/proactive/rules',
      queryParameters: {if (characterId != null && characterId.isNotEmpty) 'characterId': characterId},
    );
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

  Future<Map<String, dynamic>?> testRule(String id) {
    return _api.post<Map<String, dynamic>>('/api/proactive/rules/test/$id');
  }

  Future<Map<String, dynamic>?> resetPresets({String? characterId}) {
    return _api.post<Map<String, dynamic>>(
      '/api/proactive/presets/reset',
      data: <String, dynamic>{
        if (characterId != null && characterId.isNotEmpty) 'characterId': characterId,
      },
    );
  }

  Future<List<Map<String, dynamic>>> ruleMessages(String id) async {
    final resp = await _api.get<List<dynamic>>('/api/proactive/rules/$id/messages');
    if (resp == null) return const <Map<String, dynamic>>[];
    return resp.whereType<Map>().map((item) => Map<String, dynamic>.from(item)).toList(growable: false);
  }

  Future<Map<String, dynamic>?> status({String? characterId}) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/proactive/status',
      queryParameters: {if (characterId != null && characterId.isNotEmpty) 'characterId': characterId},
    );
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

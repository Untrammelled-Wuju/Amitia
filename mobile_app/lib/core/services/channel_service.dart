import '../backend_transport/backend_service_api.dart';

class MCPService {
  final BackendServiceApi _api;

  MCPService(this._api);

  dynamic _unwrapData(dynamic response) {
    if (response is Map && response.containsKey('data'))
      return response['data'];
    return response;
  }

  Map<String, dynamic>? _asMap(dynamic response) {
    final value = _unwrapData(response);
    if (value is Map<String, dynamic>) return Map<String, dynamic>.from(value);
    if (value is Map) {
      return value.map((key, item) => MapEntry(key.toString(), item));
    }
    return null;
  }

  List<Map<String, dynamic>> _asMapList(dynamic response) {
    final value = _unwrapData(response);
    if (value is! List) return const <Map<String, dynamic>>[];
    return value
        .whereType<Map>()
        .map(
          (item) => item.map((key, value) => MapEntry(key.toString(), value)),
        )
        .toList(growable: false);
  }

  Future<List<Map<String, dynamic>>> servers() async {
    return _asMapList(await _api.get<dynamic>('/api/mcp/servers'));
  }

  Future<Map<String, dynamic>?> createServer(Map<String, dynamic> data) async {
    return _asMap(await _api.post<dynamic>('/api/mcp/servers', data: data));
  }

  Future<Map<String, dynamic>?> updateServer(
    String id,
    Map<String, dynamic> data,
  ) async {
    return _asMap(await _api.put<dynamic>('/api/mcp/servers/$id', data: data));
  }

  Future<bool> deleteServer(String id) async {
    await _api.delete('/api/mcp/servers/$id');
    return true;
  }

  Future<Map<String, dynamic>?> getServer(String id) async {
    return _asMap(await _api.get<dynamic>('/api/mcp/servers/$id'));
  }

  Future<Map<String, dynamic>?> testServer(String id) async {
    return _asMap(await _api.post<dynamic>('/api/mcp/servers/$id/test'));
  }

  Future<Map<String, dynamic>?> connectServer(String id) async {
    return _asMap(await _api.post<dynamic>('/api/mcp/servers/$id/connect'));
  }

  Future<Map<String, dynamic>?> disconnectServer(String id) async {
    return _asMap(await _api.post<dynamic>('/api/mcp/servers/$id/disconnect'));
  }

  Future<Map<String, dynamic>?> reconnectServer(String id) async {
    return _asMap(await _api.post<dynamic>('/api/mcp/servers/$id/reconnect'));
  }

  Future<Map<String, dynamic>?> refreshTools(String id) async {
    return _asMap(await _api.post<dynamic>('/api/mcp/servers/$id/refresh'));
  }

  Future<List<Map<String, dynamic>>> tools(String id) async {
    return _asMapList(await _api.get<dynamic>('/api/mcp/servers/$id/tools'));
  }

  Future<List<Map<String, dynamic>>> prompts(String id) async {
    return _asMapList(await _api.get<dynamic>('/api/mcp/servers/$id/prompts'));
  }

  Future<Map<String, dynamic>> resources(String id) async {
    return _asMap(await _api.get<dynamic>('/api/mcp/servers/$id/resources')) ??
        <String, dynamic>{
          'resources': <dynamic>[],
          'resourceTemplates': <dynamic>[],
        };
  }

  Future<List<Map<String, dynamic>>> tasks(String id) async {
    return _asMapList(await _api.get<dynamic>('/api/mcp/servers/$id/tasks'));
  }

  Future<List<Map<String, dynamic>>> logs(String id, {int limit = 100}) async {
    return _asMapList(
      await _api.get<dynamic>(
        '/api/mcp/servers/$id/logs',
        queryParameters: {'limit': limit},
      ),
    );
  }

  Future<List<Map<String, dynamic>>> capabilities(String id) async {
    return _asMapList(
      await _api.get<dynamic>('/api/mcp/servers/$id/capabilities'),
    );
  }

  Future<void> setToolEnabled(
    String serverId,
    String toolId,
    bool enabled,
  ) async {
    await _api.put<dynamic>(
      '/api/mcp/servers/$serverId/tools/$toolId/scope',
      data: {'enabled': enabled},
    );
  }

  Future<void> setCapability(
    String serverId,
    String capability,
    bool enabled, {
    Map<String, dynamic>? configuration,
  }) async {
    await _api.put<dynamic>(
      '/api/mcp/servers/$serverId/capabilities/$capability',
      data: {
        'enabled': enabled,
        'configuration': configuration ?? <String, dynamic>{},
      },
    );
  }

  Future<Map<String, dynamic>?> readResource(
    String serverId,
    String uri,
  ) async {
    return _asMap(
      await _api.post<dynamic>(
        '/api/mcp/servers/$serverId/resources/read',
        data: {'uri': uri},
      ),
    );
  }

  Future<Map<String, dynamic>?> setResourceSubscription(
    String serverId,
    String uri,
    bool subscribed,
  ) async {
    return _asMap(
      await _api.post<dynamic>(
        '/api/mcp/servers/$serverId/resources/${subscribed ? 'subscribe' : 'unsubscribe'}',
        data: {'uri': uri},
      ),
    );
  }

  Future<Map<String, dynamic>?> completePromptArgument(
    String serverId, {
    required String promptName,
    required String argumentName,
    String value = '',
    Map<String, String> contextArguments = const <String, String>{},
  }) async {
    return _asMap(
      await _api.post<dynamic>(
        '/api/mcp/servers/$serverId/completion',
        data: {
          'ref': {'type': 'ref/prompt', 'name': promptName},
          'argument': {'name': argumentName, 'value': value},
          'contextArguments': contextArguments,
        },
      ),
    );
  }

  Future<Map<String, dynamic>?> getPrompt(
    String serverId,
    String name, {
    Map<String, String> arguments = const {},
  }) async {
    return _asMap(
      await _api.post<dynamic>(
        '/api/mcp/servers/$serverId/prompts/get',
        data: {'name': name, 'arguments': arguments},
      ),
    );
  }

  Future<Map<String, dynamic>?> cancelTask(
    String serverId,
    String taskId,
  ) async {
    return _asMap(
      await _api.post<dynamic>(
        '/api/mcp/servers/$serverId/tasks/$taskId/cancel',
      ),
    );
  }

  Future<Map<String, dynamic>?> startOAuth(
    String serverId, {
    required String resourceUrl,
    String redirectUri = '',
    List<String> scopes = const [],
  }) async {
    return _asMap(
      await _api.post<dynamic>(
        '/api/mcp/servers/$serverId/oauth/start',
        data: {
          'resourceUrl': resourceUrl,
          'redirectUri': redirectUri,
          'scopes': scopes,
        },
      ),
    );
  }

  Future<Map<String, dynamic>?> revokeOAuth(String serverId) async {
    return _asMap(
      await _api.post<dynamic>('/api/mcp/servers/$serverId/oauth/revoke'),
    );
  }

  Future<Map<String, dynamic>?> previewAgentSkillDependencies({
    required String agentSkillExtensionId,
    required List<dynamic> dependencies,
  }) async {
    return _asMap(
      await _api.post<dynamic>(
        '/api/mcp/agent-skills/dependencies/preview',
        data: {
          'agentSkillExtensionId': agentSkillExtensionId,
          'dependencies': dependencies,
        },
      ),
    );
  }

  Future<Map<String, dynamic>?> installAgentSkillDependencies(
    Map<String, dynamic> plan, {
    bool installOptional = false,
    bool confirmHttp = false,
    bool confirmStdio = false,
  }) async {
    return _asMap(
      await _api.post<dynamic>(
        '/api/mcp/agent-skills/dependencies/install',
        data: {
          'plan': plan,
          'installOptional': installOptional,
          'confirmHttp': confirmHttp,
          'confirmStdio': confirmStdio,
          'enableServers': true,
        },
      ),
    );
  }

  Future<Map<String, dynamic>?> removeAgentSkillDependencies(
    String skillId,
  ) async {
    return _asMap(
      await _api.deleteWithResponse<dynamic>(
        '/api/mcp/agent-skills/$skillId/dependencies',
      ),
    );
  }

  Future<List<Map<String, dynamic>>> agentSkillDependencies(
    String skillId,
  ) async {
    return _asMapList(
      await _api.get<dynamic>('/api/mcp/agent-skills/$skillId/dependencies'),
    );
  }

  Future<List<Map<String, dynamic>>> interactions() async {
    return _asMapList(await _api.get<dynamic>('/api/mcp/interactions'));
  }

  Future<Map<String, dynamic>?> resolveInteraction(
    String id,
    Map<String, dynamic> data,
  ) async {
    return _asMap(
      await _api.post<dynamic>('/api/mcp/interactions/$id/resolve', data: data),
    );
  }

  Future<List<Map<String, dynamic>>> operations() async {
    return _asMapList(await _api.get<dynamic>('/api/mcp/operations'));
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
    return _api.post<Map<String, dynamic>>('/api/imagegen/configs', data: data);
  }

  Future<Map<String, dynamic>?> updateConfig(
    String id,
    Map<String, dynamic> data,
  ) async {
    return _api.put<Map<String, dynamic>>(
      '/api/imagegen/configs/$id',
      data: data,
    );
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
    return _api.post<Map<String, dynamic>>('/api/imagegen/configs/$id/test');
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
    return _api.post<Map<String, dynamic>>('/api/vision/configs', data: data);
  }

  Future<Map<String, dynamic>?> updateConfig(
    String id,
    Map<String, dynamic> data,
  ) async {
    return _api.put<Map<String, dynamic>>(
      '/api/vision/configs/$id',
      data: data,
    );
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
    return _api.post<Map<String, dynamic>>('/api/vision/configs/$id/test');
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
    return _api.post<Map<String, dynamic>>(
      '/api/embedding/configs',
      data: data,
    );
  }

  Future<Map<String, dynamic>?> updateConfig(
    String id,
    Map<String, dynamic> data,
  ) async {
    return _api.put<Map<String, dynamic>>(
      '/api/embedding/configs/$id',
      data: data,
    );
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
    return _api.post<Map<String, dynamic>>('/api/embedding/configs/$id/test');
  }

  Future<List<Map<String, dynamic>>> providers() async {
    final resp = await _api.get<List<dynamic>>('/api/embedding/providers');
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }
}

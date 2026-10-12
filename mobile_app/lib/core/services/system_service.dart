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

  Future<Map<String, dynamic>?> config() async {
    return _api.get<Map<String, dynamic>>('/api/config');
  }

  Future<Map<String, dynamic>?> updateConfig(Map<String, dynamic> data) async {
    return _api.put<Map<String, dynamic>>('/api/config', data: data);
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

  Future<Map<String, dynamic>?> notificationDevice({
    String deviceId = '',
  }) async {
    final response = await _api.get<dynamic>('/api/notifications/devices');
    final source = response is List
        ? response
        : response is Map && response['items'] is List
        ? response['items'] as List
        : const <dynamic>[];
    final requested = deviceId.trim();
    Map<String, dynamic>? first;
    for (final raw in source.whereType<Map>()) {
      final row = Map<String, dynamic>.from(raw);
      first ??= row;
      if (requested.isNotEmpty &&
          (row['deviceId'] ?? '').toString().trim() == requested) {
        return row;
      }
    }
    return requested.isEmpty ? first : null;
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
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/tools/route',
      data: data,
    );
    return resp;
  }

  Future<Map<String, dynamic>?> chatStats() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/chats/stats');
    return resp;
  }

  Future<Map<String, dynamic>?> chatCompressionStatus(String id) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/chats/conversations/$id/compression-status',
    );
    return resp;
  }

  Future<Map<String, dynamic>?> cleanupPreview([
    Map<String, dynamic>? data,
  ]) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/chats/cleanup/preview',
      data: data ?? const <String, dynamic>{},
    );
    return resp;
  }

  Future<Map<String, dynamic>?> cleanupConfirm(
    Map<String, dynamic> data,
  ) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/chats/cleanup/confirm',
      data: data,
    );
    return resp;
  }

  Future<bool> cleanupVacuum() async {
    await _api.post('/api/chats/cleanup/vacuum');
    return true;
  }

  Future<Map<String, dynamic>?> export(Map<String, dynamic> data) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/chats/export',
      data: data,
    );
    return resp;
  }



  Future<Map<String, dynamic>?> pipelineStatus() async {
    final characterId = await _firstCharacterId();
    if (characterId.isEmpty) return null;
    return _api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/projections',
      queryParameters: <String, dynamic>{'characterId': characterId},
    );
  }

  Future<String> _firstCharacterId() async {
    final roles = await _api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/roles',
    );
    final list = roles?['roles'];
    return list is List && list.isNotEmpty && list.first is Map
        ? ((list.first as Map)['id'] ?? '').toString()
        : '';
  }

  Future<Map<String, dynamic>?> graphStats() async {
    final resources = await _ownedGraphResources();
    var nodes = 0;
    var edges = 0;
    for (final resource in resources) {
      final body = resource['body'];
      final content = body is Map ? body['content'] : null;
      if (content is Map) {
        if (content['nodes'] is List) nodes += (content['nodes'] as List).length;
        if (content['edges'] is List) edges += (content['edges'] as List).length;
      }
    }
    return <String, dynamic>{
      'nodeCount': nodes,
      'edgeCount': edges,
      'nodes': nodes,
      'edges': edges,
    };
  }

  Future<List<Map<String, dynamic>>> graphNodes() async {
    final nodes = <Map<String, dynamic>>[];
    for (final resource in await _ownedGraphResources()) {
      final body = resource['body'];
      final content = body is Map ? body['content'] : null;
      if (content is! Map || content['nodes'] is! List) continue;
      for (final item in content['nodes'] as List) {
        if (item is Map) {
          nodes.add(<String, dynamic>{
            'id': item['id']?.toString() ?? '',
            'label': (item['label'] ?? item['name'] ?? item['id'] ?? '')
                .toString(),
            'type': (item['type'] ?? 'entity').toString(),
          });
        } else if (item != null) {
          nodes.add(<String, dynamic>{
            'id': item.toString(),
            'label': item.toString(),
            'type': 'entity',
          });
        }
      }
    }
    return nodes;
  }

  Future<List<Map<String, dynamic>>> graphEdges() async {
    final edges = <Map<String, dynamic>>[];
    for (final resource in await _ownedGraphResources()) {
      final body = resource['body'];
      final content = body is Map ? body['content'] : null;
      if (content is! Map) continue;
      final list = content['edges'] ?? content['links'] ?? content['relations'];
      if (list is List) {
        for (final item in list) {
          if (item is! Map) continue;
          edges.add(<String, dynamic>{
            'source': (item['source'] ?? item['subject'] ?? item['from'] ?? '')
                .toString(),
            'target': (item['target'] ?? item['object'] ?? item['to'] ?? '')
                .toString(),
            'relation': (item['relation'] ??
                    item['predicate'] ??
                    item['label'] ??
                    '')
                .toString(),
          });
        }
      } else {
        final target = content['target'] ?? content['object'] ?? content['to'];
        if (target != null) {
          edges.add(<String, dynamic>{
            'source': (content['source'] ??
                    content['subject'] ??
                    content['from'] ??
                    '')
                .toString(),
            'target': target.toString(),
            'relation':
                (content['relation'] ?? content['predicate'] ?? '').toString(),
          });
        }
      }
    }
    return edges;
  }

  Future<Map<String, dynamic>?> graphNeighbors(
    String nodeId, {
    int depth = 2,
  }) async {
    final nodes = await graphNodes();
    final edges = await graphEdges();
    final visible = <String>{nodeId};
    for (var round = 0; round < depth; round++) {
      for (final edge in edges) {
        final source = edge['source']?.toString() ?? '';
        final target = edge['target']?.toString() ?? '';
        if (visible.contains(source)) visible.add(target);
        if (visible.contains(target)) visible.add(source);
      }
    }
    return <String, dynamic>{
      'nodes': nodes
          .where((node) => visible.contains(node['id']?.toString() ?? ''))
          .toList(growable: false),
      'edges': edges
          .where((edge) =>
              visible.contains(edge['source']?.toString() ?? '') &&
              visible.contains(edge['target']?.toString() ?? ''))
          .toList(growable: false),
    };
  }

  Future<List<Map<String, dynamic>>> _ownedGraphResources() async {
    final roles = await _api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/roles',
    );
    final list = roles?['roles'];
    final characterId = list is List && list.isNotEmpty && list.first is Map
        ? ((list.first as Map)['id'] ?? '').toString()
        : '';
    if (characterId.isEmpty) return const [];
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/data',
      queryParameters: <String, dynamic>{
        'kind': 'graph',
        'characterId': characterId,
        'memoryView': 'all',
      },
    );
    final nested = resp?['data'];
    final payload = nested is Map ? nested : resp;
    final snapshot = payload is Map ? payload['snapshot'] : null;
    final resources = snapshot is Map && snapshot['resources'] is List
        ? snapshot['resources'] as List
        : const [];
    return resources
        .whereType<Map>()
        .map((row) => Map<String, dynamic>.from(row))
        .where((row) => row['kind'] == 'graph')
        .toList(growable: false);
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

  Future<Map<String, dynamic>> safetyEvents({
    int page = 1,
    int pageSize = 20,
  }) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/safety/events',
      queryParameters: <String, dynamic>{'page': page, 'pageSize': pageSize},
    );
    return resp ?? <String, dynamic>{'items': <dynamic>[], 'total': 0};
  }

  Future<void> clearSafetyEvents() async {
    await _api.delete('/api/safety/events');
  }

  Future<List<Map<String, dynamic>>> auditLogs() async {
    final resp = await _api.get<List<dynamic>>('/api/safety/audit-logs');
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<bool> checkInput(String text) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/safety/check-input',
      data: {'text': text},
    );
    return resp?['safe'] == true;
  }

  Future<bool> checkOutput(String text) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/safety/check-output',
      data: {'text': text},
    );
    return resp?['safe'] == true;
  }
}

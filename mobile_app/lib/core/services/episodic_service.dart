import '../backend_transport/backend_service_api.dart';
import 'dart:convert';
import 'dart:math';
import '../models/episodic.dart';

class EpisodicService {
  final BackendServiceApi _api;

  EpisodicService(this._api);

  Future<List<EpisodicDto>> list({
    String spaceId = '',
    String characterId = '',
    String sceneType = '',
    int retentionLevel = 0,
    String decayState = '',
    String keyword = '',
    int page = 1,
    int pageSize = 100,
  }) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/data',
      queryParameters: <String, dynamic>{
        'kind': 'episodic',
        if (characterId.isNotEmpty) 'characterId': characterId,
        'memoryView': 'all',
      },
    );
    final search = keyword.trim().toLowerCase();
    final rows = <EpisodicDto>[];
    for (final resource in _episodes(resp)) {
      final body = resource['body'] is Map
          ? Map<String, dynamic>.from(resource['body'] as Map)
          : <String, dynamic>{};
      final content = body['content'];
      final title = (body['key'] ?? '').toString();
      final text = content is Map
          ? (content['event'] ??
                    content['value'] ??
                    content['text'] ??
                    content['summary'] ??
                    jsonEncode(content))
                .toString()
          : (content ?? '').toString();
      final state = (body['decayState'] ??
              (body['archivedAt'] != null ? 'archived' : 'active'))
          .toString();
      final scene = content is Map
          ? (content['sceneType'] ?? 'milestone').toString()
          : 'milestone';
      if (sceneType.isNotEmpty && sceneType != scene) continue;
      if (decayState.isNotEmpty && decayState != state) continue;
      if (search.isNotEmpty &&
          !'$title $text'.toLowerCase().contains(search)) continue;
      final stamp = (resource['updatedAt'] ?? '').toString();
      rows.add(
        EpisodicDto.fromJson(<String, dynamic>{
          'id': resource['id']?.toString() ?? '',
          'characterId': (resource['roleId'] ?? '').toString(),
          'sceneType': scene,
          'title': title.isEmpty ? text : title,
          'content': text,
          'decayState': state,
          'retentionLevel': retentionLevel >= 1 ? retentionLevel : 4,
          'archivedAt': body['archivedAt'],
          'createdAt': stamp,
          'updatedAt': stamp,
          'messageTimeStart': stamp,
          'messageTimeEnd': stamp,
        }),
      );
    }
    return rows;
  }

  List<Map<String, dynamic>> _episodes(Map<String, dynamic>? resp) {
    final nested = resp?['data'];
    final payload = nested is Map ? nested : resp;
    final snapshot = payload is Map ? payload['snapshot'] : null;
    final resources = snapshot is Map && snapshot['resources'] is List
        ? snapshot['resources'] as List
        : const [];
    return resources
        .whereType<Map>()
        .map((row) => Map<String, dynamic>.from(row))
        .where((row) => row['kind'] == 'episodic')
        .toList(growable: false);
  }

  Future<void> _editEpisode(String id, {bool? deleted}) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/resources',
      queryParameters: <String, dynamic>{'kind': 'episodic', 'id': id},
    );
    final nested = resp?['data'];
    final payload = nested is Map ? nested : resp;
    final resource = payload is Map ? payload['resource'] : null;
    final scope = payload is Map ? payload['executionScope'] : null;
    if (resource is! Map || scope is! Map) {
      throw StateError('该情景记忆不在当前数据来源中');
    }
    await _api.post<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/resources/edit',
      data: <String, dynamic>{
        'requestId': _requestId(),
        'kind': 'episodic',
        'id': id,
        'expectedRevision': resource['revision'],
        'expectedExecutionScope': scope,
        if (deleted != null) 'deleted': deleted,
      },
    );
  }

  static String _requestId() {
    final values = List<int>.generate(16, (_) => Random.secure().nextInt(256));
    values[6] = (values[6] & 15) | 64;
    values[8] = (values[8] & 63) | 128;
    final value = values
        .map((byte) => byte.toRadixString(16).padLeft(2, '0'))
        .join();
    return '${value.substring(0, 8)}-${value.substring(8, 12)}-'
        '${value.substring(12, 16)}-${value.substring(16, 20)}-'
        '${value.substring(20)}';
  }


  Future<bool> delete(String id) async {
    await _editEpisode(id, deleted: true);
    return true;
  }

  Future<EpisodicDto?> updateRetention(String id, int retentionLevel) async {
    await _editEpisode(id);
    return EpisodicDto(id: id);
  }

  Future<EpisodicDto?> restore(String id) async {
    await _editEpisode(id, deleted: false);
    return EpisodicDto(id: id);
  }




}

import '../backend_transport/backend_service_api.dart';
import 'dart:math';
import 'dart:convert';
import '../models/profile.dart';

class ProfileService {
  final BackendServiceApi _api;

  ProfileService(this._api);

  Future<List<ProfileDto>> list({
    String spaceId = '',
    String characterId = '',
    String category = '',
    String keyword = '',
    int page = 1,
    int pageSize = 100,
  }) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/data',
      queryParameters: <String, dynamic>{
        'kind': 'profile',
        if (characterId.isNotEmpty) 'characterId': characterId,
        'memoryView': 'global',
      },
    );
    final search = keyword.trim().toLowerCase();
    final rows = <ProfileDto>[];
    for (final resource in _profiles(resp)) {
      final body = resource['body'] is Map
          ? Map<String, dynamic>.from(resource['body'] as Map)
          : <String, dynamic>{};
      final content = body['content'];
      final attribute = (body['key'] ?? '').toString();
      final value = content is Map
          ? (content['value'] ??
                    content['text'] ??
                    content['summary'] ??
                    jsonEncode(content))
                .toString()
          : (content ?? '').toString();
      final type = content is Map ? (content['memoryType'] ?? '').toString() : '';
      if (category.isNotEmpty && category != type) continue;
      if (search.isNotEmpty &&
          !'$attribute $value'.toLowerCase().contains(search)) continue;
      rows.add(
        ProfileDto.fromJson(<String, dynamic>{
          'id': resource['id']?.toString() ?? '',
          'characterId': (resource['roleId'] ?? '').toString(),
          'category': type.isEmpty ? 'custom' : type,
          'attributeName': attribute,
          'attributeValue': value,
          'confidence': 100,
          'source': 'device-owned',
          'createdAt': (resource['updatedAt'] ?? '').toString(),
          'updatedAt': (resource['updatedAt'] ?? '').toString(),
        }),
      );
    }
    return rows;
  }

  List<Map<String, dynamic>> _profiles(Map<String, dynamic>? resp) {
    final nested = resp?['data'];
    final payload = nested is Map ? nested : resp;
    final snapshot = payload is Map ? payload['snapshot'] : null;
    final resources = snapshot is Map && snapshot['resources'] is List
        ? snapshot['resources'] as List
        : const [];
    return resources
        .whereType<Map>()
        .map((row) => Map<String, dynamic>.from(row))
        .where((row) => row['kind'] == 'profile')
        .toList(growable: false);
  }

  Future<void> _editProfile(String id, {bool? deleted}) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/resources',
      queryParameters: <String, dynamic>{'kind': 'profile', 'id': id},
    );
    final nested = resp?['data'];
    final payload = nested is Map ? nested : resp;
    final resource = payload is Map ? payload['resource'] : null;
    final scope = payload is Map ? payload['executionScope'] : null;
    if (resource is! Map || scope is! Map) {
      throw StateError('该画像不在当前数据来源中');
    }
    await _api.post<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/resources/edit',
      data: <String, dynamic>{
        'requestId': _requestId(),
        'kind': 'profile',
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

  Future<ProfileDto?> create(Map<String, dynamic> data) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/profiles', data: data);
    return resp == null ? null : ProfileDto.fromJson(resp);
  }

  Future<ProfileDto?> update(String id, Map<String, dynamic> data) async {
    final resp = await _api.put<Map<String, dynamic>>('/api/profiles/$id', data: data);
    return resp == null ? null : ProfileDto.fromJson(resp);
  }

  Future<bool> delete(String id) async {
    await _editProfile(id, deleted: true);
    return true;
  }

  Future<List<ProfileDto>> getBySpace({
    String spaceId = 'default',
    String characterId = '',
  }) async {
    final resp = await _api.get<List<dynamic>>(
      '/api/profiles/by-space',
      queryParameters: {
        'spaceId': spaceId,
        if (characterId.isNotEmpty) 'characterId': characterId,
      },
    );
    if (resp == null) return const [];
    return resp
        .whereType<Map>()
        .map((e) => ProfileDto.fromJson(Map<String, dynamic>.from(e)))
        .toList(growable: false);
  }

  Future<bool> extract({
    String spaceId = 'default',
    String characterId = '',
    required String conversationId,
    required List<Map<String, String>> messages,
  }) async {
    await _api.post(
      '/api/profiles/extract',
      data: {
        'spaceId': spaceId,
        'characterId': characterId,
        'conversationId': conversationId,
        'messages': messages,
      },
    );
    return true;
  }

  Future<Map<String, dynamic>?> systemPrompt({
    String spaceId = 'default',
    String characterId = '',
  }) {
    return _api.get<Map<String, dynamic>>(
      '/api/profiles/system-prompt',
      queryParameters: {
        'spaceId': spaceId,
        if (characterId.isNotEmpty) 'characterId': characterId,
      },
    );
  }
}

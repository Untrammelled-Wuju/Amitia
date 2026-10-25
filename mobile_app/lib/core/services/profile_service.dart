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

  Future<Map<String, dynamic>?> _profileScope(String characterId) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/data',
      queryParameters: <String, dynamic>{
        if (characterId.isNotEmpty) 'characterId': characterId,
        'kind': 'profile',
      },
    );
    final nested = resp?['data'];
    final payload = nested is Map ? nested : resp;
    return payload is Map ? Map<String, dynamic>.from(payload) : null;
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


  Future<ProfileDto?> create(Map<String, dynamic> data) async {
    final payload = await _profileScope(
      (data['characterId'] ?? '').toString(),
    );
    if (payload == null) return null;
    await _api.post<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/memories/manage',
      data: <String, dynamic>{
        'action': 'create',
        'key': (data['attributeName'] ?? '').toString(),
        'value': (data['attributeValue'] ?? '').toString(),
        'memoryType': (data['category'] ?? 'custom').toString(),
        'importance': 5,
        'partition': 'profile',
        'requestId': _requestId(),
        'characterId': (data['characterId'] ?? '').toString(),
        'expectedExecutionScope': payload,
      },
    );
    return ProfileDto.fromJson(<String, dynamic>{
      'id': (data['attributeName'] ?? '').toString(),
      'characterId': (data['characterId'] ?? '').toString(),
      'category': (data['category'] ?? 'custom').toString(),
      'attributeName': (data['attributeName'] ?? '').toString(),
      'attributeValue': (data['attributeValue'] ?? '').toString(),
      'confidence': 100,
    });
  }

  Future<ProfileDto?> update(String id, Map<String, dynamic> data) async {
    final payload = await _profileScope(
      (data['characterId'] ?? '').toString(),
    );
    if (payload == null) return null;
    await _api.post<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/resources/edit',
      data: <String, dynamic>{
        'requestId': _requestId(),
        'kind': 'profile',
        'id': id,
        'characterId': (data['characterId'] ?? '').toString(),
        'expectedRevision': payload['revision'],
        'expectedExecutionScope': payload['executionScope'],
        'changes': <String, dynamic>{
          'content': <String, dynamic>{
            'value': (data['attributeValue'] ?? '').toString(),
          },
        },
      },
    );
    return ProfileDto.fromJson(<String, dynamic>{
      'id': id,
      'category': (data['category'] ?? 'custom').toString(),
      'attributeName': (data['attributeName'] ?? '').toString(),
      'attributeValue': (data['attributeValue'] ?? '').toString(),
      'confidence': 100,
    });
  }

  Future<bool> delete(String id) async {
    await _editProfile(id, deleted: true);
    return true;
  }



}

import 'dart:convert';

import '../models/profile.dart';
import 'device_owned_chat_service.dart';
import 'device_owned_memory_service.dart';

class ProfileService {
  final DeviceOwnedChatService owned;
  final DeviceOwnedMemoryService Function() memory;

  ProfileService(this.owned, {required this.memory});

  Future<List<ProfileDto>> list({
    String spaceId = '',
    String characterId = '',
    String category = '',
    String keyword = '',
    int page = 1,
    int pageSize = 100,
  }) async {
    if (page < 1 || pageSize < 1) throw ArgumentError('画像分页参数无效');
    if (!await owned.refresh()) {
      throw StateError(owned.notice.isEmpty ? '设备数据服务尚未就绪' : owned.notice);
    }
    final result = await owned.data(
      'profile',
      characterId: owned.selectRole(characterId),
      memoryView: 'global',
    );
    final search = keyword.trim().toLowerCase();
    final rows = <ProfileDto>[];
    for (final resource in profileResources(result)) {
      final dto = profileDto(resource);
      if (category.isNotEmpty && category != dto.category) continue;
      if (search.isNotEmpty) {
        final haystack =
            '${dto.attributeName} ${dto.attributeValue}'.toLowerCase();
        if (!haystack.contains(search)) continue;
      }
      rows.add(dto);
    }
    final offset = (page - 1) * pageSize;
    if (offset >= rows.length) return const <ProfileDto>[];
    final end = offset + pageSize;
    return rows.sublist(offset, end > rows.length ? rows.length : end);
  }

  Future<ProfileDto?> create(Map<String, dynamic> data) async {
    final role = (data['characterId'] ?? '').toString().trim();
    final name = (data['attributeName'] ?? '').toString().trim();
    final value = (data['attributeValue'] ?? '').toString().trim();
    final category = (data['category'] ?? 'custom').toString().trim();
    if (name.isEmpty || value.isEmpty) throw StateError('画像内容不能为空');
    if (!await owned.refresh()) {
      throw StateError(owned.notice.isEmpty ? '设备数据服务尚未就绪' : owned.notice);
    }
    final service = memory();
    final page = await service.query(
      owned.selectRole(role),
      memoryView: 'global',
      limit: 1,
    );
    final input = <String, dynamic>{
      'action': 'create',
      'key': name,
      'value': value,
      'memoryType': category.isEmpty ? 'custom' : category,
      'importance': 5,
      'partition': 'profile',
    };
    try {
      final saved = await service.manage(page, input);
      for (final row in profileResources(saved)) {
        final dto = profileDto(row);
        if (dto.attributeName == name) return dto;
      }
    } on OwnedMemoryConflict catch (conflict) {
      final existing = conflict.resources
          .where(
            (row) =>
                row['kind'] == 'profile' && profileDto(row).attributeName == name,
          )
          .toList(growable: false);
      if (existing.isEmpty) rethrow;
      return profileDto(existing.first);
    }
    return ProfileDto(
      id: name,
      characterId: role,
      category: category.isEmpty ? 'custom' : category,
      attributeName: name,
      attributeValue: value,
      confidence: 100,
    );
  }

  static List<Map<String, dynamic>> profileResources(
    Map<String, dynamic> result,
  ) {
    final snapshot = result['snapshot'];
    final resources = result['resources'] is List
        ? result['resources'] as List
        : snapshot is Map && snapshot['resources'] is List
        ? snapshot['resources'] as List
        : const [];
    return resources
        .whereType<Map>()
        .map((row) => Map<String, dynamic>.from(row))
        .where((row) => row['kind'] == 'profile')
        .toList(growable: false);
  }

  static String profileAttribute(dynamic content, String key, String category) {
    final named = content is Map
        ? (content['attributeName'] ?? '').toString().trim()
        : '';
    if (named.isNotEmpty) return named;
    final trimmed = key.trim();
    final prefix = '$category/';
    return trimmed.startsWith(prefix)
        ? trimmed.substring(prefix.length).trim()
        : trimmed;
  }

  static ProfileDto profileDto(Map<String, dynamic> resource) {
    final body = resource['body'] is Map
        ? Map<String, dynamic>.from(resource['body'] as Map)
        : <String, dynamic>{};
    final content = body['content'];
    final value = content is Map
        ? (content['value'] ??
                  content['text'] ??
                  content['summary'] ??
                  jsonEncode(content))
              .toString()
        : (content ?? '').toString();
    final type = content is Map
        ? (content['memoryType'] ?? content['category'] ?? '').toString().trim()
        : '';
    final category = type.isEmpty ? 'custom' : type;
    final attribute = profileAttribute(
      content,
      (body['key'] ?? '').toString(),
      category,
    );
    final stamp = (resource['updatedAt'] ?? '').toString();
    return ProfileDto.fromJson(<String, dynamic>{
      'id': resource['id']?.toString() ?? '',
      'characterId': (resource['roleId'] ?? '').toString(),
      'category': category,
      'attributeName': attribute,
      'attributeValue': value,
      'confidence': 100,
      'source': 'device-owned',
      'createdAt': stamp,
      'updatedAt': stamp,
    });
  }
}

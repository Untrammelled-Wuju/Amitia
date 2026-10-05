Map<String, String>? parseConversationReference(String reference) {
  if (!reference.startsWith('meshconv1:')) return null;
  final parts = reference.split(':');
  if (parts.length != 3) throw StateError('会话来源无效');
  final origin = {
    'ownerId': Uri.decodeComponent(parts[1]),
    'id': Uri.decodeComponent(parts[2]),
  };
  if (conversationReference(origin) != reference) {
    throw StateError('会话来源无效');
  }
  return origin;
}

String conversationReference(Map origin) {
  final owner = origin['ownerId'];
  final id = origin['id'];
  if (owner is! String ||
      id is! String ||
      owner.isEmpty ||
      id.isEmpty ||
      owner.length > 512 ||
      id.length > 512 ||
      owner.contains('\u0000') ||
      id.contains('\u0000')) {
    throw StateError('会话来源无效');
  }
  return 'meshconv1:${Uri.encodeComponent(owner)}:${Uri.encodeComponent(id)}';
}

Map<String, dynamic> ownedConversationRow(Map row, String owner) {
  final origin = row['conversationOrigin'] is Map
      ? Map<String, dynamic>.from(row['conversationOrigin'] as Map)
      : {'ownerId': owner, 'id': row['id']};
  return {
    ...Map<String, dynamic>.from(row),
    'id': conversationReference(origin),
    'resourceId': row['id'],
    'ownerId': owner,
    'conversationOrigin': origin,
    'pinnedAt': row['pinned'] is bool
        ? row['pinned'] == true
              ? row['updatedAt'] ?? 'pinned'
              : ''
        : row['pinnedAt'],
    'archivedAt': row['archived'] is bool
        ? row['archived'] == true
              ? row['updatedAt'] ?? 'archived'
              : ''
        : row['archivedAt'],
  };
}

const globalMemoryRole = '@global-memory';

bool isMemoryKind(String kind) => const [
  'memory',
  'fact',
  'episodic',
  'vector',
  'graph',
  'continuity',
  'profile',
].contains(kind);

String memoryStorageRole(Map scope, String kind) {
  if (kind == 'continuity' && scope['continuityRoleId'] != null)
    return scope['continuityRoleId'].toString();
  if (scope['memoryRoleId'] == null) return scope['roleId'].toString();
  return isMemoryKind(kind)
      ? scope['memoryRoleId'].toString()
      : scope['roleId'].toString();
}

String memoryViewForScope(Map scope) => scope['memoryViewAll'] == true
    ? 'all'
    : scope['memoryRoleId'] == globalMemoryRole
    ? 'global'
    : scope['memoryRoleId'] != null
    ? 'role:${scope['memoryRoleId']}'
    : '';
String continuityViewForScope(Map scope) =>
    scope['continuityRoleId'] == globalMemoryRole
    ? 'global'
    : scope['continuityRoleId'] != null
    ? 'role:${scope['continuityRoleId']}'
    : memoryViewForScope(scope);

Map<String, dynamic> memoryRecordScope(Map scope, String partition) =>
    Map<String, dynamic>.from(scope)
      ..remove('memoryViewAll')
      ..['memoryRoleId'] = partition;

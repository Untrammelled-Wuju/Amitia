List<({String label, String value})> ownedMemoryViewOptions(
  String source,
  List<Map<String, dynamic>> roles,
  List<Map<String, dynamic>> historicalRoles, {
  String kind = 'memory',
}) {
  if (source != 'current') {
    final selected = historicalRoles.where(
      (role) => source == 'history:${role['id']}',
    );
    final name = selected.isEmpty
        ? ''
        : (selected.first['name'] ?? selected.first['id']).toString();
    return [
      (
        label: kind == 'profile'
            ? '原设备用户画像'
            : name.isEmpty
            ? '原设备历史'
            : '原设备历史 · $name',
        value: 'all',
      ),
    ];
  }
  final noun = kind == 'continuity' ? '事项' : '记忆';
  return [
    (label: '全部$noun', value: 'all'),
    (label: '全局$noun', value: 'global'),
    ...roles.map(
      (role) => (
        label: (role['name'] ?? role['id']).toString(),
        value: 'role:${role['id']}',
      ),
    ),
  ];
}

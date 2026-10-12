import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/services/owned_memory_view_options.dart';

void main() {
  final roles = [
    {'id': 'current-role', 'name': '当前角色'},
  ];
  final historical = [
    {'id': 'old-role', 'name': '旧角色'},
  ];
  for (final kind in [
    'memory',
    'profile',
    'episodic',
    'graph',
    'timeline',
    'continuity',
  ]) {
    test('$kind 范围仅展示所选来源', () {
      final current = ownedMemoryViewOptions(
        'current',
        roles,
        historical,
        kind: kind,
      );
      expect(
        current.map((option) => option.value),
        kind == 'profile' ? ['global'] : ['all', 'global', 'role:current-role'],
      );
      final history = ownedMemoryViewOptions(
        'history:old-role',
        roles,
        historical,
        kind: kind,
      );
      expect(history, [
        (label: kind == 'profile' ? '原设备用户画像' : '原设备历史 · 旧角色', value: 'all'),
      ]);
      expect(
        history.any((option) => option.value == 'role:current-role'),
        isFalse,
      );
    });
  }
}

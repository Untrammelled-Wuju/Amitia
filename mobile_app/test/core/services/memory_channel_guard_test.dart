import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

String read(String path) => File(path).readAsStringSync();

Iterable<File> dartFiles(String root) => Directory(
  root,
).listSync(recursive: true).whereType<File>().where(
  (file) => file.path.endsWith('.dart'),
);

void main() {
  test('旧情景记忆服务与 DTO 已移除，且不再被 provider 暴露', () {
    expect(
      File('lib/core/services/episodic_service.dart').existsSync(),
      isFalse,
    );
    expect(File('lib/core/models/episodic.dart').existsSync(), isFalse);
    final providers = read('lib/core/services/providers.dart');
    expect(providers.contains('EpisodicService'), isFalse);
    expect(providers.contains('episodicServiceProvider'), isFalse);
    final leftovers = dartFiles('lib')
        .where(
          (file) =>
              read(file.path).contains('EpisodicService') ||
              read(file.path).contains('EpisodicDto'),
        )
        .map((file) => file.path)
        .toList();
    expect(leftovers, isEmpty);
  });

  test('画像服务只经统一设备归属通道，不再自行拼接记忆接口', () {
    final profile = read('lib/core/services/profile_service.dart');
    expect(profile.contains('/api/'), isFalse);
    expect(profile.contains('resources/edit'), isFalse);
    expect(profile.contains('business/data'), isFalse);
    expect(profile.contains('DeviceOwnedChatService'), isTrue);
    expect(profile.contains('DeviceOwnedMemoryService'), isTrue);
  });

  test('记忆服务按角色显式传视图，不再依赖空字符串默认值', () {
    final memory = read('lib/core/services/device_owned_memory_service.dart');
    final matches = RegExp(
      r"role\.isEmpty \? 'all' : 'role:\$role'",
    ).allMatches(memory).length;
    expect(matches, 3);
    expect(memory.contains("role.isEmpty ? 'all' : ''"), isFalse);
  });
}

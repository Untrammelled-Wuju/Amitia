import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  List<File> sources() => Directory('lib')
      .listSync(recursive: true)
      .whereType<File>()
      .where((file) => file.path.endsWith('.dart'))
      .toList();

  test('记忆列表只有 DeviceOwnedMemoryService 一条通道', () {
    final service = File('lib/core/services/memory_service.dart');
    expect(service.existsSync(), isFalse);
    final duplicate = sources()
        .where(
          (file) => !file.path.endsWith('device_owned_memory_service.dart'),
        )
        .where(
          (file) => RegExp(
            r'business/memories(?!\/manage)|business/memory-candidates',
          ).hasMatch(file.readAsStringSync()),
        )
        .map((file) => file.path)
        .toList();
    expect(duplicate, isEmpty);
  });

  test('记忆管理必须提交服务返回的快照而不是自建范围', () {
    final pattern = RegExp(r'\.manage\(\s*\{');
    final offenders = sources()
        .where((file) => pattern.hasMatch(file.readAsStringSync()))
        .map((file) => file.path)
        .toList();
    expect(offenders, isEmpty);
  });

  test('新增画像必须提交用户选择的分类', () {
    final source = File(
      'lib/features/memory/presentation/pages/owned_memory_page.dart',
    ).readAsStringSync();
    expect(source.contains("'memoryType': 'personal_info'"), isFalse);
    expect(source, contains("'memoryType': category"));
  });

  test('记忆列表不再暴露第二条 Riverpod 通道', () {
    final pattern = RegExp(
      r'\b(memoryServiceProvider|memoryListProvider|memoryListByCharacterProvider|memoryCandidateListProvider)\b',
    );
    final offenders = sources()
        .where((file) => pattern.hasMatch(file.readAsStringSync()))
        .map((file) => file.path)
        .toList();
    expect(offenders, isEmpty);
  });
}

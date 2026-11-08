import 'package:amitia_app/core/services/shared_core_data_access.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('服务端可写权限驱动共享中心数据写状态', () {
    final access = SharedCoreDataAccess.fromCoordination({
      'coreId': 'core-a',
      'coordinationAvailable': true,
      'policy': {'permissionRevision': 4},
      'coreDataAccess': {'read': true, 'write': true, 'reason': ''},
    });

    expect(access.read, isTrue);
    expect(access.write, isTrue);
    expect(access.readOnly, isFalse);
    expect(access.coreId, 'core-a');
    access.ensureWritable();
  });

  test('服务端只读权限保留原因并拒绝客户端写入', () {
    final access = SharedCoreDataAccess.fromCoordination({
      'coreId': 'core-a',
      'coordinationAvailable': true,
      'policy': {'permissionRevision': 5},
      'coreDataAccess': {
        'read': true,
        'write': false,
        'reason': '当前设备未获统筹中心管理员授权，只能查看中心数据',
      },
    });

    expect(access.readOnly, isTrue);
    expect(
      access.ensureWritable,
      throwsA(
        isA<StateError>().having(
          (error) => error.message,
          'message',
          contains('只能查看中心数据'),
        ),
      ),
    );
  });

  test('缺失或自相矛盾的权限响应直接失败', () {
    for (final value in <Map<String, dynamic>>[
      {
        'coreId': 'core-a',
        'coordinationAvailable': true,
        'policy': {'permissionRevision': 1},
      },
      {
        'coreId': 'core-a',
        'coordinationAvailable': true,
        'policy': {'permissionRevision': 1},
        'coreDataAccess': {'read': false, 'write': false, 'reason': ''},
      },
      {
        'coreId': 'core-a',
        'coordinationAvailable': true,
        'policy': {'permissionRevision': 1},
        'coreDataAccess': {'read': true, 'write': false, 'reason': ''},
      },
    ]) {
      expect(
        () => SharedCoreDataAccess.fromCoordination(value),
        throwsA(isA<StateError>()),
      );
    }
  });
}

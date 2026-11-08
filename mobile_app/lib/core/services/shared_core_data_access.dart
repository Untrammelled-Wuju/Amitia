class SharedCoreDataAccess {
  final bool read;
  final bool write;
  final String reason;
  final String coreId;

  const SharedCoreDataAccess({
    required this.read,
    required this.write,
    required this.reason,
    required this.coreId,
  });

  const SharedCoreDataAccess.local()
    : read = true,
      write = true,
      reason = '',
      coreId = '';

  bool get readOnly => !write;

  void ensureWritable() {
    if (!write) {
      throw StateError(reason.isEmpty ? '当前设备只能查看统筹中心数据' : reason);
    }
  }

  factory SharedCoreDataAccess.fromCoordination(Map<String, dynamic> value) {
    final root = value['data'] is Map
        ? Map<String, dynamic>.from(value['data'] as Map)
        : value;
    final access = root['coreDataAccess'];
    final policy = root['policy'];
    final coreId = (root['coreId'] ?? '').toString();
    if (coreId.isEmpty ||
        root['coordinationAvailable'] != true ||
        access is! Map ||
        access['read'] is! bool ||
        access['write'] is! bool ||
        policy is! Map ||
        policy['permissionRevision'] is! int) {
      throw StateError('统筹中心未返回有效的数据访问权限');
    }
    final reason = (access['reason'] ?? '').toString();
    if (access['read'] != true || access['write'] != true && reason.isEmpty) {
      throw StateError('统筹中心数据访问权限状态不一致');
    }
    return SharedCoreDataAccess(
      read: access['read'] as bool,
      write: access['write'] as bool,
      reason: reason,
      coreId: coreId,
    );
  }
}

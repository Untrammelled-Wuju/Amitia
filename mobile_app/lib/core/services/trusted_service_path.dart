String trustedServicePath(String serviceId, {String? operation}) {
  if (serviceId.trim().isEmpty) {
    throw ArgumentError('服务 ID 不能为空');
  }
  const operations = {
    'start',
    'stop',
    'status',
    'health',
    'invoke',
    'quarantine/release',
  };
  if (operation != null && !operations.contains(operation)) {
    throw ArgumentError('服务操作无效');
  }
  final suffix = operation == null ? '' : '/$operation';
  return '/api/extensions/services$suffix?service_id=${Uri.encodeComponent(serviceId)}';
}

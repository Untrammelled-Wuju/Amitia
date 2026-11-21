const String coreOwnerPrefix = '统筹中心';
const String deviceOwnerPrefix = '设备';
const String unnamedOwnerDevice = '未命名设备';

const Set<String> ownershipCategories = {
  '角色',
  '对话',
  '群聊',
  '团队',
  'skill',
  'mcp',
  '任务',
};

String ownershipPrefix({required bool coordinated, String deviceName = ''}) {
  if (coordinated) return coreOwnerPrefix;
  final name = deviceName.trim();
  return '$deviceOwnerPrefix${name.isEmpty ? unnamedOwnerDevice : name}';
}

bool hasOwnershipPrefix(String name) {
  final trimmed = name.trim();
  if (trimmed.isEmpty) return false;
  final parts = trimmed.split('-');
  if (parts.length < 3) return false;
  if (parts.first == coreOwnerPrefix) {
    return ownershipCategories.contains(parts[1]);
  }
  return parts.first.startsWith(deviceOwnerPrefix) &&
      parts.first.length > deviceOwnerPrefix.length &&
      ownershipCategories.contains(parts[1]);
}

String ownershipLabel({
  required bool coordinated,
  required String category,
  required String name,
  String deviceName = '',
}) {
  final base = name.trim();
  final prefix = ownershipPrefix(
    coordinated: coordinated,
    deviceName: deviceName,
  );
  if (base.isEmpty) return prefix;
  if (hasOwnershipPrefix(base)) return base;
  return '$prefix-$category-$base';
}

String coreOwnershipLabel(String category, String name) =>
    ownershipLabel(coordinated: true, category: category, name: name);

String deviceOwnershipLabel(String deviceName, String category, String name) =>
    ownershipLabel(
      coordinated: false,
      category: category,
      name: name,
      deviceName: deviceName,
    );

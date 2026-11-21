const String unnamedDevice = '未命名设备';

String deviceDisplayLabel(Map<String, dynamic> device) {
  final label = (device['label'] ?? '').toString().trim();
  if (label.isNotEmpty) return label;
  final reported = (device['reportedName'] ?? '').toString().trim();
  return reported.isEmpty ? unnamedDevice : reported;
}

Map<String, String> resolveDeviceDisplayNames(
  List<Map<String, dynamic>> devices,
) {
  final entries = devices
      .map(
        (device) => (
          deviceId: (device['deviceId'] ?? '').toString(),
          name: deviceDisplayLabel(device),
        ),
      )
      .toList(growable: false);
  final counts = <String, int>{};
  for (final entry in entries) {
    counts[entry.name] = (counts[entry.name] ?? 0) + 1;
  }
  final seen = <String, int>{};
  final result = <String, String>{};
  for (final entry in entries) {
    if ((counts[entry.name] ?? 0) <= 1) {
      result[entry.deviceId] = entry.name;
      continue;
    }
    final index = (seen[entry.name] ?? 0) + 1;
    seen[entry.name] = index;
    result[entry.deviceId] = index == 1 ? entry.name : '${entry.name} ($index)';
  }
  return result;
}

String deviceDisplayName(String deviceId, List<Map<String, dynamic>> devices) {
  return resolveDeviceDisplayNames(devices)[deviceId] ?? unnamedDevice;
}

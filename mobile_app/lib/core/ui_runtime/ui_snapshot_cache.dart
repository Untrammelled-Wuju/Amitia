import 'dart:convert';
import 'package:shared_preferences/shared_preferences.dart';

class UISnapshotCache {
  static const _prefix = 'amitia.ui.snapshot.lkg.v1';

  String _key(String namespace, String platform, String deviceId) =>
      '$_prefix:${Uri.encodeComponent(namespace)}:$platform:${deviceId.isEmpty ? 'anonymous' : deviceId}';

  Future<void> save(String namespace, String platform, String deviceId, Map<String, dynamic> snapshot) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_key(namespace, platform, deviceId), jsonEncode({
      'savedAt': DateTime.now().millisecondsSinceEpoch,
      'snapshot': snapshot,
    }));
  }

  Future<Map<String, dynamic>?> load(String namespace, String platform, String deviceId) async {
    final prefs = await SharedPreferences.getInstance();
    final raw = prefs.getString(_key(namespace, platform, deviceId));
    if (raw == null || raw.isEmpty) return null;
    try {
      final decoded = jsonDecode(raw);
      if (decoded is! Map) return null;
      final snapshot = decoded['snapshot'];
      if (snapshot is Map<String, dynamic>) return snapshot;
      if (snapshot is Map) return snapshot.cast<String, dynamic>();
    } catch (_) {
      return null;
    }
    return null;
  }
}

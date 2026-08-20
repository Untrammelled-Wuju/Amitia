import 'dart:math';
import 'package:shared_preferences/shared_preferences.dart';

class UIDeviceIdentity {
  static const _key = 'amitia.ui.device-id.v1';
  String? _cached;

  Future<String> getOrCreate() async {
    if (_cached case final value?) return value;
    final prefs = await SharedPreferences.getInstance();
    final existing = prefs.getString(_key)?.trim() ?? '';
    if (existing.isNotEmpty) {
      _cached = existing;
      return existing;
    }
    final random = Random.secure();
    final bytes = List<int>.generate(16, (_) => random.nextInt(256));
    final hex = bytes.map((b) => b.toRadixString(16).padLeft(2, '0')).join();
    final created = 'ui-$hex';
    await prefs.setString(_key, created);
    _cached = created;
    return created;
  }
}

import 'runtime_bridge_state.dart';
import 'runtime_bridge_error.dart';
import 'runtime_manifest_summary.dart';

class RuntimeBridgeSnapshot {
  final int schemaVersion;
  final RuntimeBridgeState state;
  final int generation;
  final bool runtimeInstalled;
  final bool runtimeAvailable;
  final String? activeProfile;
  final RuntimeBridgeError? lastError;
  final RuntimeManifestSummary? manifest;

  const RuntimeBridgeSnapshot({
    required this.schemaVersion,
    required this.state,
    required this.generation,
    required this.runtimeInstalled,
    required this.runtimeAvailable,
    this.activeProfile,
    this.lastError,
    this.manifest,
  });

  factory RuntimeBridgeSnapshot.fromMap(Map<String, dynamic> map) {
    return RuntimeBridgeSnapshot(
      schemaVersion: map['schemaVersion'] as int? ?? 1,
      state: RuntimeBridgeState.fromNative(map['state'] as String?),
      generation: map['generation'] as int? ?? 0,
      runtimeInstalled: map['runtimeInstalled'] as bool? ?? false,
      runtimeAvailable: map['runtimeAvailable'] as bool? ?? false,
      activeProfile: _normalizedOptionalString(map['activeProfile']),
      lastError: RuntimeBridgeError.tryFromMap(
        map['lastError'] is Map
            ? Map<String, dynamic>.from(map['lastError'] as Map)
            : null,
      ),
      manifest: RuntimeManifestSummary.fromMap(
        map['manifest'] is Map
            ? Map<String, dynamic>.from(map['manifest'] as Map)
            : null,
      ),
    );
  }

  static String? _normalizedOptionalString(Object? value) {
    if (value is! String) return null;
    final normalized = value.trim();
    return normalized.isEmpty ? null : normalized;
  }

  factory RuntimeBridgeSnapshot.initial() {
    return const RuntimeBridgeSnapshot(
      schemaVersion: 1,
      state: RuntimeBridgeState.unavailable,
      generation: 0,
      runtimeInstalled: false,
      runtimeAvailable: false,
      activeProfile: null,
    );
  }

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is RuntimeBridgeSnapshot &&
          schemaVersion == other.schemaVersion &&
          state == other.state &&
          generation == other.generation &&
          runtimeInstalled == other.runtimeInstalled &&
          runtimeAvailable == other.runtimeAvailable &&
          activeProfile == other.activeProfile &&
          lastError == other.lastError &&
          manifest == other.manifest;

  @override
  int get hashCode =>
      schemaVersion.hashCode ^
      state.hashCode ^
      generation.hashCode ^
      runtimeInstalled.hashCode ^
      runtimeAvailable.hashCode ^
      activeProfile.hashCode ^
      lastError.hashCode ^
      manifest.hashCode;
}

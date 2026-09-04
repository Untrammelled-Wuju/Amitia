class RuntimeManifestSummary {
  final int schemaVersion;
  final String runtimeVersion;
  final String? packageId;
  final String? targetPlatform;
  final String? targetArch;
  final bool verified;

  const RuntimeManifestSummary({
    required this.schemaVersion,
    required this.runtimeVersion,
    this.packageId,
    this.targetPlatform,
    this.targetArch,
    required this.verified,
  });

  factory RuntimeManifestSummary.fromMap(Map<String, dynamic>? map) {
    if (map == null) {
      return const RuntimeManifestSummary(
        schemaVersion: 0,
        runtimeVersion: '',
        verified: false,
      );
    }
    return RuntimeManifestSummary(
      schemaVersion: map['schemaVersion'] as int? ?? 0,
      runtimeVersion: map['runtimeVersion'] as String? ?? '',
      packageId: map['packageId'] as String?,
      targetPlatform: map['targetPlatform'] as String?,
      targetArch: map['targetArch'] as String?,
      verified: map['verified'] as bool? ?? false,
    );
  }

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is RuntimeManifestSummary &&
          schemaVersion == other.schemaVersion &&
          runtimeVersion == other.runtimeVersion &&
          packageId == other.packageId &&
          targetPlatform == other.targetPlatform &&
          targetArch == other.targetArch &&
          verified == other.verified;

  @override
  int get hashCode =>
      schemaVersion.hashCode ^
      runtimeVersion.hashCode ^
      packageId.hashCode ^
      targetPlatform.hashCode ^
      targetArch.hashCode ^
      verified.hashCode;
}

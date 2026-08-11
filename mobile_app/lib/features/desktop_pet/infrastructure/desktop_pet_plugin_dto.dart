class DesktopPetPluginPermissionSummary {
  final List<String> declared;
  final List<String> granted;

  const DesktopPetPluginPermissionSummary({
    required this.declared,
    required this.granted,
  });

  factory DesktopPetPluginPermissionSummary.fromJson(Map<String, dynamic> json) {
    return DesktopPetPluginPermissionSummary(
      declared: ((json['declared'] as List?) ?? []).map((e) => e.toString()).toList(),
      granted: ((json['granted'] as List?) ?? []).map((e) => e.toString()).toList(),
    );
  }

  bool get hasPermissions => declared.isNotEmpty || granted.isNotEmpty;
}

class DesktopPetPluginSummary {
  final String extensionId;
  final String pluginId;
  final String name;
  final String description;
  final String version;
  final bool enabled;
  final String installState;
  final String? publisher;
  final DesktopPetPluginPermissionSummary? permissionSummary;

  const DesktopPetPluginSummary({
    required this.extensionId,
    required this.pluginId,
    required this.name,
    required this.description,
    required this.version,
    required this.enabled,
    required this.installState,
    this.publisher,
    this.permissionSummary,
  });

  factory DesktopPetPluginSummary.fromJson(Map<String, dynamic> json) {
    final permJson = json['permissionSummary'] as Map<String, dynamic>?;
    return DesktopPetPluginSummary(
      extensionId: (json['extensionId'] ?? '').toString(),
      pluginId: (json['pluginId'] ?? '').toString(),
      name: (json['name'] ?? '').toString(),
      description: (json['description'] ?? '').toString(),
      version: (json['version'] ?? '').toString(),
      enabled: (json['enabled'] as bool?) ?? false,
      installState: (json['installState'] ?? '').toString(),
      publisher: json['publisher'] as String?,
      permissionSummary: permJson != null
          ? DesktopPetPluginPermissionSummary.fromJson(permJson)
          : null,
    );
  }
}

class DesktopPetPluginDetail {
  final String extensionId;
  final String pluginId;
  final String name;
  final String description;
  final String version;
  final bool enabled;
  final String installState;
  final String? publisher;
  final DesktopPetPluginPermissionSummary? permissionSummary;
  final List<String> requiredPermissions;
  final String? packageVersion;
  final String? installedAt;
  final String? updatedAt;
  final String? source;

  const DesktopPetPluginDetail({
    required this.extensionId,
    required this.pluginId,
    required this.name,
    required this.description,
    required this.version,
    required this.enabled,
    required this.installState,
    this.publisher,
    this.permissionSummary,
    this.requiredPermissions = const [],
    this.packageVersion,
    this.installedAt,
    this.updatedAt,
    this.source,
  });

  factory DesktopPetPluginDetail.fromJson(Map<String, dynamic> json) {
    final permJson = json['permissionSummary'] as Map<String, dynamic>?;
    return DesktopPetPluginDetail(
      extensionId: (json['extensionId'] ?? '').toString(),
      pluginId: (json['pluginId'] ?? '').toString(),
      name: (json['name'] ?? '').toString(),
      description: (json['description'] ?? '').toString(),
      version: (json['version'] ?? '').toString(),
      enabled: (json['enabled'] as bool?) ?? false,
      installState: (json['installState'] ?? '').toString(),
      publisher: json['publisher'] as String?,
      permissionSummary: permJson != null
          ? DesktopPetPluginPermissionSummary.fromJson(permJson)
          : null,
      requiredPermissions:
          ((json['requiredPermissions'] as List?) ?? []).map((e) => e.toString()).toList(),
      packageVersion: json['packageVersion'] as String?,
      installedAt: json['installedAt'] as String?,
      updatedAt: json['updatedAt'] as String?,
      source: json['source'] as String?,
    );
  }
}

class DesktopPetPluginList {
  final List<DesktopPetPluginSummary> plugins;
  final int total;
  final int page;
  final int pageSize;

  const DesktopPetPluginList({
    required this.plugins,
    required this.total,
    required this.page,
    required this.pageSize,
  });

  factory DesktopPetPluginList.fromJson(Map<String, dynamic> json) {
    return DesktopPetPluginList(
      plugins: ((json['plugins'] as List?) ?? [])
          .map((e) => DesktopPetPluginSummary.fromJson(e as Map<String, dynamic>))
          .toList(),
      total: (json['total'] as int?) ?? 0,
      page: (json['page'] as int?) ?? 1,
      pageSize: (json['pageSize'] as int?) ?? 20,
    );
  }

  bool get isEmpty => plugins.isEmpty;
}

class DesktopPetPluginInstallResult {
  final String extensionId;
  final String version;
  final String installState;

  const DesktopPetPluginInstallResult({
    required this.extensionId,
    required this.version,
    required this.installState,
  });

  factory DesktopPetPluginInstallResult.fromJson(Map<String, dynamic> json) {
    return DesktopPetPluginInstallResult(
      extensionId: (json['extensionId'] ?? '').toString(),
      version: (json['version'] ?? '').toString(),
      installState: (json['installState'] ?? '').toString(),
    );
  }
}

class DesktopPetPluginMutationResult {
  final String extensionId;
  final bool success;

  const DesktopPetPluginMutationResult({
    required this.extensionId,
    required this.success,
  });

  factory DesktopPetPluginMutationResult.fromJson(Map<String, dynamic> json) {
    return DesktopPetPluginMutationResult(
      extensionId: (json['extensionId'] ?? '').toString(),
      success: (json['success'] as bool?) ?? false,
    );
  }
}

enum DesktopPetPluginOperation {
  none,
  installing,
  updating,
  enabling,
  disabling,
  uninstalling,
}

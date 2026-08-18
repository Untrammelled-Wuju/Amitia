class GamePluginSummary {
  final String extensionId;
  final String pluginId;
  final String name;
  final String version;
  final String description;
  final bool enabled;
  final String installState;
  final String health;
  final int runtimeCount;
  final String managementTarget;

  const GamePluginSummary({
    required this.extensionId,
    required this.pluginId,
    required this.name,
    required this.version,
    required this.description,
    required this.enabled,
    required this.installState,
    required this.health,
    required this.runtimeCount,
    required this.managementTarget,
  });

  factory GamePluginSummary.fromJson(Map<String, dynamic> json) {
    return GamePluginSummary(
      extensionId: (json['extensionId'] ?? '').toString(),
      pluginId: (json['pluginId'] ?? '').toString(),
      name: (json['name'] ?? '').toString(),
      version: (json['version'] ?? '').toString(),
      description: (json['description'] ?? '').toString(),
      enabled: (json['enabled'] as bool?) ?? false,
      installState: (json['installState'] ?? '').toString(),
      health: (json['health'] ?? '').toString(),
      runtimeCount: (json['runtimeCount'] as int?) ?? 0,
      managementTarget: (json['managementTarget'] ?? '').toString(),
    );
  }

  factory GamePluginSummary.fromJsonChecked(Map<String, dynamic> json) {
    final extensionId = (json['extensionId'] ?? '').toString();
    final pluginId = (json['pluginId'] ?? '').toString();
    if (extensionId.isEmpty || pluginId.isEmpty) {
      throw StateError('GamePluginSummary: empty extensionId or pluginId');
    }
    return GamePluginSummary(
      extensionId: extensionId,
      pluginId: pluginId,
      name: (json['name'] ?? '').toString(),
      version: (json['version'] ?? '').toString(),
      description: (json['description'] ?? '').toString(),
      enabled: (json['enabled'] as bool?) ?? false,
      installState: (json['installState'] ?? '').toString(),
      health: (json['health'] ?? '').toString(),
      runtimeCount: (json['runtimeCount'] as int?) ?? 0,
      managementTarget: (json['managementTarget'] ?? '').toString(),
    );
  }
}

class GamePluginDetail {
  final String extensionId;
  final String pluginId;
  final String name;
  final String version;
  final String description;
  final bool enabled;
  final String installState;
  final String? packageRevision;
  final String? descriptorRevision;
  final String managementTarget;
  final List<String> capabilities;
  final List<String> permissions;
  final String? provider;
  final List<GameRuntimeSummary> runtimes;
  final HealthSummary? healthSummary;

  const GamePluginDetail({
    required this.extensionId,
    required this.pluginId,
    required this.name,
    required this.version,
    required this.description,
    required this.enabled,
    required this.installState,
    this.packageRevision,
    this.descriptorRevision,
    required this.managementTarget,
    this.capabilities = const [],
    this.permissions = const [],
    this.provider,
    this.runtimes = const [],
    this.healthSummary,
  });

  factory GamePluginDetail.fromJson(Map<String, dynamic> json) {
    return GamePluginDetail(
      extensionId: (json['extensionId'] ?? '').toString(),
      pluginId: (json['pluginId'] ?? '').toString(),
      name: (json['name'] ?? '').toString(),
      version: (json['version'] ?? '').toString(),
      description: (json['description'] ?? '').toString(),
      enabled: (json['enabled'] as bool?) ?? false,
      installState: (json['installState'] ?? '').toString(),
      packageRevision: json['packageRevision'] as String?,
      descriptorRevision: json['descriptorRevision'] as String?,
      managementTarget: (json['managementTarget'] ?? '').toString(),
      capabilities: ((json['capabilities'] as List?) ?? []).map((e) => e.toString()).toList(),
      permissions: ((json['permissions'] as List?) ?? []).map((e) => e.toString()).toList(),
      provider: json['provider'] as String?,
      runtimes: ((json['runtimes'] as List?) ?? [])
          .map((e) => GameRuntimeSummary.fromJson(e as Map<String, dynamic>))
          .toList(),
      healthSummary: json['healthSummary'] != null
          ? HealthSummary.fromJson(json['healthSummary'] as Map<String, dynamic>)
          : null,
    );
  }

  factory GamePluginDetail.fromJsonChecked(Map<String, dynamic> json) {
    final extensionId = (json['extensionId'] ?? '').toString();
    final pluginId = (json['pluginId'] ?? '').toString();
    if (extensionId.isEmpty || pluginId.isEmpty) {
      throw StateError('GamePluginDetail: empty extensionId or pluginId');
    }
    return GamePluginDetail(
      extensionId: extensionId,
      pluginId: pluginId,
      name: (json['name'] ?? '').toString(),
      version: (json['version'] ?? '').toString(),
      description: (json['description'] ?? '').toString(),
      enabled: (json['enabled'] as bool?) ?? false,
      installState: (json['installState'] ?? '').toString(),
      packageRevision: json['packageRevision'] as String?,
      descriptorRevision: json['descriptorRevision'] as String?,
      managementTarget: (json['managementTarget'] ?? '').toString(),
      capabilities: ((json['capabilities'] as List?) ?? []).map((e) => e.toString()).toList(),
      permissions: ((json['permissions'] as List?) ?? []).map((e) => e.toString()).toList(),
      provider: json['provider'] as String?,
      runtimes: ((json['runtimes'] as List?) ?? [])
          .map((e) => GameRuntimeSummary.fromJson(e as Map<String, dynamic>))
          .toList(),
      healthSummary: json['healthSummary'] != null
          ? HealthSummary.fromJson(json['healthSummary'] as Map<String, dynamic>)
          : null,
    );
  }
}

class GameRuntimeSummary {
  final String runtimeId;
  final String pluginId;
  final String extensionId;
  final String state;
  final String health;
  final int serviceCount;
  final bool connected;
  final bool ready;
  final String controlMode;
  final int authorityEpoch;

  const GameRuntimeSummary({
    required this.runtimeId,
    required this.pluginId,
    required this.extensionId,
    required this.state,
    required this.health,
    required this.serviceCount,
    required this.connected,
    required this.ready,
    required this.controlMode,
    required this.authorityEpoch,
  });

  factory GameRuntimeSummary.fromJson(Map<String, dynamic> json) {
    return GameRuntimeSummary(
      runtimeId: (json['runtimeId'] ?? '').toString(),
      pluginId: (json['pluginId'] ?? '').toString(),
      extensionId: (json['extensionId'] ?? '').toString(),
      state: (json['state'] ?? '').toString(),
      health: (json['health'] ?? '').toString(),
      serviceCount: (json['serviceCount'] as int?) ?? 0,
      connected: (json['connected'] as bool?) ?? false,
      ready: (json['ready'] as bool?) ?? false,
      controlMode: (json['controlMode'] ?? '').toString(),
      authorityEpoch: (json['authorityEpoch'] as int?) ?? 0,
    );
  }

  factory GameRuntimeSummary.fromJsonChecked(Map<String, dynamic> json) {
    final runtimeId = (json['runtimeId'] ?? '').toString();
    if (runtimeId.isEmpty) {
      throw StateError('GameRuntimeSummary: empty runtimeId');
    }
    return GameRuntimeSummary(
      runtimeId: runtimeId,
      pluginId: (json['pluginId'] ?? '').toString(),
      extensionId: (json['extensionId'] ?? '').toString(),
      state: (json['state'] ?? '').toString(),
      health: (json['health'] ?? '').toString(),
      serviceCount: (json['serviceCount'] as int?) ?? 0,
      connected: (json['connected'] as bool?) ?? false,
      ready: (json['ready'] as bool?) ?? false,
      controlMode: (json['controlMode'] ?? '').toString(),
      authorityEpoch: (json['authorityEpoch'] as int?) ?? 0,
    );
  }
}

class GameRuntimeDetail {
  final String runtimeId;
  final String pluginId;
  final String extensionId;
  final String runtimeState;
  final String? desiredState;
  final DateTime? createdAt;
  final DateTime? updatedAt;
  final ProcessSummary? process;
  final ConnectionSummary? connection;
  final HandshakeSummary? handshake;
  final List<GameService> services;
  final ControlAuthority? controlAuthority;
  final HealthSummary? healthSummary;

  const GameRuntimeDetail({
    required this.runtimeId,
    required this.pluginId,
    required this.extensionId,
    required this.runtimeState,
    this.desiredState,
    this.createdAt,
    this.updatedAt,
    this.process,
    this.connection,
    this.handshake,
    this.services = const [],
    this.controlAuthority,
    this.healthSummary,
  });

  factory GameRuntimeDetail.fromJson(Map<String, dynamic> json) {
    return GameRuntimeDetail(
      runtimeId: (json['runtimeId'] ?? '').toString(),
      pluginId: (json['pluginId'] ?? '').toString(),
      extensionId: (json['extensionId'] ?? '').toString(),
      runtimeState: (json['runtimeState'] ?? '').toString(),
      desiredState: json['desiredState'] as String?,
      createdAt: json['createdAt'] != null ? DateTime.tryParse(json['createdAt']) : null,
      updatedAt: json['updatedAt'] != null ? DateTime.tryParse(json['updatedAt']) : null,
      process: json['process'] != null
          ? ProcessSummary.fromJson(json['process'] as Map<String, dynamic>)
          : null,
      connection: json['connection'] != null
          ? ConnectionSummary.fromJson(json['connection'] as Map<String, dynamic>)
          : null,
      handshake: json['handshake'] != null
          ? HandshakeSummary.fromJson(json['handshake'] as Map<String, dynamic>)
          : null,
      services: ((json['services'] as List?) ?? [])
          .map((e) => GameService.fromJson(e as Map<String, dynamic>))
          .toList(),
      controlAuthority: json['controlAuthority'] != null
          ? ControlAuthority.fromJson(json['controlAuthority'] as Map<String, dynamic>)
          : null,
      healthSummary: json['healthSummary'] != null
          ? HealthSummary.fromJson(json['healthSummary'] as Map<String, dynamic>)
          : null,
    );
  }

  factory GameRuntimeDetail.fromJsonChecked(Map<String, dynamic> json) {
    final runtimeId = (json['runtimeId'] ?? '').toString();
    if (runtimeId.isEmpty) {
      throw StateError('GameRuntimeDetail: empty runtimeId');
    }
    return GameRuntimeDetail(
      runtimeId: runtimeId,
      pluginId: (json['pluginId'] ?? '').toString(),
      extensionId: (json['extensionId'] ?? '').toString(),
      runtimeState: (json['runtimeState'] ?? '').toString(),
      desiredState: json['desiredState'] as String?,
      createdAt: json['createdAt'] != null ? DateTime.tryParse(json['createdAt']) : null,
      updatedAt: json['updatedAt'] != null ? DateTime.tryParse(json['updatedAt']) : null,
      process: json['process'] != null
          ? ProcessSummary.fromJson(json['process'] as Map<String, dynamic>)
          : null,
      connection: json['connection'] != null
          ? ConnectionSummary.fromJson(json['connection'] as Map<String, dynamic>)
          : null,
      handshake: json['handshake'] != null
          ? HandshakeSummary.fromJson(json['handshake'] as Map<String, dynamic>)
          : null,
      services: ((json['services'] as List?) ?? [])
          .map((e) => GameService.fromJson(e as Map<String, dynamic>))
          .toList(),
      controlAuthority: json['controlAuthority'] != null
          ? ControlAuthority.fromJsonChecked(json['controlAuthority'] as Map<String, dynamic>)
          : null,
      healthSummary: json['healthSummary'] != null
          ? HealthSummary.fromJson(json['healthSummary'] as Map<String, dynamic>)
          : null,
    );
  }
}

class GameService {
  final String serviceId;
  final String runtimeId;
  final String definitionId;
  final String moduleId;
  final String state;
  final String health;
  final bool connected;
  final bool ready;

  const GameService({
    required this.serviceId,
    required this.runtimeId,
    required this.definitionId,
    required this.moduleId,
    required this.state,
    required this.health,
    required this.connected,
    required this.ready,
  });

  factory GameService.fromJson(Map<String, dynamic> json) {
    return GameService(
      serviceId: (json['serviceId'] ?? '').toString(),
      runtimeId: (json['runtimeId'] ?? '').toString(),
      definitionId: (json['definitionId'] ?? '').toString(),
      moduleId: (json['moduleId'] ?? '').toString(),
      state: (json['state'] ?? '').toString(),
      health: (json['health'] ?? '').toString(),
      connected: (json['connected'] as bool?) ?? false,
      ready: (json['ready'] as bool?) ?? false,
    );
  }

  factory GameService.fromJsonChecked(Map<String, dynamic> json) {
    final serviceId = (json['serviceId'] ?? '').toString();
    final runtimeId = (json['runtimeId'] ?? '').toString();
    if (serviceId.isEmpty || runtimeId.isEmpty) {
      throw StateError('GameService: empty serviceId or runtimeId');
    }
    return GameService(
      serviceId: serviceId,
      runtimeId: runtimeId,
      definitionId: (json['definitionId'] ?? '').toString(),
      moduleId: (json['moduleId'] ?? '').toString(),
      state: (json['state'] ?? '').toString(),
      health: (json['health'] ?? '').toString(),
      connected: (json['connected'] as bool?) ?? false,
      ready: (json['ready'] as bool?) ?? false,
    );
  }
}

class HealthSummary {
  final String status;
  final String? message;
  final DateTime? updatedAt;

  const HealthSummary({
    required this.status,
    this.message,
    this.updatedAt,
  });

  factory HealthSummary.fromJson(Map<String, dynamic> json) {
    return HealthSummary(
      status: (json['status'] ?? '').toString(),
      message: json['message'] as String?,
      updatedAt: json['updatedAt'] != null ? DateTime.tryParse(json['updatedAt']) : null,
    );
  }
}

class ProcessSummary {
  final bool managed;
  final bool running;
  final int processGeneration;
  final int restartCount;

  const ProcessSummary({
    required this.managed,
    required this.running,
    required this.processGeneration,
    required this.restartCount,
  });

  factory ProcessSummary.fromJson(Map<String, dynamic> json) {
    return ProcessSummary(
      managed: (json['managed'] as bool?) ?? false,
      running: (json['running'] as bool?) ?? false,
      processGeneration: (json['processGeneration'] as int?) ?? 0,
      restartCount: (json['restartCount'] as int?) ?? 0,
    );
  }
}

class ConnectionSummary {
  final bool connected;
  final String? protocolVersion;
  final int? peerGeneration;
  final DateTime? lastHeartbeatAt;

  const ConnectionSummary({
    required this.connected,
    this.protocolVersion,
    this.peerGeneration,
    this.lastHeartbeatAt,
  });

  factory ConnectionSummary.fromJson(Map<String, dynamic> json) {
    return ConnectionSummary(
      connected: (json['connected'] as bool?) ?? false,
      protocolVersion: json['protocolVersion'] as String?,
      peerGeneration: json['peerGeneration'] as int?,
      lastHeartbeatAt:
          json['lastHeartbeatAt'] != null ? DateTime.tryParse(json['lastHeartbeatAt']) : null,
    );
  }
}

class HandshakeSummary {
  final String handshakeState;
  final bool ready;
  final String? protocol;
  final String? sdkName;
  final String? sdkVersion;

  const HandshakeSummary({
    required this.handshakeState,
    required this.ready,
    this.protocol,
    this.sdkName,
    this.sdkVersion,
  });

  factory HandshakeSummary.fromJson(Map<String, dynamic> json) {
    return HandshakeSummary(
      handshakeState: (json['handshakeState'] ?? '').toString(),
      ready: (json['ready'] as bool?) ?? false,
      protocol: json['protocol'] as String?,
      sdkName: json['sdkName'] as String?,
      sdkVersion: json['sdkVersion'] as String?,
    );
  }
}

class ControlAuthority {
  final String runtimeId;
  final String mode;
  final int epoch;
  final DateTime? updatedAt;

  const ControlAuthority({
    required this.runtimeId,
    required this.mode,
    required this.epoch,
    this.updatedAt,
  });

  factory ControlAuthority.fromJson(Map<String, dynamic> json) {
    return ControlAuthority(
      runtimeId: (json['runtimeId'] ?? '').toString(),
      mode: (json['mode'] ?? '').toString(),
      epoch: (json['epoch'] as int?) ?? 0,
      updatedAt: json['updatedAt'] != null ? DateTime.tryParse(json['updatedAt']) : null,
    );
  }

  factory ControlAuthority.fromJsonChecked(Map<String, dynamic> json) {
    final runtimeId = (json['runtimeId'] ?? '').toString();
    if (runtimeId.isEmpty) {
      throw StateError('ControlAuthority: empty runtimeId');
    }
    return ControlAuthority(
      runtimeId: runtimeId,
      mode: (json['mode'] ?? '').toString(),
      epoch: (json['epoch'] as int?) ?? 0,
      updatedAt: json['updatedAt'] != null ? DateTime.tryParse(json['updatedAt']) : null,
    );
  }
}

class GameCenterPluginList {
  final List<GamePluginSummary> items;
  final int total;
  final int page;
  final int pageSize;

  const GameCenterPluginList({
    required this.items,
    required this.total,
    required this.page,
    required this.pageSize,
  });

  factory GameCenterPluginList.fromJson(Map<String, dynamic> json) {
    return GameCenterPluginList(
      items: ((json['items'] as List?) ?? [])
          .map((e) => GamePluginSummary.fromJson(e as Map<String, dynamic>))
          .toList(),
      total: (json['total'] as int?) ?? 0,
      page: (json['page'] as int?) ?? 1,
      pageSize: (json['pageSize'] as int?) ?? 20,
    );
  }
}

class GameCenterRuntimeList {
  final List<GameRuntimeSummary> items;
  final int total;

  const GameCenterRuntimeList({
    required this.items,
    required this.total,
  });

  factory GameCenterRuntimeList.fromJson(Map<String, dynamic> json) {
    return GameCenterRuntimeList(
      items: ((json['items'] as List?) ?? [])
          .map((e) => GameRuntimeSummary.fromJson(e as Map<String, dynamic>))
          .toList(),
      total: (json['total'] as int?) ?? 0,
    );
  }
}

class GameCenterServiceList {
  final List<GameService> items;
  final int total;

  const GameCenterServiceList({
    required this.items,
    required this.total,
  });

  factory GameCenterServiceList.fromJson(Map<String, dynamic> json) {
    return GameCenterServiceList(
      items: ((json['items'] as List?) ?? [])
          .map((e) => GameService.fromJson(e as Map<String, dynamic>))
          .toList(),
      total: (json['total'] as int?) ?? 0,
    );
  }
}

class PackageMutationResult {
  final String extensionId;
  final String operation;
  final String state;
  final String? currentVersion;
  final String? warnings;

  const PackageMutationResult({
    required this.extensionId,
    required this.operation,
    required this.state,
    this.currentVersion,
    this.warnings,
  });

  factory PackageMutationResult.fromJson(Map<String, dynamic> json) {
    return PackageMutationResult(
      extensionId: (json['extensionId'] ?? '').toString(),
      operation: (json['operation'] ?? '').toString(),
      state: (json['state'] ?? '').toString(),
      currentVersion: json['currentVersion']?.toString(),
      warnings: json['warnings']?.toString(),
    );
  }

  factory PackageMutationResult.fromJsonChecked(Map<String, dynamic> json) {
    final extensionId = (json['extensionId'] ?? '').toString();
    if (extensionId.isEmpty) {
      throw StateError('PackageMutationResult: empty extensionId');
    }
    return PackageMutationResult(
      extensionId: extensionId,
      operation: (json['operation'] ?? '').toString(),
      state: (json['state'] ?? '').toString(),
      currentVersion: json['currentVersion']?.toString(),
      warnings: json['warnings']?.toString(),
    );
  }
}

class RuntimeMutationResult {
  final String runtimeId;
  final String operation;

  const RuntimeMutationResult({
    required this.runtimeId,
    required this.operation,
  });

  factory RuntimeMutationResult.fromJson(Map<String, dynamic> json) {
    return RuntimeMutationResult(
      runtimeId: (json['runtimeId'] ?? '').toString(),
      operation: (json['operation'] ?? '').toString(),
    );
  }

  factory RuntimeMutationResult.fromJsonChecked(Map<String, dynamic> json) {
    final runtimeId = (json['runtimeId'] ?? '').toString();
    if (runtimeId.isEmpty) {
      throw StateError('RuntimeMutationResult: empty runtimeId');
    }
    return RuntimeMutationResult(
      runtimeId: runtimeId,
      operation: (json['operation'] ?? '').toString(),
    );
  }
}

class ControlMutationResult {
  final bool success;
  final int? newEpoch;
  final String? mode;

  const ControlMutationResult({
    required this.success,
    this.newEpoch,
    this.mode,
  });

  factory ControlMutationResult.fromJson(Map<String, dynamic> json) {
    return ControlMutationResult(
      success: (json['success'] as bool?) ?? false,
      newEpoch: json['newEpoch'] as int?,
      mode: json['mode'] as String?,
    );
  }
}

class GameCenterHealthResponse {
  final String status;
  final String? version;
  final DateTime? timestamp;

  const GameCenterHealthResponse({
    required this.status,
    this.version,
    this.timestamp,
  });

  factory GameCenterHealthResponse.fromJson(Map<String, dynamic> json) {
    return GameCenterHealthResponse(
      status: (json['status'] ?? '').toString(),
      version: json['version'] as String?,
      timestamp: json['timestamp'] != null ? DateTime.tryParse(json['timestamp']) : null,
    );
  }
}

class GameRuntimeServicesResponse {
  final List<GameServiceSummary> services;

  const GameRuntimeServicesResponse({
    required this.services,
  });

  factory GameRuntimeServicesResponse.fromJson(Map<String, dynamic> json) {
    return GameRuntimeServicesResponse(
      services: ((json['services'] as List?) ?? [])
          .map((e) => GameServiceSummary.fromJson(e as Map<String, dynamic>))
          .toList(),
    );
  }
}

class GameServiceSummary {
  final String serviceId;
  final String state;
  final String health;

  const GameServiceSummary({
    required this.serviceId,
    required this.state,
    required this.health,
  });

  factory GameServiceSummary.fromJson(Map<String, dynamic> json) {
    return GameServiceSummary(
      serviceId: (json['serviceId'] ?? '').toString(),
      state: (json['state'] ?? '').toString(),
      health: (json['health'] ?? '').toString(),
    );
  }
}

class GameRuntimeHealthResponse {
  final String status;
  final String? runtimeState;
  final DateTime? lastHeartbeat;

  const GameRuntimeHealthResponse({
    required this.status,
    this.runtimeState,
    this.lastHeartbeat,
  });

  factory GameRuntimeHealthResponse.fromJson(Map<String, dynamic> json) {
    return GameRuntimeHealthResponse(
      status: (json['status'] ?? '').toString(),
      runtimeState: json['runtimeState'] as String?,
      lastHeartbeat: json['lastHeartbeat'] != null ? DateTime.tryParse(json['lastHeartbeat']) : null,
    );
  }
}

class GamePluginHandshakeResponse {
  final bool accepted;
  final String? protocol;
  final String? error;

  const GamePluginHandshakeResponse({
    required this.accepted,
    this.protocol,
    this.error,
  });

  factory GamePluginHandshakeResponse.fromJson(Map<String, dynamic> json) {
    return GamePluginHandshakeResponse(
      accepted: (json['accepted'] as bool?) ?? false,
      protocol: json['protocol'] as String?,
      error: json['error'] as String?,
    );
  }
}

enum MobileDeploymentMode {
  local,
  cloud,
  hybrid;

  static MobileDeploymentMode fromStorage(String? raw) {
    switch (raw) {
      case 'local':
        return MobileDeploymentMode.local;
      case 'cloud':
      case 'remote':
        return MobileDeploymentMode.cloud;
      case 'hybrid':
        return MobileDeploymentMode.hybrid;
      default:
        return MobileDeploymentMode.local;
    }
  }

  String get storageValue {
    switch (this) {
      case MobileDeploymentMode.local:
        return 'local';
      case MobileDeploymentMode.cloud:
        return 'cloud';
      case MobileDeploymentMode.hybrid:
        return 'hybrid';
    }
  }
}

class MobileDeploymentConfig {
  final MobileDeploymentMode mode;
  final String? remoteCoreUri;

  const MobileDeploymentConfig({
    required this.mode,
    this.remoteCoreUri,
  });

  MobileDeploymentConfig copyWith({
    MobileDeploymentMode? mode,
    String? remoteCoreUri,
  }) {
    return MobileDeploymentConfig(
      mode: mode ?? this.mode,
      remoteCoreUri: remoteCoreUri ?? this.remoteCoreUri,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'mode': mode.storageValue,
      if (remoteCoreUri != null) 'remoteCoreUri': remoteCoreUri,
    };
  }

  factory MobileDeploymentConfig.fromJson(Map<String, dynamic> json) {
    return MobileDeploymentConfig(
      mode: MobileDeploymentMode.fromStorage(json['mode'] as String?),
      remoteCoreUri: json['remoteCoreUri'] as String?,
    );
  }

  static MobileDeploymentConfig get local => const MobileDeploymentConfig(
        mode: MobileDeploymentMode.local,
      );
}

class DeploymentConfigValidationError extends Error {
  final String message;
  DeploymentConfigValidationError(this.message);

  @override
  String toString() => 'DeploymentConfigValidationError: $message';
}

DeploymentConfigValidationError? validateDeploymentConfigForSave(
  MobileDeploymentConfig? config,
) {
  if (config == null) {
    return DeploymentConfigValidationError('config is null');
  }
  if (config.mode == MobileDeploymentMode.cloud ||
      config.mode == MobileDeploymentMode.hybrid) {
    if (config.remoteCoreUri == null || config.remoteCoreUri!.trim().isEmpty) {
      return DeploymentConfigValidationError(
        'cloud/hybrid mode requires remote core URI',
      );
    }
  }
  return null;
}

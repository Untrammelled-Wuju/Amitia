import 'mobile_deployment_mode.dart';

enum BackendEndpointRole {
  businessCore,
  localRuntime,
}

enum EmbeddedRuntimeProfile {
  local,
  deviceAgent;

  String get runtimeProfileArg {
    switch (this) {
      case EmbeddedRuntimeProfile.local:
        return 'local';
      case EmbeddedRuntimeProfile.deviceAgent:
        return 'device-agent';
    }
  }

  static EmbeddedRuntimeProfile fromRuntimeProfile(String? raw) {
    switch (raw) {
      case 'device-agent':
        return EmbeddedRuntimeProfile.deviceAgent;
      case 'local':
      default:
        return EmbeddedRuntimeProfile.local;
    }
  }
}

class BackendEndpoint {
  final BackendEndpointRole role;
  final Uri httpBaseUri;
  final Uri websocketBaseUri;
  final bool isRemote;

  const BackendEndpoint({
    required this.role,
    required this.httpBaseUri,
    required this.websocketBaseUri,
    required this.isRemote,
  });

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is BackendEndpoint &&
          role == other.role &&
          httpBaseUri == other.httpBaseUri &&
          websocketBaseUri == other.websocketBaseUri &&
          isRemote == other.isRemote;

  @override
  int get hashCode =>
      role.hashCode ^
      httpBaseUri.hashCode ^
      websocketBaseUri.hashCode ^
      isRemote.hashCode;
}

class MobileBackendTopology {
  final MobileDeploymentMode mode;
  final BackendEndpoint businessCore;
  final BackendEndpoint? localRuntime;
  final EmbeddedRuntimeProfile? embeddedRuntimeProfile;
  final bool requiresEmbeddedRuntime;

  const MobileBackendTopology({
    required this.mode,
    required this.businessCore,
    this.localRuntime,
    this.embeddedRuntimeProfile,
    required this.requiresEmbeddedRuntime,
  });

  bool sameBusinessEndpoint(MobileBackendTopology other) {
    return businessCore == other.businessCore;
  }

  bool sameEmbeddedRuntime(MobileBackendTopology other) {
    if (embeddedRuntimeProfile != other.embeddedRuntimeProfile) return false;
    if (localRuntime == null && other.localRuntime == null) return true;
    if (localRuntime == null || other.localRuntime == null) return false;
    return localRuntime == other.localRuntime;
  }

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is MobileBackendTopology &&
          mode == other.mode &&
          businessCore == other.businessCore &&
          localRuntime == other.localRuntime &&
          embeddedRuntimeProfile == other.embeddedRuntimeProfile &&
          requiresEmbeddedRuntime == other.requiresEmbeddedRuntime;

  @override
  int get hashCode =>
      mode.hashCode ^
      businessCore.hashCode ^
      localRuntime.hashCode ^
      embeddedRuntimeProfile.hashCode ^
      requiresEmbeddedRuntime.hashCode;
}

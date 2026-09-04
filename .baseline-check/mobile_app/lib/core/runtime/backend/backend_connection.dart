import 'backend_topology.dart';

enum BackendAuthStrategy {
  localTrusted,
  bearer,
}

class BackendConnection {
  final Uri httpBaseUri;
  final Uri websocketBaseUri;
  final bool isRemote;
  final BackendAuthStrategy authStrategy;

  const BackendConnection({
    required this.httpBaseUri,
    required this.websocketBaseUri,
    required this.isRemote,
    required this.authStrategy,
  });

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is BackendConnection &&
          httpBaseUri == other.httpBaseUri &&
          websocketBaseUri == other.websocketBaseUri &&
          authStrategy == other.authStrategy;

  @override
  int get hashCode =>
      httpBaseUri.hashCode ^
      websocketBaseUri.hashCode ^
      authStrategy.hashCode;

  static BackendConnection fromBusinessCore(
    MobileBackendTopology topology,
  ) {
    return BackendConnection(
      httpBaseUri: topology.businessCore.httpBaseUri,
      websocketBaseUri: topology.businessCore.websocketBaseUri,
      isRemote: topology.businessCore.isRemote,
      authStrategy: topology.businessCore.isRemote
          ? BackendAuthStrategy.bearer
          : BackendAuthStrategy.localTrusted,
    );
  }
}

import '../backend/backend_topology.dart';

enum EmbeddedRuntimeStatus {
  notInstalled,
  installing,
  stopped,
  starting,
  ready,
  stopping,
  failed,
  unsupported,
}

abstract interface class EmbeddedRuntimeController {
  Future<EmbeddedRuntimeStatus> ensureRunning(EmbeddedRuntimeProfile profile);
  Future<void> stop();
  Future<EmbeddedRuntimeStatus> getStatus();
  Future<BackendEndpoint> getEndpoint();
}

final class UnsupportedEmbeddedRuntimeController
    implements EmbeddedRuntimeController {
  const UnsupportedEmbeddedRuntimeController();

  @override
  Future<EmbeddedRuntimeStatus> ensureRunning(
    EmbeddedRuntimeProfile profile,
  ) async => EmbeddedRuntimeStatus.unsupported;

  @override
  Future<EmbeddedRuntimeStatus> getStatus() async =>
      EmbeddedRuntimeStatus.unsupported;

  @override
  Future<void> stop() async {}

  @override
  Future<BackendEndpoint> getEndpoint() {
    throw UnsupportedError(
      'embedded runtime is not available on this mobile platform',
    );
  }
}

import 'dart:async';
import 'mobile_deployment_mode.dart';
import 'backend_topology.dart';
import 'backend_topology_resolver.dart';
import 'backend_connection.dart';
import '../embedded/embedded_runtime_controller.dart';

export '../embedded/embedded_runtime_controller.dart' show EmbeddedRuntimeStatus;

enum RuntimeDeploymentState {
  unavailable,
  starting,
  ready,
  stopping,
  failed,
}

class BackendNodeStatus {
  final RuntimeDeploymentState state;
  final Uri? baseUri;
  final String? profile;
  final String? message;

  const BackendNodeStatus({
    required this.state,
    this.baseUri,
    this.profile,
    this.message,
  });

  factory BackendNodeStatus.initial() => const BackendNodeStatus(
        state: RuntimeDeploymentState.unavailable,
      );

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is BackendNodeStatus &&
          state == other.state &&
          baseUri == other.baseUri &&
          profile == other.profile &&
          message == other.message;

  @override
  int get hashCode =>
      state.hashCode ^
      baseUri.hashCode ^
      profile.hashCode ^
      message.hashCode;
}

class MobileBackendStatus {
  final MobileDeploymentMode mode;
  final RuntimeDeploymentState state;
  final BackendNodeStatus businessCore;
  final BackendNodeStatus? localRuntime;
  final int generation;

  const MobileBackendStatus({
    required this.mode,
    required this.state,
    required this.businessCore,
    this.localRuntime,
    required this.generation,
  });

  factory MobileBackendStatus.initial() => MobileBackendStatus(
        mode: MobileDeploymentMode.local,
        state: RuntimeDeploymentState.unavailable,
        businessCore: BackendNodeStatus.initial(),
        generation: 0,
      );

  bool get isBusinessReady =>
      businessCore.state == RuntimeDeploymentState.ready;

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is MobileBackendStatus &&
          mode == other.mode &&
          state == other.state &&
          businessCore == other.businessCore &&
          localRuntime == other.localRuntime &&
          generation == other.generation;

  @override
  int get hashCode =>
      mode.hashCode ^
      state.hashCode ^
      businessCore.hashCode ^
      localRuntime.hashCode ^
      generation.hashCode;
}

abstract interface class RemoteCoreProbe {
  Future<bool> probe(Uri baseUri, {Duration timeout});
}

abstract interface class MobileBackendLifecycle {
  Stream<MobileBackendStatus> get statusStream;
  MobileBackendStatus get currentStatus;
  Future<void> reconcile(MobileDeploymentConfig config);
  Future<void> shutdown();
}

class DefaultMobileBackendLifecycle implements MobileBackendLifecycle {
  final BackendTopologyResolver _resolver;
  final EmbeddedRuntimeController _embeddedRuntime;
  final RemoteCoreProbe _remoteProbe;

  final StreamController<MobileBackendStatus> _statusController =
      StreamController<MobileBackendStatus>.broadcast();

  MobileBackendStatus _currentStatus = MobileBackendStatus.initial();
  int _generation = 0;
  Future<void> _reconcileChain = Future.value();
  bool _disposed = false;

  DefaultMobileBackendLifecycle({
    required BackendTopologyResolver resolver,
    required EmbeddedRuntimeController embeddedRuntime,
    required RemoteCoreProbe remoteProbe,
  })  : _resolver = resolver,
        _embeddedRuntime = embeddedRuntime,
        _remoteProbe = remoteProbe;

  @override
  Stream<MobileBackendStatus> get statusStream => _statusController.stream;

  @override
  MobileBackendStatus get currentStatus => _currentStatus;

  @override
  Future<void> reconcile(MobileDeploymentConfig config) async {
    final capturedGeneration = ++_generation;
    _reconcileChain = _reconcileChain.then((_) async {
      if (_disposed || capturedGeneration != _generation) return;
      await _performReconcile(config, capturedGeneration);
    });
    await _reconcileChain;
  }

  Future<void> _performReconcile(
    MobileDeploymentConfig config,
    int expectedGeneration,
  ) async {
    if (_disposed || expectedGeneration != _generation) return;

    MobileBackendTopology topology;
    try {
      topology = _resolver.resolve(config);
    } on DeploymentConfigValidationError catch (e) {
      _emitStatus(MobileBackendStatus(
        mode: config.mode,
        state: RuntimeDeploymentState.failed,
        businessCore: BackendNodeStatus(
          state: RuntimeDeploymentState.failed,
          message: e.message,
        ),
        generation: expectedGeneration,
      ));
      return;
    }

    if (!topology.requiresEmbeddedRuntime) {
      await _reconcileCloud(topology, expectedGeneration);
    } else if (topology.mode == MobileDeploymentMode.local) {
      await _reconcileLocal(topology, expectedGeneration);
    } else {
      await _reconcileHybrid(topology, expectedGeneration);
    }
  }

  Future<void> _reconcileCloud(
    MobileBackendTopology topology,
    int expectedGeneration,
  ) async {
    await _stopEmbeddedRuntimeIfRunning(expectedGeneration);
    if (expectedGeneration != _generation) return;

    _updateBusinessNode(
      expectedGeneration,
      RuntimeDeploymentState.starting,
      topology.businessCore.httpBaseUri,
    );

    final reachable = await _remoteProbe.probe(topology.businessCore.httpBaseUri);
    if (expectedGeneration != _generation) return;

    if (reachable) {
      _emitStatus(MobileBackendStatus(
        mode: MobileDeploymentMode.cloud,
        state: RuntimeDeploymentState.ready,
        businessCore: BackendNodeStatus(
          state: RuntimeDeploymentState.ready,
          baseUri: topology.businessCore.httpBaseUri,
        ),
        generation: expectedGeneration,
      ));
    } else {
      _emitStatus(MobileBackendStatus(
        mode: MobileDeploymentMode.cloud,
        state: RuntimeDeploymentState.failed,
        businessCore: BackendNodeStatus(
          state: RuntimeDeploymentState.failed,
          baseUri: topology.businessCore.httpBaseUri,
          message: 'remote core not reachable',
        ),
        generation: expectedGeneration,
      ));
    }
  }

  Future<void> _reconcileLocal(
    MobileBackendTopology topology,
    int expectedGeneration,
  ) async {
    _updateBusinessNode(
      expectedGeneration,
      RuntimeDeploymentState.starting,
      topology.businessCore.httpBaseUri,
    );

    try {
      final status = await _embeddedRuntime.ensureRunning(
        EmbeddedRuntimeProfile.local,
      );
      if (expectedGeneration != _generation) return;

      if (status != EmbeddedRuntimeStatus.ready) {
        _emitStatus(MobileBackendStatus(
          mode: MobileDeploymentMode.local,
          state: RuntimeDeploymentState.failed,
          businessCore: BackendNodeStatus(
            state: RuntimeDeploymentState.failed,
            baseUri: topology.businessCore.httpBaseUri,
            profile: 'local',
            message: 'embedded runtime failed to start',
          ),
          localRuntime: BackendNodeStatus(
            state: RuntimeDeploymentState.failed,
            profile: 'local',
            message: 'embedded runtime failed to start',
          ),
          generation: expectedGeneration,
        ));
        return;
      }

      _emitStatus(MobileBackendStatus(
        mode: MobileDeploymentMode.local,
        state: RuntimeDeploymentState.ready,
        businessCore: BackendNodeStatus(
          state: RuntimeDeploymentState.ready,
          baseUri: topology.businessCore.httpBaseUri,
          profile: 'local',
        ),
        localRuntime: BackendNodeStatus(
          state: RuntimeDeploymentState.ready,
          baseUri: topology.localRuntime?.httpBaseUri,
          profile: 'local',
        ),
        generation: expectedGeneration,
      ));
    } catch (e) {
      if (expectedGeneration != _generation) return;
      _emitStatus(MobileBackendStatus(
        mode: MobileDeploymentMode.local,
        state: RuntimeDeploymentState.failed,
        businessCore: BackendNodeStatus(
          state: RuntimeDeploymentState.failed,
          message: e.toString(),
        ),
        generation: expectedGeneration,
      ));
    }
  }

  Future<void> _reconcileHybrid(
    MobileBackendTopology topology,
    int expectedGeneration,
  ) async {
    _updateBusinessNode(
      expectedGeneration,
      RuntimeDeploymentState.starting,
      topology.businessCore.httpBaseUri,
    );

    final localFuture = _embeddedRuntime.ensureRunning(
      EmbeddedRuntimeProfile.deviceAgent,
    );
    final remoteFuture = _remoteProbe.probe(topology.businessCore.httpBaseUri);

    final results = await Future.wait([localFuture, remoteFuture]);
    if (expectedGeneration != _generation) return;

    final localStatus = results[0] as EmbeddedRuntimeStatus;
    final remoteReachable = results[1] as bool;

    final localNodeStatus = BackendNodeStatus(
      state: localStatus == EmbeddedRuntimeStatus.ready
          ? RuntimeDeploymentState.ready
          : RuntimeDeploymentState.failed,
      baseUri: topology.localRuntime?.httpBaseUri,
      profile: 'device-agent',
      message: localStatus != EmbeddedRuntimeStatus.ready
          ? 'device-agent not ready'
          : null,
    );

    if (remoteReachable) {
      _emitStatus(MobileBackendStatus(
        mode: MobileDeploymentMode.hybrid,
        state: RuntimeDeploymentState.ready,
        businessCore: BackendNodeStatus(
          state: RuntimeDeploymentState.ready,
          baseUri: topology.businessCore.httpBaseUri,
        ),
        localRuntime: localNodeStatus,
        generation: expectedGeneration,
      ));
    } else {
      _emitStatus(MobileBackendStatus(
        mode: MobileDeploymentMode.hybrid,
        state: RuntimeDeploymentState.failed,
        businessCore: BackendNodeStatus(
          state: RuntimeDeploymentState.failed,
          baseUri: topology.businessCore.httpBaseUri,
          message: 'remote core not reachable',
        ),
        localRuntime: localNodeStatus,
        generation: expectedGeneration,
      ));
    }
  }

  Future<void> _stopEmbeddedRuntimeIfRunning(int expectedGeneration) async {
    try {
      await _embeddedRuntime.stop();
    } catch (_) {}
  }

  void _updateBusinessNode(
    int expectedGeneration,
    RuntimeDeploymentState state,
    Uri baseUri,
  ) {
    if (expectedGeneration != _generation) return;
    _emitStatus(MobileBackendStatus(
      mode: _currentStatus.mode,
      state: state,
      businessCore: BackendNodeStatus(
        state: state,
        baseUri: baseUri,
      ),
      generation: expectedGeneration,
    ));
  }

  void _emitStatus(MobileBackendStatus status) {
    if (_disposed) return;
    _currentStatus = status;
    _statusController.add(status);
  }

  @override
  Future<void> shutdown() async {
    _disposed = true;
    _generation++;
    await _embeddedRuntime.stop();
    await _statusController.close();
  }
}

BackendConnection buildBusinessConnection(MobileBackendTopology topology) {
  return BackendConnection.fromBusinessCore(topology);
}

BackendConnection? buildLocalConnection(MobileBackendTopology topology) {
  final localRuntime = topology.localRuntime;
  if (localRuntime == null) return null;
  return BackendConnection(
    httpBaseUri: localRuntime.httpBaseUri,
    websocketBaseUri: localRuntime.websocketBaseUri,
    isRemote: localRuntime.isRemote,
    authStrategy: BackendAuthStrategy.localTrusted,
  );
}

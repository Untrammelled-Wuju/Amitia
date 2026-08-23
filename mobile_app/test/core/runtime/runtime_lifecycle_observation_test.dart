import 'dart:async';
import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/runtime/backend/mobile_backend_lifecycle.dart';
import 'package:amitia_app/core/runtime/backend/mobile_deployment_mode.dart';
import 'package:amitia_app/core/runtime/backend/backend_topology.dart';
import 'package:amitia_app/core/runtime/backend/backend_topology_resolver.dart';
import 'package:amitia_app/core/runtime/embedded/embedded_runtime_controller.dart';
import 'package:amitia_app/core/runtime/backend_transport/connectivity/backend_connectivity_probe.dart';

class FakeEmbeddedRuntimeController implements EmbeddedRuntimeController {
  EmbeddedRuntimeStatus _status = EmbeddedRuntimeStatus.stopped;
  int ensureRunningCallCount = 0;
  int getStatusCallCount = 0;
  int stopCallCount = 0;

  void setStatus(EmbeddedRuntimeStatus status) {
    _status = status;
  }

  @override
  Future<EmbeddedRuntimeStatus> ensureRunning(EmbeddedRuntimeProfile profile) async {
    ensureRunningCallCount++;
    return _status;
  }

  @override
  Future<EmbeddedRuntimeStatus> getStatus() async {
    getStatusCallCount++;
    return _status;
  }

  @override
  Future<void> stop() async {
    stopCallCount++;
  }

  @override
  Future<BackendEndpoint> getEndpoint() async {
    return BackendEndpoint(
      role: BackendEndpointRole.localRuntime,
      httpBaseUri: Uri(scheme: 'http', host: '127.0.0.1', port: 18899),
      websocketBaseUri: Uri(scheme: 'ws', host: '127.0.0.1', port: 18899),
      isRemote: false,
    );
  }
}

class FakeBackendConnectivityProbe implements BackendConnectivityProbe {
  BackendConnectivityResult _result = BackendConnectivityResult.unreachable;

  void setResult(BackendConnectivityResult result) {
    _result = result;
  }

  @override
  Future<BackendConnectivityResult> probe() async {
    return _result;
  }
}

class FakeBackendTopologyResolver implements BackendTopologyResolver {
  final MobileBackendTopology _topology;

  FakeBackendTopologyResolver(this._topology);

  @override
  MobileBackendTopology resolve(MobileDeploymentConfig config) {
    return _topology;
  }
}

MobileBackendTopology _createLocalTopology() {
  return MobileBackendTopology(
    mode: MobileDeploymentMode.local,
    businessCore: BackendEndpoint(
      role: BackendEndpointRole.businessCore,
      httpBaseUri: Uri(scheme: 'http', host: '127.0.0.1', port: 18899),
      websocketBaseUri: Uri(scheme: 'ws', host: '127.0.0.1', port: 18899),
      isRemote: false,
    ),
    localRuntime: BackendEndpoint(
      role: BackendEndpointRole.localRuntime,
      httpBaseUri: Uri(scheme: 'http', host: '127.0.0.1', port: 18899),
      websocketBaseUri: Uri(scheme: 'ws', host: '127.0.0.1', port: 18899),
      isRemote: false,
    ),
    requiresEmbeddedRuntime: true,
  );
}

MobileBackendTopology _createCloudTopology() {
  return MobileBackendTopology(
    mode: MobileDeploymentMode.cloud,
    businessCore: BackendEndpoint(
      role: BackendEndpointRole.businessCore,
      httpBaseUri: Uri(scheme: 'https', host: 'remote.example.com', port: 443),
      websocketBaseUri: Uri(scheme: 'wss', host: 'remote.example.com', port: 443),
      isRemote: true,
    ),
    localRuntime: null,
    requiresEmbeddedRuntime: false,
  );
}

void main() {
  group('Embedded runtime lifecycle observation', () {
    test('local mode: stopped → ensureRunning → ready', () async {
      final embedded = FakeEmbeddedRuntimeController();
      final probe = FakeBackendConnectivityProbe();
      final resolver = FakeBackendTopologyResolver(_createLocalTopology());

      final lifecycle = DefaultMobileBackendLifecycle(
        resolver: resolver,
        embeddedRuntime: embedded,
        remoteProbe: _ConnectivityProbeAdapter(probe),
      );

      final statuses = <MobileBackendStatus>[];
      final sub = lifecycle.statusStream.listen(statuses.add);

      embedded.setStatus(EmbeddedRuntimeStatus.ready);

      await lifecycle.reconcile(const MobileDeploymentConfig(mode: MobileDeploymentMode.local));
      await Future.delayed(const Duration(milliseconds: 100));

      expect(statuses.isNotEmpty, isTrue);
      expect(statuses.last.mode, equals(MobileDeploymentMode.local));
      expect(statuses.last.state, equals(RuntimeDeploymentState.ready));
      expect(statuses.last.businessCore.state, equals(RuntimeDeploymentState.ready));
      expect(embedded.ensureRunningCallCount, greaterThan(0));

      await lifecycle.shutdown();
      await sub.cancel();
    });

    test('local mode: ensureRunning returns failed → state failed', () async {
      final embedded = FakeEmbeddedRuntimeController();
      final probe = FakeBackendConnectivityProbe();
      final resolver = FakeBackendTopologyResolver(_createLocalTopology());

      final lifecycle = DefaultMobileBackendLifecycle(
        resolver: resolver,
        embeddedRuntime: embedded,
        remoteProbe: _ConnectivityProbeAdapter(probe),
      );

      final statuses = <MobileBackendStatus>[];
      final sub = lifecycle.statusStream.listen(statuses.add);

      embedded.setStatus(EmbeddedRuntimeStatus.failed);

      await lifecycle.reconcile(const MobileDeploymentConfig(mode: MobileDeploymentMode.local));
      await Future.delayed(const Duration(milliseconds: 100));

      expect(statuses.isNotEmpty, isTrue);
      expect(statuses.last.state, equals(RuntimeDeploymentState.failed));
      expect(statuses.last.businessCore.state, equals(RuntimeDeploymentState.failed));

      await lifecycle.shutdown();
      await sub.cancel();
    });

    test('local mode: notInstalled → state failed with message', () async {
      final embedded = FakeEmbeddedRuntimeController();
      final probe = FakeBackendConnectivityProbe();
      final resolver = FakeBackendTopologyResolver(_createLocalTopology());

      final lifecycle = DefaultMobileBackendLifecycle(
        resolver: resolver,
        embeddedRuntime: embedded,
        remoteProbe: _ConnectivityProbeAdapter(probe),
      );

      final statuses = <MobileBackendStatus>[];
      final sub = lifecycle.statusStream.listen(statuses.add);

      embedded.setStatus(EmbeddedRuntimeStatus.notInstalled);

      await lifecycle.reconcile(const MobileDeploymentConfig(mode: MobileDeploymentMode.local));
      await Future.delayed(const Duration(milliseconds: 100));

      expect(statuses.isNotEmpty, isTrue);
      expect(statuses.last.state, equals(RuntimeDeploymentState.failed));
      expect(statuses.last.businessCore.message, contains('not installed'));

      await lifecycle.shutdown();
      await sub.cancel();
    });

    test('cloud mode: remote reachable → state ready', () async {
      final embedded = FakeEmbeddedRuntimeController();
      final probe = FakeBackendConnectivityProbe();
      final resolver = FakeBackendTopologyResolver(_createCloudTopology());

      final lifecycle = DefaultMobileBackendLifecycle(
        resolver: resolver,
        embeddedRuntime: embedded,
        remoteProbe: _ConnectivityProbeAdapter(probe),
      );

      final statuses = <MobileBackendStatus>[];
      final sub = lifecycle.statusStream.listen(statuses.add);

      probe.setResult(BackendConnectivityResult.ready);

      await lifecycle.reconcile(const MobileDeploymentConfig(mode: MobileDeploymentMode.cloud));
      await Future.delayed(const Duration(milliseconds: 100));

      expect(statuses.isNotEmpty, isTrue);
      expect(statuses.last.mode, equals(MobileDeploymentMode.cloud));
      expect(statuses.last.state, equals(RuntimeDeploymentState.ready));
      expect(statuses.last.businessCore.state, equals(RuntimeDeploymentState.ready));

      await lifecycle.shutdown();
      await sub.cancel();
    });

    test('cloud mode: remote unreachable → state failed', () async {
      final embedded = FakeEmbeddedRuntimeController();
      final probe = FakeBackendConnectivityProbe();
      final resolver = FakeBackendTopologyResolver(_createCloudTopology());

      final lifecycle = DefaultMobileBackendLifecycle(
        resolver: resolver,
        embeddedRuntime: embedded,
        remoteProbe: _ConnectivityProbeAdapter(probe),
      );

      final statuses = <MobileBackendStatus>[];
      final sub = lifecycle.statusStream.listen(statuses.add);

      probe.setResult(BackendConnectivityResult.unreachable);

      await lifecycle.reconcile(const MobileDeploymentConfig(mode: MobileDeploymentMode.cloud));
      await Future.delayed(const Duration(milliseconds: 100));

      expect(statuses.isNotEmpty, isTrue);
      expect(statuses.last.state, equals(RuntimeDeploymentState.failed));
      expect(statuses.last.businessCore.message, contains('not reachable'));

      await lifecycle.shutdown();
      await sub.cancel();
    });

    test('generation increment prevents stale reconcile', () async {
      final embedded = FakeEmbeddedRuntimeController();
      final probe = FakeBackendConnectivityProbe();
      final resolver = FakeBackendTopologyResolver(_createLocalTopology());

      final lifecycle = DefaultMobileBackendLifecycle(
        resolver: resolver,
        embeddedRuntime: embedded,
        remoteProbe: _ConnectivityProbeAdapter(probe),
      );

      final statuses = <MobileBackendStatus>[];
      final sub = lifecycle.statusStream.listen(statuses.add);

      // First reconcile with ready status
      embedded.setStatus(EmbeddedRuntimeStatus.ready);
      await lifecycle.reconcile(const MobileDeploymentConfig(mode: MobileDeploymentMode.local));
      await Future.delayed(const Duration(milliseconds: 50));

      // Second reconcile should supersede
      await lifecycle.reconcile(const MobileDeploymentConfig(mode: MobileDeploymentMode.local));
      await Future.delayed(const Duration(milliseconds: 100));

      // Should have statuses from both reconciles
      expect(statuses.length, greaterThanOrEqualTo(2));

      await lifecycle.shutdown();
      await sub.cancel();
    });

    test('shutdown stops embedded runtime', () async {
      final embedded = FakeEmbeddedRuntimeController();
      final probe = FakeBackendConnectivityProbe();
      final resolver = FakeBackendTopologyResolver(_createLocalTopology());

      final lifecycle = DefaultMobileBackendLifecycle(
        resolver: resolver,
        embeddedRuntime: embedded,
        remoteProbe: _ConnectivityProbeAdapter(probe),
      );

      await lifecycle.shutdown();

      expect(embedded.stopCallCount, equals(1));
    });
  });
}

class _ConnectivityProbeAdapter implements RemoteCoreProbe {
  final BackendConnectivityProbe _probe;

  _ConnectivityProbeAdapter(this._probe);

  @override
  Future<BackendConnectivityResult> probe(Uri baseUri, {Duration timeout = const Duration(seconds: 5)}) async {
    return _probe.probe();
  }
}

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'mobile_deployment_mode.dart';
import 'mobile_deployment_config_repository.dart';
import 'backend_topology_resolver.dart';
import 'mobile_backend_lifecycle.dart';
import '../embedded/embedded_runtime_controller.dart';
import '../embedded/android_embedded_runtime_controller.dart';
import '../../backend_transport/connectivity/backend_connectivity_probe.dart';
import '../../backend_transport/connectivity/backend_connectivity_providers.dart';

final mobileDeploymentConfigRepositoryProvider =
    Provider<MobileDeploymentConfigRepository>((ref) {
  return const SharedPreferencesMobileDeploymentConfigRepository();
});

final mobileDeploymentConfigProvider =
    StateNotifierProvider<MobileDeploymentConfigNotifier, MobileDeploymentConfig>((ref) {
  final repo = ref.watch(mobileDeploymentConfigRepositoryProvider);
  return MobileDeploymentConfigNotifier(repo);
});

class MobileDeploymentConfigNotifier extends StateNotifier<MobileDeploymentConfig> {
  final MobileDeploymentConfigRepository _repo;
  bool _initialized = false;

  MobileDeploymentConfigNotifier(this._repo) : super(MobileDeploymentConfig.local);

  Future<void> init() async {
    if (_initialized) return;
    _initialized = true;
    state = await _repo.load();
  }

  Future<void> update(MobileDeploymentConfig config) async {
    state = config;
    await _repo.save(config);
  }
}

final backendTopologyResolverProvider =
    Provider<BackendTopologyResolver>((ref) {
  return DefaultBackendTopologyResolver();
});

final embeddedRuntimeControllerProvider =
    Provider<EmbeddedRuntimeController>((ref) {
  return AndroidEmbeddedRuntimeController();
});

final mobileBackendLifecycleProvider =
    Provider<MobileBackendLifecycle>((ref) {
  final resolver = ref.watch(backendTopologyResolverProvider);
  final embedded = ref.watch(embeddedRuntimeControllerProvider);
  final connectivityProbe = ref.watch(backendConnectivityProbeProvider);

  final lifecycle = connectivityProbe != null
      ? DefaultMobileBackendLifecycle.withProbe(
          resolver: resolver,
          embeddedRuntime: embedded,
          connectivityProbe: connectivityProbe,
        )
      : DefaultMobileBackendLifecycle(
          resolver: resolver,
          embeddedRuntime: embedded,
          remoteProbe: _NoopRemoteCoreProbe(),
        );

  ref.onDispose(() => lifecycle.shutdown());
  return lifecycle;
});

final mobileBackendStatusProvider =
    StreamProvider<MobileBackendStatus>((ref) {
  final lifecycle = ref.watch(mobileBackendLifecycleProvider);
  return lifecycle.statusStream.distinct();
});

class _NoopRemoteCoreProbe implements RemoteCoreProbe {
  @override
  Future<BackendConnectivityResult> probe(Uri baseUri, {Duration timeout = const Duration(seconds: 5)}) async {
    return BackendConnectivityResult.unreachable;
  }
}

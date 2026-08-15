import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'mobile_deployment_mode.dart';
import 'mobile_deployment_config_repository.dart';
import 'backend_topology_resolver.dart';
import 'mobile_backend_lifecycle.dart';
import '../embedded/embedded_runtime_controller.dart';
import '../embedded/android_embedded_runtime_controller.dart';

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

final remoteCoreProbeProvider = Provider<RemoteCoreProbe>((ref) {
  return DefaultRemoteCoreProbe();
});

final mobileBackendLifecycleProvider =
    Provider<MobileBackendLifecycle>((ref) {
  final resolver = ref.watch(backendTopologyResolverProvider);
  final embedded = ref.watch(embeddedRuntimeControllerProvider);
  final probe = ref.watch(remoteCoreProbeProvider);

  final lifecycle = DefaultMobileBackendLifecycle(
    resolver: resolver,
    embeddedRuntime: embedded,
    remoteProbe: probe,
  );

  ref.onDispose(() => lifecycle.shutdown());
  return lifecycle;
});

final mobileBackendStatusProvider =
    StreamProvider<MobileBackendStatus>((ref) {
  final lifecycle = ref.watch(mobileBackendLifecycleProvider);
  return lifecycle.statusStream.distinct();
});

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'runtime_bridge_provider.dart';
import 'runtime_bootstrap.dart';
import 'default_runtime_bootstrap.dart';
import 'runtime_bootstrap_snapshot.dart';
import 'runtime_bootstrap_policy.dart';

final runtimeBootstrapPolicyProvider = Provider<RuntimeBootstrapPolicy>((ref) {
  return const RuntimeBootstrapPolicy();
});

final runtimeBootstrapProvider = Provider<RuntimeBootstrap>((ref) {
  final bridge = ref.watch(runtimeBridgeProvider);
  final policy = ref.watch(runtimeBootstrapPolicyProvider);

  final bootstrap = DefaultRuntimeBootstrap(
    bridge: bridge,
    policy: policy,
  );

  ref.onDispose(() => bootstrap.dispose());

  return bootstrap;
});

final runtimeBootstrapSnapshotProvider =
    StreamProvider<RuntimeBootstrapSnapshot>((ref) {
  final bootstrap = ref.watch(runtimeBootstrapProvider);
  return bootstrap.snapshots.distinct();
});

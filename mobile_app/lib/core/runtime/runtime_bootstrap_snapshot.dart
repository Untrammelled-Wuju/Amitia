import 'runtime_bootstrap_phase.dart';
import 'runtime_bridge_snapshot.dart';
import 'runtime_bridge_error.dart';

class RuntimeBootstrapSnapshot {
  final RuntimeBootstrapPhase phase;
  final RuntimeBridgeSnapshot runtime;
  final RuntimeBridgeError? error;

  const RuntimeBootstrapSnapshot({
    required this.phase,
    required this.runtime,
    this.error,
  });

  factory RuntimeBootstrapSnapshot.initial() {
    return RuntimeBootstrapSnapshot(
      phase: RuntimeBootstrapPhase.initializing,
      runtime: RuntimeBridgeSnapshot.initial(),
    );
  }

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is RuntimeBootstrapSnapshot &&
          phase == other.phase &&
          runtime == other.runtime &&
          error == other.error;

  @override
  int get hashCode =>
      phase.hashCode ^ runtime.hashCode ^ error.hashCode;
}

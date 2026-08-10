import '../runtime_bridge_state.dart';
import 'runtime_status_phase.dart';
import 'runtime_status_error.dart';

final class RuntimeStatusSnapshot {
  final RuntimeStatusPhase phase;
  final RuntimeBridgeState runtimeState;
  final bool runtimeReady;
  final bool runtimeInstalled;
  final bool backendConfigured;
  final bool httpAvailable;
  final bool webSocketConnected;
  final bool businessAvailable;
  final int generation;
  final RuntimeStatusError? primaryError;

  const RuntimeStatusSnapshot({
    required this.phase,
    required this.runtimeState,
    required this.runtimeReady,
    required this.runtimeInstalled,
    required this.backendConfigured,
    required this.httpAvailable,
    required this.webSocketConnected,
    required this.businessAvailable,
    required this.generation,
    this.primaryError,
  });

  factory RuntimeStatusSnapshot.initial() {
    return const RuntimeStatusSnapshot(
      phase: RuntimeStatusPhase.unavailable,
      runtimeState: RuntimeBridgeState.unavailable,
      runtimeReady: false,
      runtimeInstalled: false,
      backendConfigured: false,
      httpAvailable: false,
      webSocketConnected: false,
      businessAvailable: false,
      generation: 0,
    );
  }

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is RuntimeStatusSnapshot &&
          phase == other.phase &&
          runtimeState == other.runtimeState &&
          runtimeReady == other.runtimeReady &&
          runtimeInstalled == other.runtimeInstalled &&
          backendConfigured == other.backendConfigured &&
          httpAvailable == other.httpAvailable &&
          webSocketConnected == other.webSocketConnected &&
          businessAvailable == other.businessAvailable &&
          generation == other.generation &&
          primaryError == other.primaryError;

  @override
  int get hashCode =>
      phase.hashCode ^
      runtimeState.hashCode ^
      runtimeReady.hashCode ^
      runtimeInstalled.hashCode ^
      backendConfigured.hashCode ^
      httpAvailable.hashCode ^
      webSocketConnected.hashCode ^
      businessAvailable.hashCode ^
      generation.hashCode ^
      primaryError.hashCode;
}

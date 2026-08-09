import 'dart:async';
import 'runtime_bridge_snapshot.dart';
import 'runtime_bridge_error.dart';
import 'runtime_manifest_summary.dart';

class RuntimeBridgeCommandResult {
  final bool accepted;
  final RuntimeBridgeSnapshot snapshot;
  final RuntimeBridgeError? error;

  const RuntimeBridgeCommandResult({
    required this.accepted,
    required this.snapshot,
    this.error,
  });

  factory RuntimeBridgeCommandResult.fromMap(Map<String, dynamic> map) {
    return RuntimeBridgeCommandResult(
      accepted: map['accepted'] as bool? ?? false,
      snapshot: RuntimeBridgeSnapshot.fromMap(
        map['snapshot'] as Map<String, dynamic>? ?? {},
      ),
      error: map['error'] != null && map['error'] is Map<String, dynamic>
          ? RuntimeBridgeError.fromMap(map['error'] as Map<String, dynamic>)
          : null,
    );
  }
}

abstract interface class RuntimeBridge {
  Stream<RuntimeBridgeSnapshot> get snapshots;

  Future<RuntimeBridgeSnapshot> snapshot();

  Future<RuntimeBridgeCommandResult> start();

  Future<RuntimeBridgeCommandResult> stop();

  Future<RuntimeBridgeCommandResult> install();

  Future<RuntimeBridgeCommandResult> verify();

  Future<RuntimeBridgeCommandResult> repair();

  Future<RuntimeManifestSummary?> manifestSummary();

  Future<void> dispose();
}

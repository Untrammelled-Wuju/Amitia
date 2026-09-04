import 'dart:async';
import 'runtime_bootstrap_snapshot.dart';

abstract interface class RuntimeBootstrap {
  Future<void> initialize();

  Stream<RuntimeBootstrapSnapshot> get snapshots;

  Future<void> dispose();
}

class RuntimeBootstrapResult {
  final bool initialized;
  final bool autoStartRequested;

  const RuntimeBootstrapResult({
    required this.initialized,
    required this.autoStartRequested,
  });
}

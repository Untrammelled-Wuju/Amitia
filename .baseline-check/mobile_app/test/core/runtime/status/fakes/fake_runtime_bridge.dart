import 'dart:async';

import 'package:amitia_app/core/runtime/runtime_bridge.dart';
import 'package:amitia_app/core/runtime/runtime_bridge_snapshot.dart';
import 'package:amitia_app/core/runtime/runtime_manifest_summary.dart';

class FakeRuntimeBridge implements RuntimeBridge {
  final StreamController<RuntimeBridgeSnapshot> _controller =
      StreamController<RuntimeBridgeSnapshot>.broadcast();

  RuntimeBridgeSnapshot _current = RuntimeBridgeSnapshot.initial();
  RuntimeManifestSummary? _manifest;
  int _startCallCount = 0;
  int _stopCallCount = 0;
  int _installCallCount = 0;
  int _verifyCallCount = 0;
  int _repairCallCount = 0;
  int _disposeCallCount = 0;

  int get startCallCount => _startCallCount;
  int get stopCallCount => _stopCallCount;
  int get installCallCount => _installCallCount;
  int get verifyCallCount => _verifyCallCount;
  int get repairCallCount => _repairCallCount;
  int get disposeCallCount => _disposeCallCount;

  void setSnapshot(RuntimeBridgeSnapshot snapshot) {
    _current = snapshot;
    _controller.add(snapshot);
  }

  void setManifest(RuntimeManifestSummary? manifest) {
    _manifest = manifest;
  }

  @override
  Stream<RuntimeBridgeSnapshot> get snapshots => _controller.stream;

  @override
  Future<RuntimeBridgeSnapshot> snapshot() async => _current;

  @override
  Future<RuntimeBridgeCommandResult> start() async {
    _startCallCount++;
    return RuntimeBridgeCommandResult(
      accepted: true,
      snapshot: _current,
    );
  }

  @override
  Future<RuntimeBridgeCommandResult> stop() async {
    _stopCallCount++;
    return RuntimeBridgeCommandResult(
      accepted: true,
      snapshot: _current,
    );
  }

  @override
  Future<RuntimeBridgeCommandResult> install() async {
    _installCallCount++;
    return RuntimeBridgeCommandResult(
      accepted: true,
      snapshot: _current,
    );
  }

  @override
  Future<RuntimeBridgeCommandResult> verify() async {
    _verifyCallCount++;
    return RuntimeBridgeCommandResult(
      accepted: true,
      snapshot: _current,
    );
  }

  @override
  Future<RuntimeBridgeCommandResult> repair() async {
    _repairCallCount++;
    return RuntimeBridgeCommandResult(
      accepted: true,
      snapshot: _current,
    );
  }

  @override
  Future<RuntimeManifestSummary?> manifestSummary() async => _manifest;

  @override
  Future<void> dispose() async {
    _disposeCallCount++;
    await _controller.close();
  }

  void reset() {
    _startCallCount = 0;
    _stopCallCount = 0;
    _installCallCount = 0;
    _verifyCallCount = 0;
    _repairCallCount = 0;
    _disposeCallCount = 0;
  }
}

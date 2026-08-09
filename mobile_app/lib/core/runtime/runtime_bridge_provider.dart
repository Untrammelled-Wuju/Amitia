import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'runtime_bridge.dart';
import 'method_channel_runtime_bridge.dart';
import 'runtime_bridge_snapshot.dart';

final runtimeBridgeProvider = Provider<RuntimeBridge>((ref) {
  final bridge = MethodChannelRuntimeBridge();
  ref.onDispose(() => bridge.dispose());
  return bridge;
});

final runtimeSnapshotProvider = StreamProvider<RuntimeBridgeSnapshot>((ref) {
  final bridge = ref.watch(runtimeBridgeProvider);
  return bridge.snapshots.distinct();
});

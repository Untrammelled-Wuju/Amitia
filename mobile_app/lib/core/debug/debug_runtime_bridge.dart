import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend_transport/providers/backend_transport_providers.dart';
import '../backend_transport/state/backend_transport_state.dart';
import '../runtime/runtime_bootstrap_provider.dart';
import '../runtime/status/runtime_status_provider.dart';
import '../runtime/status/runtime_status_snapshot.dart';
import 'debug_log_service.dart';
import 'runtime_log_stream.dart';

final debugRuntimeLogBridgeProvider = Provider<void>((ref) {
  final logService = ref.read(debugLogServiceProvider);
  final logStream = RuntimeLogStream(logService);
  logStream.start();
  ref.onDispose(logStream.stop);
  String? lastTransportSignature;
  String? lastBootstrapErrorSignature;

  ref.listen(runtimeBootstrapSnapshotProvider, (prev, next) {
    final snapshot = next.asData?.value;
    final error = snapshot?.error;
    if (error == null) return;
    final signature = '${error.code}:${error.message}';
    if (signature == lastBootstrapErrorSignature) return;
    lastBootstrapErrorSignature = signature;
    logService.addRuntimeLog(
      'Bootstrap error [${error.code}]: ${error.message}',
      DebugLogLevel.error,
    );
  });

  ref.listen<RuntimeStatusSnapshot>(runtimeStatusCurrentProvider, (prev, next) {
    if (prev?.phase != next.phase ||
        prev?.runtimeState != next.runtimeState ||
        prev?.generation != next.generation) {
      logService.addRuntimeLog(
        'Phase: ${next.phase.name} | State: ${next.runtimeState.name} | Gen: ${next.generation}',
        next.primaryError != null ? DebugLogLevel.warn : DebugLogLevel.info,
      );
    }
    if (prev?.httpAvailable == false && next.httpAvailable) {
      logService.addBackendLog('HTTP available');
    }
    if (prev?.webSocketConnected == false && next.webSocketConnected) {
      logService.addBackendLog('WebSocket connected');
    }
    if (prev?.primaryError != next.primaryError && next.primaryError != null) {
      final error = next.primaryError!;
      logService.addRuntimeLog(
        'Error [${error.code}]: ${error.message}',
        DebugLogLevel.error,
      );
    }
  });

  ref.listen<AsyncValue<BackendTransportState>>(
    backendTransportProvider,
    (prev, next) {
      final signature = next.when(
        data: (state) => 'data:${state.runtimeType}',
        loading: () => 'loading',
        error: (err, _) => 'error:${err.runtimeType}:$err',
      );
      if (signature == lastTransportSignature) return;
      lastTransportSignature = signature;

      next.when(
        data: (state) {
          logService.addBackendLog('Transport: ${state.runtimeType}');
        },
        loading: () {
          logService.addBackendLog('Transport: loading...');
        },
        error: (err, _) {
          logService.addBackendLog('Transport error: $err', DebugLogLevel.error);
        },
      );
    },
  );
});

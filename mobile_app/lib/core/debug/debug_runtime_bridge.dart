import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../runtime/status/runtime_status_snapshot.dart';
import '../runtime/status/runtime_status_provider.dart';
import '../runtime/status/runtime_status_phase.dart';
import '../backend_transport/state/backend_transport_state.dart';
import '../backend_transport/providers/backend_transport_providers.dart';
import 'debug_log_service.dart';

final debugRuntimeLogBridgeProvider = Provider<void>((ref) {
  final logService = ref.read(debugLogServiceProvider);

  ref.listen<RuntimeStatusSnapshot>(runtimeStatusCurrentProvider, (prev, next) {
    if (prev?.phase != next.phase) {
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
      logService.addRuntimeLog(
        'Error: ${next.primaryError!.message}',
        DebugLogLevel.error,
      );
    }
  });

  ref.listen<AsyncValue<BackendTransportState>>(
    backendTransportProvider,
    (prev, next) {
      next.when(
        data: (state) {
          final stateName = state.runtimeType.toString();
          logService.addBackendLog('Transport: $stateName');
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

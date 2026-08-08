import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../api/api_client.dart';
import '../../backend_connection/backend_connection_availability.dart';
import '../../backend_connection/providers/backend_connection_providers.dart';
import '../connectivity/backend_connectivity_probe.dart';
import '../providers/backend_transport_providers.dart';

final backendConnectivityProbeProvider =
    Provider<BackendConnectivityProbe?>((ref) {
  final transport = ref.watch(backendCurrentTransportProvider);
  if (transport == null) return null;
  return BackendConnectivityProbe(transport.http);
});

final apiClientSyncProvider = Provider<void>((ref) {
  final configAsync = ref.watch(backendConnectionProvider);
  configAsync.when(
    data: (connection) {
      if (connection is BackendConnectionAvailable) {
        ApiClient().updateConfig(connection.config);
      }
    },
    loading: () {},
    error: (_, __) {},
  );
});

final backendConnectivityProvider =
    FutureProvider<BackendConnectivityResult>((ref) async {
  final probe = ref.watch(backendConnectivityProbeProvider);
  if (probe == null) return BackendConnectivityResult.unreachable;
  return probe.probe();
});

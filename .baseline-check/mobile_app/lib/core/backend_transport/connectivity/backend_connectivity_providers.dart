import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../connectivity/backend_connectivity_probe.dart';
import '../providers/backend_transport_providers.dart';

final backendConnectivityProbeProvider =
    Provider<BackendConnectivityProbe?>((ref) {
  final transport = ref.watch(backendCurrentTransportProvider);
  if (transport == null) return null;
  return BackendConnectivityProbe(transport.http);
});

final backendConnectivityProvider =
    FutureProvider<BackendConnectivityResult>((ref) async {
  final probe = ref.watch(backendConnectivityProbeProvider);
  if (probe == null) return BackendConnectivityResult.unreachable;
  return probe.probe();
});

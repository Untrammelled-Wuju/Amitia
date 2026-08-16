import 'dart:io' show Platform;

import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend_connection/backend_connection_availability.dart';
import '../../backend_connection/providers/backend_connection_providers.dart';
import '../../backend_transport/providers/backend_transport_providers.dart';
import 'native_bridge_relay_provider.dart';

final nativeBridgeRelayBootstrapProvider = Provider((ref) {
  final connectionAsync = ref.watch(backendConnectionProvider);
  final transportGeneration = ref.watch(backendTransportGenerationProvider);
  final relayNotifier = ref.watch(nativeBridgeRelayClientProvider.notifier);

  final connection = connectionAsync.asData?.value;
  if (connection is! BackendConnectionAvailable) {
    relayNotifier.attachBackend(null, isAndroid: Platform.isAndroid, isIOS: Platform.isIOS);
    return;
  }
  if (kIsWeb) {
    return;
  }
  final isAndroid = Platform.isAndroid;
  final isIOS = Platform.isIOS;
  if (!isAndroid && !isIOS) {
    return;
  }
  relayNotifier.attachBackend(
    connection.config,
    isAndroid: isAndroid,
    isIOS: isIOS,
  );

  return transportGeneration;
});

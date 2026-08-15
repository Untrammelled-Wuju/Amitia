import 'dart:io' show Platform;

import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend_connection/backend_connection_availability.dart';
import '../../backend_connection/backend_connection_endpoint.dart';
import '../../backend_connection/providers/backend_connection_providers.dart';
import '../../backend_transport/providers/backend_transport_providers.dart';
import 'native_bridge_relay_provider.dart';

final nativeBridgeRelayBootstrapProvider = Provider((ref) {
  final connectionAsync = ref.watch(backendConnectionProvider);
  final transportGeneration = ref.watch(backendTransportGenerationProvider);
  final relayNotifier = ref.watch(nativeBridgeRelayClientProvider.notifier);

  final connection = connectionAsync.asData?.value;
  if (connection is! BackendConnectionAvailable) {
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
  final baseUrl = _computeBaseUrl(connection.config.endpoint);
  if (baseUrl.isEmpty) {
    return;
  }
  relayNotifier.attachBackend(
    baseUrl,
    isAndroid: isAndroid,
    isIOS: isIOS,
  );

  return transportGeneration;
});

String _computeBaseUrl(BackendConnectionEndpoint? endpoint) {
  if (endpoint == null) return '';
  final host = endpoint.host;
  final port = endpoint.port;
  if (host.isEmpty || port == 0) return '';
  return '${endpoint.httpScheme}://$host:$port';
}

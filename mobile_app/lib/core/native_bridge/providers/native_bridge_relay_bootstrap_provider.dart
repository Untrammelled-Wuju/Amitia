import 'dart:io' show Platform;

import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend_connection/backend_connection_availability.dart';
import '../../backend_connection/providers/backend_connection_providers.dart';
import '../../backend_transport/providers/backend_transport_providers.dart';
import 'native_bridge_relay_provider.dart';

/// Binds the native device relay to the local Device Agent.
///
/// Business WebSocket/API traffic may point at Cloud Core, but native device
/// capabilities (SAF, alarms, calendar, media, etc.) are device-local and must
/// remain attached to the local Runtime/Device Agent. iOS App Intents use an
/// authenticated device-local HTTP action endpoint instead of tunnelling
/// backend actions through the relay WebSocket.
final nativeBridgeRelayBootstrapProvider = Provider<int?>((ref) {
  if (kIsWeb) return null;

  final isAndroid = Platform.isAndroid;
  final isIOS = Platform.isIOS;
  if (!isAndroid && !isIOS) return null;

  final relayNotifier = ref.watch(nativeBridgeRelayClientProvider.notifier);
  final dispatcher = ref.watch(nativeBridgePlatformDispatcherProvider);

  if (isIOS) {
    dispatcher.setBackendActionHandler((actionId, payload) async {
      try {
        final api = ref.read(backendServiceProvider);
        final response = await api.post<Map<String, dynamic>>(
          '/api/native-bridge/backend-action',
          data: <String, dynamic>{
            'platform': 'ios',
            'requestId': 'shortcut-${DateTime.now().microsecondsSinceEpoch}',
            'actionId': actionId,
            'payload': payload ?? const <String, dynamic>{},
          },
        );
        return response ??
            const <String, dynamic>{
              'status': 'error',
              'error': <String, dynamic>{
                'code': 'BACKEND_ACTION_EMPTY_RESPONSE',
                'message': 'backend action returned no response',
              },
            };
      } catch (error) {
        return <String, dynamic>{
          'status': 'error',
          'error': <String, dynamic>{
            'code': 'BACKEND_ACTION_FAILED',
            'message': error.toString(),
          },
        };
      }
    });
  } else {
    dispatcher.setBackendActionHandler(null);
  }

  final localConnectionAsync = ref.watch(deviceLocalBackendConnectionProvider);
  final connection = localConnectionAsync.asData?.value;
  if (connection is! BackendConnectionAvailable) {
    relayNotifier.attachBackend(
      null,
      isAndroid: isAndroid,
      isIOS: isIOS,
    );
    return null;
  }

  relayNotifier.attachBackend(
    connection.config,
    isAndroid: isAndroid,
    isIOS: isIOS,
  );
  return connection.config.generation;
});

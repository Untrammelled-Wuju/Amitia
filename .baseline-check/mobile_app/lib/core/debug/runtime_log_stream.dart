import 'dart:async';

import 'package:flutter/services.dart';

import 'debug_log_service.dart';

const String _logEventChannel = 'com.amitia.runtime/logs';

class RuntimeLogStream {
  static const EventChannel _channel = EventChannel(_logEventChannel);
  StreamSubscription<dynamic>? _subscription;
  final DebugLogService _logService;

  RuntimeLogStream(this._logService);

  void start() {
    _subscription?.cancel();
    _subscription = _channel.receiveBroadcastStream().listen(
      _onLogEvent,
      onError: _onError,
    );
  }

  void _onLogEvent(dynamic event) {
    if (event is! Map) return;
    final level = event['level'] as String? ?? 'INFO';
    final message = event['message'] as String? ?? '';
    if (message.isEmpty) return;

    final debugLevel = _mapLevel(level);
    _logService.addBackendLog('[backend] $message', debugLevel);
  }

  void _onError(dynamic error) {
    _logService.addBackendLog('Log stream error: $error', DebugLogLevel.error);
  }

  DebugLogLevel _mapLevel(String level) {
    switch (level.toUpperCase()) {
      case 'ERROR':
      case 'ERR':
        return DebugLogLevel.error;
      case 'WARN':
      case 'WRN':
        return DebugLogLevel.warn;
      case 'DEBUG':
      case 'DBG':
        return DebugLogLevel.debug;
      default:
        return DebugLogLevel.info;
    }
  }

  void stop() {
    _subscription?.cancel();
    _subscription = null;
  }
}

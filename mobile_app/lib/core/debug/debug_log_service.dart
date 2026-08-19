import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

enum DebugLogLevel { debug, info, warn, error }

class DebugLogEntry {
  final DateTime time;
  final DebugLogLevel level;
  final String source;
  final String message;

  DebugLogEntry({
    required this.time,
    required this.level,
    required this.source,
    required this.message,
  });

  String get timeStr {
    final h = time.hour.toString().padLeft(2, '0');
    final m = time.minute.toString().padLeft(2, '0');
    final s = time.second.toString().padLeft(2, '0');
    final ms = time.millisecond.toString().padLeft(3, '0');
    return '$h:$m:$s.$ms';
  }

  String get levelStr {
    switch (level) {
      case DebugLogLevel.debug:
        return 'DBG';
      case DebugLogLevel.info:
        return 'INF';
      case DebugLogLevel.warn:
        return 'WRN';
      case DebugLogLevel.error:
        return 'ERR';
    }
  }
}

final debugLogServiceProvider = Provider<DebugLogService>((ref) {
  final service = DebugLogService();
  ref.onDispose(service.dispose);
  return service;
});

final debugLogEntriesProvider = StreamProvider<List<DebugLogEntry>>((ref) {
  final service = ref.watch(debugLogServiceProvider);
  return service.stream;
});

class DebugLogService {
  static const int _maxEntries = 500;
  final List<DebugLogEntry> _entries = [];
  final StreamController<List<DebugLogEntry>> _controller =
      StreamController<List<DebugLogEntry>>.broadcast();
  bool _initialized = false;
  String _error = '';

  String get error => _error;

  Stream<List<DebugLogEntry>> get stream => _controller.stream;

  List<DebugLogEntry> get entries => List.unmodifiable(_entries);

  void init() {
    if (_initialized) return;
    _initialized = true;

    debugPrint = (String? message, {int? wrapWidth}) {
      if (message != null && message.isNotEmpty) {
        final level = message.contains('Error') || message.contains('Exception')
            ? DebugLogLevel.error
            : DebugLogLevel.debug;
        _addEntry(
          level: level,
          source: 'flutter',
          message: message,
        );
      }
    };

    FlutterError.onError = (details) {
      _addEntry(
        level: DebugLogLevel.error,
        source: 'flutter.error',
        message: '${details.exception}\n${details.stack}',
      );
    };

    PlatformDispatcher.instance.onError = (error, stack) {
      _addEntry(
        level: DebugLogLevel.error,
        source: 'unhandled',
        message: '$error\n$stack',
      );
      return false;
    };
  }

  void log(DebugLogLevel level, String source, String message) {
    _addEntry(level: level, source: source, message: message);
  }

  void addRuntimeLog(String message, [DebugLogLevel level = DebugLogLevel.info]) {
    _addEntry(level: level, source: 'runtime', message: message);
  }

  void addBackendLog(String message, [DebugLogLevel level = DebugLogLevel.info]) {
    _addEntry(level: level, source: 'backend', message: message);
  }

  void addSidecarLog(String message, [DebugLogLevel level = DebugLogLevel.info]) {
    _addEntry(level: level, source: 'sidecar', message: message);
  }

  void addProotLog(String message, [DebugLogLevel level = DebugLogLevel.debug]) {
    _addEntry(level: level, source: 'proot', message: message);
  }

  void clear() {
    _entries.clear();
    _safeAdd();
  }

  void setError(String error) {
    _error = error;
  }

  void _addEntry({
    required DebugLogLevel level,
    required String source,
    required String message,
  }) {
    final entry = DebugLogEntry(
      time: DateTime.now(),
      level: level,
      source: source,
      message: message,
    );
    _entries.add(entry);
    if (_entries.length > _maxEntries) {
      _entries.removeRange(0, _entries.length - _maxEntries);
    }
    _safeAdd();
  }

  void _safeAdd() {
    if (!_controller.isClosed) {
      _controller.add(List.unmodifiable(_entries));
    }
  }

  void dispose() {
    _controller.close();
  }
}

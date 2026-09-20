import 'dart:async';

class MarkdownStreamScheduler {
  MarkdownStreamScheduler({this.interval = const Duration(milliseconds: 40)});

  final Duration interval;
  Timer? _timer;
  VoidCallback? _pending;
  bool _disposed = false;

  void schedule(VoidCallback callback) {
    if (_disposed) return;
    _pending = callback;
    _timer ??= Timer(interval, flush);
  }

  void flush() {
    _timer?.cancel();
    _timer = null;
    final pending = _pending;
    _pending = null;
    pending?.call();
  }

  void dispose() {
    _disposed = true;
    _timer?.cancel();
    _timer = null;
    _pending = null;
  }
}

typedef VoidCallback = void Function();


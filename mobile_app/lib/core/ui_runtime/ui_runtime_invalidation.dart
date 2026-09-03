import 'dart:async';

/// Process-local invalidation signal for UI provider/slot/profile snapshots.
///
/// Extension mutations can happen outside the UI Runtime settings surface. The
/// built-in mobile shell subscribes to this bus so an install/enable/disable or
/// rollback is reflected immediately instead of waiting for periodic polling.
abstract final class UIRuntimeInvalidationBus {
  static final StreamController<void> _controller =
      StreamController<void>.broadcast(sync: true);

  static Stream<void> get changes => _controller.stream;

  static void notifyChanged() {
    if (!_controller.isClosed) _controller.add(null);
  }
}

import 'dart:async';

abstract final class MCPInteractionEvents {
  static final StreamController<void> _controller =
      StreamController<void>.broadcast();

  static Stream<void> get pending => _controller.stream;

  static void notifyPending() {
    if (!_controller.isClosed) _controller.add(null);
  }
}

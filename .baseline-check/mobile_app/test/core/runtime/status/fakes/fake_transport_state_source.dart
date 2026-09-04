import 'dart:async';

import 'package:amitia_app/core/runtime/status/default_runtime_status_projection.dart';

class FakeTransportStateSource implements TransportStateSource {
  final StreamController<TransportStateSnapshot> _controller =
      StreamController<TransportStateSnapshot>.broadcast();

  TransportStateSnapshot _current = TransportStateSnapshot.initial();

  void emit(TransportStateSnapshot snapshot) {
    _current = snapshot;
    _controller.add(snapshot);
  }

  @override
  Stream<TransportStateSnapshot> get snapshots => _controller.stream;

  @override
  TransportStateSnapshot get current => _current;

  Future<void> close() async {
    await _controller.close();
  }
}

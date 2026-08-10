import 'dart:async';
import 'runtime_status_snapshot.dart';

abstract interface class RuntimeStatusProjection {
  Stream<RuntimeStatusSnapshot> get snapshots;

  RuntimeStatusSnapshot get current;

  Future<void> dispose();
}

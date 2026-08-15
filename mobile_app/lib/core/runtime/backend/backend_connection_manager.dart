import 'dart:async';
import 'backend_connection.dart';
import 'backend_topology.dart';

abstract interface class BackendConnectionManager {
  Stream<BackendConnection?> get connectionStream;
  BackendConnection? get currentConnection;
  bool get hasActiveConnection;
}

class DefaultBackendConnectionManager implements BackendConnectionManager {
  final StreamController<BackendConnection?> _controller =
      StreamController<BackendConnection?>.broadcast();

  BackendConnection? _currentConnection;
  int _connectionGeneration = 0;

  @override
  Stream<BackendConnection?> get connectionStream => _controller.stream;

  @override
  BackendConnection? get currentConnection => _currentConnection;

  @override
  bool get hasActiveConnection => _currentConnection != null;

  int get generation => _connectionGeneration;

  void updateFromTopology(MobileBackendTopology topology) {
    final newConnection = BackendConnection.fromBusinessCore(topology);
    _connectionGeneration++;
    if (_currentConnection == newConnection) return;
    _currentConnection = newConnection;
    _controller.add(newConnection);
  }

  void updateConnection(BackendConnection connection) {
    _connectionGeneration++;
    if (_currentConnection == connection) return;
    _currentConnection = connection;
    _controller.add(connection);
  }

  void clear() {
    _currentConnection = null;
    _controller.add(null);
  }

  bool isStale(int generation) {
    return generation != _connectionGeneration;
  }

  Future<void> dispose() async {
    await _controller.close();
  }
}

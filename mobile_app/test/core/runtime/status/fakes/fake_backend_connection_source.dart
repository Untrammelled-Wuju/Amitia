import 'package:amitia_app/core/backend_connection/backend_connection_availability.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_source.dart';

class FakeBackendConnectionSource implements BackendConnectionSource {
  BackendConnectionAvailability _availability = BackendConnectionUnavailable();
  int _resolveCallCount = 0;

  int get resolveCallCount => _resolveCallCount;

  void setAvailability(BackendConnectionAvailability availability) {
    _availability = availability;
  }

  @override
  Future<BackendConnectionAvailability> resolve() async {
    _resolveCallCount++;
    return _availability;
  }

  void reset() {
    _resolveCallCount = 0;
    _availability = BackendConnectionUnavailable();
  }
}

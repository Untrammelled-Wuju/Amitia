import '../backend_transport/backend_service_api.dart';
import '../runtime/status/runtime_status_projection.dart';
import '../runtime/status/runtime_status_snapshot.dart';
import 'business_backend_unavailable.dart';

class BusinessBackendAccess {
  final RuntimeStatusProjection _projection;

  BusinessBackendAccess(this._projection);

  RuntimeStatusSnapshot get snapshot => _projection.current;

  bool get businessAvailable => _projection.current.businessAvailable;

  int get businessGeneration => _projection.current.generation;

  void requireBusinessAvailable() {
    final snap = _projection.current;
    if (!snap.businessAvailable) {
      throw BusinessBackendUnavailable(
        phase: snap.phase,
        generation: snap.generation,
        primaryError: snap.primaryError,
      );
    }
  }

  BackendServiceApi acquireApi(BackendServiceApi? rawApi) {
    final snap = _projection.current;
    if (!snap.businessAvailable) {
      throw BusinessBackendUnavailable(
        phase: snap.phase,
        generation: snap.generation,
        primaryError: snap.primaryError,
      );
    }
    if (rawApi == null) {
      throw BusinessBackendUnavailable(
        phase: snap.phase,
        generation: snap.generation,
        primaryError: snap.primaryError,
      );
    }
    if (rawApi.generation != snap.generation) {
      throw BusinessBackendUnavailable(
        phase: snap.phase,
        generation: snap.generation,
        primaryError: snap.primaryError,
      );
    }
    return rawApi;
  }
}

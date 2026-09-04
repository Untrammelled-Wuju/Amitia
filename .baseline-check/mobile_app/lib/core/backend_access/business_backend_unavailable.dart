import '../runtime/status/runtime_status_phase.dart';
import '../runtime/status/runtime_status_error.dart';

class BusinessBackendUnavailable implements Exception {
  final RuntimeStatusPhase phase;
  final int generation;
  final RuntimeStatusError? primaryError;

  const BusinessBackendUnavailable({
    required this.phase,
    required this.generation,
    this.primaryError,
  });

  @override
  String toString() {
    final code = primaryError?.code ?? 'BUSINESS_UNAVAILABLE';
    return 'BusinessBackendUnavailable(${phase.name}, gen=$generation, code=$code)';
  }
}

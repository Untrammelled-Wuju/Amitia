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
    final error = primaryError;
    final buffer = StringBuffer()
      ..writeln('BusinessBackendUnavailable(')
      ..writeln('  phase: ${phase.name}')
      ..writeln('  generation: $generation')
      ..writeln('  source: ${error?.source.name ?? 'unavailable'}')
      ..writeln('  code: ${error?.code ?? 'BUSINESS_UNAVAILABLE'}')
      ..writeln(
        '  message: ${error?.message ?? 'No primary RuntimeStatusError is attached. Inspect the preceding Runtime bootstrap error for the root cause.'}',
      );
    if (error != null && error.details.isNotEmpty) {
      for (final entry in error.details.entries) {
        buffer.writeln('  ${entry.key}: ${entry.value}');
      }
    }
    buffer.write(')');
    return buffer.toString();
  }
}

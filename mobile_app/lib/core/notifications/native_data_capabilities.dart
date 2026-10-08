/// The manufacturer data plane is active only when both the installed app
/// adapter and the currently connected Core advertise the same provider.
/// Tokens or a previous Cloud Core's capability must never enable it alone.
List<String> effectiveNativeDataProviders(
  Iterable<String> deviceProviders,
  Object? cloudProviders,
) {
  if (cloudProviders is! Map) return const <String>[];
  final available = deviceProviders
      .map((provider) => provider.trim().toLowerCase())
      .where((provider) => provider.isNotEmpty)
      .toSet();
  final ready = <String>[];
  for (final entry in cloudProviders.entries) {
    final provider = entry.key.toString().trim().toLowerCase();
    if (entry.value == true && available.contains(provider)) {
      ready.add(provider);
    }
  }
  ready.sort();
  return ready;
}

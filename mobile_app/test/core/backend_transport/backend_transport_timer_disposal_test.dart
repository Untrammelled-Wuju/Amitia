import 'package:amitia_app/core/backend_access/business_backend_unavailable.dart';
import 'package:amitia_app/core/backend_transport/providers/backend_transport_providers.dart';
import 'package:amitia_app/core/runtime/status/runtime_status_provider.dart';
import 'package:amitia_app/core/runtime/status/runtime_status_snapshot.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('backend availability polling is cancelled on disposal', (
    tester,
  ) async {
    final container = ProviderContainer(
      overrides: [
        runtimeStatusCurrentProvider.overrideWithValue(
          RuntimeStatusSnapshot.initial(),
        ),
        rawBackendServiceApiProvider.overrideWithValue(null),
      ],
    );
    final api = container.read(backendServiceProvider);
    final pending = api.get<Map<String, dynamic>>('/api/not-ready');
    await tester.pump();

    container.dispose();
    await expectLater(pending, throwsA(isA<BusinessBackendUnavailable>()));
    await tester.pump();
  });
}

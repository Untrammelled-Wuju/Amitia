import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../runtime/status/runtime_status_provider.dart';
import 'business_backend_access.dart';

final businessBackendAccessProvider = Provider<BusinessBackendAccess>((ref) {
  final projection = ref.watch(runtimeStatusProjectionProvider);
  return BusinessBackendAccess(projection);
});

import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/backend_transport/providers/backend_transport_providers.dart';
import '../core/notifications/notification_coordinator.dart';
import '../core/services/providers.dart';
import 'router.dart';

final notificationCoordinatorProvider = Provider<NotificationCoordinator>((
  ref,
) {
  final coordinator = NotificationCoordinator(
    api: ref.read(backendServiceProvider),
    chat: ref.read(chatServiceProvider),
    router: ref.read(goRouterProvider),
  );
  ref.listen(rawBackendServiceApiProvider, (previous, next) {
    if (next != null && !identical(previous, next)) {
      unawaited(coordinator.refreshRegistration());
    }
  });
  ref.onDispose(() => unawaited(coordinator.dispose()));
  unawaited(coordinator.start());
  return coordinator;
});

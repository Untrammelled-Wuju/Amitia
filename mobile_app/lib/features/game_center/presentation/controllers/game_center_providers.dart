import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../data/game_center_api.dart';
import '../../data/game_center_package_lifecycle.dart';
import 'game_center_controller.dart';

final gameCenterPackageLifecycleProvider = Provider<GameCenterPackageLifecycleClient>((ref) {
  return GameCenterPackageLifecycleClient(ref);
});

final gameCenterApiProvider = Provider<GameCenterApi>((ref) {
  final api = ref.watch(backendServiceProvider);
  return GameCenterApi(api);
});

final gameCenterControllerProvider =
    StateNotifierProvider<GameCenterController, GameCenterState>(
  (ref) => GameCenterController(api: ref.watch(gameCenterApiProvider)),
);

import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../data/game_center_api.dart';
import 'game_center_controller.dart';

final gameCenterApiProvider = Provider<GameCenterApi>((ref) {
  final api = ref.watch(backendServiceProvider);
  if (api == null) {
    throw StateError('BackendServiceApi not available');
  }
  return GameCenterApi(api);
});

final gameCenterControllerProvider =
    StateNotifierProvider<GameCenterController, GameCenterState>(
  (ref) => GameCenterController(api: ref.watch(gameCenterApiProvider)),
);

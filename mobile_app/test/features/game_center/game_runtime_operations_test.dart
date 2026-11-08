import 'package:amitia_app/features/game_center/domain/game_runtime_operations.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('created runtime exposes start instead of invalid restart', () {
    expect(availableGameRuntimeOperations('created'), {
      GameRuntimeOperation.start,
    });
  });

  test('running runtime exposes stop and restart', () {
    expect(availableGameRuntimeOperations('running'), {
      GameRuntimeOperation.stop,
      GameRuntimeOperation.restart,
    });
  });

  test('failed and transitional runtimes expose no unsafe mutation', () {
    expect(availableGameRuntimeOperations('failed'), isEmpty);
    expect(availableGameRuntimeOperations('restarting'), isEmpty);
    expect(availableGameRuntimeOperations('stopping'), isEmpty);
  });
}

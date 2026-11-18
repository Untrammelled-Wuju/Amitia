import 'package:amitia_app/core/ui_runtime/mobile_right_pane_runtime.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('opens and closes the generic mobile right pane', () {
    final controller = MobileRightPaneController();
    controller.open(
      'plugin-resource://plan/session/call',
      title: 'Plan',
      params: <String, dynamic>{'callId': 'call'},
      sessionId: 'session',
    );
    expect(controller.state.open, isTrue);
    expect(controller.state.address, 'plugin-resource://plan/session/call');
    expect(controller.state.title, 'Plan');
    expect(controller.state.params['callId'], 'call');
    expect(controller.state.sessionId, 'session');
    expect(controller.state.toContext()['address'], controller.state.address);
    controller.close();
    expect(controller.state.open, isFalse);
    expect(controller.state.address, isNull);
  });

  test('rejects an empty right pane address', () {
    final controller = MobileRightPaneController();
    expect(() => controller.open('  '), throwsStateError);
  });
}

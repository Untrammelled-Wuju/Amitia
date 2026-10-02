import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/models/conversation.dart';
import 'package:amitia_app/features/chat/runtime/automation_status.dart';
import 'package:amitia_app/features/chat/presentation/widgets/automation_status_indicator.dart';

AssistantTurnDto turn({
  String status = 'running',
  String name = 'android_interaction_click',
  String itemStatus = 'running',
  String id = 'root',
  String parent = '',
  int sequence = 1,
}) => AssistantTurnDto(
  id: id,
  status: status,
  parentTurnId: parent,
  sequence: sequence,
  items: [
    AssistantTurnItemDto(
      id: 'call',
      sequence: 1,
      type: 'tool_call',
      status: itemStatus,
      toolName: name,
    ),
  ],
);

void main() {
  test(
    'browser and Android operations share labels, other tools stay hidden',
    () {
      expect(automationAction('android_interaction_visual_locate'), '正在观察画面');
      expect(automationAction('android.interaction.input_text'), '正在输入');
      expect(automationAction('browser_dom_scroll_to_element'), '正在滚动');
      expect(automationAction('browser.agent.run'), '正在自动化操作');
      expect(automationAction('exec_command'), isNull);
      expect(automationAction('android_interaction_status'), isNull);
      expect(automationAction('browser_tab_list'), isNull);
    },
  );
  test('terminal turns and finished calls remove indication', () {
    for (final status in ['completed', 'failed', 'interrupted']) {
      expect(projectAutomationStatus([turn(status: status)]), isNull);
      expect(projectAutomationStatus([turn(itemStatus: status)]), isNull);
    }
    expect(projectAutomationStatus([]), isNull);
  });
  test('queue, approval and cancellation have distinct states', () {
    expect(
      projectAutomationStatus([turn(status: 'waiting_approval')])?.label,
      '等待授权',
    );
    expect(
      projectAutomationStatus([turn(itemStatus: 'queued')])?.label,
      '等待自动化执行',
    );
    expect(
      projectAutomationStatus([turn(status: 'cancelling')])?.phase,
      'cancelling',
    );
  });
  test('parallel child calls follow root lifecycle', () {
    final child = turn(
      id: 'child',
      parent: 'root',
      sequence: 2,
      name: 'browser_interact_input',
    );
    final status = projectAutomationStatus([turn(), child]);
    expect(status?.count, 2);
    expect(status?.label, '正在输入');
    expect(projectAutomationStatus([turn(status: 'completed'), child]), isNull);
    expect(projectAutomationStatus([child]), isNull);
    expect(
      projectAutomationStatus([
        turn(),
        turn(id: 'new', sequence: 3, status: 'completed'),
      ]),
      isNull,
    );
  });
  testWidgets(
    'small screen and enlarged text remain readable and clicks pass through',
    (tester) async {
      tester.view.physicalSize = const Size(320, 640);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      var taps = 0;
      await tester.pumpWidget(
        MaterialApp(
          home: MediaQuery(
            data: const MediaQueryData(
              size: Size(320, 640),
              textScaler: TextScaler.linear(1.5),
              disableAnimations: true,
            ),
            child: Scaffold(
              body: Stack(
                fit: StackFit.expand,
                children: [
                  GestureDetector(
                    onTap: () => taps++,
                    behavior: HitTestBehavior.opaque,
                    child: const SizedBox.expand(),
                  ),
                  AutomationStatusIndicator(
                    status: projectAutomationStatus([turn()])!,
                  ),
                ],
              ),
            ),
          ),
        ),
      );
      expect(find.text('正在点击'), findsOneWidget);
      await tester.tapAt(const Offset(160, 90));
      expect(taps, 1);
      await tester.pumpAndSettle();
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox.shrink());
    },
  );
}

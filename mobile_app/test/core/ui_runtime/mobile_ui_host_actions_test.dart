import 'dart:async';

import 'package:amitia_app/core/ui_runtime/mobile_right_pane_runtime.dart';
import 'package:amitia_app/core/ui_runtime/mobile_ui_host_actions.dart';
import 'package:amitia_app/core/ui_runtime/mobile_workspace_sidebar_runtime.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('shares right pane and validated host actions', (tester) async {
    final container = ProviderContainer();
    addTearDown(container.dispose);
    late Map<String, FutureOr<dynamic> Function(dynamic)> actions;

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          home: Scaffold(
            body: Consumer(
              builder: (context, ref, _) {
                actions = buildMobileUIHostActions(context: context, ref: ref);
                return const SizedBox.shrink();
              },
            ),
          ),
        ),
      ),
    );

    final opened = await actions['openRightPane']!(<String, dynamic>{
      'address': 'dsh://resource/alpha',
      'title': '资源',
    });
    expect(opened, <String, dynamic>{
      'open': true,
      'address': 'dsh://resource/alpha',
    });
    expect(container.read(mobileRightPaneProvider).open, isTrue);
    expect(container.read(mobileRightPaneProvider).title, '资源');

    final closed = await actions['closeRightPane']!(null);
    expect(closed, <String, dynamic>{'open': false});
    expect(container.read(mobileRightPaneProvider).open, isFalse);

    final sidebarOpened = await actions['openWorkspaceSidebar']!(null);
    expect(sidebarOpened, <String, dynamic>{'open': true});
    expect(container.read(mobileWorkspaceSidebarProvider).open, isTrue);
    final sidebarClosed = await actions['closeWorkspaceSidebar']!(null);
    expect(sidebarClosed, <String, dynamic>{'open': false});
    expect(container.read(mobileWorkspaceSidebarProvider).open, isFalse);

    expect(
      () => actions['navigate']!(<String, dynamic>{
        'target': 'https://example.com',
      }),
      throwsStateError,
    );
    expect(
      () => actions['showToast']!(<String, dynamic>{'message': ''}),
      throwsStateError,
    );
  });
}

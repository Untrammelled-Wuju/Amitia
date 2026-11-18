import 'package:amitia_app/core/ui_runtime/mobile_workspace_sidebar_runtime.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('opens and closes the generic mobile workspace sidebar', () {
    final container = ProviderContainer();
    addTearDown(container.dispose);
    final controller = container.read(mobileWorkspaceSidebarProvider.notifier);
    expect(container.read(mobileWorkspaceSidebarProvider).open, isFalse);
    controller.open();
    expect(container.read(mobileWorkspaceSidebarProvider).open, isTrue);
    expect(
      container.read(mobileWorkspaceSidebarProvider).toContext(),
      <String, dynamic>{
        'workspaceSidebar': <String, dynamic>{'open': true},
      },
    );
    controller.close();
    expect(container.read(mobileWorkspaceSidebarProvider).open, isFalse);
  });
}

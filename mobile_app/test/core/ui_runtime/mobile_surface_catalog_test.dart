import 'package:amitia_app/core/ui_runtime/routing/ui_surface_catalog.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('maps every primary mobile surface to a generic extension spec', () {
    const routes = <String, String>{
      '/chat': 'surface.conversation',
      '/characters': 'surface.character.list',
      '/characters/alpha': 'surface.character.detail',
      '/memory-hub': 'surface.memory.hub',
      '/memory/graph': 'surface.memory.detail',
      '/tasks': 'surface.tasks',
      '/settings/system': 'surface.settings.section',
      '/extensions': 'surface.extension',
      '/custom/page': 'surface.page',
    };
    for (final entry in routes.entries) {
      expect(canonicalUISurfaceId(entry.key), entry.value);
      expect(mobileUISurfaceForRoute(entry.key), isNotNull);
    }
  });

  test('exposes the full generic conversation composition surface', () {
    final surface = mobileUISurfaceSpec('surface.conversation')!;
    for (final slotId in <String>[
      'chat.message.renderer',
      'chat.message.images',
      'chat.message.user.images',
      'chat.message.custom_renderer',
      'chat.message.attachment_renderer',
      'chat.message.action',
      'chat.message.badge',
      'chat.composer.action',
      'chat.composer.dock.top',
      'chat.composer.dock.bottom',
      'chat.composer.left',
      'chat.composer.right',
      'chat.composer.overlay',
      'chat.composer.model',
      'chat.composer.model.label',
      'chat.composer.model.options',
      'chat.composer.permission',
      'chat.header.action',
      'chat.sidebar.panel',
      'chat.status.item',
    ]) {
      expect(surface.supportsSlot(slotId), isTrue, reason: slotId);
    }
    for (final capability in <String>[
      'conversation.shell',
      'conversation.messages',
      'conversation.message_renderer',
      'conversation.composer',
      'conversation.overlay',
    ]) {
      expect(
        surface.supportsProviderCapability(capability),
        isTrue,
        reason: capability,
      );
    }
  });

  test('exposes app, portal, right pane, memory, tasks and settings slots', () {
    expect(mobileUISurfaceSpec('surface.app')!.supportsSlot('root'), isTrue);
    expect(
      mobileUISurfaceSpec(
        'surface.app',
      )!.supportsSlot('app.shell.quota_notice'),
      isTrue,
    );
    expect(
      mobileUISurfaceSpec('surface.app')!.supportsSlot('app.shell.bottom'),
      isTrue,
    );
    final overlay = mobileUISurfaceSpec('surface.overlay')!;
    for (final slotId in <String>[
      'ui.portal.overlay',
      'ui.portal.modal',
      'ui.portal.drawer',
      'ui.portal.toast',
      'ui.sidebar.right.pane.tab',
      'ui.sidebar.right.pane.tab.title',
    ]) {
      expect(overlay.supportsSlot(slotId), isTrue, reason: slotId);
    }
    expect(
      mobileUISurfaceSpec(
        'surface.memory.detail',
      )!.supportsSlot('memory.page.section'),
      isTrue,
    );
    expect(
      mobileUISurfaceSpec('surface.tasks')!.supportsSlot('tasks.page.section'),
      isTrue,
    );
    expect(
      mobileUISurfaceSpec(
        'surface.settings.section',
      )!.supportsSlot('system.settings.section'),
      isTrue,
    );
    expect(
      mobileUISurfaceSpec(
        'surface.workspace',
      )!.supportsSlot('provider.app.workspace.sidebar'),
      isTrue,
    );
  });
}

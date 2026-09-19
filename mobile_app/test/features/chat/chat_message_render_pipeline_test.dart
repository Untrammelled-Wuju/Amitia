import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  test('message content cannot be replaced by a blank extension slot', () {
    final page = File(
      'lib/features/chat/presentation/pages/chat_page.dart',
    ).readAsStringSync();
    final providerHost = File(
      'lib/core/ui_runtime/ui_provider_host.dart',
    ).readAsStringSync();
    final extensionSlot = File(
      'lib/core/ui_runtime/mobile_extension_slot.dart',
    ).readAsStringSync();

    expect(page, isNot(contains("slotId: 'chat.message.renderer'")));
    expect(page, isNot(contains("slotId: 'chat.message.custom_renderer'")));
    expect(
      page,
      isNot(contains('capability: UICapability.conversationMessages')),
    );
    expect(page, isNot(contains("slotId: 'chat.empty_state.card'")));
    expect(page, contains('providerMessage,'));
    expect(
      providerHost,
      contains('if (_fallbackIndex >= chain.length) return widget.fallback;'),
    );
    expect(
      extensionSlot,
      contains('if (snapshot.hasError) return widget.fallback;'),
    );
    expect(extensionSlot, contains('fallback: widget.fallback'));
    expect(page, contains('AnimatedSwitcher('));
    expect(page, contains('FadeTransition('));
    expect(page, contains("'conversation-messages:"));
    expect(page, contains('showEmptyState'));
    expect(page, contains('return Center('));
    expect(page, contains('crossAxisAlignment: CrossAxisAlignment.stretch'));
  });
}

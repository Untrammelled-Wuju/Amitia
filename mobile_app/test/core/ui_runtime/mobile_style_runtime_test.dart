import 'dart:io';

import 'package:amitia_app/core/settings/background_preferences.dart';
import 'package:amitia_app/core/settings/ui_style_command_service.dart';
import 'package:amitia_app/core/settings/ui_style_profile.dart';
import 'package:amitia_app/core/ui_runtime/mobile_style_runtime.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  late Directory root;

  setUp(() async {
    SharedPreferences.setMockInitialValues(<String, Object>{});
    root = await Directory.systemTemp.createTemp('amitia-style-runtime-test-');
  });

  tearDown(() async {
    if (await root.exists()) await root.delete(recursive: true);
  });

  test('parses rules with page scope and AI priority', () {
    final document = AmitiaStyleDocument(
      revision: 2,
      rules: <AmitiaStyleRule>[
        AmitiaStyleRule(
          id: 'user-rule',
          source: 'user',
          scope: const AmitiaStyleScopeData(platform: 'mobile', page: 'chat'),
          target: 'chat.composer.input',
          properties: const <String, dynamic>{'backgroundColor': '#111111'},
        ),
        AmitiaStyleRule(
          id: 'ai-rule',
          source: 'ai',
          scope: const AmitiaStyleScopeData(platform: 'mobile', page: 'chat'),
          target: 'chat.composer.input',
          properties: const <String, dynamic>{'backgroundColor': '#000000'},
        ),
      ],
    );

    final chat = AmitiaStyleResolver.resolve(
      document: document,
      scope: const AmitiaStyleScopeData(platform: 'mobile', page: 'chat'),
      target: 'chat.composer.input',
    );
    final memory = AmitiaStyleResolver.resolve(
      document: document,
      scope: const AmitiaStyleScopeData(platform: 'mobile', page: 'memory'),
      target: 'chat.composer.input',
    );

    expect(chat.color('backgroundColor'), const Color(0xFF000000));
    expect(memory.properties, isEmpty);
  });

  test('normalizes page identifiers consistently across platforms', () {
    expect(amitiaStylePageId('/chat'), 'chat');
    expect(amitiaStylePageId('/settings/appearance'), 'settings.appearance');
    expect(amitiaStylePageId('settings.appearance'), 'settings.appearance');
    expect(
      amitiaStylePageId('/extension/page/emote.main'),
      'plugin.emote-main',
    );
    expect(
      amitiaStylePageId('/extension/page/main?extensionId=emote'),
      'plugin.emote.main',
    );
    expect(amitiaStylePageId('/'), 'root');
  });

  test(
    'supports state and property scoped reset without touching other rules',
    () async {
      final container = ProviderContainer(
        overrides: <Override>[
          backgroundPreferencesProvider.overrideWith(
            (ref) => BackgroundPreferencesNotifier(directory: () async => root),
          ),
        ],
      );
      addTearDown(container.dispose);
      final service = container.read(uiStyleCommandServiceProvider);

      await service.execute(
        action: 'apply',
        expectedRevision: 0,
        changes: <String, dynamic>{
          'targets': <Map<String, dynamic>>[
            <String, dynamic>{
              'scope': <String, dynamic>{'platform': 'mobile', 'page': 'chat'},
              'target': 'chat.composer.input',
              'states': <String>['default', 'focus'],
              'properties': <String, dynamic>{
                'backgroundColor': '#000000',
                'foregroundColor': '#ffffff',
              },
            },
          ],
        },
      );
      expect(container.read(uiStylePatchProvider).rules, hasLength(1));

      final reset = await service.execute(
        action: 'reset',
        expectedRevision: 1,
        changes: <String, dynamic>{
          'scope': <String, dynamic>{'platform': 'mobile', 'page': 'chat'},
          'target': 'chat.composer.input',
          'states': <String>['focus'],
          'properties': <String>['backgroundColor'],
        },
      );

      expect(reset['ok'], true);
      final rules = container.read(uiStylePatchProvider).rules;
      expect(rules, hasLength(2));
      final defaultRule = rules.firstWhere(
        (rule) => rule.states.join(',') == 'default',
      );
      final focusRule = rules.firstWhere(
        (rule) => rule.states.join(',') == 'focus',
      );
      expect(defaultRule.properties.containsKey('backgroundColor'), isTrue);
      expect(defaultRule.properties.containsKey('foregroundColor'), isTrue);
      expect(focusRule.properties.containsKey('backgroundColor'), isFalse);
      expect(focusRule.properties.containsKey('foregroundColor'), isTrue);
    },
  );

  test('inspect returns page targets and stored rules', () async {
    final container = ProviderContainer(
      overrides: <Override>[
        backgroundPreferencesProvider.overrideWith(
          (ref) => BackgroundPreferencesNotifier(directory: () async => root),
        ),
      ],
    );
    addTearDown(container.dispose);
    final service = container.read(uiStyleCommandServiceProvider);

    await service.execute(
      action: 'apply',
      expectedRevision: 0,
      changes: <String, dynamic>{
        'scope': <String, dynamic>{'platform': 'mobile', 'page': 'chat'},
        'target': 'chat.composer.surface',
        'properties': <String, dynamic>{'backgroundColor': '#101010'},
      },
    );
    final inspected = await service.execute(
      action: 'inspect',
      expectedRevision: 1,
      changes: <String, dynamic>{
        'scope': <String, dynamic>{'platform': 'mobile', 'page': 'chat'},
      },
    );

    expect(inspected['page'], 'chat');
    expect(
      (inspected['targets'] as Map<String, dynamic>).containsKey(
        'chat.composer.surface',
      ),
      isTrue,
    );
    expect((inspected['rules'] as List<dynamic>), hasLength(1));
  });

  test('registers editing and message targets', () {
    for (final target in <String>[
      'editor',
      'text_field',
      'switch',
      'segmented',
      'message.user',
      'message.assistant',
      'code_block',
      'attachment',
    ]) {
      expect(AmitiaStyleTargetRegistry.descriptor(target), isNotNull);
    }
    expect(
      AmitiaStyleTargetRegistry.descriptor(
        'text_field',
      )!.properties.containsKey('backgroundColor'),
      isTrue,
    );
    expect(
      AmitiaStyleTargetRegistry.descriptor(
        'message.assistant',
      )!.properties.containsKey('foregroundColor'),
      isTrue,
    );
  });

  test('page scoped reset leaves global rules intact', () async {
    final container = ProviderContainer(
      overrides: <Override>[
        backgroundPreferencesProvider.overrideWith(
          (ref) => BackgroundPreferencesNotifier(directory: () async => root),
        ),
      ],
    );
    addTearDown(container.dispose);
    final service = container.read(uiStyleCommandServiceProvider);

    await service.execute(
      action: 'apply',
      expectedRevision: 0,
      changes: <String, dynamic>{
        'target': 'chat.composer.input',
        'properties': <String, dynamic>{'backgroundColor': '#222222'},
      },
    );
    await service.execute(
      action: 'apply',
      expectedRevision: 1,
      changes: <String, dynamic>{
        'scope': <String, dynamic>{'platform': 'mobile', 'page': 'chat'},
        'target': 'chat.composer.input',
        'properties': <String, dynamic>{'backgroundColor': '#000000'},
      },
    );
    await service.execute(
      action: 'reset',
      expectedRevision: 2,
      changes: <String, dynamic>{
        'scope': <String, dynamic>{'platform': 'mobile', 'page': 'chat'},
        'target': 'chat.composer.input',
      },
    );

    final rules = container.read(uiStylePatchProvider).rules;
    expect(rules, hasLength(1));
    expect(rules.single.scope.page, isNull);
    expect(rules.single.properties['backgroundColor'], '#222222');
  });
}

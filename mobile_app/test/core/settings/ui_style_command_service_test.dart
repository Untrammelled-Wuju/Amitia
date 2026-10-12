import 'dart:io';

import 'package:amitia_app/core/settings/appearance_preferences.dart';
import 'package:amitia_app/core/settings/background_preferences.dart';
import 'package:amitia_app/core/settings/ui_style_command_service.dart';
import 'package:amitia_app/core/settings/ui_style_profile.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  late Directory root;

  setUp(() async {
    SharedPreferences.setMockInitialValues(<String, Object>{});
    root = await Directory.systemTemp.createTemp('amitia-style-command-test-');
  });

  tearDown(() async {
    if (await root.exists()) await root.delete(recursive: true);
  });

  test('applies appearance, token patch and background, then undo and reset', () async {
    final container = ProviderContainer(
      overrides: <Override>[
        backgroundPreferencesProvider.overrideWith(
          (ref) => BackgroundPreferencesNotifier(directory: () async => root),
        ),
      ],
    );
    addTearDown(container.dispose);
    final service = container.read(uiStyleCommandServiceProvider);

    final applied = await service.execute(
      action: 'apply',
      expectedRevision: 0,
      changes: <String, dynamic>{
        'themeMode': 'dark',
        'accentColor': '#52B788',
        'fontScale': 1.15,
        'cornerStyle': 2,
        'palette': <String, dynamic>{'accentPrimary': '#52B788'},
        'background': <String, dynamic>{
          'mode': 'gradient',
          'gradientStart': '#111214',
          'gradientEnd': '#1A2635',
          'opacity': 0.7,
        },
      },
    );

    expect(applied['ok'], true);
    expect(container.read(uiStylePatchProvider).revision, 1);
    expect(container.read(appearancePreferencesProvider).fontScale, 1.15);
    expect(
      container.read(backgroundPreferencesProvider).mode,
      BackgroundDisplayMode.gradient,
    );

    final undone = await service.execute(
      action: 'undo',
      expectedRevision: 1,
      changes: const <String, dynamic>{},
    );
    expect(undone['ok'], true);
    expect(container.read(uiStylePatchProvider).patch, isEmpty);
    expect(container.read(backgroundPreferencesProvider).active, false);

    await service.execute(
      action: 'apply',
      expectedRevision: 2,
      changes: <String, dynamic>{'themeMode': 'light'},
    );
    final reset = await service.execute(
      action: 'reset',
      expectedRevision: 3,
      changes: const <String, dynamic>{},
    );
    expect(reset['ok'], true);
    expect(container.read(uiStylePatchProvider).patch, isEmpty);
    expect(container.read(appearancePreferencesProvider).themeMode.name, 'system');
  });

  test('rejects stale revisions', () async {
    final container = ProviderContainer(
      overrides: <Override>[
        backgroundPreferencesProvider.overrideWith(
          (ref) => BackgroundPreferencesNotifier(directory: () async => root),
        ),
      ],
    );
    addTearDown(container.dispose);
    final service = container.read(uiStyleCommandServiceProvider);
    await expectLater(
      service.execute(
        action: 'apply',
        expectedRevision: 4,
        changes: <String, dynamic>{'themeMode': 'dark'},
      ),
      throwsA(isA<StateError>()),
    );
  });
}

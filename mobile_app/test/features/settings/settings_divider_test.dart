import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  test('settings pages do not render option dividers', () {
    const paths = <String>[
      'lib/features/settings/presentation/pages/advanced_system_page.dart',
      'lib/features/settings/presentation/pages/appearance_settings_page.dart',
      'lib/features/settings/presentation/pages/decision_viz_page.dart',
      'lib/features/settings/presentation/pages/device_settings_page.dart',
      'lib/features/settings/presentation/pages/devices_page.dart',
      'lib/features/settings/presentation/pages/safety_page.dart',
      'lib/features/settings/presentation/pages/settings_page.dart',
      'lib/features/settings/presentation/pages/system_settings_page.dart',
      'lib/features/settings/presentation/pages/temporal_settings_page.dart',
      'lib/features/settings/presentation/pages/ui_provider_settings_page.dart',
      'lib/features/settings/presentation/pages/user_settings_page.dart',
    ];

    for (final path in paths) {
      expect(
        File(path).readAsStringSync(),
        isNot(contains('Divider(')),
        reason: path,
      );
    }
  });

  test('settings options are slightly larger than the sidebar menu', () {
    final settingsSource = File(
      'lib/features/settings/presentation/pages/settings_page.dart',
    ).readAsStringSync();
    final drawerSource = File(
      'lib/core/widgets/amitia_drawer.dart',
    ).readAsStringSync();

    expect(settingsSource, contains('_settingsOptionFontSize = 15'));
    expect(settingsSource, contains('width: 32'));
    expect(settingsSource, contains('shape: BoxShape.circle'));
    expect(
      settingsSource,
      contains('Icon(item.icon, size: 17, color: context.accentPrimary)'),
    );
    expect(drawerSource, contains('fontSize: 14.5'));
    expect(settingsSource, isNot(contains("title: '主题设置'")));
    expect(settingsSource, isNot(contains("title: '个人空间'")));
    expect(settingsSource, isNot(contains('_DevModeToggle')));
    expect(settingsSource, isNot(contains('subtitle:')));
    expect(settingsSource, isNot(contains('Switch.adaptive')));
  });

  test('voice recognition lives in model settings', () {
    final settingsSource = File(
      'lib/features/settings/presentation/pages/settings_page.dart',
    ).readAsStringSync();
    final modelSource = File(
      'lib/features/settings/presentation/pages/model_settings_page.dart',
    ).readAsStringSync();

    expect(settingsSource, isNot(contains("title: '语音识别'")));
    expect(modelSource, contains("'语音识别'"));
    expect(modelSource, contains('AppRoutes.settingsAsr'));
  });

  test('appearance settings include the former theme settings', () {
    final source = File(
      'lib/features/settings/presentation/pages/appearance_settings_page.dart',
    ).readAsStringSync();

    expect(source, contains("'主题模式'"));
    expect(source, contains("'字体大小'"));
    expect(source, contains("'主题色调'"));
    expect(source, contains("'圆角风格'"));
    expect(
      File(
        'lib/features/settings/presentation/pages/theme_settings_page.dart',
      ).existsSync(),
      isFalse,
    );
  });

  test('ui provider settings are responsive', () {
    final source = File(
      'lib/features/settings/presentation/pages/ui_provider_settings_page.dart',
    ).readAsStringSync();

    expect(source, contains('LayoutBuilder'));
    expect(source, contains('constraints.maxWidth < 560'));
    expect(source, contains('isExpanded: true'));
  });
}

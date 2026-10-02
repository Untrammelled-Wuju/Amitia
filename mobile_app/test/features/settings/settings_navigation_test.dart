import 'package:flutter/foundation.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/settings/settings_navigation.dart';

void main() {
  tearDown(() => debugDefaultTargetPlatformOverride = null);

  test('settings routes remain unique across categories', () {
    final categories = buildSettingsCategories(
      modelSummary: '默认模型',
      appearanceSummary: '自动',
    );
    final routes = categories
        .expand((category) => category.group.items)
        .map((item) => item.route)
        .toList();
    expect(routes.toSet().length, routes.length);
    expect(categories.map((category) => category.id).toSet().length, 6);
    expect(
      categories
          .firstWhere((category) => category.id == 'appearance')
          .group
          .items
          .map((item) => item.title),
      ['通用', '外观设置', '通知', '时间与地区'],
    );
  });

  test('advanced tools and legal documents have distinct homes', () {
    final categories = buildSettingsCategories(
      modelSummary: '',
      appearanceSummary: '',
    );
    final advanced = categories.firstWhere((item) => item.id == 'maintenance');
    final about = categories.firstWhere((item) => item.id == 'about');
    expect(
      advanced.group.items.map((item) => item.title),
      containsAll(['界面提供者', 'Core 运行模式', '开发者模式']),
    );
    final privacy = categories.firstWhere((item) => item.id == 'privacy');
    expect(privacy.group.title, '隐私');
    expect(
      privacy.group.items.map((item) => item.title),
      containsAll(['隐私说明', '使用边界', '隐私扫描']),
    );
    expect(
      about.group.items.map((item) => item.title),
      isNot(contains('隐私说明')),
    );
  });

  test('Android automation is offered only on Android', () {
    for (final platform in [TargetPlatform.android, TargetPlatform.iOS]) {
      debugDefaultTargetPlatformOverride = platform;
      final items = buildSettingsCategories(
        modelSummary: '',
        appearanceSummary: '',
      ).expand((category) => category.group.items);
      expect(
        items.any((item) => item.title == 'Android 自动化'),
        platform == TargetPlatform.android,
      );
    }
  });
}

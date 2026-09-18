import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  test('extension center keeps a single-column list layout', () {
    final source = File(
      'lib/features/extensions/presentation/pages/extension_center_page.dart',
    ).readAsStringSync();

    expect(source, isNot(contains("'扩展能力'")));
    expect(source, contains("label: '工作流'"));
    expect(source, contains("label: 'Agent Skill'"));
    expect(source, isNot(contains("label: '执行记录'")));
    expect(source, isNot(contains("label: '兼容 Skill'")));
    expect(source, isNot(contains("tooltip: '刷新'")));
    expect(source, contains('Column('));
    expect(source, isNot(contains('Wrap(')));
  });

  test('workshop home stays simple and single-column', () {
    final source = File(
      'lib/features/workshop/presentation/pages/workshop_home_page.dart',
    ).readAsStringSync();

    expect(source, isNot(contains("'制作工具'")));
    expect(source, contains("title: '角色卡工坊'"));
    expect(source, contains("title: '桌宠制作'"));
    expect(source, isNot(contains("title: '工作流'")));
    expect(source, isNot(contains("title: '技能制作'")));
    expect(source, contains('color: context.surfaceSecondary'));
    expect(source, contains('Icons.arrow_forward_rounded'));
    expect(source, isNot(contains('LinearGradient')));
    expect(source, isNot(contains('_buildTipCard')));
  });

  test('extension package cards keep actions in the requested positions', () {
    final source = File(
      'lib/features/extensions/presentation/pages/extension_packages_page.dart',
    ).readAsStringSync();

    expect(source, contains('PopupMenuButton<String>'));
    expect(source, contains("label: '权限管理'"));
    expect(source, contains("label: '暂停'"));
    expect(source, contains("label: '更新'"));
    expect(source, contains("label: '回滚'"));
    expect(source, contains("label: '诊断'"));
    expect(source, contains("label: '卸载'"));
    expect(source, contains("label: '详情'"));
    expect(source, contains('_MiniButton('));
    expect(source, contains('Switch.adaptive('));
    expect(source, contains('AmitiaStatusBadge('));
    expect(source, contains("label: '可信服务运行时'"));
    expect(source, contains("label: 'WASM 运行时'"));
    expect(source, contains("label: 'Hook 中心'"));
    expect(source, contains("label: '任务运行时'"));
    expect(source, contains("label: '事件中心'"));
    expect(source, contains("label: '调度中心'"));
    expect(source, contains("label: '桌面贡献中心'"));
    expect(source, contains("label: '开发者诊断控制台'"));
    expect(source, contains("label: '迁移与灰度中心'"));
    expect(source, contains("label: '开发模式中心'"));
    expect(source, isNot(contains('floatingActionButton:')));
    expect(source, contains('AmitiaSearchField('));
    expect(source, contains("title: _searchVisible ? '搜索扩展包' : '扩展包'"));
    expect(source, contains("hintText: '搜索扩展名称、扩展 ID、版本或状态'"));
  });

  test('mcp page keeps search and add actions in the app bar', () {
    final source = File(
      'lib/features/extensions/presentation/pages/mcp_list_page.dart',
    ).readAsStringSync();

    expect(source, contains("title: _searchVisible ? '搜索 MCP 服务' : 'MCP 服务'"));
    expect(source, contains("tooltip: '搜索'"));
    expect(source, contains("tooltip: '添加 MCP 服务'"));
    expect(source, contains('AmitiaSearchField('));
    expect(source, contains("title: '暂无 MCP 服务'"));
    expect(source, isNot(contains('floatingActionButton:')));
  });

  test('agent skill page keeps search and import actions in the app bar', () {
    final source = File(
      'lib/features/extensions/presentation/pages/agent_skills_page.dart',
    ).readAsStringSync();

    expect(
      source,
      contains(
        "title: _searchVisible ? '搜索 Agent Skill' : 'Agent Skills'",
      ),
    );
    expect(source, contains("tooltip: '搜索'"));
    expect(source, contains("tooltip: '导入 Agent Skill'"));
    expect(source, contains('AmitiaSearchField('));
    expect(source, isNot(contains('floatingActionButton:')));
  });

  test('compatible skill page and route are removed', () {
    final source = File(
      'lib/core/ui_runtime/routing/builtin_route_catalog.dart',
    ).readAsStringSync();

    expect(source, isNot(contains('/extensions/skills')));
    expect(source, isNot(contains('CompatibleSkillsPage')));
    expect(
      File(
        'lib/features/extensions/presentation/pages/compatible_skills_page.dart',
      ).existsSync(),
      isFalse,
    );
    expect(
      File(
        'lib/features/extensions/presentation/pages/skill_detail_page.dart',
      ).existsSync(),
      isFalse,
    );
  });
}

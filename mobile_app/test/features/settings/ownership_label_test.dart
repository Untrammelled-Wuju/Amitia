import 'package:amitia_app/core/device_mesh/ownership_label.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('统筹中心资源使用统筹中心前缀', () {
    expect(coreOwnershipLabel('skill', '翻译助手'), '统筹中心-skill-翻译助手');
    expect(coreOwnershipLabel('mcp', '天气服务'), '统筹中心-mcp-天气服务');
    expect(coreOwnershipLabel('角色', '小美'), '统筹中心-角色-小美');
  });

  test('设备资源使用设备名前缀', () {
    expect(deviceOwnershipLabel('红米K50', '角色', '小美'), '设备红米K50-角色-小美');
    expect(deviceOwnershipLabel('  ', '团队', '研发组'), '设备未命名设备-团队-研发组');
  });

  test('已带归属前缀的名称不重复追加', () {
    expect(coreOwnershipLabel('skill', '统筹中心-skill-翻译助手'), '统筹中心-skill-翻译助手');
    expect(
      deviceOwnershipLabel('红米K50', '角色', '设备红米K50-角色-小美'),
      '设备红米K50-角色-小美',
    );
  });

  test('普通名称不会被误判为已带前缀', () {
    expect(hasOwnershipPrefix('翻译助手'), isFalse);
    expect(hasOwnershipPrefix('统筹中心-未知类别-东西'), isFalse);
    expect(hasOwnershipPrefix('设备-角色-小美'), isFalse);
  });
}

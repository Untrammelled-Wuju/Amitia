import 'package:amitia_app/features/memory/presentation/owned_memory_format.dart';
import 'package:flutter_test/flutter_test.dart';

Map<String, dynamic> row(String key, {String? title}) => {
  'kind': 'profile',
  'id': 'memory-id',
  'body': {
    'title': ?title,
    'key': key,
    'content': {'value': '杭州', 'memoryType': 'fact'},
  },
};

void main() {
  test('画像标题只展示属性名，不暴露分区前缀', () {
    expect(ownedMemoryTitle(row('personal_info/city')), 'city');
    expect(ownedMemoryTitle(row('preference/饮品')), '饮品');
    expect(ownedMemoryTitle(row('habit/早起')), '早起');
  });

  test('画像标题优先使用后端给出的标题', () {
    expect(ownedMemoryTitle(row('fact/city', title: '常住城市')), '常住城市');
  });

  test('普通记忆键保持原样', () {
    expect(ownedMemoryTitle(row('其他层')), '其他层');
  });

  test('记忆生命周期明确区分休眠与自动归档', () {
    expect(ownedMemoryDecayLabel({'effectiveDecayState': 'active'}), '活跃');
    expect(ownedMemoryDecayLabel({'effectiveDecayState': 'fading'}), '淡化');
    expect(ownedMemoryDecayLabel({'effectiveDecayState': 'dormant'}), '休眠 · 可检索');
    expect(ownedMemoryDecayLabel({'effectiveDecayState': 'archived'}), '自动归档 · 可检索');
    expect(ownedMemoryDecayLabel({'effectiveDecayState': 'dormant', 'pinned': true}), '已固定');
  });

  test('记忆确认状态和召回解释可供手机界面展示', () {
    expect(ownedMemoryVerificationLabel({'verifiedStatus': 'user_corrected'}), '用户已更正');
    expect(ownedMemoryVerificationLabel({'verifiedStatus': 'disputed'}), '有争议');
    final summary = ownedMemoryScoreSummary({
      'finalScore': .82,
      'semanticScore': .73,
      'keywordMatch': true,
      'importance': .8,
      'confidence': 1,
      'strength': .68,
      'validity': 1,
      'verification': 1,
    });
    expect(summary, contains('关键词 命中'));
    expect(summary, contains('时效 100'));
  });
}

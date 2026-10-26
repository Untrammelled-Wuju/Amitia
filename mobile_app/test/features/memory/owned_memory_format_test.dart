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
}

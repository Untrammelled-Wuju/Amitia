import 'package:flutter_test/flutter_test.dart';
import 'package:markdown/markdown.dart' as md;
import 'package:amitia_app/features/conversation/rendering/markdown/amitia_markdown.dart';

void main() {
  test('citations do not override normal markdown links or images', () {
    final document = md.Document(inlineSyntaxes: [AmitiaCitationSyntax()]);
    final nodes = document.parseInline(
      '引用 [K1] 与 [1]，链接 [K1](https://example.org/a) '
      '和 [1](https://example.org/b)，图片 ![K1](https://example.org/image.png)。',
    );
    final citations = nodes.whereType<md.Element>().where((node) => node.tag == 'citation').toList();
    final links = nodes.whereType<md.Element>().where((node) => node.tag == 'a').toList();
    expect(citations.map((node) => node.textContent), ['K1', '1']);
    expect(links.length, 2);
    expect(links.first.attributes['href'], 'https://example.org/a');
  });
}

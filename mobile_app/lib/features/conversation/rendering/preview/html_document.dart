import 'package:html/dom.dart';
import 'package:html/parser.dart' as html;

Future<String> buildHtmlPreviewDocument(
  String source, {
  String baseUrl = '',
  Future<Uri> Function(String)? resolveMedia,
  Future<Uri> Function(String)? embedMedia,
}) async {
  final document = html.parse(source);
  final base =
      document.querySelector('base[href]')?.attributes['href'] ?? baseUrl;
  for (final node in document.querySelectorAll('base')) {
    node.remove();
  }
  if (resolveMedia != null) {
    for (final element in document.querySelectorAll(
      '[src],link[href],video[poster]',
    )) {
      final attribute = element.localName == 'link'
          ? 'href'
          : element.attributes.containsKey('src')
          ? 'src'
          : 'poster';
      final raw = element.attributes[attribute] ?? '';
      if (raw.startsWith('amitia://artifacts/') ||
          RegExp(r'/api/artifacts/v1/[^/]+/content').hasMatch(raw)) {
        final resolver = (element.localName == 'img' || attribute == 'poster')
            ? embedMedia ?? resolveMedia
            : resolveMedia;
        element.attributes[attribute] = (await resolver(raw)).toString();
      }
    }
  }
  final head = html.parseFragment(
    '''<meta charset="utf-8"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src https: http: data: blob:; style-src 'unsafe-inline' https: http:; script-src 'unsafe-inline' https: http:; font-src https: http: data:; connect-src 'none'; frame-src https: http: data: blob:; media-src https: http: data: blob:; base-uri https: http:; form-action 'none';"><meta name="viewport" content="width=device-width,initial-scale=1"><style>html,body{min-height:100%;margin:0;background:#fff;color:#19191c;font:14px/1.6 system-ui,sans-serif}body{padding:20px}*{box-sizing:border-box}</style>''',
  );
  document.head!.nodes.insertAll(0, head.nodes.toList());
  if (base.startsWith('https://') || base.startsWith('http://')) {
    document.head!.nodes.insert(
      2,
      Element.tag('base')..attributes['href'] = base,
    );
  }
  return '<!doctype html>${document.documentElement!.outerHtml}';
}

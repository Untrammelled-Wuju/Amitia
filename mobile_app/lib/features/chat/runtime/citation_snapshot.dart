List<Map<String, dynamic>> normalizeCitationSnapshot(Object? raw) {
  if (raw is! List) return const <Map<String, dynamic>>[];
  final entries = <Map<String, dynamic>>[];
  final seen = <String>{};
  for (final value in raw.take(96)) {
    if (entries.length >= 24) break;
    if (value is! Map) continue;
    final kind = value['kind'];
    final index = value['index'];
    if (kind != 'knowledge' && kind != 'web') continue;
    if (index is! int || index < 1 || index > 999999) continue;
    final marker = kind == 'knowledge' ? '[K$index]' : '[$index]';
    if (value['marker'] != marker || !seen.add(marker)) continue;
    final citation = <String, dynamic>{
      'marker': marker,
      'kind': kind,
      'index': index,
    };
    const limits = <String, int>{
      'evidenceId': 256, 'refId': 256, 'sourceId': 256, 'unitId': 256,
      'title': 200, 'excerpt': 1200,
      'contentHash': 128, 'checksum': 128,
    };
    for (final entry in limits.entries) {
      final content = value[entry.key];
      if (content is String) {
        final trimmed = content.trim();
        citation[entry.key] = trimmed.runes.length > entry.value
            ? String.fromCharCodes(trimmed.runes.take(entry.value))
            : trimmed;
      }
    }
    final url = normalizeCitationURL(value['url']);
    if (url != null) citation['url'] = url;
    final revision = value['sourceRevision'];
    if (revision is int && revision >= 0) citation['sourceRevision'] = revision;
    final locator = normalizeCitationLocator(value['locator']);
    if (locator != null) citation['locator'] = locator;
    entries.add(Map<String, dynamic>.unmodifiable(citation));
  }
  return List<Map<String, dynamic>>.unmodifiable(entries);
}

const _sensitiveCitationQueryKeys = <String>[
  'token', 'secret', 'password', 'passwd', 'apikey', 'api_key',
  'auth', 'signature', 'credential', 'session', 'access_key', 'code',
];

String? normalizeCitationURL(Object? raw) {
  if (raw is! String || raw.trim().isEmpty || raw.length > 2048 ||
      raw.contains(RegExp(r'[\r\n\t\\]'))) {
    return null;
  }
  final url = Uri.tryParse(raw.trim());
  if (url == null || (url.scheme != 'https' && url.scheme != 'http') ||
      url.host.isEmpty || url.userInfo.isNotEmpty) {
    return null;
  }
  final safeQuery = <String, List<String>>{};
  for (final entry in url.queryParametersAll.entries) {
    final key = entry.key.toLowerCase();
    if (_sensitiveCitationQueryKeys.any(key.contains)) continue;
    safeQuery[entry.key] = entry.value;
  }
  return url.replace(
    query: safeQuery.isEmpty ? '' : null,
    queryParameters: safeQuery.isEmpty ? null : safeQuery,
  ).toString();
}

Map<String, dynamic>? normalizeCitationLocator(Object? raw) {
  if (raw is! Map) return null;
  final result = <String, dynamic>{};
  const aliases = <String, List<String>>{
    'page': ['page', 'pageNumber', 'page_number'],
    'row': ['row', 'rowStart', 'row_start', 'rowEnd', 'row_end'],
    'sheet': ['sheet', 'sheetName', 'sheet_name'],
    'paragraph': ['paragraph', 'paragraphIndex', 'paragraph_index'],
    'section': ['section'],
    'index': ['index'],
  };
  for (final key in ['page', 'row', 'sheet', 'paragraph', 'section', 'index']) {
    Object? rawValue;
    var selected = '';
    for (final alias in aliases[key]!) {
      if (raw.containsKey(alias)) {
        rawValue = raw[alias];
        selected = alias;
        break;
      }
    }
    if (key == 'sheet' && selected != 'sheet' && rawValue is String &&
        _safeCitationLocatorLabel(rawValue)) {
      result[key] = rawValue.trim();
      continue;
    }
    final value = rawValue is String ? int.tryParse(rawValue.trim()) : rawValue;
    if (value is int && value >= 0 && value <= 1000000000) result[key] = value;
    else if (key == 'sheet' && rawValue is String && _safeCitationLocatorLabel(rawValue)) {
      result[key] = rawValue.trim();
    }
  }
  for (final key in ['kind', 'field']) {
    final value = raw[key];
    if (value is String && _safeCitationLocatorLabel(value)) {
      result[key] = value.trim();
    }
  }
  return result.isEmpty ? null : Map<String, dynamic>.unmodifiable(result);
}

bool _safeCitationLocatorLabel(String value) =>
    value.trim().isNotEmpty && value.trim().runes.length <= 128 &&
    !value.contains(RegExp(r'[/\\:\x00-\x1F\x7F-\x9F]'));

import 'dart:convert';

class OwnedMemoryField {
  const OwnedMemoryField(this.label, this.value);

  final String label;
  final String value;
}

const Map<String, String> _fieldLabels = <String, String>{
  'name': '名字',
  'likes': '喜欢',
  'dislikes': '不喜欢',
  'preference': '偏好',
  'habit': '习惯',
  'plan': '计划',
  'relationship': '关系',
  'event': '事件',
  'details': '详情',
  'summary': '摘要',
  'topic': '话题',
  'user_input': '用户输入',
  'ai_reply_summary': '回复要点',
  'status': '状态',
  'subject': '主语',
  'predicate': '谓语',
  'object': '宾语',
  'relation': '关系',
  'relations': '关系',
  'facts': '关联事实',
  'nodes': '节点',
  'edges': '连线',
  'links': '连线',
  'source': '来源',
  'target': '目标',
  'from': '起点',
  'to': '终点',
  'content': '内容',
  'text': '文本',
  'value': '内容',
  'attributeName': '属性',
  'attributeValue': '取值',
  'fieldName': '字段',
  'fieldValue': '取值',
  'memoryType': '类型',
  'importance': '重要度',
  'createdAt': '创建时间',
  'updatedAt': '更新时间',
  'file': '文件',
  'file_name': '文件',
  'filename': '文件',
  'path': '路径',
  'target_file': '目标文件',
  'target_workspace': '目标工作区',
  'expected_content': '期望内容',
  'actual_content': '实际内容',
  'created': '已创建',
  'resolved_path': '解析路径',
  'workspace': '工作区',
  'workspace_path_available': '工作区路径可用',
  'file_content_available': '文件内容可用',
  'note': '说明',
  'outcome': '结果',
  'result': '结果',
  'reason': '原因',
  'error': '错误',
  'errors': '错误',
  'refused': '拒绝行为',
  'tools_tried': '尝试的工具',
  'extra_probe': '额外探测',
  'color': '颜色',
  'color_name': '颜色',
  'hex': '色值',
  'scope': '范围',
  'property': '属性',
  'statement': '结论',
  'values': '向量',
  'modelFingerprint': '模型',
};

const Map<String, String> _titleLabels = <String, String>{
  'profile_name': '名字',
  'profile_likes': '喜欢的',
  'profile_dislikes': '不喜欢的',
  'profile_communication_preference': '沟通偏好',
  'profile_preference': '偏好',
  'user_name_is_axu': '用户名字',
  'user_self_introduction': '用户自我介绍',
  'conversation_summary': '对话摘要',
  'current_turn_context': '当前对话上下文',
};

const List<String> _primaryTextKeys = <String>['value', 'text', 'summary'];

const Map<String, String> _wordLabels = <String, String>{
  'page': '页面',
  'background': '背景',
  'image': '图片',
  'mode': '模式',
  'enabled': '启用',
  'name': '名称',
  'count': '数量',
  'previous': '原',
  'opacity': '不透明度',
  'revision': '版本',
  'version': '版本',
  'selector': '选择器',
  'computed': '实际',
  'value': '值',
  'description': '说明',
  'action': '操作',
  'result': '结果',
  'status': '状态',
  'custom': '自定义',
  'rules': '规则',
  'theme': '主题',
  'preset': '预设',
  'primary': '主',
  'color': '颜色',
  'border': '边框',
  'radius': '圆角',
  'root': '根路径',
  'includes': '包含',
  'include': '包含',
  'not': '不',
  'source': '来源',
  'uri': '地址',
  'communication': '沟通',
  'preference': '偏好',
  'reset': '重置',
  'all': '全部',
  'default': '默认',
  'transparent': '透明',
  'sidebar': '侧边栏',
  'chat': '聊天',
  'mount': '挂载',
  'workspace': '工作区',
  'file': '文件',
  'content': '内容',
  'expected': '期望',
  'actual': '实际',
  'target': '目标',
  'event': '事件',
  'details': '详情',
  'summary': '摘要',
  'topic': '话题',
  'user': '用户',
  'input': '输入',
  'reply': '回复',
  'self': '自我',
  'introduction': '介绍',
  'likes': '喜欢',
  'dislikes': '不喜欢',
  'setting': '设置',
  'settings': '设置',
  'restore': '恢复',
  'change': '变更',
  'state': '状态',
  'current': '当前',
  'style': '样式',
  'host': '宿主',
  'session': '会话',
  'required': '必需',
  'caveat': '备注',
  'note': '说明',
  'proof': '凭据',
  'creation': '创建',
  'created': '创建',
  'attempt': '尝试',
  'failed': '失败',
  'retry': '重试',
  'availability': '可用性',
  'capability': '能力',
  'window': '窗口',
  'ui': '界面',
  'applied': '已应用',
  'drive': '磁盘',
  'read': '读取',
  'request': '请求',
};

const Set<String> _metaKeys = <String>{
  'id',
  'memoryId',
  'memory_id',
  'source',
  'memoryType',
  'importance',
  'createdAt',
  'updatedAt',
  'updated_at',
  'verifiedAt',
  'projectionStatus',
  'conversationId',
  'deleted',
};

String? _translateKey(String key) {
  final exact = _fieldLabels[key] ?? _titleLabels[key];
  if (exact != null) return exact;
  final snake = key.replaceAllMapped(
    RegExp(r'([a-z0-9])([A-Z])'),
    (match) => '${match[1]}_${match[2]}',
  );
  final words = snake
      .split(RegExp(r'[_\-\s]+'))
      .where((word) => word.isNotEmpty)
      .toList();
  if (words.isEmpty) return null;
  final parts = <String>[];
  for (final word in words) {
    final label = _wordLabels[word.toLowerCase()];
    if (label == null) return null;
    parts.add(label);
  }
  return parts.join();
}

String _stripPrefix(String raw) => raw
    .replaceFirst(
      RegExp(r'^(profile_|user_|episodic_|system_|fact_|memory_|graph_)'),
      '',
    )
    .replaceFirst(RegExp(r'^[a-z_]+:'), '');

String _fieldLabel(String key) =>
    _translateKey(key) ?? key.replaceAll(RegExp(r'[_-]+'), ' ');

String ownedMemoryValueText(Object? value) {
  if (value == null) return '';
  if (value is String) return value.trim();
  if (value is bool) return value ? '是' : '否';
  if (value is num) return value.toString();
  if (value is List) {
    return value
        .map(ownedMemoryValueText)
        .where((item) => item.isNotEmpty)
        .join('、');
  }
  if (value is Map) {
    final parts = <String>[];
    value.forEach((key, item) {
      final text = ownedMemoryValueText(item);
      if (text.isNotEmpty) parts.add('${_fieldLabel(key.toString())}：$text');
    });
    return parts.join('；');
  }
  return value.toString();
}

Map<String, dynamic> ownedMemoryBody(Map<String, dynamic> row) {
  final body = row['body'];
  return body is Map ? Map<String, dynamic>.from(body) : <String, dynamic>{};
}

Object? ownedMemoryContent(Map<String, dynamic> row) {
  final body = ownedMemoryBody(row);
  if (body['content'] != null) return body['content'];
  return body['value'];
}

String ownedMemoryTitle(Map<String, dynamic> row) {
  final body = ownedMemoryBody(row);
  final raw = (body['title'] ?? body['key'] ?? '').toString().trim();
  if (raw.isEmpty) return (row['id'] ?? '记忆记录').toString();
  return _translateKey(raw) ?? _translateKey(_stripPrefix(raw)) ?? raw;
}

List<OwnedMemoryField> ownedMemoryFields(
  Map<String, dynamic> row, {
  String? kind,
}) {
  final body = ownedMemoryBody(row);
  final resolvedKind = (row['kind'] ?? kind ?? '').toString();
  if (body['type'] == 'owned-memory-candidate' || kind == 'candidate') {
    final value = ownedMemoryValueText(body['value']);
    return value.isEmpty
        ? const <OwnedMemoryField>[]
        : <OwnedMemoryField>[OwnedMemoryField('候选内容', value)];
  }
  final content = ownedMemoryContent(row);
  if (resolvedKind == 'vector' && content is Map) {
    final values = content['values'];
    final dim = values is List ? values.length : 0;
    final model = (content['modelFingerprint'] ?? '待重建').toString();
    return <OwnedMemoryField>[
      OwnedMemoryField('向量', '$dim 维 · 模型 ${model.length > 16 ? model.substring(0, 16) : model}'),
    ];
  }
  if (content is String) {
    return content.trim().isEmpty
        ? const <OwnedMemoryField>[]
        : <OwnedMemoryField>[OwnedMemoryField('内容', content)];
  }
  if (content is! Map) {
    final value = ownedMemoryValueText(content);
    return value.isEmpty
        ? const <OwnedMemoryField>[]
        : <OwnedMemoryField>[OwnedMemoryField('内容', value)];
  }
  final map = Map<String, dynamic>.from(content);
  if (resolvedKind == 'graph' && map['nodes'] is List) {
    final nodes = (map['nodes'] as List).length;
    final edges = map['edges'] is List ? (map['edges'] as List).length : 0;
    final fields = <OwnedMemoryField>[OwnedMemoryField('图谱', '$nodes 个节点 · $edges 条关系')];
    for (final entry in map.entries) {
      if (const ['nodes', 'edges', 'links', 'relations'].contains(entry.key)) {
        continue;
      }
      final text = ownedMemoryValueText(entry.value);
      if (text.isNotEmpty) fields.add(OwnedMemoryField(_fieldLabel(entry.key), text));
    }
    return fields;
  }
  if (resolvedKind == 'fact' && map.containsKey('subject') && map.containsKey('object')) {
    final sentence = [map['subject'], map['predicate'], map['object']]
        .map(ownedMemoryValueText)
        .where((item) => item.isNotEmpty)
        .join(' · ');
    return sentence.isEmpty
        ? const <OwnedMemoryField>[]
        : <OwnedMemoryField>[OwnedMemoryField('事实', sentence)];
  }
  String? primary;
  for (final key in _primaryTextKeys) {
    final value = map[key];
    if (value is String && value.trim().isNotEmpty) {
      primary = key;
      break;
    }
  }
  final fields = <OwnedMemoryField>[];
  if (primary != null) {
    fields.add(OwnedMemoryField('内容', map[primary].toString()));
  }
  for (final entry in map.entries) {
    if (primary != null && entry.key == primary) continue;
    if (_metaKeys.contains(entry.key) &&
        !(primary != null &&
            const ['source', 'importance'].contains(entry.key))) {
      continue;
    }
    final text = ownedMemoryValueText(entry.value);
    if (text.isNotEmpty) fields.add(OwnedMemoryField(_fieldLabel(entry.key), text));
  }
  return fields;
}

String ownedMemoryText(Map<String, dynamic> row, {String? kind}) {
  final fields = ownedMemoryFields(row, kind: kind);
  if (fields.isEmpty) return '';
  if (fields.length == 1) return fields.single.value;
  return fields.map((field) => '${field.label}：${field.value}').join(' · ');
}

String ownedMemoryEditableText(Map<String, dynamic> row) {
  final content = ownedMemoryContent(row);
  if (content == null) return '';
  if (content is String) return content;
  if (content is Map) {
    for (final key in _primaryTextKeys) {
      final value = content[key];
      if (value is String && value.trim().isNotEmpty) return value;
    }
    return const JsonEncoder.withIndent('  ').convert(content);
  }
  return content.toString();
}

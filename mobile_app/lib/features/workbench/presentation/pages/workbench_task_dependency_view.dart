class WorkbenchTaskDependencyView {
  static List<Map<String, dynamic>> project(List children) {
    final nodes = <String, Map<String, dynamic>>{};
    for (final value in children) {
      if (value is! Map) throw const FormatException('子任务格式无效');
      final child = Map<String, dynamic>.from(value);
      final id = child['id'];
      if (id is! String || id.isEmpty || nodes.containsKey(id)) throw const FormatException('子任务标识无效');
      nodes[id] = child;
    }
    final depths = <String, int>{};
    int depth(String id, Set<String> visiting) {
      if (depths.containsKey(id)) return depths[id]!;
      if (!nodes.containsKey(id) || !visiting.add(id)) throw const FormatException('子任务依赖引用或循环无效');
      final raw = nodes[id]!['dependencies'];
      if (raw is! List) throw const FormatException('子任务依赖格式无效');
      final parents = raw.map((value) {
        if (value is! String) throw const FormatException('子任务依赖标识无效');
        return value;
      }).toList();
      final level = parents.isEmpty ? 0 : 1 + parents.map((parent) => depth(parent, visiting)).reduce((a, b) => a > b ? a : b);
      visiting.remove(id);
      depths[id] = level;
      return level;
    }
    for (final id in nodes.keys) { depth(id, <String>{}); }
    final rows = <Map<String, dynamic>>[];
    for (final node in nodes.values) {
      final parents = (node['dependencies'] as List).map((id) => nodes[id as String]!).toList();
      final unfinished = parents.where((parent) => parent['status'] != 'succeeded').toList();
      final status = node['status'];
      final inconsistent = unfinished.isNotEmpty && (status == 'running' || status == 'succeeded');
      final blocked = status == 'pending' || status == 'waiting' || status == 'paused';
      final failed = unfinished.any((parent) => parent['status'] == 'failed' || parent['status'] == 'cancelled');
      rows.add({
        'assignment': node,
        'depth': depth(node['id'] as String, <String>{}),
        'inconsistent': inconsistent,
        'notice': inconsistent ? '依赖证据冲突，需核查' : blocked && failed ? '前置任务失败或取消，需等待协调' : blocked && unfinished.isNotEmpty ? '等待 ${unfinished.length} 个前置任务' : '',
      });
    }
    rows.sort((a, b) {
      final result = (a['depth'] as int).compareTo(b['depth'] as int);
      return result != 0 ? result : children.indexWhere((node) => node is Map && node['id'] == (a['assignment'] as Map)['id']).compareTo(children.indexWhere((node) => node is Map && node['id'] == (b['assignment'] as Map)['id']));
    });
    return rows;
  }
}

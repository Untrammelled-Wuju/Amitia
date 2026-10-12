import 'dart:convert';
import 'dart:math' as math;
import 'package:flutter/material.dart';

class OwnedMemoryGraphData {
  const OwnedMemoryGraphData(this.nodes, this.links);
  final List<Map<String, String>> nodes;
  final List<Map<String, String>> links;
}

OwnedMemoryGraphData buildOwnedMemoryGraph(Iterable<Map> rows) {
  final nodes = <String, Map<String, String>>{};
  final links = <String, Map<String, String>>{};
  String text(dynamic value) {
    if (value is String || value is num) return value.toString();
    if (value is Map)
      return text(
        value['id'] ?? value['name'] ?? value['label'] ?? value['value'],
      );
    return '';
  }

  for (final row in rows) {
    if (row['kind'] != 'graph' || row['deleted'] == true) continue;
    final prefix = jsonEncode([row['ownerId'], row['roleId']]);
    String node(dynamic value, [dynamic label]) {
      final identity = text(value);
      if (identity.isEmpty) return '';
      final id = jsonEncode([prefix, identity]);
      nodes.putIfAbsent(
        id,
        () => {
          'id': id,
          'name': text(label).isNotEmpty
              ? text(label)
              : value is Map
              ? text(value['label'] ?? value['name'] ?? identity)
              : identity,
        },
      );
      return id;
    }

    void edge(dynamic from, dynamic to, dynamic relation) {
      if (text(from).isEmpty || text(to).isEmpty) return;
      final source = node(from), target = node(to), name = text(relation);
      links[jsonEncode([source, target, name])] = {
        'source': source,
        'target': target,
        'name': name,
      };
    }

    final body = row['body'] as Map? ?? {};
    final content = body['content'];
    bool recognized = false;
    if (content is Map) {
      final rawNodes = content['nodes'];
      if (rawNodes is List) {
        recognized = true;
        for (final item in rawNodes) {
          if (item != null)
            node(
              item is Map ? item['id'] ?? item : item,
              item is Map ? item['label'] ?? item['name'] : null,
            );
        }
      }
      final edges =
          content['edges'] ?? content['links'] ?? content['relations'];
      if (edges is List) {
        recognized = true;
        for (final item in edges.whereType<Map>()) {
          edge(
            item['source'] ?? item['subject'] ?? item['from'] ?? item['in'],
            item['target'] ?? item['object'] ?? item['to'] ?? item['out'],
            item['relation'] ??
                item['predicate'] ??
                item['label'] ??
                item['type'],
          );
        }
      }
      final target = content['target'] ?? content['object'] ?? content['to'];
      if (target != null) {
        recognized = true;
        edge(
          content['source'] ??
              content['subject'] ??
              content['from'] ??
              body['key'],
          target,
          content['relation'] ?? content['predicate'] ?? content['type'],
        );
      }
    }
    if (!recognized)
      node(row['id'], body['key'] ?? body['title'] ?? text(content));
  }
  return OwnedMemoryGraphData(nodes.values.toList(), links.values.toList());
}

class OwnedMemoryGraph extends StatefulWidget {
  const OwnedMemoryGraph({super.key, required this.rows});
  final List<Map<String, dynamic>> rows;
  @override
  State<OwnedMemoryGraph> createState() => _OwnedMemoryGraphState();
}

class _OwnedMemoryGraphState extends State<OwnedMemoryGraph> {
  final _transform = TransformationController();
  final _positions = <String, Offset>{};
  String _query = '', _selected = '';
  bool _fitted = false;
  @override
  void dispose() {
    _transform.dispose();
    super.dispose();
  }

  @override
  void didUpdateWidget(covariant OwnedMemoryGraph oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (!identical(oldWidget.rows, widget.rows)) {
      _positions.clear();
      _selected = '';
      _fitted = false;
    }
  }

  @override
  Widget build(BuildContext context) {
    final graph = buildOwnedMemoryGraph(widget.rows);
    final matching = graph.nodes
        .where((n) => n['name']!.toLowerCase().contains(_query.toLowerCase()))
        .map((n) => n['id']!)
        .toSet();
    final visibleIds = <String>{...matching};
    for (final link in graph.links) {
      if (matching.contains(link['source']) ||
          matching.contains(link['target'])) {
        visibleIds.add(link['source']!);
        visibleIds.add(link['target']!);
      }
    }
    final nodes = graph.nodes
        .where((n) => visibleIds.contains(n['id']))
        .toList();
    final edges = graph.links
        .where(
          (e) =>
              visibleIds.contains(e['source']) &&
              visibleIds.contains(e['target']),
        )
        .toList();
    final extent = math.max(360.0, math.sqrt(math.max(nodes.length, 1)) * 150);
    for (var i = 0; i < nodes.length; i++) {
      final angle = 2 * math.pi * i / math.max(nodes.length, 1);
      _positions.putIfAbsent(
        nodes[i]['id']!,
        () => Offset(
          extent / 2 + math.cos(angle) * extent * .36,
          extent / 2 + math.sin(angle) * extent * .36,
        ),
      );
    }
    return Column(
      children: [
        Padding(
          padding: const EdgeInsets.all(12),
          child: Row(
            children: [
              Expanded(
                child: TextField(
                  decoration: const InputDecoration(labelText: '搜索图谱节点'),
                  onChanged: (v) => setState(() {
                    _query = v;
                    _fitted = false;
                  }),
                ),
              ),
              IconButton(
                tooltip: '重置图谱',
                onPressed: () {
                  setState(() {
                    _positions.clear();
                    _fitted = false;
                  });
                  _transform.value = Matrix4.identity();
                },
                icon: const Icon(Icons.center_focus_strong),
              ),
            ],
          ),
        ),
        Text('${nodes.length} 个节点 · ${edges.length} 条关系 · 可拖动节点、缩放和平移'),
        SizedBox(
          height: 380,
          child: nodes.isEmpty
              ? const Center(child: Text('暂无图谱数据'))
              : LayoutBuilder(
                  builder: (context, constraints) {
                    if (!_fitted) {
                      final scale =
                          math.min(
                            constraints.maxWidth / extent,
                            380 / extent,
                          ) *
                          .9;
                      _transform.value = Matrix4.identity()
                        ..translate(
                          (constraints.maxWidth - extent * scale) / 2,
                          (380 - extent * scale) / 2,
                        )
                        ..scale(scale);
                      _fitted = true;
                    }
                    return ClipRect(
                      child: InteractiveViewer(
                        transformationController: _transform,
                        minScale: .05,
                        maxScale: 4,
                        boundaryMargin: const EdgeInsets.all(500),
                        constrained: false,
                        child: SizedBox(
                          width: extent,
                          height: extent,
                          child: Stack(
                            children: [
                              Positioned.fill(
                                child: CustomPaint(
                                  painter: _OwnedGraphPainter(
                                    edges,
                                    _positions,
                                    Theme.of(context).colorScheme.outline,
                                  ),
                                ),
                              ),
                              for (final n in nodes)
                                Positioned(
                                  left: _positions[n['id']]!.dx - 65,
                                  top: _positions[n['id']]!.dy - 28,
                                  child: GestureDetector(
                                    onPanUpdate: (d) => setState(
                                      () => _positions[n['id']!] =
                                          _positions[n['id']]! + d.delta,
                                    ),
                                    onTap: () =>
                                        setState(() => _selected = n['id']!),
                                    child: Semantics(
                                      button: true,
                                      label: '图谱节点 ${n['name']}',
                                      child: Container(
                                        width: 130,
                                        padding: const EdgeInsets.all(10),
                                        decoration: BoxDecoration(
                                          color: _selected == n['id']
                                              ? Theme.of(
                                                  context,
                                                ).colorScheme.primaryContainer
                                              : Theme.of(context)
                                                    .colorScheme
                                                    .surfaceContainerHighest,
                                          borderRadius: BorderRadius.circular(
                                            16,
                                          ),
                                          border: Border.all(
                                            color: Theme.of(
                                              context,
                                            ).colorScheme.primary,
                                          ),
                                        ),
                                        child: Text(
                                          n['name']!,
                                          textAlign: TextAlign.center,
                                          maxLines: 2,
                                          overflow: TextOverflow.ellipsis,
                                        ),
                                      ),
                                    ),
                                  ),
                                ),
                            ],
                          ),
                        ),
                      ),
                    );
                  },
                ),
        ),
        if (_selected.isNotEmpty)
          for (final edge in edges.where(
            (e) => e['source'] == _selected || e['target'] == _selected,
          ))
            Padding(
              padding: const EdgeInsets.all(6),
              child: Text(
                '${graph.nodes.firstWhere((n) => n['id'] == edge['source'])['name']} → ${edge['name']} → ${graph.nodes.firstWhere((n) => n['id'] == edge['target'])['name']}',
              ),
            ),
      ],
    );
  }
}

class _OwnedGraphPainter extends CustomPainter {
  _OwnedGraphPainter(this.edges, Map<String, Offset> positions, this.color)
    : positions = Map.of(positions);
  final List<Map<String, String>> edges;
  final Map<String, Offset> positions;
  final Color color;
  @override
  void paint(Canvas canvas, Size size) {
    final paint = Paint()
      ..color = color
      ..strokeWidth = 1.5;
    for (final e in edges) {
      final a = positions[e['source']], b = positions[e['target']];
      if (a == null || b == null) continue;
      canvas.drawLine(a, b, paint);
      final label = TextPainter(
        text: TextSpan(
          text: e['name'],
          style: TextStyle(color: color, fontSize: 12),
        ),
        textDirection: TextDirection.ltr,
      )..layout(maxWidth: 150);
      label.paint(
        canvas,
        Offset(
          (a.dx + b.dx - label.width) / 2,
          (a.dy + b.dy - label.height) / 2,
        ),
      );
    }
  }

  @override
  bool shouldRepaint(covariant _OwnedGraphPainter oldDelegate) => true;
}

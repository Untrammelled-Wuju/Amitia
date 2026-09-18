import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class MemoryGraphPage extends ConsumerStatefulWidget {
  const MemoryGraphPage({super.key});

  @override
  ConsumerState<MemoryGraphPage> createState() => _MemoryGraphPageState();
}

class _MemoryGraphPageState extends ConsumerState<MemoryGraphPage> {
  String _query = '';
  String _type = '';
  bool _showCanvas = true;

  @override
  Widget build(BuildContext context) {
    final graph = ref.watch(_graphProvider);
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '记忆图谱',
        showBackButton: true,
        fallbackRoute: AppRoutes.memory,
        actions: [
          AmitiaIconButton(
            icon: _showCanvas ? Icons.list_alt : Icons.hub_outlined,
            tooltip: _showCanvas ? '列表视图' : '图谱视图',
            onPressed: () => setState(() => _showCanvas = !_showCanvas),
          ),
          AmitiaIconButton(
            icon: Icons.refresh,
            tooltip: '刷新图谱',
            onPressed: () => ref.invalidate(_graphProvider),
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: graph.when(
          loading: () => const AmitiaLoadingState(message: '正在加载图谱...'),
          error: (error, _) => AmitiaErrorState(
            message: error.toString(),
            onRetry: () => ref.invalidate(_graphProvider),
          ),
          data: (data) => _buildContent(context, data),
        ),
      ),
    );
  }

  Widget _buildContent(BuildContext context, _GraphData data) {
    final types = data.nodes
        .map((node) => _string(node, const ['entity_type', 'entityType']))
        .where((value) => value.isNotEmpty)
        .toSet()
        .toList()
      ..sort();
    final filtered = data.nodes.where((node) {
      final type = _string(node, const ['entity_type', 'entityType']);
      final label = _nodeLabel(node).toLowerCase();
      final properties = (node['properties'] ?? '').toString().toLowerCase();
      if (_type.isNotEmpty && type != _type) return false;
      if (_query.isNotEmpty &&
          !label.contains(_query.toLowerCase()) &&
          !properties.contains(_query.toLowerCase())) {
        return false;
      }
      return true;
    }).toList(growable: false);

    return Column(
      children: [
        Padding(
          padding: EdgeInsets.fromLTRB(
            AppSpacing.pagePadding,
            AppSpacing.sm,
            AppSpacing.pagePadding,
            0,
          ),
          child: Column(
            children: [
              Row(
                children: [
                  Expanded(
                    child: AmitiaTextField(
                      hintText: '搜索节点标签或属性',
                      prefixIcon: const Icon(Icons.search),
                      onChanged: (value) => setState(() => _query = value.trim()),
                    ),
                  ),
                  SizedBox(width: AppSpacing.sm),
                  PopupMenuButton<String>(
                    tooltip: '节点类型',
                    onSelected: (value) => setState(() => _type = value),
                    itemBuilder: (_) => [
                      const PopupMenuItem(value: '', child: Text('全部类型')),
                      ...types.map(
                        (type) => PopupMenuItem(
                          value: type,
                          child: Text(type),
                        ),
                      ),
                    ],
                    child: Container(
                      height: 44,
                      padding: const EdgeInsets.symmetric(horizontal: 12),
                      decoration: BoxDecoration(
                        color: context.surfaceSecondary,
                        borderRadius: AppRadius.brMedium,
                      ),
                      child: Row(
                        children: [
                          const Icon(Icons.filter_alt_outlined, size: 18),
                          const SizedBox(width: 6),
                          Text(_type.isEmpty ? '全部类型' : _type),
                        ],
                      ),
                    ),
                  ),
                ],
              ),
              SizedBox(height: AppSpacing.sm),
              _buildStats(context, data),
            ],
          ),
        ),
        Expanded(
          child: filtered.isEmpty
              ? const AmitiaEmptyState(
                  icon: Icons.hub_outlined,
                  title: '暂无图谱节点',
                  subtitle: '记忆写入并完成图谱同步后会显示在这里',
                )
              : _showCanvas
                  ? _buildCanvas(context, filtered, data.edges)
                  : _buildList(context, filtered, data.edges),
        ),
      ],
    );
  }

  Widget _buildStats(BuildContext context, _GraphData data) {
    final nodeCount = data.stats['nodeCount'] ?? data.nodes.length;
    final edgeCount = data.stats['edgeCount'] ?? data.edges.length;
    final byType = data.stats['byType'];
    return AmitiaCard(
      child: Wrap(
        spacing: AppSpacing.lg,
        runSpacing: AppSpacing.sm,
        children: [
          _Stat(label: '节点', value: '$nodeCount'),
          _Stat(label: '关系', value: '$edgeCount'),
          if (byType is List) _Stat(label: '节点类型', value: '${byType.length}'),
          _Stat(label: '当前显示', value: '${data.nodes.length}'),
        ],
      ),
    );
  }

  Widget _buildCanvas(
    BuildContext context,
    List<Map<String, dynamic>> nodes,
    List<Map<String, dynamic>> edges,
  ) {
    final visibleNodes = nodes.take(80).toList(growable: false);
    final visibleIds = visibleNodes.map(_nodeId).toSet();
    final visibleEdges = edges.where((edge) {
      return visibleIds.contains(_recordId(edge['in'])) &&
          visibleIds.contains(_recordId(edge['out']));
    }).toList(growable: false);

    return Padding(
      padding: EdgeInsets.all(AppSpacing.pagePadding),
      child: ClipRRect(
        borderRadius: AppRadius.brLarge,
        child: Container(
          color: context.surfaceSecondary,
          child: InteractiveViewer(
            minScale: 0.5,
            maxScale: 3.5,
            boundaryMargin: const EdgeInsets.all(300),
            child: SizedBox(
              width: 900,
              height: 720,
              child: Stack(
                children: [
                  Positioned.fill(
                    child: CustomPaint(
                      painter: _GraphPainter(
                        nodes: visibleNodes,
                        edges: visibleEdges,
                        lineColor: context.borderPrimary,
                        nodeColor: context.accentPrimary,
                      ),
                    ),
                  ),
                  ..._nodeWidgets(context, visibleNodes),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }

  List<Widget> _nodeWidgets(
    BuildContext context,
    List<Map<String, dynamic>> nodes,
  ) {
    final positions = _positions(nodes.length);
    return List.generate(nodes.length, (index) {
      final node = nodes[index];
      final position = positions[index];
      final type = _string(node, const ['entity_type', 'entityType']);
      final label = _nodeLabel(node);
      return Positioned(
        left: position.dx - 58,
        top: position.dy - 27,
        child: GestureDetector(
          onTap: () => _showNode(context, node),
          child: Container(
            width: 116,
            constraints: const BoxConstraints(minHeight: 54),
            padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 7),
            decoration: BoxDecoration(
              color: context.surfacePrimary,
              borderRadius: AppRadius.brMedium,
              border: Border.all(color: _typeColor(context, type), width: 1.4),
              boxShadow: [
                BoxShadow(
                  color: Colors.black.withValues(alpha: 0.06),
                  blurRadius: 8,
                  offset: const Offset(0, 2),
                ),
              ],
            ),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  label,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  textAlign: TextAlign.center,
                  style: AppTypography.label(context).copyWith(
                    color: context.textPrimary,
                    fontWeight: FontWeight.w600,
                  ),
                ),
                if (type.isNotEmpty)
                  Text(
                    type,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: AppTypography.caption(context).copyWith(
                      color: _typeColor(context, type),
                    ),
                  ),
              ],
            ),
          ),
        ),
      );
    });
  }

  Widget _buildList(
    BuildContext context,
    List<Map<String, dynamic>> nodes,
    List<Map<String, dynamic>> edges,
  ) {
    return ListView.separated(
      padding: EdgeInsets.all(AppSpacing.pagePadding),
      itemCount: nodes.length,
      separatorBuilder: (_, _) => SizedBox(height: AppSpacing.sm),
      itemBuilder: (context, index) {
        final node = nodes[index];
        final id = _nodeId(node);
        final type = _string(node, const ['entity_type', 'entityType']);
        final relationCount = edges.where((edge) {
          return _recordId(edge['in']) == id || _recordId(edge['out']) == id;
        }).length;
        return AmitiaCard(
          onTap: () => _showNode(context, node),
          child: Row(
            children: [
              Container(
                width: 42,
                height: 42,
                decoration: BoxDecoration(
                  color: _typeColor(context, type).withValues(alpha: 0.12),
                  borderRadius: AppRadius.brSmall,
                ),
                child: Icon(
                  _typeIcon(type),
                  color: _typeColor(context, type),
                  size: 21,
                ),
              ),
              SizedBox(width: AppSpacing.md),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(_nodeLabel(node), style: AppTypography.cardTitle(context)),
                    const SizedBox(height: 2),
                    Text(id, style: AppTypography.caption(context)),
                  ],
                ),
              ),
              AmitiaStatusBadge(label: type.isEmpty ? 'node' : type, type: BadgeType.info),
              SizedBox(width: AppSpacing.sm),
              Text('$relationCount 关系', style: AppTypography.label(context)),
            ],
          ),
        );
      },
    );
  }

  Future<void> _showNode(
    BuildContext context,
    Map<String, dynamic> node,
  ) async {
    final id = _nodeId(node);
    final type = _string(node, const ['entity_type', 'entityType']);
    final properties = node['properties'] is Map
        ? Map<String, dynamic>.from(node['properties'] as Map)
        : <String, dynamic>{};

    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      builder: (sheetContext) => DraggableScrollableSheet(
        expand: false,
        initialChildSize: 0.64,
        minChildSize: 0.35,
        maxChildSize: 0.92,
        builder: (context, controller) => ListView(
          controller: controller,
          padding: EdgeInsets.all(AppSpacing.pagePadding),
          children: [
            Text(_nodeLabel(node), style: AppTypography.sectionTitle(context)),
            SizedBox(height: AppSpacing.xs),
            Text(id, style: AppTypography.caption(context)),
            SizedBox(height: AppSpacing.md),
            Wrap(
              spacing: AppSpacing.sm,
              children: [
                AmitiaStatusBadge(label: type.isEmpty ? 'node' : type, type: BadgeType.info),
              ],
            ),
            SizedBox(height: AppSpacing.lg),
            const AmitiaSectionHeader(title: '节点属性'),
            SizedBox(height: AppSpacing.sm),
            if (properties.isEmpty)
              Text('无附加属性', style: AppTypography.caption(context))
            else
              ...properties.entries.map(
                (entry) => Padding(
                  padding: const EdgeInsets.symmetric(vertical: 5),
                  child: Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      SizedBox(
                        width: 120,
                        child: Text(entry.key, style: AppTypography.label(context)),
                      ),
                      Expanded(
                        child: Text(
                          entry.value?.toString() ?? '',
                          style: AppTypography.bodySmall(context),
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            SizedBox(height: AppSpacing.lg),
            AmitiaButton(
              label: '查询邻居节点',
              isSecondary: true,
              icon: Icons.account_tree_outlined,
              onPressed: () async {
                try {
                  final result = await ref
                      .read(systemServiceProvider)
                      .graphNeighbors(id, depth: 2);
                  if (!sheetContext.mounted) return;
                  await showDialog<void>(
                    context: sheetContext,
                    builder: (dialogContext) => AlertDialog(
                      title: const Text('邻居查询结果'),
                      content: SizedBox(
                        width: double.maxFinite,
                        child: SingleChildScrollView(
                          child: SelectableText(
                            result?.toString() ?? '没有邻居节点',
                            style: AppTypography.bodySmall(dialogContext),
                          ),
                        ),
                      ),
                      actions: [
                        TextButton(
                          onPressed: () => Navigator.pop(dialogContext),
                          child: const Text('关闭'),
                        ),
                      ],
                    ),
                  );
                } catch (e) {
                  if (sheetContext.mounted) {
                    ScaffoldMessenger.of(sheetContext).showSnackBar(
                      SnackBar(content: Text('查询邻居失败: $e')),
                    );
                  }
                }
              },
            ),
          ],
        ),
      ),
    );
  }

  List<Offset> _positions(int count) {
    if (count <= 0) return const [];
    const center = Offset(450, 360);
    final result = <Offset>[];
    for (var i = 0; i < count; i++) {
      final ring = 1 + i ~/ 18;
      final inRing = i % 18;
      final angle = 2 * math.pi * inRing / math.min(18, count);
      final radius = 115.0 + ring * 90.0;
      result.add(
        Offset(
          center.dx + math.cos(angle) * radius,
          center.dy + math.sin(angle) * radius,
        ),
      );
    }
    return result;
  }

  String _nodeId(Map<String, dynamic> node) => _recordId(node['id']);

  String _recordId(dynamic value) {
    if (value == null) return '';
    if (value is String) {
      return value
          .replaceFirst('entity_node:', '')
          .replaceAll('⟨', '')
          .replaceAll('⟩', '')
          .replaceAll('`', '');
    }
    if (value is Map) {
      final table = value['tb'] ?? value['table'];
      final id = value['id'];
      if (table != null && id != null) return '$table:$id'.replaceFirst('entity_node:', '');
      if (id != null) return id.toString();
    }
    return value.toString()
        .replaceFirst('entity_node:', '')
        .replaceAll('`', '');
  }

  String _nodeLabel(Map<String, dynamic> node) {
    final label = _string(node, const ['label', 'name']);
    return label.isEmpty ? _nodeId(node) : label;
  }

  String _string(Map<String, dynamic> value, List<String> keys) {
    for (final key in keys) {
      final raw = value[key];
      if (raw != null && raw.toString().trim().isNotEmpty) return raw.toString();
    }
    return '';
  }

  Color _typeColor(BuildContext context, String type) {
    switch (type) {
      case 'memory':
        return context.accentPrimary;
      case 'character':
        return context.success;
      case 'user':
        return context.info;
      case 'episodic':
        return context.warning;
      case 'worldbook':
        return context.accentSecondary;
      default:
        return context.textSecondary;
    }
  }

  IconData _typeIcon(String type) {
    switch (type) {
      case 'memory':
        return Icons.memory;
      case 'character':
        return Icons.face_outlined;
      case 'user':
        return Icons.person_outline;
      case 'episodic':
        return Icons.auto_stories_outlined;
      case 'worldbook':
        return Icons.menu_book_outlined;
      default:
        return Icons.circle_outlined;
    }
  }
}

class _Stat extends StatelessWidget {
  final String label;
  final String value;

  const _Stat({required this.label, required this.value});

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(label, style: AppTypography.caption(context)),
        const SizedBox(width: 5),
        Text(
          value,
          style: AppTypography.cardTitle(context).copyWith(
            color: context.accentPrimary,
          ),
        ),
      ],
    );
  }
}

class _GraphData {
  final Map<String, dynamic> stats;
  final List<Map<String, dynamic>> nodes;
  final List<Map<String, dynamic>> edges;

  const _GraphData({
    required this.stats,
    required this.nodes,
    required this.edges,
  });
}

final _graphProvider = FutureProvider.autoDispose<_GraphData>((ref) async {
  final service = ref.read(systemServiceProvider);
  final results = await Future.wait([
    service.graphStats(),
    service.graphNodes(),
    service.graphEdges(),
  ]);
  return _GraphData(
    stats: results[0] is Map
        ? Map<String, dynamic>.from(results[0] as Map)
        : const <String, dynamic>{},
    nodes: (results[1] as List<Map<String, dynamic>>),
    edges: (results[2] as List<Map<String, dynamic>>),
  );
});

class _GraphPainter extends CustomPainter {
  final List<Map<String, dynamic>> nodes;
  final List<Map<String, dynamic>> edges;
  final Color lineColor;
  final Color nodeColor;

  _GraphPainter({
    required this.nodes,
    required this.edges,
    required this.lineColor,
    required this.nodeColor,
  });

  @override
  void paint(Canvas canvas, Size size) {
    if (nodes.isEmpty) return;
    const center = Offset(450, 360);
    final positions = <String, Offset>{};
    for (var i = 0; i < nodes.length; i++) {
      final ring = 1 + i ~/ 18;
      final inRing = i % 18;
      final angle = 2 * math.pi * inRing / math.min(18, nodes.length);
      final radius = 115.0 + ring * 90.0;
      final id = _id(nodes[i]['id']);
      positions[id] = Offset(
        center.dx + math.cos(angle) * radius,
        center.dy + math.sin(angle) * radius,
      );
    }

    final linePaint = Paint()
      ..color = lineColor
      ..strokeWidth = 1.2;
    for (final edge in edges) {
      final from = positions[_id(edge['in'])];
      final to = positions[_id(edge['out'])];
      if (from != null && to != null) canvas.drawLine(from, to, linePaint);
    }

    final pointPaint = Paint()..color = nodeColor.withValues(alpha: 0.3);
    for (final point in positions.values) {
      canvas.drawCircle(point, 5, pointPaint);
    }
  }

  static String _id(dynamic value) {
    if (value == null) return '';
    if (value is String) {
      return value
          .replaceFirst('entity_node:', '')
          .replaceAll('⟨', '')
          .replaceAll('⟩', '')
          .replaceAll('`', '');
    }
    if (value is Map) {
      final id = value['id'];
      return id?.toString() ?? value.toString();
    }
    return value.toString();
  }

  @override
  bool shouldRepaint(covariant _GraphPainter oldDelegate) {
    return oldDelegate.nodes != nodes ||
        oldDelegate.edges != edges ||
        oldDelegate.lineColor != lineColor ||
        oldDelegate.nodeColor != nodeColor;
  }
}

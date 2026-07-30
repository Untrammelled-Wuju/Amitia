import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../shared/models/models.dart';
import '../../../../shared/mock_data/mock_data.dart';

class MemoryGraphPage extends ConsumerStatefulWidget {
  const MemoryGraphPage({super.key});

  @override
  ConsumerState<MemoryGraphPage> createState() => _MemoryGraphPageState();
}

class _MemoryGraphPageState extends ConsumerState<MemoryGraphPage> {
  final List<MemoryGraphNode> _allNodes = MockMemory.graphNodes;
  final List<MemoryGraphEdge> _allEdges = MockMemory.graphEdges;
  double _zoom = 1.0;
  String _nodeTypeFilter = '全部';
  String _characterFilter = '全部';
  String _searchQuery = '';
  MemoryGraphNode? _selectedNode;
  final _searchController = TextEditingController();

  final _nodeTypes = ['全部', '角色', '实体', '记忆', '关系', '地点'];
  final _characters = ['全部', '阿米娅', '小雨', '用户'];

  List<MemoryGraphNode> get _filteredNodes {
    return _allNodes.where((n) {
      if (_nodeTypeFilter != '全部' && n.type != _nodeTypeFilter) return false;
      if (_characterFilter != '全部' && !n.label.contains(_characterFilter == '阿米娅' ? '阿米娅' : _characterFilter == '小雨' ? '小雨' : _characterFilter)) return false;
      if (_searchQuery.isNotEmpty && !n.label.toLowerCase().contains(_searchQuery.toLowerCase())) return false;
      return true;
    }).toList();
  }

  Set<String> get _visibleNodeIds => _filteredNodes.map((n) => n.id).toSet();

  List<MemoryGraphEdge> get _visibleEdges {
    return _allEdges.where((e) => _visibleNodeIds.contains(e.source) && _visibleNodeIds.contains(e.target)).toList();
  }

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '记忆图谱',
        showBackButton: true,
        actions: [
          AmitiaIconButton(
            icon: Icons.refresh,
            tooltip: '重置视图',
            onPressed: () {
              setState(() {
                _zoom = 1.0;
                _nodeTypeFilter = '全部';
                _characterFilter = '全部';
                _searchQuery = '';
                _searchController.clear();
                _selectedNode = null;
              });
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(content: Text('视图已重置'), duration: Duration(seconds: 1)),
              );
            },
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: Column(
          children: [
            _buildSearchBar(context),
            _buildFilterChips(context),
            Expanded(child: _buildGraphCanvas(context)),
            _buildZoomControl(context),
          ],
        ),
      ),
    );
  }

  Widget _buildSearchBar(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.sm, AppSpacing.pagePadding, AppSpacing.xs),
      child: AmitiaSearchField(
        hintText: '搜索节点...',
        controller: _searchController,
        onChanged: (v) => setState(() => _searchQuery = v),
      ),
    );
  }

  Widget _buildFilterChips(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.xs),
      child: SingleChildScrollView(
        scrollDirection: Axis.horizontal,
        child: Row(
          children: [
            _buildFilterPopup(context, '节点类型', _nodeTypeFilter, _nodeTypes, (v) => setState(() => _nodeTypeFilter = v)),
            const SizedBox(width: AppSpacing.sm),
            _buildFilterPopup(context, '角色', _characterFilter, _characters, (v) => setState(() => _characterFilter = v)),
          ],
        ),
      ),
    );
  }

  Widget _buildFilterPopup(BuildContext context, String label, String current, List<String> options, ValueChanged<String> onSelected) {
    return GestureDetector(
      onTap: () => showModalBottomSheet(
        context: context,
        builder: (ctx) => Container(
          padding: const EdgeInsets.all(AppSpacing.xl),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(label, style: AppTypography.sectionTitle(context)),
              const SizedBox(height: AppSpacing.md),
              Wrap(
                spacing: AppSpacing.sm,
                runSpacing: AppSpacing.sm,
                children: options.map((o) => GestureDetector(
                  onTap: () { onSelected(o); Navigator.pop(ctx); },
                  child: Container(
                    padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
                    decoration: BoxDecoration(
                      color: o == current ? context.accentPrimary : context.accentSoft,
                      borderRadius: AppRadius.brTag,
                    ),
                    child: Text(o, style: TextStyle(fontSize: 14, color: o == current ? Colors.white : context.accentPrimary)),
                  ),
                )).toList(),
              ),
              const SizedBox(height: AppSpacing.xl),
            ],
          ),
        ),
      ),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
        decoration: BoxDecoration(
          color: context.surfaceSecondary,
          borderRadius: AppRadius.brTag,
          border: Border.all(color: context.borderPrimary, width: 0.5),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text('$label: $current', style: TextStyle(fontSize: 12, color: context.textSecondary)),
            const SizedBox(width: 4),
            Icon(Icons.arrow_drop_down, size: 16, color: context.textTertiary),
          ],
        ),
      ),
    );
  }

  Widget _buildGraphCanvas(BuildContext context) {
    return Container(
      margin: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      decoration: BoxDecoration(
        color: context.surfaceSecondary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: ClipRRect(
        borderRadius: AppRadius.brMedium,
        child: InteractiveViewer(
          boundaryMargin: const EdgeInsets.all(50),
          minScale: 0.5,
          maxScale: 3.0,
          child: SizedBox(
            width: 400 * _zoom,
            height: 400 * _zoom,
            child: CustomPaint(
              painter: _GraphPainter(
                nodes: _filteredNodes,
                edges: _visibleEdges,
                selectedNodeId: _selectedNode?.id,
                accentColor: context.accentPrimary,
                accentSoftColor: context.accentSoft,
                textPrimary: context.textPrimary,
                textSecondary: context.textSecondary,
                borderPrimary: context.borderPrimary,
                successColor: context.success,
                infoColor: context.info,
                warningColor: context.warning,
                errorColor: context.error,
              ),
              child: Stack(
                children: _filteredNodes.map((node) {
                  return Positioned(
                    left: node.x * 400 * _zoom - 30,
                    top: node.y * 400 * _zoom - 30,
                    child: GestureDetector(
                      onTap: () {
                        setState(() {
                          _selectedNode = node;
                        });
                        _showNodeDetailSheet(context, node);
                      },
                      child: Container(
                        width: 60,
                        height: 60,
                        color: Colors.transparent,
                      ),
                    ),
                  );
                }).toList(),
              ),
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildZoomControl(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.all(AppSpacing.pagePadding),
      child: Row(
        children: [
          AmitiaIconButton(
            icon: Icons.remove_circle_outline,
            color: context.accentPrimary,
            onPressed: () => setState(() => _zoom = (_zoom - 0.2).clamp(0.5, 2.0)),
          ),
          Expanded(
            child: Slider(
              value: _zoom,
              min: 0.5,
              max: 2.0,
              divisions: 15,
              activeColor: context.accentPrimary,
              onChanged: (v) => setState(() => _zoom = v),
            ),
          ),
          AmitiaIconButton(
            icon: Icons.add_circle_outline,
            color: context.accentPrimary,
            onPressed: () => setState(() => _zoom = (_zoom + 0.2).clamp(0.5, 2.0)),
          ),
          const SizedBox(width: AppSpacing.sm),
          Text('${(_zoom * 100).round()}%', style: AppTypography.label(context).copyWith(color: context.accentPrimary, fontWeight: FontWeight.w600)),
        ],
      ),
    );
  }

  void _showNodeDetailSheet(BuildContext context, MemoryGraphNode node) {
    final relatedEdges = _allEdges.where((e) => e.source == node.id || e.target == node.id).toList();
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      builder: (ctx) => DraggableScrollableSheet(
        initialChildSize: 0.6,
        maxChildSize: 0.9,
        minChildSize: 0.4,
        expand: false,
        builder: (ctx, controller) => Container(
          padding: const EdgeInsets.all(AppSpacing.xl),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Center(child: Container(width: 40, height: 4, decoration: BoxDecoration(color: context.borderPrimary, borderRadius: BorderRadius.circular(2)))),
              const SizedBox(height: AppSpacing.lg),
              Row(
                children: [
                  Container(
                    width: 48,
                    height: 48,
                    decoration: BoxDecoration(
                      color: _getTypeColor(context, node.type).withValues(alpha: 0.12),
                      shape: BoxShape.circle,
                    ),
                    child: Icon(_getTypeIcon(node.type), size: 24, color: _getTypeColor(context, node.type)),
                  ),
                  const SizedBox(width: AppSpacing.md),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(node.label, style: AppTypography.sectionTitle(context)),
                        const SizedBox(height: 2),
                        Row(
                          children: [
                            AmitiaStatusBadge(label: node.type, type: _getTypeBadge(node.type)),
                            const SizedBox(width: AppSpacing.sm),
                            Text(node.category, style: AppTypography.label(context)),
                          ],
                        ),
                      ],
                    ),
                  ),
                ],
              ),
              const SizedBox(height: AppSpacing.lg),
              Text('关联关系 (${relatedEdges.length})', style: AppTypography.cardTitle(context)),
              const SizedBox(height: AppSpacing.sm),
              Expanded(
                child: ListView.separated(
                  controller: controller,
                  itemCount: relatedEdges.length,
                  separatorBuilder: (_, _) => const Divider(height: 1),
                  itemBuilder: (context, index) {
                    final edge = relatedEdges[index];
                    final isSource = edge.source == node.id;
                    final otherNodeId = isSource ? edge.target : edge.source;
                    final otherNode = _allNodes.firstWhere((n) => n.id == otherNodeId, orElse: () => _allNodes.first);
                    return ListTile(
                      contentPadding: EdgeInsets.zero,
                      leading: Icon(Icons.link, size: 18, color: context.accentPrimary),
                      title: Text(otherNode.label, style: AppTypography.bodySmall(context)),
                      subtitle: Text(isSource ? '→ ${edge.relation} →' : '← ${edge.relation} ←', style: AppTypography.label(context)),
                      trailing: AmitiaStatusBadge(label: otherNode.type, type: _getTypeBadge(otherNode.type)),
                      onTap: () {
                        Navigator.pop(ctx);
                        setState(() => _selectedNode = otherNode);
                        _showNodeDetailSheet(context, otherNode);
                      },
                    );
                  },
                ),
              ),
              const SizedBox(height: AppSpacing.lg),
              AmitiaButton(
                label: '关闭',
                isFullWidth: true,
                isSecondary: true,
                onPressed: () => Navigator.pop(ctx),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Color _getTypeColor(BuildContext context, String type) {
    switch (type) {
      case '角色': return context.accentPrimary;
      case '实体': return context.info;
      case '记忆': return context.success;
      case '关系': return context.warning;
      case '地点': return context.error;
      default: return context.textSecondary;
    }
  }

  IconData _getTypeIcon(String type) {
    switch (type) {
      case '角色': return Icons.person;
      case '实体': return Icons.category;
      case '记忆': return Icons.memory;
      case '关系': return Icons.link;
      case '地点': return Icons.location_on;
      default: return Icons.circle;
    }
  }

  BadgeType _getTypeBadge(String type) {
    switch (type) {
      case '角色': return BadgeType.accent;
      case '实体': return BadgeType.info;
      case '记忆': return BadgeType.success;
      case '关系': return BadgeType.warning;
      case '地点': return BadgeType.error;
      default: return BadgeType.neutral;
    }
  }
}

class _GraphPainter extends CustomPainter {
  final List<MemoryGraphNode> nodes;
  final List<MemoryGraphEdge> edges;
  final String? selectedNodeId;
  final Color accentColor;
  final Color accentSoftColor;
  final Color textPrimary;
  final Color textSecondary;
  final Color borderPrimary;
  final Color successColor;
  final Color infoColor;
  final Color warningColor;
  final Color errorColor;

  _GraphPainter({
    required this.nodes,
    required this.edges,
    this.selectedNodeId,
    required this.accentColor,
    required this.accentSoftColor,
    required this.textPrimary,
    required this.textSecondary,
    required this.borderPrimary,
    required this.successColor,
    required this.infoColor,
    required this.warningColor,
    required this.errorColor,
  });

  Color _nodeColor(MemoryGraphNode node) {
    switch (node.type) {
      case '角色': return accentColor;
      case '实体': return infoColor;
      case '记忆': return successColor;
      case '关系': return warningColor;
      case '地点': return errorColor;
      default: return textSecondary;
    }
  }

  @override
  void paint(Canvas canvas, Size size) {
    final edgePaint = Paint()
      ..color = borderPrimary
      ..strokeWidth = 1.5
      ..style = PaintingStyle.stroke;

    for (final edge in edges) {
      final source = nodes.firstWhere((n) => n.id == edge.source, orElse: () => nodes.first);
      final target = nodes.firstWhere((n) => n.id == edge.target, orElse: () => nodes.first);
      final startX = source.x * size.width;
      final startY = source.y * size.height;
      final endX = target.x * size.width;
      final endY = target.y * size.height;
      canvas.drawLine(Offset(startX, startY), Offset(endX, endY), edgePaint);
      final midX = (startX + endX) / 2;
      final midY = (startY + endY) / 2;
      final relationPainter = TextPainter(
        text: TextSpan(text: edge.relation, style: TextStyle(fontSize: 9, color: textSecondary)),
        textDirection: TextDirection.ltr,
      );
      relationPainter.layout();
      relationPainter.paint(canvas, Offset(midX - relationPainter.width / 2, midY - relationPainter.height / 2));
    }

    for (final node in nodes) {
      final cx = node.x * size.width;
      final cy = node.y * size.height;
      final isSelected = node.id == selectedNodeId;
      final radius = isSelected ? 26.0 : 22.0;
      final color = _nodeColor(node);

      final nodePaint = Paint()
        ..color = color.withValues(alpha: 0.15)
        ..style = PaintingStyle.fill;
      canvas.drawCircle(Offset(cx, cy), radius, nodePaint);

      final borderPaint = Paint()
        ..color = color
        ..strokeWidth = isSelected ? 3 : 2
        ..style = PaintingStyle.stroke;
      canvas.drawCircle(Offset(cx, cy), radius, borderPaint);

      final labelPainter = TextPainter(
        text: TextSpan(
          text: node.label,
          style: TextStyle(fontSize: 10, color: textPrimary, fontWeight: FontWeight.w600),
        ),
        textDirection: TextDirection.ltr,
        textAlign: TextAlign.center,
      );
      labelPainter.layout(maxWidth: 70);
      labelPainter.paint(canvas, Offset(cx - labelPainter.width / 2, cy + radius + 4));
    }
  }

  @override
  bool shouldRepaint(covariant _GraphPainter oldDelegate) {
    return oldDelegate.selectedNodeId != selectedNodeId ||
        oldDelegate.nodes != nodes ||
        oldDelegate.edges != edges;
  }
}

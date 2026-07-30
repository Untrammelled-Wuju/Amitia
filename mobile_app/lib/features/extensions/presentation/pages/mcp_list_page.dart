import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/app_routes.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../shared/models/models.dart';
import '../../../../shared/mock_data/mock_extensions.dart';

class McpListPage extends ConsumerStatefulWidget {
  const McpListPage({super.key});

  @override
  ConsumerState<McpListPage> createState() => _McpListPageState();
}

class _McpListPageState extends ConsumerState<McpListPage> {
  late List<McpServer> _servers;

  @override
  void initState() {
    super.initState();
    _servers = List.from(MockExtensions.mcpServers);
  }

  String _transportLabel(McpTransport transport) {
    switch (transport) {
      case McpTransport.stdio:
        return 'STDIO';
      case McpTransport.sse:
        return 'SSE';
      case McpTransport.websocket:
        return 'WebSocket';
    }
  }

  String _statusLabel(McpStatus status) {
    switch (status) {
      case McpStatus.connected:
        return '已连接';
      case McpStatus.disconnected:
        return '未连接';
      case McpStatus.error:
        return '错误';
      case McpStatus.connecting:
        return '连接中';
    }
  }

  BadgeType _statusBadgeType(McpStatus status) {
    switch (status) {
      case McpStatus.connected:
        return BadgeType.success;
      case McpStatus.disconnected:
        return BadgeType.neutral;
      case McpStatus.error:
        return BadgeType.error;
      case McpStatus.connecting:
        return BadgeType.warning;
    }
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: 'MCP 服务',
        showBackButton: true,
      ),
      body: SafeArea(
        top: false,
        child: ListView.separated(
          padding: const EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.sm, AppSpacing.pagePadding, AppSpacing.xxxl),
          itemCount: _servers.length,
          separatorBuilder: (_, _) => const SizedBox(height: AppSpacing.sm),
          itemBuilder: (context, index) => _buildServerCard(context, _servers[index]),
        ),
      ),
      floatingActionButton: FloatingActionButton(
        onPressed: _showAddServerSheet,
        backgroundColor: context.accentPrimary,
        child: const Icon(Icons.add, color: Colors.white),
      ),
    );
  }

  Widget _buildServerCard(BuildContext context, McpServer server) {
    return AmitiaCard(
      onTap: () => context.push(AppRoutes.mcpDetail(server.id)),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                width: 44,
                height: 44,
                decoration: BoxDecoration(
                  color: context.accentSoft,
                  borderRadius: AppRadius.brSmall,
                ),
                child: Icon(Icons.dns_outlined, size: 22, color: context.accentPrimary),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(server.name, style: AppTypography.cardTitle(context)),
                    const SizedBox(height: 4),
                    Row(
                      children: [
                        Container(
                          padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                          decoration: BoxDecoration(
                            color: context.surfaceSecondary,
                            borderRadius: AppRadius.brTag,
                          ),
                          child: Text(_transportLabel(server.transport), style: TextStyle(fontSize: 10, color: context.textTertiary, fontWeight: FontWeight.w600)),
                        ),
                        const SizedBox(width: 8),
                        Expanded(child: Text(server.address, style: AppTypography.label(context), maxLines: 1, overflow: TextOverflow.ellipsis)),
                      ],
                    ),
                  ],
                ),
              ),
              AmitiaStatusBadge(label: _statusLabel(server.status), type: _statusBadgeType(server.status)),
            ],
          ),
          const SizedBox(height: AppSpacing.md),
          Wrap(
            spacing: 6,
            runSpacing: 6,
            children: [
              _CapabilityTag(label: '工具 ${server.toolCount}', icon: Icons.build_outlined, color: context.accentPrimary),
              _CapabilityTag(label: 'Prompt ${server.promptCount}', icon: Icons.chat_outlined, color: context.info),
              _CapabilityTag(label: 'Resource ${server.resourceCount}', icon: Icons.folder_outlined, color: context.success),
              if (server.hasSampling)
                _CapabilityTag(label: 'Sampling', icon: Icons.graphic_eq, color: context.warning),
              if (server.hasTasks)
                _CapabilityTag(label: 'Tasks', icon: Icons.task_outlined, color: context.accentSecondary),
              if (server.hasRoots)
                _CapabilityTag(label: 'Roots', icon: Icons.account_tree_outlined, color: context.info),
              if (server.hasOAuth)
                _CapabilityTag(label: 'OAuth', icon: Icons.lock_outline, color: context.error),
            ],
          ),
          const SizedBox(height: AppSpacing.md),
          Row(
            children: [
              GestureDetector(
                onTap: () => context.push(AppRoutes.mcpEdit(server.id)),
                child: Container(
                  padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 7),
                  decoration: BoxDecoration(
                    color: context.accentSoft,
                    borderRadius: AppRadius.brTag,
                  ),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(Icons.edit_outlined, size: 15, color: context.accentPrimary),
                      const SizedBox(width: 5),
                      Text('编辑', style: TextStyle(fontSize: 13, color: context.accentPrimary, fontWeight: FontWeight.w500)),
                    ],
                  ),
                ),
              ),
              const SizedBox(width: 8),
              GestureDetector(
                onTap: () => _showDeleteConfirm(server),
                child: Container(
                  padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 7),
                  decoration: BoxDecoration(
                    color: context.error.withValues(alpha: 0.1),
                    borderRadius: AppRadius.brTag,
                  ),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(Icons.delete_outline, size: 15, color: context.error),
                      const SizedBox(width: 5),
                      Text('删除', style: TextStyle(fontSize: 13, color: context.error, fontWeight: FontWeight.w500)),
                    ],
                  ),
                ),
              ),
              const Spacer(),
              Icon(Icons.chevron_right, color: context.textTertiary, size: 20),
            ],
          ),
        ],
      ),
    );
  }

  void _showAddServerSheet() {
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(22))),
      builder: (context) => _AddServerSheet(onConfirm: () {
        Navigator.pop(context);
        ScaffoldMessenger.of(this.context).showSnackBar(
          SnackBar(content: const Text('MCP 服务添加成功'), backgroundColor: context.success),
        );
      }),
    );
  }

  void _showDeleteConfirm(McpServer server) {
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        backgroundColor: context.surfacePrimary,
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
        title: Text('删除 MCP 服务', style: AppTypography.cardTitle(context)),
        content: Text('确定要删除「${server.name}」吗？此操作不可撤销，相关配置将被清除。', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context), child: Text('取消', style: TextStyle(color: context.textSecondary))),
          TextButton(
            onPressed: () {
              Navigator.pop(context);
              setState(() {
                _servers.removeWhere((s) => s.id == server.id);
              });
              ScaffoldMessenger.of(this.context).showSnackBar(
                SnackBar(content: Text('${server.name} 已删除'), backgroundColor: context.error),
              );
            },
            child: Text('删除', style: TextStyle(color: context.error)),
          ),
        ],
      ),
    );
  }
}

class _CapabilityTag extends StatelessWidget {
  final String label;
  final IconData icon;
  final Color color;

  const _CapabilityTag({required this.label, required this.icon, required this.color});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.1),
        borderRadius: AppRadius.brTag,
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 13, color: color),
          const SizedBox(width: 4),
          Text(label, style: TextStyle(fontSize: 11, color: color, fontWeight: FontWeight.w500)),
        ],
      ),
    );
  }
}

class _AddServerSheet extends StatefulWidget {
  final VoidCallback onConfirm;

  const _AddServerSheet({required this.onConfirm});

  @override
  State<_AddServerSheet> createState() => _AddServerSheetState();
}

class _AddServerSheetState extends State<_AddServerSheet> {
  int _transportIndex = 0;
  final _nameController = TextEditingController();
  final _addressController = TextEditingController();

  @override
  void dispose() {
    _nameController.dispose();
    _addressController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final transports = ['STDIO', 'SSE', 'WebSocket'];
    return Padding(
      padding: EdgeInsets.fromLTRB(20, 12, 20, 34 + MediaQuery.of(context).viewInsets.bottom),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Center(
            child: Container(width: 40, height: 4, decoration: BoxDecoration(color: context.borderPrimary, borderRadius: BorderRadius.circular(2))),
          ),
          const SizedBox(height: 20),
          Text('添加 MCP 服务', style: AppTypography.pageTitle(context)),
          const SizedBox(height: 20),
          Text('服务名称', style: AppTypography.caption(context)),
          const SizedBox(height: 6),
          AmitiaTextField(hintText: '输入服务名称', controller: _nameController),
          const SizedBox(height: 16),
          Text('Transport 类型', style: AppTypography.caption(context)),
          const SizedBox(height: 6),
          AmitiaSegmentedControl(
            segments: transports,
            selectedIndex: _transportIndex,
            onChanged: (i) => setState(() => _transportIndex = i),
          ),
          const SizedBox(height: 16),
          Text(_transportIndex == 0 ? '命令' : '地址', style: AppTypography.caption(context)),
          const SizedBox(height: 6),
          AmitiaTextField(
            hintText: _transportIndex == 0 ? '输入启动命令' : '输入服务地址',
            controller: _addressController,
          ),
          const SizedBox(height: 20),
          Row(
            children: [
              Expanded(
                child: AmitiaButton(label: '取消', isSecondary: true, onPressed: () => Navigator.pop(context)),
              ),
              const SizedBox(width: AppSpacing.sm),
              Expanded(
                child: AmitiaButton(label: '添加', onPressed: widget.onConfirm),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

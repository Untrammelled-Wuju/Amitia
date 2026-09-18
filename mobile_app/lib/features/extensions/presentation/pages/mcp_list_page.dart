import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/app_routes.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/services/providers.dart';

class McpListPage extends ConsumerStatefulWidget {
  const McpListPage({super.key});

  @override
  ConsumerState<McpListPage> createState() => _McpListPageState();
}

class _McpListPageState extends ConsumerState<McpListPage> {
  List<Map<String, dynamic>> _servers = [];
  Map<String, Set<String>> _enabledCapabilities = const <String, Set<String>>{};
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _loadServers();
  }

  Future<void> _loadServers() async {
    setState(() { _loading = true; _error = null; });
    try {
      final svc = ref.read(mcpServiceProvider);
      final data = await svc.servers();
      final capabilityEntries = await Future.wait(
        data.map((server) async {
          final id = (server['id'] ?? '').toString();
          if (id.isEmpty) return MapEntry(id, <String>{});
          try {
            final items = await svc.capabilities(id);
            final enabled = items
                .where((item) => _asBool(item['enabled']))
                .map((item) => (item['capability'] ?? '').toString())
                .where((name) => name.isNotEmpty)
                .toSet();
            return MapEntry(id, enabled);
          } catch (_) {
            return MapEntry(id, <String>{});
          }
        }),
      );
      if (mounted) {
        setState(() {
          _servers = data;
          _enabledCapabilities = Map<String, Set<String>>.fromEntries(capabilityEntries);
          _loading = false;
        });
      }
    } catch (e) {
      if (mounted) setState(() { _error = e.toString(); _loading = false; });
    }
  }

  String _transportLabel(dynamic transport) {
    final t = transport.toString().toLowerCase();
    if (t.contains('stdio')) return 'STDIO';
    if (t.contains('streamable_http')) return 'HTTP';
    return transport.toString();
  }

  String _statusLabel(dynamic status) {
    final s = status.toString().toLowerCase();
    if (s.contains('connected') && !s.contains('dis')) return '已连接';
    if (s.contains('disconnected') || s.contains('disconnect')) return '未连接';
    if (s.contains('error')) return '错误';
    if (s.contains('connecting')) return '连接中';
    return status.toString();
  }

  BadgeType _statusBadgeType(dynamic status) {
    final s = status.toString().toLowerCase();
    if (s.contains('connected') && !s.contains('dis')) return BadgeType.success;
    if (s.contains('disconnected') || s.contains('disconnect')) return BadgeType.neutral;
    if (s.contains('error')) return BadgeType.error;
    if (s.contains('connecting')) return BadgeType.warning;
    return BadgeType.neutral;
  }

  static bool _asBool(dynamic value) {
    if (value is bool) return value;
    if (value is num) return value != 0;
    return value.toString().toLowerCase() == 'true';
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return AmitiaScaffold(
        appBar: AmitiaAppBar(title: 'MCP 服务', showBackButton: true),
        body: SafeArea(top: false, child: const AmitiaLoadingState(message: '加载中...')),
      );
    }
    if (_error != null) {
      return AmitiaScaffold(
        appBar: AmitiaAppBar(title: 'MCP 服务', showBackButton: true),
        body: SafeArea(top: false, child: AmitiaErrorState(message: '加载失败: $_error', onRetry: _loadServers)),
      );
    }
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: 'MCP 服务',
        showBackButton: true,
      ),
      body: SafeArea(
        top: false,
        child: ListView.separated(
          padding: EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.sm, AppSpacing.pagePadding, AppSpacing.xxxl),
          itemCount: _servers.length,
          separatorBuilder: (_, _) => SizedBox(height: AppSpacing.sm),
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

  Widget _buildServerCard(BuildContext context, Map<String, dynamic> server) {
    final id = (server['id'] ?? '').toString();
    final name = (server['name'] ?? '').toString();
    final transport = server['transport'];
    final address = (server['transport'] ?? '').toString() == 'stdio'
        ? (server['command'] ?? '').toString()
        : (server['endpoint'] ?? '').toString();
    final status = server['status'];
    final enabledCapabilities = _enabledCapabilities[id] ?? const <String>{};
    final hasSampling = enabledCapabilities.contains('sampling');
    final hasTasks = enabledCapabilities.contains('tasks');
    final hasRoots = enabledCapabilities.contains('roots');
    final hasElicitation = enabledCapabilities.contains('elicitation');
    final hasOAuth = (server['authType'] ?? '').toString() == 'oauth';

    return AmitiaCard(
      onTap: () => context.push(AppRoutes.mcpDetail(id)),
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
                    Text(name, style: AppTypography.cardTitle(context)),
                    const SizedBox(height: 4),
                    Row(
                      children: [
                        Container(
                          padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                          decoration: BoxDecoration(
                            color: context.surfaceSecondary,
                            borderRadius: AppRadius.brTag,
                          ),
                          child: Text(_transportLabel(transport), style: TextStyle(fontSize: 10, color: context.textTertiary, fontWeight: FontWeight.w600)),
                        ),
                        const SizedBox(width: 8),
                        Expanded(child: Text(address, style: AppTypography.label(context), maxLines: 1, overflow: TextOverflow.ellipsis)),
                      ],
                    ),
                  ],
                ),
              ),
              AmitiaStatusBadge(label: _statusLabel(status), type: _statusBadgeType(status)),
            ],
          ),
          SizedBox(height: AppSpacing.md),
          Wrap(
            spacing: 6,
            runSpacing: 6,
            children: [
              if (hasSampling)
                _CapabilityTag(label: 'Sampling', icon: Icons.graphic_eq, color: context.warning),
              if (hasTasks)
                _CapabilityTag(label: 'Tasks', icon: Icons.task_outlined, color: context.accentSecondary),
              if (hasRoots)
                _CapabilityTag(label: 'Roots', icon: Icons.account_tree_outlined, color: context.info),
              if (hasElicitation)
                _CapabilityTag(label: 'Elicitation', icon: Icons.dynamic_form_outlined, color: context.warning),
              if (hasOAuth)
                _CapabilityTag(label: 'OAuth', icon: Icons.lock_outline, color: context.error),
            ],
          ),
          SizedBox(height: AppSpacing.md),
          Row(
            children: [
              GestureDetector(
                onTap: () => context.push(AppRoutes.mcpEdit(id)),
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
    context.push<bool>(AppRoutes.extensionsMcpNew).then((changed) {
      if (changed == true && mounted) _loadServers();
    });
  }

  Future<void> _showDeleteConfirm(Map<String, dynamic> server) async {
    final id = (server['id'] ?? '').toString();
    final name = (server['name'] ?? '').toString();
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        backgroundColor: context.surfacePrimary,
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
        title: Text('删除 MCP 服务', style: AppTypography.cardTitle(context)),
        content: Text('确定要删除「$name」吗？此操作不可撤销，相关配置将被清除。', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context), child: Text('取消', style: TextStyle(color: context.textSecondary))),
          TextButton(
            onPressed: () async {
              Navigator.pop(context);
              try {
                final svc = ref.read(mcpServiceProvider);
                await svc.deleteServer(id);
                _loadServers();
                if (mounted) {
                  ScaffoldMessenger.of(this.context).showSnackBar(
                    SnackBar(content: Text('$name 已删除'), backgroundColor: context.error),
                  );
                }
              } catch (e) {
                if (mounted) {
                  ScaffoldMessenger.of(this.context).showSnackBar(
                    SnackBar(content: Text('删除失败: $e'), backgroundColor: context.error),
                  );
                }
              }
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

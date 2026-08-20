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
import '../../../../core/services/providers.dart';

class McpDetailPage extends ConsumerStatefulWidget {
  final String mcpId;

  const McpDetailPage({super.key, required this.mcpId});

  @override
  ConsumerState<McpDetailPage> createState() => _McpDetailPageState();
}

class _McpDetailPageState extends ConsumerState<McpDetailPage> {
  int _selectedTab = 0;
  Map<String, dynamic>? _server;
  bool _loading = true;
  String? _error;
  late Map<String, bool> _toolEnabledState;
  late Map<String, bool> _capabilityState;

  final _tabs = ['概览', '工具', 'Prompts', 'Resources', 'Tasks', '权限', '日志'];

  @override
  void initState() {
    super.initState();
    _toolEnabledState = {};
    _capabilityState = {};
    _loadServer();
  }

  Future<void> _loadServer() async {
    setState(() { _loading = true; _error = null; });
    try {
      final svc = ref.read(mcpServiceProvider);
      final data = await svc.getServer(widget.mcpId);
      if (data != null) {
        final tools = data['tools'] as List? ?? [];
        for (final tool in tools) {
          final t = tool as Map<String, dynamic>;
          _toolEnabledState[(t['name'] ?? '').toString()] = (t['isEnabled'] as bool?) ?? ((t['enabled'] as int?) == 1);
        }
        _capabilityState = {
          'sampling': (data['hasSampling'] ?? false) as bool,
          'tasks': (data['hasTasks'] ?? false) as bool,
          'roots': (data['hasRoots'] ?? false) as bool,
        };
      }
      if (mounted) setState(() { _server = data; _loading = false; });
    } catch (e) {
      if (mounted) setState(() { _error = e.toString(); _loading = false; });
    }
  }

  String _transportLabel(dynamic transport) {
    final t = transport.toString().toLowerCase();
    if (t.contains('stdio')) return 'STDIO';
    if (t.contains('sse')) return 'SSE';
    if (t.contains('websocket') || t.contains('ws')) return 'WebSocket';
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

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return AmitiaScaffold(
        appBar: AmitiaAppBar(title: 'MCP 详情', showBackButton: true, fallbackRoute: AppRoutes.extensions),
        body: SafeArea(top: false, child: const AmitiaLoadingState(message: '加载中...')),
      );
    }
    if (_error != null || _server == null) {
      return AmitiaScaffold(
        appBar: AmitiaAppBar(title: 'MCP 详情', showBackButton: true, fallbackRoute: AppRoutes.extensions),
        body: SafeArea(top: false, child: AmitiaErrorState(message: _error ?? '未找到该 MCP 服务', onRetry: () {
          context.pop();
        })),
      );
    }

    final server = _server!;
    final name = (server['name'] ?? '').toString();

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: name,
        showBackButton: true,
        fallbackRoute: AppRoutes.extensions,
        actions: [
          AmitiaIconButton(
            icon: Icons.edit_outlined,
            onPressed: () => context.push(AppRoutes.mcpEdit(widget.mcpId)),
            tooltip: '编辑',
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: Column(
          children: [
            _buildTabBar(context),
            Expanded(
              child: SingleChildScrollView(
                padding: EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.sm, AppSpacing.pagePadding, AppSpacing.xxxl),
                child: _buildTabContent(context, server),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildTabBar(BuildContext context) {
    return Container(
      height: 44,
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      child: ListView.separated(
        scrollDirection: Axis.horizontal,
        itemCount: _tabs.length,
        separatorBuilder: (_, _) => SizedBox(width: AppSpacing.sm),
        itemBuilder: (context, index) {
          final isSelected = _selectedTab == index;
          return GestureDetector(
            onTap: () => setState(() => _selectedTab = index),
            child: Container(
              padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
              decoration: BoxDecoration(
                color: isSelected ? context.accentPrimary : context.surfaceSecondary,
                borderRadius: AppRadius.brTag,
              ),
              child: Center(
                child: Text(
                  _tabs[index],
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: isSelected ? FontWeight.w600 : FontWeight.w400,
                    color: isSelected ? Colors.white : context.textSecondary,
                  ),
                ),
              ),
            ),
          );
        },
      ),
    );
  }

  Widget _buildTabContent(BuildContext context, Map<String, dynamic> server) {
    switch (_selectedTab) {
      case 0:
        return _buildOverviewTab(context, server);
      case 1:
        return _buildToolsTab(context, server);
      case 2:
        return _buildPromptsTab(context, server);
      case 3:
        return _buildResourcesTab(context, server);
      case 4:
        return _buildTasksTab(context, server);
      case 5:
        return _buildPermissionsTab(context, server);
      case 6:
        return _buildLogsTab(context, server);
      default:
        return const SizedBox();
    }
  }

  Widget _buildOverviewTab(BuildContext context, Map<String, dynamic> server) {
    final status = server['status'];
    final transport = server['transport'];
    final address = (server['address'] ?? server['url'] ?? '').toString();
    final toolCount = (server['toolCount'] ?? server['tools'] ?? 0) as int;
    final promptCount = (server['promptCount'] ?? server['prompts'] ?? 0) as int;
    final resourceCount = (server['resourceCount'] ?? server['resources'] ?? 0) as int;
    final hasSampling = (server['hasSampling'] ?? false) as bool;
    final hasTasks = (server['hasTasks'] ?? false) as bool;
    final hasRoots = (server['hasRoots'] ?? false) as bool;
    final hasOAuth = (server['hasOAuth'] ?? false) as bool;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Text('连接状态', style: AppTypography.sectionTitle(context)),
                  const Spacer(),
                  AmitiaStatusBadge(label: _statusLabel(status), type: _statusBadgeType(status)),
                ],
              ),
              SizedBox(height: AppSpacing.md),
              _InfoRow(label: 'Transport', value: _transportLabel(transport)),
              _InfoRow(label: '地址/命令', value: address),
              _InfoRow(label: '工具数', value: '$toolCount'),
              _InfoRow(label: 'Prompt 数', value: '$promptCount'),
              _InfoRow(label: 'Resource 数', value: '$resourceCount'),
            ],
          ),
        ),
        SizedBox(height: AppSpacing.sectionGap),
        Text('能力统计', style: AppTypography.sectionTitle(context)),
        SizedBox(height: AppSpacing.md),
        Wrap(
          spacing: 8,
          runSpacing: 8,
          children: [
            _CapabilityChip(label: '工具', count: toolCount, icon: Icons.build_outlined),
            _CapabilityChip(label: 'Prompt', count: promptCount, icon: Icons.chat_outlined),
            _CapabilityChip(label: 'Resource', count: resourceCount, icon: Icons.folder_outlined),
            _CapabilityChip(label: 'Sampling', enabled: hasSampling, icon: Icons.graphic_eq),
            _CapabilityChip(label: 'Tasks', enabled: hasTasks, icon: Icons.task_outlined),
            _CapabilityChip(label: 'Roots', enabled: hasRoots, icon: Icons.account_tree_outlined),
            _CapabilityChip(label: 'OAuth', enabled: hasOAuth, icon: Icons.lock_outline),
          ],
        ),
      ],
    );
  }

  Widget _buildToolsTab(BuildContext context, Map<String, dynamic> server) {
    final tools = (server['tools'] as List?)?.cast<Map<String, dynamic>>() ?? [];
    if (tools.isEmpty) {
      return AmitiaEmptyState(icon: Icons.build_outlined, title: '暂无工具', subtitle: '该 MCP 服务未提供工具');
    }
    return Column(
      children: tools.map((tool) {
        final name = (tool['name'] ?? '').toString();
        final description = (tool['description'] ?? '').toString();
        final isEnabled = _toolEnabledState[name] ?? ((tool['isEnabled'] as bool?) ?? ((tool['enabled'] as int?) == 1));
        return Padding(
          padding: EdgeInsets.only(bottom: AppSpacing.sm),
          child: AmitiaCard(
            child: Row(
              children: [
                Icon(Icons.build_circle_outlined, size: 20, color: context.accentPrimary),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(name, style: AppTypography.bodySmall(context).copyWith(fontWeight: FontWeight.w600)),
                      const SizedBox(height: 2),
                      Text(description, style: AppTypography.caption(context)),
                    ],
                  ),
                ),
                Switch(
                  value: isEnabled,
                  onChanged: (val) => setState(() => _toolEnabledState[name] = val),
                  materialTapTargetSize: MaterialTapTargetSize.shrinkWrap,
                ),
              ],
            ),
          ),
        );
      }).toList(),
    );
  }

  Widget _buildPromptsTab(BuildContext context, Map<String, dynamic> server) {
    final prompts = (server['prompts'] as List?)?.cast<Map<String, dynamic>>() ?? [];
    if (prompts.isEmpty) {
      return AmitiaEmptyState(icon: Icons.chat_outlined, title: '暂无 Prompt', subtitle: '该 MCP 服务未提供 Prompt');
    }
    return Column(
      children: prompts.map((prompt) {
        final name = (prompt['name'] ?? '').toString();
        final description = (prompt['description'] ?? '').toString();
        final content = (prompt['content'] ?? '').toString();
        return Padding(
          padding: EdgeInsets.only(bottom: AppSpacing.sm),
          child: AmitiaCard(
            onTap: () => _showUsePromptDialog(name, description, content),
            child: Row(
              children: [
                Icon(Icons.short_text, size: 20, color: context.info),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(name, style: AppTypography.bodySmall(context).copyWith(fontWeight: FontWeight.w600)),
                      const SizedBox(height: 2),
                      Text(description, style: AppTypography.caption(context)),
                    ],
                  ),
                ),
                Icon(Icons.chevron_right, color: context.textTertiary, size: 20),
              ],
            ),
          ),
        );
      }).toList(),
    );
  }

  Widget _buildResourcesTab(BuildContext context, Map<String, dynamic> server) {
    final resources = (server['resources'] as List?)?.cast<Map<String, dynamic>>() ?? [];
    if (resources.isEmpty) {
      return AmitiaEmptyState(icon: Icons.folder_outlined, title: '暂无 Resource', subtitle: '该 MCP 服务未提供 Resource');
    }
    return Column(
      children: resources.map((resource) {
        final name = (resource['name'] ?? '').toString();
        final uri = (resource['uri'] ?? '').toString();
        final mimeType = (resource['mimeType'] ?? resource['mime_type'] ?? '').toString();
        final content = resource['content']?.toString();
        return Padding(
          padding: EdgeInsets.only(bottom: AppSpacing.sm),
          child: AmitiaCard(
            onTap: () => _showResourceContentDialog(name, uri, mimeType, content),
            child: Row(
              children: [
                Icon(_resourceIcon(mimeType), size: 20, color: context.success),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(name, style: AppTypography.bodySmall(context).copyWith(fontWeight: FontWeight.w600)),
                      const SizedBox(height: 2),
                      Text(uri, style: AppTypography.label(context), maxLines: 1, overflow: TextOverflow.ellipsis),
                      const SizedBox(height: 2),
                      Container(
                        padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                        decoration: BoxDecoration(
                          color: context.surfaceSecondary,
                          borderRadius: AppRadius.brTag,
                        ),
                        child: Text(mimeType, style: TextStyle(fontSize: 10, color: context.textTertiary)),
                      ),
                    ],
                  ),
                ),
                Icon(Icons.chevron_right, color: context.textTertiary, size: 20),
              ],
            ),
          ),
        );
      }).toList(),
    );
  }

  Widget _buildTasksTab(BuildContext context, Map<String, dynamic> server) {
    final hasTasks = (server['hasTasks'] ?? false) as bool;
    final mockTasks = [
      {'id': 'task1', 'name': '文件索引更新', 'status': '运行中', 'progress': 0.65},
      {'id': 'task2', 'name': '目录扫描', 'status': '已完成', 'progress': 1.0},
      {'id': 'task3', 'name': '权限验证', 'status': '等待中', 'progress': 0.0},
    ];
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (!hasTasks)
          AmitiaEmptyState(icon: Icons.task_outlined, title: '不支持 Tasks', subtitle: '该 MCP 服务未启用 Tasks 能力')
        else
          ...mockTasks.map((task) => Padding(
                padding: EdgeInsets.only(bottom: AppSpacing.sm),
                child: AmitiaCard(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          Icon(Icons.task_alt, size: 18, color: context.accentPrimary),
                          const SizedBox(width: 8),
                          Expanded(child: Text(task['name'] as String, style: AppTypography.bodySmall(context).copyWith(fontWeight: FontWeight.w600))),
                          AmitiaStatusBadge(
                            label: task['status'] as String,
                            type: (task['status'] == '已完成')
                                ? BadgeType.success
                                : (task['status'] == '运行中')
                                    ? BadgeType.accent
                                    : BadgeType.warning,
                          ),
                        ],
                      ),
                      SizedBox(height: AppSpacing.sm),
                      AmitiaProgressBar(progress: task['progress'] as double),
                    ],
                  ),
                ),
              )),
      ],
    );
  }

  Widget _buildPermissionsTab(BuildContext context, Map<String, dynamic> server) {
    final transport = server['transport'];
    final hasOAuth = (server['hasOAuth'] ?? false) as bool;
    final isStdio = transport.toString().toLowerCase().contains('stdio');
    final isNetwork = !isStdio;

    final permissions = [
      {'name': '文件系统访问', 'desc': '读写本地文件和目录', 'enabled': true},
      {'name': '网络访问', 'desc': '访问互联网资源', 'enabled': isNetwork},
      {'name': '执行子进程', 'desc': '启动和管理子进程', 'enabled': isStdio},
      {'name': '环境变量读取', 'desc': '读取系统环境变量', 'enabled': true},
    ];
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Icon(Icons.shield_outlined, size: 20, color: context.warning),
                  const SizedBox(width: 8),
                  Text('权限设置', style: AppTypography.sectionTitle(context)),
                ],
              ),
              SizedBox(height: AppSpacing.md),
              ...permissions.map((p) => AmitiaSwitchTile(
                    title: p['name'] as String,
                    subtitle: p['desc'] as String,
                    value: p['enabled'] as bool,
                    onChanged: (val) {
                      ScaffoldMessenger.of(context).showSnackBar(
                        SnackBar(content: Text('${p['name']} 已${val ? '开启' : '关闭'}'), backgroundColor: context.accentPrimary),
                      );
                    },
                  )),
            ],
          ),
        ),
        SizedBox(height: AppSpacing.md),
        if (hasOAuth)
          AmitiaCard(
            onTap: () => _showOAuthDialog(),
            child: Row(
              children: [
                Icon(Icons.lock_outline, size: 20, color: context.error),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text('OAuth 授权', style: AppTypography.bodySmall(context).copyWith(fontWeight: FontWeight.w600)),
                      const SizedBox(height: 2),
                      Text('该服务需要 OAuth 授权', style: AppTypography.caption(context)),
                    ],
                  ),
                ),
                AmitiaStatusBadge(label: '需授权', type: BadgeType.warning),
                const SizedBox(width: 8),
                Icon(Icons.chevron_right, color: context.textTertiary, size: 20),
              ],
            ),
          ),
        SizedBox(height: AppSpacing.md),
        AmitiaButton(
          label: '管理能力配置',
          isFullWidth: true,
          isSecondary: true,
          icon: Icons.tune,
          onPressed: () => _showCapabilityDialog(),
        ),
      ],
    );
  }

  Widget _buildLogsTab(BuildContext context, Map<String, dynamic> server) {
    final toolCount = (server['toolCount'] ?? server['tools'] ?? 0) as int;
    final promptCount = (server['promptCount'] ?? server['prompts'] ?? 0) as int;
    final mockLogs = [
      {'time': '09:18:23', 'level': 'INFO', 'message': 'MCP 服务已连接'},
      {'time': '09:18:25', 'level': 'INFO', 'message': '工具列表已同步 ($toolCount 个)'},
      {'time': '09:18:26', 'level': 'INFO', 'message': 'Prompt 列表已同步 ($promptCount 个)'},
      {'time': '09:19:01', 'level': 'DEBUG', 'message': '调用工具 read_file'},
      {'time': '09:19:03', 'level': 'DEBUG', 'message': '工具返回结果，耗时 1.8s'},
      {'time': '09:20:15', 'level': 'WARN', 'message': '工具 delete_file 已被禁用'},
    ];
    return Column(
      children: mockLogs.map((log) {
        Color levelColor;
        switch (log['level']) {
          case 'INFO':
            levelColor = context.info;
          case 'WARN':
            levelColor = context.warning;
          case 'ERROR':
            levelColor = context.error;
          default:
            levelColor = context.textTertiary;
        }
        return Padding(
          padding: const EdgeInsets.only(bottom: 6),
          child: Container(
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
            decoration: BoxDecoration(
              color: context.surfacePrimary,
              borderRadius: AppRadius.brSmall,
              border: Border.all(color: context.borderPrimary, width: 0.5),
            ),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(log['time'] as String, style: AppTypography.label(context).copyWith(fontFamily: 'monospace')),
                const SizedBox(width: 10),
                Container(
                  padding: const EdgeInsets.symmetric(horizontal: 5, vertical: 1),
                  decoration: BoxDecoration(
                    color: levelColor.withValues(alpha: 0.12),
                    borderRadius: AppRadius.brTag,
                  ),
                  child: Text(log['level'] as String, style: TextStyle(fontSize: 10, color: levelColor, fontWeight: FontWeight.w600)),
                ),
                const SizedBox(width: 10),
                Expanded(child: Text(log['message'] as String, style: AppTypography.bodySmall(context))),
              ],
            ),
          ),
        );
      }).toList(),
    );
  }

  IconData _resourceIcon(String mimeType) {
    if (mimeType.contains('json')) return Icons.code;
    if (mimeType.contains('text')) return Icons.description_outlined;
    if (mimeType.contains('directory')) return Icons.folder_outlined;
    if (mimeType.contains('image')) return Icons.image_outlined;
    return Icons.insert_drive_file_outlined;
  }

  void _showUsePromptDialog(String name, String description, String content) {
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        backgroundColor: context.surfacePrimary,
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
        title: Text('使用 Prompt', style: AppTypography.cardTitle(context)),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(name, style: AppTypography.bodySmall(context).copyWith(fontWeight: FontWeight.w600)),
            const SizedBox(height: 8),
            Text(description, style: AppTypography.caption(context)),
            const SizedBox(height: 12),
            Container(
              padding: const EdgeInsets.all(12),
              decoration: BoxDecoration(
                color: context.surfaceSecondary,
                borderRadius: AppRadius.brSmall,
              ),
              child: Text(content, style: AppTypography.label(context).copyWith(fontFamily: 'monospace')),
            ),
          ],
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context), child: Text('取消', style: TextStyle(color: context.textSecondary))),
          TextButton(
            onPressed: () {
              Navigator.pop(context);
              ScaffoldMessenger.of(this.context).showSnackBar(
                SnackBar(content: Text('Prompt「$name」已使用'), backgroundColor: context.accentPrimary),
              );
            },
            child: Text('使用', style: TextStyle(color: context.accentPrimary)),
          ),
        ],
      ),
    );
  }

  void _showResourceContentDialog(String name, String uri, String mimeType, String? content) {
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        backgroundColor: context.surfacePrimary,
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
        title: Text(name, style: AppTypography.cardTitle(context)),
        content: SizedBox(
          width: double.maxFinite,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _InfoRow(label: 'URI', value: uri),
              _InfoRow(label: '类型', value: mimeType),
              const SizedBox(height: 12),
              Text('内容', style: AppTypography.caption(context)),
              const SizedBox(height: 6),
              Container(
                constraints: const BoxConstraints(maxHeight: 200),
                padding: const EdgeInsets.all(12),
                decoration: BoxDecoration(
                  color: context.surfaceSecondary,
                  borderRadius: AppRadius.brSmall,
                ),
                child: SingleChildScrollView(
                  child: Text(
                    content ?? '(无内容或目录类型资源)',
                    style: AppTypography.label(context).copyWith(fontFamily: 'monospace'),
                  ),
                ),
              ),
            ],
          ),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context), child: Text('关闭', style: TextStyle(color: context.textSecondary))),
        ],
      ),
    );
  }

  void _showCapabilityDialog() {
    showDialog(
      context: context,
      builder: (context) => StatefulBuilder(
        builder: (context, setDialogState) => AlertDialog(
          backgroundColor: context.surfacePrimary,
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
          title: Text('能力配置', style: AppTypography.cardTitle(context)),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              _CapabilityToggle(
                label: 'Sampling',
                desc: '允许服务端请求 LLM 采样',
                value: _capabilityState['sampling'] ?? false,
                onChanged: (val) {
                  setDialogState(() => _capabilityState['sampling'] = val);
                  if (val) {
                    Navigator.pop(context);
                    _showEnableCapabilityConfirm('Sampling');
                  }
                },
              ),
              _CapabilityToggle(
                label: 'Tasks',
                desc: '启用任务管理能力',
                value: _capabilityState['tasks'] ?? false,
                onChanged: (val) {
                  setDialogState(() => _capabilityState['tasks'] = val);
                  if (val) {
                    Navigator.pop(context);
                    _showEnableCapabilityConfirm('Tasks');
                  }
                },
              ),
              _CapabilityToggle(
                label: 'Roots',
                desc: '启用根目录能力',
                value: _capabilityState['roots'] ?? false,
                onChanged: (val) {
                  setDialogState(() => _capabilityState['roots'] = val);
                  if (val) {
                    Navigator.pop(context);
                    _showEnableCapabilityConfirm('Roots');
                  }
                },
              ),
            ],
          ),
          actions: [
            TextButton(onPressed: () => Navigator.pop(context), child: Text('关闭', style: TextStyle(color: context.textSecondary))),
          ],
        ),
      ),
    );
  }

  void _showEnableCapabilityConfirm(String capability) {
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        backgroundColor: context.surfacePrimary,
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
        title: Text('启用能力', style: AppTypography.cardTitle(context)),
        content: Text('确定要启用 $capability 能力吗？启用后该 MCP 服务将获得相应权限。', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context), child: Text('取消', style: TextStyle(color: context.textSecondary))),
          TextButton(
            onPressed: () {
              Navigator.pop(context);
              ScaffoldMessenger.of(this.context).showSnackBar(
                SnackBar(content: Text('$capability 能力已启用'), backgroundColor: context.success),
              );
            },
            child: Text('确认启用', style: TextStyle(color: context.accentPrimary)),
          ),
        ],
      ),
    );
  }

  void _showOAuthDialog() {
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        backgroundColor: context.surfacePrimary,
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
        title: Row(
          children: [
            Icon(Icons.lock_outline, color: context.warning, size: 22),
            const SizedBox(width: 8),
            Text('OAuth 授权', style: AppTypography.cardTitle(context)),
          ],
        ),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('该 MCP 服务需要 OAuth 授权才能使用全部功能。', style: AppTypography.bodySmall(context)),
            const SizedBox(height: 12),
            Container(
              padding: const EdgeInsets.all(12),
              decoration: BoxDecoration(
                color: context.warning.withValues(alpha: 0.08),
                borderRadius: AppRadius.brSmall,
              ),
              child: Row(
                children: [
                  Icon(Icons.security, size: 16, color: context.warning),
                  const SizedBox(width: 8),
                  Expanded(child: Text('授权后将获取访问令牌，请确认你信任该服务。', style: AppTypography.label(context).copyWith(color: context.warning))),
                ],
              ),
            ),
          ],
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context), child: Text('暂不授权', style: TextStyle(color: context.textSecondary))),
          TextButton(
            onPressed: () {
              Navigator.pop(context);
              ScaffoldMessenger.of(this.context).showSnackBar(
                SnackBar(content: const Text('OAuth 授权流程已启动'), backgroundColor: context.accentPrimary),
              );
            },
            child: Text('前往授权', style: TextStyle(color: context.accentPrimary)),
          ),
        ],
      ),
    );
  }
}

class _InfoRow extends StatelessWidget {
  final String label;
  final String value;

  const _InfoRow({required this.label, required this.value});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(width: 80, child: Text(label, style: AppTypography.label(context))),
          Expanded(child: Text(value, style: AppTypography.bodySmall(context))),
        ],
      ),
    );
  }
}

class _CapabilityChip extends StatelessWidget {
  final String label;
  final int? count;
  final bool? enabled;
  final IconData icon;

  const _CapabilityChip({required this.label, this.count, this.enabled, required this.icon});

  @override
  Widget build(BuildContext context) {
    final isActive = count != null ? count! > 0 : (enabled ?? false);
    final color = isActive ? context.accentPrimary : context.textTertiary;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      decoration: BoxDecoration(
        color: (isActive ? context.accentPrimary : context.textTertiary).withValues(alpha: 0.1),
        borderRadius: AppRadius.brTag,
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 15, color: color),
          const SizedBox(width: 6),
          Text(
            count != null ? '$label $count' : label,
            style: TextStyle(fontSize: 12, color: color, fontWeight: FontWeight.w500),
          ),
        ],
      ),
    );
  }
}

class _CapabilityToggle extends StatelessWidget {
  final String label;
  final String desc;
  final bool value;
  final ValueChanged<bool> onChanged;

  const _CapabilityToggle({required this.label, required this.desc, required this.value, required this.onChanged});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(label, style: AppTypography.bodySmall(context).copyWith(fontWeight: FontWeight.w600)),
                Text(desc, style: AppTypography.label(context)),
              ],
            ),
          ),
          Switch(value: value, onChanged: onChanged, materialTapTargetSize: MaterialTapTargetSize.shrinkWrap),
        ],
      ),
    );
  }
}

import 'dart:convert';
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
  final Set<String> _resourceSubscriptions = <String>{};
  bool _connectionAction = false;

  final _tabs = ['概览', '工具', 'Prompts', 'Resources', 'Tasks', '权限', '日志'];

  @override
  void initState() {
    super.initState();
    _toolEnabledState = {};
    _capabilityState = {};
    _loadServer();
  }

  Future<void> _loadServer() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final svc = ref.read(mcpServiceProvider);
      final data = await svc.getServer(widget.mcpId);
      if (data != null) {
        final tools = await svc.tools(widget.mcpId);
        final prompts = await svc.prompts(widget.mcpId);
        final resourceData = await svc.resources(widget.mcpId);
        final capabilities = await svc.capabilities(widget.mcpId);
        final logs = await svc.logs(widget.mcpId);

        _toolEnabledState = {};
        for (final tool in tools) {
          final key = (tool['remoteName'] ?? tool['name'] ?? '').toString();
          _toolEnabledState[key] = (tool['enabled'] as int? ?? 0) == 1;
        }
        _capabilityState = {
          for (final capability in capabilities)
            (capability['capability'] ?? '').toString(): (capability['enabled'] as int? ?? 0) == 1,
        };

        final tasksEnabled = _capabilityState['tasks'] ?? false;
        final tasks = tasksEnabled ? await svc.tasks(widget.mcpId) : <Map<String, dynamic>>[];
        data['tools'] = tools;
        data['prompts'] = prompts;
        data['resources'] = (resourceData['resources'] as List? ?? []).cast<Map<String, dynamic>>();
        data['resourceTemplates'] = (resourceData['resourceTemplates'] as List? ?? []).cast<Map<String, dynamic>>();
        data['tasks'] = tasks;
        data['logs'] = logs;
        data['toolCount'] = tools.length;
        data['promptCount'] = prompts.length;
        data['resourceCount'] = (data['resources'] as List).length;
        data['hasSampling'] = _capabilityState['sampling'] ?? false;
        data['hasTasks'] = tasksEnabled;
        data['hasRoots'] = _capabilityState['roots'] ?? false;
        data['hasOAuth'] = (data['authType'] ?? '').toString().toLowerCase() == 'oauth';
      }
      if (mounted) {
        setState(() {
          _server = data;
          _loading = false;
        });
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _error = e.toString();
          _loading = false;
        });
      }
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
    if (s == 'ready' || (s.contains('connected') && !s.contains('dis'))) return '已连接';
    if (s.contains('disconnected') || s.contains('disconnect')) return '未连接';
    if (s.contains('error')) return '错误';
    if (s.contains('connecting')) return '连接中';
    return status.toString();
  }

  BadgeType _statusBadgeType(dynamic status) {
    final s = status.toString().toLowerCase();
    if (s == 'ready' || (s.contains('connected') && !s.contains('dis'))) return BadgeType.success;
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
    final address = (server['endpoint'] ?? server['command'] ?? '').toString();
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
        SizedBox(height: AppSpacing.md),
        Row(
          children: [
            Expanded(
              child: AmitiaButton(
                label: _connectionAction ? '处理中...' : '重新连接',
                isSecondary: true,
                icon: Icons.sync,
                onPressed: _connectionAction ? null : _reconnectServer,
              ),
            ),
            const SizedBox(width: 8),
            Expanded(
              child: AmitiaButton(
                label: '重新发现能力',
                isSecondary: true,
                icon: Icons.refresh,
                onPressed: _connectionAction ? null : _refreshServer,
              ),
            ),
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
        final name = (tool['remoteName'] ?? tool['name'] ?? '').toString();
        final description = (tool['description'] ?? '').toString();
        final isEnabled = _toolEnabledState[name] ?? ((tool['enabled'] as int? ?? 0) == 1);
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
                  onChanged: (val) => _setToolEnabled(tool, name, val),
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
        final name = (prompt['remoteName'] ?? prompt['name'] ?? '').toString();
        final description = (prompt['description'] ?? '').toString();
        return Padding(
          padding: EdgeInsets.only(bottom: AppSpacing.sm),
          child: AmitiaCard(
            onTap: () => _showUsePromptDialog(name, description),
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
                Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    IconButton(
                      tooltip: '参数补全',
                      onPressed: () => _showPromptCompletionDialog(prompt),
                      icon: Icon(Icons.auto_fix_high_outlined, size: 18, color: context.accentPrimary),
                    ),
                    Icon(Icons.chevron_right, color: context.textTertiary, size: 20),
                  ],
                ),
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
        return Padding(
          padding: EdgeInsets.only(bottom: AppSpacing.sm),
          child: AmitiaCard(
            onTap: () => _showResourceContentDialog(name, uri, mimeType),
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
                Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    IconButton(
                      tooltip: _resourceSubscriptions.contains(uri) ? '取消订阅' : '订阅资源更新',
                      icon: Icon(
                        _resourceSubscriptions.contains(uri) ? Icons.notifications_active_outlined : Icons.notifications_none_outlined,
                        size: 19,
                        color: _resourceSubscriptions.contains(uri) ? context.accentPrimary : context.textTertiary,
                      ),
                      onPressed: uri.isEmpty ? null : () => _toggleResourceSubscription(uri),
                    ),
                    Icon(Icons.chevron_right, color: context.textTertiary, size: 20),
                  ],
                ),
              ],
            ),
          ),
        );
      }).toList(),
    );
  }

  Widget _buildTasksTab(BuildContext context, Map<String, dynamic> server) {
    final hasTasks = (server['hasTasks'] ?? false) as bool;
    final tasks = (server['tasks'] as List?)?.cast<Map<String, dynamic>>() ?? [];
    if (!hasTasks) {
      return AmitiaEmptyState(icon: Icons.task_outlined, title: '不支持 Tasks', subtitle: '该 MCP 服务未启用 Tasks 能力');
    }
    if (tasks.isEmpty) {
      return AmitiaEmptyState(icon: Icons.task_outlined, title: '暂无任务', subtitle: '服务当前没有远端任务');
    }
    return Column(
      children: tasks.map((task) {
        final status = (task['status'] ?? 'unknown').toString();
        final message = (task['statusMessage'] ?? '').toString();
        final remoteId = (task['remoteTaskId'] ?? task['id'] ?? '').toString();
        return Padding(
          padding: EdgeInsets.only(bottom: AppSpacing.sm),
          child: AmitiaCard(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Icon(Icons.task_alt, size: 18, color: context.accentPrimary),
                    const SizedBox(width: 8),
                    Expanded(child: Text(remoteId, style: AppTypography.bodySmall(context).copyWith(fontWeight: FontWeight.w600))),
                    AmitiaStatusBadge(label: _taskStatusLabel(status), type: _taskStatusType(status)),
                  ],
                ),
                if (message.isNotEmpty) ...[
                  SizedBox(height: AppSpacing.sm),
                  Text(message, style: AppTypography.caption(context)),
                ],
                if (<String>{'working', 'running', 'input_required'}.contains(status.toLowerCase())) ...[
                  SizedBox(height: AppSpacing.sm),
                  Align(
                    alignment: Alignment.centerRight,
                    child: TextButton.icon(
                      onPressed: remoteId.isEmpty ? null : () => _cancelRemoteTask(remoteId),
                      icon: Icon(Icons.stop_circle_outlined, size: 18, color: context.error),
                      label: Text('取消任务', style: TextStyle(color: context.error)),
                    ),
                  ),
                ],
              ],
            ),
          ),
        );
      }).toList(),
    );
  }

  Widget _buildPermissionsTab(BuildContext context, Map<String, dynamic> server) {
    final transport = (server['transport'] ?? '').toString();
    final hasOAuth = (server['hasOAuth'] ?? false) as bool;
    final isStdio = transport.toLowerCase().contains('stdio');
    final privateNetwork = _capabilityState['private_network'] ?? false;

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
                  Text('运行权限', style: AppTypography.sectionTitle(context)),
                ],
              ),
              SizedBox(height: AppSpacing.md),
              _InfoRow(label: '运行方式', value: isStdio ? '本地子进程（STDIO）' : '远程网络服务'),
              _InfoRow(label: '私网访问', value: privateNetwork ? '已授权' : '未授权'),
              _InfoRow(label: '认证方式', value: (server['authType'] ?? 'none').toString()),
            ],
          ),
        ),
        if (hasOAuth) ...[
          SizedBox(height: AppSpacing.md),
          AmitiaCard(
            child: Column(
              children: [
                Row(
                  children: [
                    Icon(Icons.lock_outline, size: 20, color: context.error),
                    const SizedBox(width: 12),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text('OAuth 授权', style: AppTypography.bodySmall(context).copyWith(fontWeight: FontWeight.w600)),
                          const SizedBox(height: 2),
                          Text('授权与撤销都会直接调用当前 MCP Server 的真实 OAuth 生命周期接口', style: AppTypography.caption(context)),
                        ],
                      ),
                    ),
                  ],
                ),
                SizedBox(height: AppSpacing.sm),
                Row(
                  children: [
                    Expanded(child: AmitiaButton(label: '开始授权', isSecondary: true, onPressed: _showOAuthDialog)),
                    const SizedBox(width: 8),
                    Expanded(child: AmitiaButton(label: '撤销授权', isSecondary: true, onPressed: _revokeOAuth)),
                  ],
                ),
              ],
            ),
          ),
        ],
        SizedBox(height: AppSpacing.md),
        AmitiaButton(
          label: '管理能力配置',
          isFullWidth: true,
          isSecondary: true,
          icon: Icons.tune,
          onPressed: _showCapabilityDialog,
        ),
      ],
    );
  }

  Widget _buildLogsTab(BuildContext context, Map<String, dynamic> server) {
    final logs = (server['logs'] as List?)?.cast<Map<String, dynamic>>() ?? [];
    if (logs.isEmpty) {
      return AmitiaEmptyState(icon: Icons.receipt_long_outlined, title: '暂无日志', subtitle: '该 MCP 服务还没有审计记录');
    }
    return Column(
      children: logs.map((log) {
        final status = (log['status'] ?? '').toString();
        final operation = (log['operation'] ?? '').toString();
        final createdAt = (log['createdAt'] ?? '').toString();
        final message = (log['errorMessage'] ?? '').toString();
        final isError = status.toLowerCase().contains('fail') || message.isNotEmpty;
        final levelColor = isError ? context.error : context.info;
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
                Container(
                  padding: const EdgeInsets.symmetric(horizontal: 5, vertical: 1),
                  decoration: BoxDecoration(color: levelColor.withValues(alpha: 0.12), borderRadius: AppRadius.brTag),
                  child: Text(isError ? 'ERROR' : 'INFO', style: TextStyle(fontSize: 10, color: levelColor, fontWeight: FontWeight.w600)),
                ),
                const SizedBox(width: 10),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(operation.isEmpty ? status : operation, style: AppTypography.bodySmall(context)),
                      if (message.isNotEmpty) Text(message, style: AppTypography.caption(context)),
                      if (createdAt.isNotEmpty) Text(createdAt, style: AppTypography.label(context)),
                    ],
                  ),
                ),
              ],
            ),
          ),
        );
      }).toList(),
    );
  }

  Future<void> _reconnectServer() async {
    if (_connectionAction) return;
    setState(() => _connectionAction = true);
    try {
      await ref.read(mcpServiceProvider).reconnectServer(widget.mcpId);
      await _loadServer();
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('MCP 服务已重新连接')));
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('重新连接失败：$e')));
    } finally {
      if (mounted) setState(() => _connectionAction = false);
    }
  }

  Future<void> _refreshServer() async {
    if (_connectionAction) return;
    setState(() => _connectionAction = true);
    try {
      await ref.read(mcpServiceProvider).refreshTools(widget.mcpId);
      await _loadServer();
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('MCP 能力已重新发现')));
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('重新发现失败：$e')));
    } finally {
      if (mounted) setState(() => _connectionAction = false);
    }
  }

  Future<void> _toggleResourceSubscription(String uri) async {
    final next = !_resourceSubscriptions.contains(uri);
    try {
      await ref.read(mcpServiceProvider).setResourceSubscription(widget.mcpId, uri, next);
      if (!mounted) return;
      setState(() {
        if (next) {
          _resourceSubscriptions.add(uri);
        } else {
          _resourceSubscriptions.remove(uri);
        }
      });
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(next ? '已订阅 Resource 更新' : '已取消 Resource 订阅')));
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Resource 订阅操作失败：$e')));
    }
  }

  Future<void> _cancelRemoteTask(String taskId) async {
    try {
      await ref.read(mcpServiceProvider).cancelTask(widget.mcpId, taskId);
      await _loadServer();
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('远端任务已取消')));
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('取消任务失败：$e')));
    }
  }

  Future<void> _revokeOAuth() async {
    final confirmed = await showDialog<bool>(
          context: context,
          builder: (dialogContext) => AlertDialog(
            title: const Text('撤销 OAuth 授权'),
            content: const Text('将删除该 MCP Server 已保存的 OAuth 授权凭据。后续调用可能需要重新授权。'),
            actions: [
              TextButton(onPressed: () => Navigator.pop(dialogContext, false), child: const Text('取消')),
              FilledButton(onPressed: () => Navigator.pop(dialogContext, true), child: const Text('撤销')),
            ],
          ),
        ) ??
        false;
    if (!confirmed) return;
    try {
      await ref.read(mcpServiceProvider).revokeOAuth(widget.mcpId);
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('OAuth 授权已撤销')));
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('撤销授权失败：$e')));
    }
  }

  Future<void> _setToolEnabled(Map<String, dynamic> tool, String name, bool enabled) async {
    final toolId = (tool['id'] ?? '').toString();
    if (toolId.isEmpty) return;
    final previous = _toolEnabledState[name] ?? false;
    setState(() => _toolEnabledState[name] = enabled);
    try {
      await ref.read(mcpServiceProvider).setToolEnabled(widget.mcpId, toolId, enabled);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('$name 已${enabled ? '启用' : '禁用'}')),
        );
      }
    } catch (e) {
      if (mounted) {
        setState(() => _toolEnabledState[name] = previous);
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('更新工具状态失败：$e')));
      }
    }
  }

  String _taskStatusLabel(String status) {
    switch (status.toLowerCase()) {
      case 'working':
      case 'running':
        return '运行中';
      case 'completed':
      case 'succeeded':
        return '已完成';
      case 'failed':
        return '失败';
      case 'cancelled':
      case 'canceled':
        return '已取消';
      case 'input_required':
        return '等待输入';
      default:
        return status;
    }
  }

  BadgeType _taskStatusType(String status) {
    switch (status.toLowerCase()) {
      case 'completed':
      case 'succeeded':
        return BadgeType.success;
      case 'failed':
        return BadgeType.error;
      case 'working':
      case 'running':
        return BadgeType.accent;
      default:
        return BadgeType.warning;
    }
  }

  IconData _resourceIcon(String mimeType) {
    if (mimeType.contains('json')) return Icons.code;
    if (mimeType.contains('text')) return Icons.description_outlined;
    if (mimeType.contains('directory')) return Icons.folder_outlined;
    if (mimeType.contains('image')) return Icons.image_outlined;
    return Icons.insert_drive_file_outlined;
  }

  Future<void> _showPromptCompletionDialog(Map<String, dynamic> prompt) async {
    final name = (prompt['remoteName'] ?? prompt['name'] ?? '').toString();
    final rawArguments = prompt['arguments'];
    List<Map<String, dynamic>> arguments = <Map<String, dynamic>>[];
    try {
      final decoded = rawArguments is String ? jsonDecode(rawArguments) : rawArguments;
      if (decoded is List) {
        arguments = decoded.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList();
      }
    } catch (_) {}
    if (arguments.isEmpty) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('该 Prompt 没有声明可补全参数')));
      return;
    }
    String selected = (arguments.first['name'] ?? '').toString();
    final valueController = TextEditingController();
    List<String> suggestions = <String>[];
    bool loading = false;
    if (!mounted) return;
    await showDialog<void>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (dialogContext, setDialogState) => AlertDialog(
          title: const Text('Prompt 参数补全'),
          content: SizedBox(
            width: 520,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                DropdownButtonFormField<String>(
                  initialValue: selected,
                  decoration: const InputDecoration(labelText: '参数'),
                  items: arguments
                      .map((item) => (item['name'] ?? '').toString())
                      .where((item) => item.isNotEmpty)
                      .map((item) => DropdownMenuItem(value: item, child: Text(item)))
                      .toList(),
                  onChanged: (value) => setDialogState(() {
                    selected = value ?? selected;
                    suggestions = <String>[];
                  }),
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: valueController,
                  decoration: const InputDecoration(labelText: '当前输入', hintText: '输入部分内容后请求服务端补全'),
                ),
                const SizedBox(height: 12),
                Align(
                  alignment: Alignment.centerRight,
                  child: FilledButton.icon(
                    onPressed: loading || selected.isEmpty
                        ? null
                        : () async {
                            setDialogState(() => loading = true);
                            try {
                              final result = await ref.read(mcpServiceProvider).completePromptArgument(
                                    widget.mcpId,
                                    promptName: name,
                                    argumentName: selected,
                                    value: valueController.text,
                                  );
                              final values = (result?['values'] as List? ?? const <dynamic>[]).map((e) => e.toString()).where((e) => e.isNotEmpty).toList();
                              if (dialogContext.mounted) setDialogState(() => suggestions = values);
                            } catch (e) {
                              if (dialogContext.mounted) ScaffoldMessenger.of(dialogContext).showSnackBar(SnackBar(content: Text('补全失败：$e')));
                            } finally {
                              if (dialogContext.mounted) setDialogState(() => loading = false);
                            }
                          },
                    icon: loading ? const SizedBox.square(dimension: 16, child: CircularProgressIndicator(strokeWidth: 2)) : const Icon(Icons.auto_fix_high_outlined),
                    label: const Text('获取建议'),
                  ),
                ),
                if (suggestions.isNotEmpty) ...[
                  const SizedBox(height: 12),
                  Wrap(
                    spacing: 8,
                    runSpacing: 8,
                    children: suggestions
                        .map((value) => ActionChip(
                              label: Text(value),
                              onPressed: () {
                                valueController.text = value;
                                valueController.selection = TextSelection.collapsed(offset: value.length);
                              },
                            ))
                        .toList(),
                  ),
                ],
              ],
            ),
          ),
          actions: [TextButton(onPressed: () => Navigator.pop(dialogContext), child: const Text('关闭'))],
        ),
      ),
    );
    valueController.dispose();
  }

  Future<void> _showUsePromptDialog(String name, String description) async {
    Map<String, dynamic>? promptResult;
    String? loadError;
    try {
      promptResult = await ref.read(mcpServiceProvider).getPrompt(widget.mcpId, name);
    } catch (e) {
      loadError = e.toString();
    }
    if (!mounted) return;
    final messages = (promptResult?['messages'] as List? ?? const []).map((item) {
      if (item is Map<String, dynamic>) {
        return '${item['role'] ?? ''}: ${item['content'] ?? ''}';
      }
      return item.toString();
    }).join('\n\n');
    showDialog(
      context: context,
      builder: (dialogContext) => AlertDialog(
        backgroundColor: dialogContext.surfacePrimary,
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
        title: Text('Prompt 预览', style: AppTypography.cardTitle(dialogContext)),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(name, style: AppTypography.bodySmall(dialogContext).copyWith(fontWeight: FontWeight.w600)),
            if (description.isNotEmpty) ...[
              const SizedBox(height: 8),
              Text(description, style: AppTypography.caption(dialogContext)),
            ],
            const SizedBox(height: 12),
            Container(
              constraints: const BoxConstraints(maxHeight: 260),
              padding: const EdgeInsets.all(12),
              decoration: BoxDecoration(color: dialogContext.surfaceSecondary, borderRadius: AppRadius.brSmall),
              child: SingleChildScrollView(
                child: Text(
                  loadError ?? (messages.isEmpty ? '服务未返回 Prompt 内容' : messages),
                  style: AppTypography.label(dialogContext).copyWith(fontFamily: 'monospace'),
                ),
              ),
            ),
          ],
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(dialogContext), child: Text('关闭', style: TextStyle(color: dialogContext.textSecondary))),
        ],
      ),
    );
  }

  Future<void> _showResourceContentDialog(String name, String uri, String mimeType) async {
    Map<String, dynamic>? result;
    String? loadError;
    try {
      result = await ref.read(mcpServiceProvider).readResource(widget.mcpId, uri);
    } catch (e) {
      loadError = e.toString();
    }
    if (!mounted) return;
    final contents = (result?['contents'] as List? ?? const []).map((item) {
      if (item is Map<String, dynamic>) {
        return (item['text'] ?? item['blob'] ?? '').toString();
      }
      return item.toString();
    }).where((value) => value.isNotEmpty).join('\n\n');
    showDialog(
      context: context,
      builder: (dialogContext) => AlertDialog(
        backgroundColor: dialogContext.surfacePrimary,
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
        title: Text(name, style: AppTypography.cardTitle(dialogContext)),
        content: SizedBox(
          width: double.maxFinite,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _InfoRow(label: 'URI', value: uri),
              _InfoRow(label: '类型', value: mimeType),
              const SizedBox(height: 12),
              Text('内容', style: AppTypography.caption(dialogContext)),
              const SizedBox(height: 6),
              Container(
                constraints: const BoxConstraints(maxHeight: 240),
                padding: const EdgeInsets.all(12),
                decoration: BoxDecoration(color: dialogContext.surfaceSecondary, borderRadius: AppRadius.brSmall),
                child: SingleChildScrollView(
                  child: Text(
                    loadError ?? (contents.isEmpty ? '(服务未返回可显示内容)' : contents),
                    style: AppTypography.label(dialogContext).copyWith(fontFamily: 'monospace'),
                  ),
                ),
              ),
            ],
          ),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(dialogContext), child: Text('关闭', style: TextStyle(color: dialogContext.textSecondary))),
        ],
      ),
    );
  }

  void _showCapabilityDialog() {
    showDialog(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (dialogContext, setDialogState) => AlertDialog(
          backgroundColor: dialogContext.surfacePrimary,
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
          title: Text('能力配置', style: AppTypography.cardTitle(dialogContext)),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              _CapabilityToggle(
                label: 'Sampling',
                desc: '允许服务端请求 LLM 采样',
                value: _capabilityState['sampling'] ?? false,
                onChanged: (value) => _updateCapability('sampling', value, setDialogState),
              ),
              _CapabilityToggle(
                label: 'Tasks',
                desc: '启用远端任务管理能力',
                value: _capabilityState['tasks'] ?? false,
                onChanged: (value) => _updateCapability('tasks', value, setDialogState),
              ),
              _CapabilityToggle(
                label: 'Roots',
                desc: '允许 MCP 服务请求 Roots',
                value: _capabilityState['roots'] ?? false,
                onChanged: (value) => _updateCapability('roots', value, setDialogState),
              ),
              _CapabilityToggle(
                label: 'Private Network',
                desc: '允许连接局域网/private 地址',
                value: _capabilityState['private_network'] ?? false,
                onChanged: (value) => _updateCapability('private_network', value, setDialogState),
              ),
            ],
          ),
          actions: [
            TextButton(onPressed: () => Navigator.pop(dialogContext), child: Text('关闭', style: TextStyle(color: dialogContext.textSecondary))),
          ],
        ),
      ),
    );
  }

  Future<void> _updateCapability(String capability, bool enabled, StateSetter setDialogState) async {
    final previous = _capabilityState[capability] ?? false;
    setState(() => _capabilityState[capability] = enabled);
    setDialogState(() {});
    Map<String, dynamic> configuration = const {};
    if (enabled && capability == 'sampling') {
      configuration = {'maxTokens': 2048, 'timeoutSeconds': 60, 'maxConcurrent': 1};
    } else if (enabled && capability == 'tasks') {
      configuration = {'maxConcurrent': 4, 'maxTTLSeconds': 86400};
    }
    try {
      await ref.read(mcpServiceProvider).setCapability(
            widget.mcpId,
            capability,
            enabled,
            configuration: configuration,
          );
      await _loadServer();
    } catch (e) {
      if (!mounted) return;
      setState(() => _capabilityState[capability] = previous);
      setDialogState(() {});
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('能力更新失败：$e')));
    }
  }

  Future<void> _showOAuthDialog() async {
    final endpoint = (_server?['endpoint'] ?? '').toString();
    if (endpoint.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('该 MCP 服务没有可用于 OAuth 的资源地址')));
      return;
    }
    try {
      final result = await ref.read(mcpServiceProvider).startOAuth(widget.mcpId, resourceUrl: endpoint);
      if (!mounted) return;
      final authorizationUrl = (result?['authorizationUrl'] ?? '').toString();
      showDialog(
        context: context,
        builder: (dialogContext) => AlertDialog(
          backgroundColor: dialogContext.surfacePrimary,
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
          title: Text('OAuth 授权地址', style: AppTypography.cardTitle(dialogContext)),
          content: SelectableText(
            authorizationUrl.isEmpty ? '服务未返回授权地址' : authorizationUrl,
            style: AppTypography.bodySmall(dialogContext),
          ),
          actions: [
            TextButton(onPressed: () => Navigator.pop(dialogContext), child: Text('关闭', style: TextStyle(color: dialogContext.textSecondary))),
          ],
        ),
      );
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('启动 OAuth 失败：$e')));
      }
    }
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

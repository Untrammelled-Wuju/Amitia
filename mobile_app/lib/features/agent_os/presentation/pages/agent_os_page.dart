import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class AgentOSPage extends ConsumerStatefulWidget {
  const AgentOSPage({super.key});

  @override
  ConsumerState<AgentOSPage> createState() => _AgentOSPageState();
}

class _AgentOSPageState extends ConsumerState<AgentOSPage> {
  int _tab = 0;
  bool _loading = true;
  String? _error;
  List<Map<String, dynamic>> _agents = const [];
  List<Map<String, dynamic>> _teams = const [];
  List<Map<String, dynamic>> _approvals = const [];
  List<Map<String, dynamic>> _reconciliations = const [];
  List<Map<String, dynamic>> _runtimes = const [];
  List<Map<String, dynamic>> _activity = const [];
  Map<String, dynamic> _summary = const {};
  String? _platformSessionToken;
  Map<String, dynamic>? _platformUser;

  @override
  void initState() {
    super.initState();
    _load();
    _loadPlatformSession();
  }

  Future<T?> _get<T>(String path, {Map<String, dynamic>? query}) {
    return ref
        .read(backendServiceProvider)
        .get<T>(path, queryParameters: query, headers: _platformHeaders);
  }

  Future<T?> _post<T>(String path, {Object? data}) {
    return ref
        .read(backendServiceProvider)
        .post<T>(path, data: data, headers: _platformHeaders);
  }

  Future<T?> _put<T>(String path, {Object? data}) {
    return ref
        .read(backendServiceProvider)
        .put<T>(path, data: data, headers: _platformHeaders);
  }

  Map<String, String>? get _platformHeaders {
    final token = _platformSessionToken;
    return token == null ? null : {'X-Amitia-Platform-Session': token};
  }

  Future<void> _loadPlatformSession() async {
    final preferences = await SharedPreferences.getInstance();
    final token = preferences.getString('agent_platform_session_token');
    final user = preferences.getString('agent_platform_session_user');
    if (!mounted) return;
    setState(() {
      _platformSessionToken = token;
      if (user != null) {
        _platformUser = Map<String, dynamic>.from(jsonDecode(user) as Map);
      }
    });
  }

  Future<void> _showMemberAccount() async {
    if (_platformSessionToken != null && _platformUser != null) {
      final logout = await showDialog<bool>(
        context: context,
        builder: (dialogContext) => AlertDialog(
          title: Text(
            (_platformUser?['displayName'] ?? _platformUser?['email'] ?? '平台成员')
                .toString(),
          ),
          content: const Text('当前会话用于团队、权限和审计归属。'),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialogContext, false),
              child: const Text('关闭'),
            ),
            FilledButton(
              onPressed: () => Navigator.pop(dialogContext, true),
              child: const Text('退出登录'),
            ),
          ],
        ),
      );
      if (logout == true) {
        try {
          await _post<dynamic>(
            '/api/agent-platform/auth/logout',
            data: const {},
          );
        } finally {
          final preferences = await SharedPreferences.getInstance();
          await preferences.remove('agent_platform_session_token');
          await preferences.remove('agent_platform_session_user');
          if (mounted) {
            setState(() {
              _platformSessionToken = null;
              _platformUser = null;
            });
          }
        }
      }
      return;
    }
    final email = TextEditingController();
    final password = TextEditingController();
    final inviteToken = TextEditingController();
    final displayName = TextEditingController();
    var inviteMode = false;
    await showDialog<void>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (context, setDialogState) => AlertDialog(
          title: Text(inviteMode ? '接受成员邀请' : '成员登录'),
          content: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: inviteMode
                  ? [
                      TextField(
                        controller: inviteToken,
                        decoration: const InputDecoration(labelText: '邀请令牌'),
                      ),
                      TextField(
                        controller: displayName,
                        decoration: const InputDecoration(labelText: '显示名称'),
                      ),
                      TextField(
                        controller: password,
                        obscureText: true,
                        decoration: const InputDecoration(labelText: '登录密码'),
                      ),
                    ]
                  : [
                      TextField(
                        controller: email,
                        decoration: const InputDecoration(labelText: '邮箱'),
                      ),
                      TextField(
                        controller: password,
                        obscureText: true,
                        decoration: const InputDecoration(labelText: '密码'),
                      ),
                    ],
            ),
          ),
          actions: [
            TextButton(
              onPressed: () => setDialogState(() => inviteMode = !inviteMode),
              child: Text(inviteMode ? '改用登录' : '使用邀请令牌'),
            ),
            TextButton(
              onPressed: () => Navigator.pop(dialogContext),
              child: const Text('取消'),
            ),
            FilledButton(
              onPressed: () async {
                try {
                  if (inviteMode) {
                    final user = await _post<Map<String, dynamic>>(
                      '/api/agent-platform/users/invitations/accept',
                      data: {
                        'token': inviteToken.text.trim(),
                        'displayName': displayName.text.trim(),
                        'password': password.text,
                      },
                    );
                    email.text = (user?['email'] ?? '').toString();
                    setDialogState(() => inviteMode = false);
                    if (context.mounted) {
                      amitiaSnackBar(context, '邀请已接受，请使用邮箱和密码登录');
                    }
                    return;
                  }
                  final login = await _post<Map<String, dynamic>>(
                    '/api/agent-platform/auth/login',
                    data: {
                      'email': email.text.trim(),
                      'password': password.text,
                    },
                  );
                  final token = (login?['token'] ?? '').toString();
                  if (token.isEmpty) throw StateError('登录响应缺少会话令牌');
                  final user = Map<String, dynamic>.from(
                    login?['user'] as Map? ?? const {},
                  );
                  final preferences = await SharedPreferences.getInstance();
                  await preferences.setString(
                    'agent_platform_session_token',
                    token,
                  );
                  await preferences.setString(
                    'agent_platform_session_user',
                    jsonEncode(user),
                  );
                  if (mounted) {
                    setState(() {
                      _platformSessionToken = token;
                      _platformUser = user;
                    });
                  }
                  if (dialogContext.mounted) Navigator.pop(dialogContext);
                  await _load();
                } catch (error) {
                  if (context.mounted) {
                    amitiaSnackBar(context, '成员操作失败：$error');
                  }
                }
              },
              child: Text(inviteMode ? '接受邀请' : '登录'),
            ),
          ],
        ),
      ),
    );
    email.dispose();
    password.dispose();
    inviteToken.dispose();
    displayName.dispose();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final values = await Future.wait<dynamic>([
        _get<List<dynamic>>(
          '/api/agent-platform/agents',
          query: {'includeDisabled': true},
        ),
        _get<Map<String, dynamic>>('/api/agent-platform/overview'),
        _get<List<dynamic>>('/api/agent-platform/teams'),
        _get<List<dynamic>>(
          '/api/agent-platform/approvals',
          query: {'status': 'pending'},
        ),
        _get<List<dynamic>>(
          '/api/agent-platform/reconciliations',
          query: {'status': 'open'},
        ),
        _get<List<dynamic>>('/api/agent-platform/runtimes'),
        _get<Map<String, dynamic>>(
          '/api/agent-platform/activity',
          query: {'limit': 100},
        ),
      ]);
      final agents = values[0] as List<dynamic>?;
      final overview = values[1] as Map<String, dynamic>?;
      final teams = values[2] as List<dynamic>?;
      final approvals = values[3] as List<dynamic>?;
      final reconciliations = values[4] as List<dynamic>?;
      final runtimes = values[5] as List<dynamic>?;
      final activity = values[6] as Map<String, dynamic>?;
      if (!mounted) return;
      setState(() {
        _agents = (agents ?? const [])
            .whereType<Map>()
            .map((item) => Map<String, dynamic>.from(item))
            .toList(growable: false);
        _teams = (teams ?? const [])
            .whereType<Map>()
            .map((item) => Map<String, dynamic>.from(item))
            .toList(growable: false);
        _approvals = (approvals ?? const [])
            .whereType<Map>()
            .map((item) => Map<String, dynamic>.from(item))
            .toList(growable: false);
        _reconciliations = (reconciliations ?? const [])
            .whereType<Map>()
            .map((item) => Map<String, dynamic>.from(item))
            .toList(growable: false);
        _runtimes = (runtimes ?? const [])
            .whereType<Map>()
            .map((item) => Map<String, dynamic>.from(item))
            .toList(growable: false);
        _activity = ((activity?['items'] as List?) ?? const [])
            .whereType<Map>()
            .map((item) => Map<String, dynamic>.from(item))
            .toList(growable: false);
        _summary = Map<String, dynamic>.from(
          (overview?['summary'] as Map?) ?? const {},
        );
      });
    } catch (error) {
      if (mounted) setState(() => _error = error.toString());
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  Future<void> _createAgent() async {
    final name = TextEditingController();
    final description = TextEditingController();
    var archetype = 'worker';
    var teamId = '';
    final created = await showModalBottomSheet<bool>(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(
          top: Radius.circular(AppRadius.large),
        ),
      ),
      builder: (sheetContext) => StatefulBuilder(
        builder: (context, setSheetState) => Padding(
          padding: EdgeInsets.fromLTRB(
            AppSpacing.pagePadding,
            AppSpacing.lg,
            AppSpacing.pagePadding,
            MediaQuery.of(context).viewInsets.bottom + AppSpacing.lg,
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text('新建 Agent', style: AppTypography.sectionTitle(context)),
              SizedBox(height: AppSpacing.md),
              TextField(
                controller: name,
                decoration: const InputDecoration(
                  labelText: '名称',
                  hintText: '例如：研究助理',
                ),
              ),
              SizedBox(height: AppSpacing.sm),
              TextField(
                controller: description,
                maxLines: 2,
                decoration: const InputDecoration(labelText: '描述'),
              ),
              SizedBox(height: AppSpacing.sm),
              DropdownButtonFormField<String>(
                initialValue: archetype,
                decoration: const InputDecoration(labelText: '类型'),
                items: const [
                  DropdownMenuItem(value: 'companion', child: Text('陪伴型')),
                  DropdownMenuItem(value: 'worker', child: Text('执行型')),
                  DropdownMenuItem(value: 'team_lead', child: Text('团队负责人')),
                ],
                onChanged: (value) {
                  if (value != null) setSheetState(() => archetype = value);
                },
              ),
              SizedBox(height: AppSpacing.sm),
              DropdownButtonFormField<String>(
                initialValue: teamId,
                decoration: const InputDecoration(labelText: '所属团队'),
                items: [
                  const DropdownMenuItem(value: '', child: Text('暂不加入团队')),
                  ..._teams.map(
                    (team) => DropdownMenuItem(
                      value: (team['id'] ?? '').toString(),
                      child: Text((team['name'] ?? '').toString()),
                    ),
                  ),
                ],
                onChanged: (value) {
                  if (value != null) setSheetState(() => teamId = value);
                },
              ),
              SizedBox(height: AppSpacing.lg),
              FilledButton(
                onPressed: () async {
                  if (name.text.trim().isEmpty) return;
                  try {
                    await _post<Map<String, dynamic>>(
                      '/api/agent-platform/agents',
                      data: {
                        'name': name.text.trim(),
                        'description': description.text.trim(),
                        'archetype': archetype,
                        'teamId': teamId,
                        'config': {
                          'identity': {'kind': archetype},
                          'instructions': '你是一个可靠、谨慎并主动验证结果的 Agent。',
                          'model': {
                            'provider': 'custom',
                            'name': 'deepseek-chat',
                          },
                          'memory': {'scope': 'agent'},
                          'permissions': {'mode': 'request_approval'},
                          'runtime': {'provider': 'isolated'},
                        },
                      },
                    );
                    if (sheetContext.mounted) Navigator.pop(sheetContext, true);
                  } catch (error) {
                    if (context.mounted) amitiaSnackBar(context, '创建失败：$error');
                  }
                },
                child: const Text('创建草稿'),
              ),
            ],
          ),
        ),
      ),
    );
    name.dispose();
    description.dispose();
    if (created == true) await _load();
  }

  Future<void> _publish(Map<String, dynamic> agent) async {
    final revision = (agent['revision'] as num?)?.toInt() ?? 0;
    try {
      final detail = await _get<Map<String, dynamic>>(
        '/api/agent-platform/agents/${agent['id']}',
      );
      final revisionId = ((detail?['revision'] as Map?)?['id'] ?? '')
          .toString();
      await _post(
        '/api/agent-platform/agents/${agent['id']}/test',
        data: {'revisionId': revisionId},
      );
      await _post(
        '/api/agent-platform/agents/${agent['id']}/publish',
        data: {
          'expectedRevision': revision,
          'version': '1.0.0',
          'releaseNotes': '移动端创建并发布',
        },
      );
      if (mounted) amitiaSnackBar(context, 'Agent 已发布');
      await _load();
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '发布失败：$error');
    }
  }

  Future<String?> _askText(
    String title,
    String label, {
    String initial = "",
    int maxLines = 1,
  }) async {
    final controller = TextEditingController(text: initial);
    final value = await showDialog<String>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: Text(title),
        content: TextField(
          controller: controller,
          maxLines: maxLines,
          decoration: InputDecoration(labelText: label),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(dialogContext, controller.text),
            child: const Text('确定'),
          ),
        ],
      ),
    );
    controller.dispose();
    return value;
  }

  Future<void> _editAgent(Map<String, dynamic> agent) async {
    try {
      final detail = await _get<Map<String, dynamic>>(
        '/api/agent-platform/agents/${agent['id']}',
      );
      if (detail == null) throw StateError('Agent 详情为空');
      final revision = Map<String, dynamic>.from(
        detail['revision'] as Map? ?? {},
      );
      final config = await _askText(
        '编辑 Agent 配置',
        '配置 JSON',
        initial: (revision['configJson'] ?? '{}').toString(),
        maxLines: 18,
      );
      if (config == null || config.trim().isEmpty) return;
      final note = await _askText('变更说明', '说明', initial: '移动端编辑');
      final decoded = jsonDecode(config);
      await ref
          .read(backendServiceProvider)
          .put<dynamic>(
            '/api/agent-platform/agents/${agent['id']}',
            data: {
              'expectedRevision': agent['revision'],
              'changeNote': note ?? '',
              'config': decoded,
            },
          );
      await _load();
      if (mounted) amitiaSnackBar(context, 'Agent 新草稿已保存');
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '编辑失败：$error');
    }
  }

  Future<void> _showAgentReleases(Map<String, dynamic> agent) async {
    try {
      final releases = await _get<List<dynamic>>(
        '/api/agent-platform/agents/${agent['id']}/releases',
      );
      if (!mounted) return;
      final selected = await showModalBottomSheet<Map<String, dynamic>>(
        context: context,
        backgroundColor: context.surfacePrimary,
        builder: (sheetContext) => SafeArea(
          child: ListView(
            padding: EdgeInsets.all(AppSpacing.pagePadding),
            children: [
              Text('版本历史', style: AppTypography.sectionTitle(sheetContext)),
              if ((releases ?? const []).isEmpty)
                const Padding(
                  padding: EdgeInsets.only(top: 24),
                  child: Text('尚无已发布版本'),
                ),
              ...(releases ?? const []).whereType<Map>().map(
                (item) => ListTile(
                  contentPadding: EdgeInsets.zero,
                  title: Text('v${item['version']} · ${item['status']}'),
                  subtitle: Text(
                    '${item['releaseNotes'] ?? ''}\n${_date(item['createdAt'])}',
                  ),
                  trailing: const Icon(Icons.undo),
                  onTap: () => Navigator.pop(
                    sheetContext,
                    Map<String, dynamic>.from(item),
                  ),
                ),
              ),
            ],
          ),
        ),
      );
      if (selected == null) return;
      await _post<dynamic>(
        '/api/agent-platform/agents/${agent['id']}/rollback',
        data: {
          'releaseId': selected['id'],
          'expectedRevision': agent['revision'],
        },
      );
      await _load();
      if (mounted) amitiaSnackBar(context, 'Agent 已回滚到 ${selected['version']}');
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '回滚失败：$error');
    }
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: 'Agent OS',
        centerTitle: true,
        navigation: AmitiaAppBarNavigation.back,
        actions: [
          AmitiaIconButton(
            icon: Icons.account_circle_outlined,
            tooltip: _platformUser == null ? '成员登录' : '成员账户',
            onPressed: _showMemberAccount,
          ),
          AmitiaIconButton(
            icon: Icons.refresh,
            tooltip: '刷新',
            onPressed: _load,
          ),
        ],
      ),
      body: Column(
        children: [
          Padding(
            padding: EdgeInsets.fromLTRB(
              AppSpacing.pagePadding,
              AppSpacing.sm,
              AppSpacing.pagePadding,
              AppSpacing.md,
            ),
            child: AmitiaSegmentedControl(
              segments: const ['Agent', '团队', '待处理', '电脑', '活动'],
              selectedIndex: _tab,
              onChanged: (value) => setState(() => _tab = value),
            ),
          ),
          Expanded(
            child: _loading
                ? const Center(child: CircularProgressIndicator())
                : _error != null
                ? Center(child: Text('加载失败：$_error'))
                : RefreshIndicator(
                    onRefresh: _load,
                    child: switch (_tab) {
                      0 => _agentsView(),
                      1 => _teamsView(),
                      2 => _inboxView(),
                      3 => _runtimesView(),
                      _ => _activityView(),
                    },
                  ),
          ),
        ],
      ),
    );
  }

  Widget _agentsView() {
    return ListView(
      padding: EdgeInsets.fromLTRB(
        AppSpacing.pagePadding,
        0,
        AppSpacing.pagePadding,
        AppSpacing.pagePadding,
      ),
      children: [
        Row(
          children: [
            Expanded(
              child: _metricCard(
                'Agent',
                '${_agents.length}',
                Icons.smart_toy_outlined,
              ),
            ),
            SizedBox(width: AppSpacing.sm),
            Expanded(
              child: _metricCard(
                '运行中',
                '${_summary['running'] ?? 0}',
                Icons.bolt_outlined,
              ),
            ),
          ],
        ),
        SizedBox(height: AppSpacing.sm),
        Row(
          children: [
            Expanded(
              child: _metricCard(
                '等待',
                '${_summary['waiting'] ?? 0}',
                Icons.pending_actions_outlined,
              ),
            ),
            SizedBox(width: AppSpacing.sm),
            Expanded(
              child: _metricCard(
                '成本',
                '\$${((_summary['costMicroUsd'] as num? ?? 0) / 1000000).toStringAsFixed(4)}',
                Icons.payments_outlined,
              ),
            ),
          ],
        ),
        SizedBox(height: AppSpacing.lg),
        OutlinedButton.icon(
          onPressed: _createAgent,
          icon: const Icon(Icons.add),
          label: const Text('新建 Agent'),
        ),
        SizedBox(height: AppSpacing.sm),
        OutlinedButton.icon(
          onPressed: _importAgent,
          icon: const Icon(Icons.file_upload_outlined),
          label: const Text('导入 Agent 包'),
        ),
        SizedBox(height: AppSpacing.md),
        if (_agents.isEmpty)
          const AmitiaEmptyState(
            icon: Icons.smart_toy_outlined,
            title: '尚未创建 Agent',
          )
        else
          ..._agents.map(_agentCard),
      ],
    );
  }

  Widget _agentCard(Map<String, dynamic> agent) {
    final status = (agent['status'] ?? 'draft').toString();
    return Container(
      margin: EdgeInsets.only(bottom: AppSpacing.sm),
      padding: EdgeInsets.all(AppSpacing.md),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: BorderRadius.circular(AppRadius.medium),
        border: Border.all(color: context.borderPrimary),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              CircleAvatar(
                backgroundColor: context.accentPrimary,
                child: Text(
                  (agent['name'] ?? 'A').toString().characters.first,
                  style: const TextStyle(color: Colors.white),
                ),
              ),
              SizedBox(width: AppSpacing.sm),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      (agent['name'] ?? '').toString(),
                      style: AppTypography.cardTitle(context),
                    ),
                    Text(
                      '${_archetype(agent['archetype'])} · r${agent['revision'] ?? 1}',
                      style: AppTypography.caption(context),
                    ),
                  ],
                ),
              ),
              _statusBadge(status),
            ],
          ),
          if ((agent['description'] ?? '').toString().isNotEmpty) ...[
            SizedBox(height: AppSpacing.sm),
            Text(
              agent['description'].toString(),
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
            ),
          ],
          SizedBox(height: AppSpacing.md),
          Row(
            children: [
              Expanded(
                child: OutlinedButton(
                  onPressed: () => _publish(agent),
                  child: const Text('测试并发布'),
                ),
              ),
              SizedBox(width: AppSpacing.sm),
              OutlinedButton(
                onPressed: agent['status'] == 'active'
                    ? () => _openAgentChat(agent)
                    : null,
                child: const Text('对话'),
              ),
              SizedBox(width: AppSpacing.sm),
              IconButton.outlined(
                tooltip: '治理',
                onPressed: () => _manageAgent(agent),
                icon: const Icon(Icons.tune_outlined),
              ),
              SizedBox(width: AppSpacing.sm),
              IconButton.outlined(
                tooltip: '导出',
                onPressed: () => _exportAgent(agent),
                icon: const Icon(Icons.archive_outlined),
              ),
            ],
          ),
        ],
      ),
    );
  }

  Future<void> _manageAgent(Map<String, dynamic> agent) async {
    try {
      final policy = await _get<Map<String, dynamic>>(
        '/api/agent-platform/agents/${agent['id']}/autonomy',
      );
      if (!mounted) return;
      var level = ((policy?['level'] as num?) ?? 1).toInt();
      var revision = ((policy?['revision'] as num?) ?? 0).toInt();
      final saved = await showModalBottomSheet<bool>(
        context: context,
        isScrollControlled: true,
        backgroundColor: context.surfacePrimary,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.vertical(
            top: Radius.circular(AppRadius.large),
          ),
        ),
        builder: (sheetContext) => StatefulBuilder(
          builder: (context, setSheetState) => Padding(
            padding: EdgeInsets.fromLTRB(
              AppSpacing.pagePadding,
              AppSpacing.lg,
              AppSpacing.pagePadding,
              MediaQuery.of(context).viewInsets.bottom + AppSpacing.lg,
            ),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Text(
                  '${agent['name']} · 自治治理',
                  style: AppTypography.sectionTitle(context),
                ),
                SizedBox(height: AppSpacing.md),
                Text('自治等级 $level', style: AppTypography.body(context)),
                Slider(
                  value: level.toDouble(),
                  min: 0,
                  max: 4,
                  divisions: 4,
                  label: '$level',
                  onChanged: (value) =>
                      setSheetState(() => level = value.round()),
                ),
                Text(
                  const [
                    '仅观察',
                    '建议并等待确认',
                    '自动执行低风险动作',
                    '外部动作逐次审批',
                    '长期托管并受预算限制',
                  ][level],
                  style: AppTypography.caption(context),
                ),
                SizedBox(height: AppSpacing.sm),
                Container(
                  padding: EdgeInsets.all(AppSpacing.md),
                  decoration: BoxDecoration(
                    color: context.surfaceSecondary,
                    borderRadius: AppRadius.brMedium,
                  ),
                  child: Text(
                    '预算、域名白名单、应用授权和评估门禁在电脑端治理面板中统一管理；手机端会使用同一 Core 策略。',
                    style: AppTypography.caption(context),
                  ),
                ),
                SizedBox(height: AppSpacing.md),
                Wrap(
                  spacing: 8,
                  runSpacing: 8,
                  children: [
                    OutlinedButton(
                      onPressed: () {
                        Navigator.pop(sheetContext);
                        _editAgent(agent);
                      },
                      child: const Text('编辑配置'),
                    ),
                    OutlinedButton(
                      onPressed: () {
                        Navigator.pop(sheetContext);
                        _showAgentReleases(agent);
                      },
                      child: const Text('版本与回滚'),
                    ),
                    OutlinedButton(
                      onPressed: () {
                        Navigator.pop(sheetContext);
                        _manageCredentials(agent);
                      },
                      child: const Text('应用授权'),
                    ),
                    OutlinedButton(
                      onPressed: () {
                        Navigator.pop(sheetContext);
                        _manageNetwork(agent);
                      },
                      child: const Text('网络策略'),
                    ),
                    OutlinedButton(
                      onPressed: () {
                        Navigator.pop(sheetContext);
                        _manageBudget(agent);
                      },
                      child: const Text('预算'),
                    ),
                    OutlinedButton(
                      onPressed: () {
                        Navigator.pop(sheetContext);
                        _manageEvaluation(agent);
                      },
                      child: const Text('评估'),
                    ),
                  ],
                ),
                SizedBox(height: AppSpacing.lg),
                FilledButton(
                  onPressed: () async {
                    try {
                      await ref
                          .read(backendServiceProvider)
                          .put<dynamic>(
                            '/api/agent-platform/agents/${agent['id']}/autonomy',
                            data: {
                              'level': level,
                              'watchOnly': level == 0,
                              'autoLowRisk': level >= 2,
                              'externalApproval': level < 4,
                              'maxActionsPerDay': 20,
                              'maxCostMicroUsd': 0,
                              'quietHours': '',
                              'escalationTarget': '',
                              'triggers': <String>[],
                              'expectedRevision': revision,
                            },
                          );
                      if (sheetContext.mounted) {
                        Navigator.pop(sheetContext, true);
                      }
                    } catch (error) {
                      if (context.mounted) {
                        amitiaSnackBar(context, '保存失败：$error');
                      }
                    }
                  },
                  child: const Text('保存自治策略'),
                ),
              ],
            ),
          ),
        ),
      );
      if (saved == true && mounted) {
        amitiaSnackBar(context, '自治策略已保存');
      }
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '读取治理策略失败：$error');
    }
  }

  Future<void> _manageCredentials(Map<String, dynamic> agent) async {
    try {
      final grants = await _get<List<dynamic>>(
        '/api/agent-platform/credentials',
        query: {'agentId': agent['id']},
      );
      if (!mounted) return;
      final provider = TextEditingController();
      final account = TextEditingController();
      final secret = TextEditingController();
      final scopes = TextEditingController(text: 'agent.identity.*');
      final changed = await showModalBottomSheet<bool>(
        context: context,
        isScrollControlled: true,
        backgroundColor: context.surfacePrimary,
        builder: (sheetContext) => Padding(
          padding: EdgeInsets.fromLTRB(
            AppSpacing.pagePadding,
            AppSpacing.lg,
            AppSpacing.pagePadding,
            MediaQuery.of(sheetContext).viewInsets.bottom + AppSpacing.lg,
          ),
          child: ListView(
            shrinkWrap: true,
            children: [
              Text(
                'Agent 应用授权',
                style: AppTypography.sectionTitle(sheetContext),
              ),
              SizedBox(height: AppSpacing.md),
              ...(grants ?? const []).whereType<Map>().map(
                (grant) => ListTile(
                  contentPadding: EdgeInsets.zero,
                  title: Text(
                    '${grant['provider']} · ${grant['accountId'] ?? grant['connectionId']}',
                  ),
                  subtitle: Text('${grant['status']} · ${grant['scopeJson']}'),
                  trailing: TextButton(
                    onPressed: () async {
                      try {
                        await _post<dynamic>(
                          '/api/agent-platform/credentials/${grant['id']}/revoke',
                          data: const {},
                        );
                        if (sheetContext.mounted) {
                          Navigator.pop(sheetContext, true);
                        }
                      } catch (error) {
                        if (sheetContext.mounted) {
                          amitiaSnackBar(sheetContext, '撤销失败：$error');
                        }
                      }
                    },
                    child: const Text('撤销'),
                  ),
                ),
              ),
              SizedBox(height: AppSpacing.sm),
              TextField(
                controller: provider,
                decoration: const InputDecoration(
                  labelText: 'Provider，例如 gmail',
                ),
              ),
              TextField(
                controller: account,
                decoration: const InputDecoration(labelText: '账号 ID'),
              ),
              TextField(
                controller: secret,
                obscureText: true,
                decoration: const InputDecoration(
                  labelText: 'OAuth Token 或应用密钥',
                ),
              ),
              TextField(
                controller: scopes,
                decoration: const InputDecoration(labelText: 'Scope，逗号分隔'),
              ),
              SizedBox(height: AppSpacing.md),
              FilledButton(
                onPressed: () async {
                  if (provider.text.trim().isEmpty || secret.text.isEmpty) {
                    return;
                  }
                  try {
                    await _post<dynamic>(
                      '/api/agent-platform/credentials',
                      data: {
                        'agentId': agent['id'],
                        'connectionId':
                            'mobile-${DateTime.now().microsecondsSinceEpoch}',
                        'provider': provider.text.trim(),
                        'accountId': account.text.trim(),
                        'scopes': scopes.text
                            .split(',')
                            .map((item) => item.trim())
                            .where((item) => item.isNotEmpty)
                            .toList(growable: false),
                        'secretValue': secret.text,
                      },
                    );
                    if (sheetContext.mounted) Navigator.pop(sheetContext, true);
                  } catch (error) {
                    if (sheetContext.mounted) {
                      amitiaSnackBar(sheetContext, '授权失败：$error');
                    }
                  }
                },
                child: const Text('保存应用授权'),
              ),
            ],
          ),
        ),
      );
      provider.dispose();
      account.dispose();
      secret.dispose();
      scopes.dispose();
      if (changed == true && mounted) {
        amitiaSnackBar(context, 'Agent 应用授权已更新');
      }
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '读取应用授权失败：$error');
    }
  }

  Future<void> _manageNetwork(Map<String, dynamic> agent) async {
    try {
      final policies = await _get<List<dynamic>>(
        '/api/agent-platform/network-policies',
        query: {'agentId': agent['id']},
      );
      final existing = (policies ?? const [])
          .whereType<Map>()
          .map((item) => Map<String, dynamic>.from(item))
          .where((item) => item['agentId'] == agent['id'])
          .firstOrNull;
      final domain = await _askText('新增网络策略', '允许域名，例如 api.example.com');
      if (domain == null || domain.trim().isEmpty) return;
      if (existing == null) {
        await _post<dynamic>(
          '/api/agent-platform/network-policies',
          data: {
            'agentId': agent['id'],
            'name': '${agent['name']} 网络策略',
            'defaultAction': 'deny',
            'allowDomains': [domain.trim()],
            'denyDomains': const <String>[],
            'allowedPorts': const [443],
            'uploadPolicy': 'approval',
            'sensitiveDataPolicy': 'block',
          },
        );
      } else {
        final allowDomains = {
          ..._jsonList(existing['allowDomains']).map((item) => item.toString()),
          domain.trim(),
        }.toList(growable: false);
        await _put<dynamic>(
          '/api/agent-platform/network-policies/${existing['id']}',
          data: {
            'name': existing['name'],
            'defaultAction': existing['defaultAction'],
            'allowDomains': allowDomains,
            'denyDomains': _jsonList(existing['denyDomains']),
            'allowedPorts': _jsonList(existing['allowedPorts']),
            'uploadPolicy': existing['uploadPolicy'],
            'sensitiveDataPolicy': existing['sensitiveDataPolicy'],
            'expectedRevision': existing['revision'],
          },
        );
      }
      if (mounted) {
        amitiaSnackBar(context, existing == null ? '网络策略已创建' : '网络策略已更新');
      }
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '保存网络策略失败：$error');
    }
  }

  List<dynamic> _jsonList(Object? value) {
    if (value is List) return value;
    final decoded = jsonDecode(
      value?.toString().isNotEmpty == true ? value.toString() : '[]',
    );
    if (decoded is! List) throw const FormatException('网络策略字段格式无效');
    return decoded;
  }

  Future<void> _manageBudget(Map<String, dynamic> agent) async {
    try {
      final policies = await _get<List<dynamic>>(
        '/api/agent-platform/budgets',
        query: {'scopeType': 'agent', 'scopeId': agent['id']},
      );
      final tokenLimit = await _askText(
        'Agent 预算',
        '每日 Token 上限',
        initial: '100000',
      );
      if (tokenLimit == null || int.tryParse(tokenLimit) == null) return;
      await _post<dynamic>(
        '/api/agent-platform/budgets',
        data: {
          'scopeType': 'agent',
          'scopeId': agent['id'],
          'period': 'daily',
          'tokenLimit': int.parse(tokenLimit),
          'costLimitMicroUsd': 1000000,
          'toolCallLimit': 200,
          'concurrencyLimit': 2,
          'warningPercent': 80,
          'hardStop': true,
        },
      );
      if (mounted) {
        amitiaSnackBar(
          context,
          '预算已保存，现有 ${(policies ?? const []).length} 条策略',
        );
      }
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '保存预算失败：$error');
    }
  }

  Future<void> _manageEvaluation(Map<String, dynamic> agent) async {
    try {
      var suites = await _get<List<dynamic>>(
        '/api/agent-platform/evaluations/suites',
        query: {'agentId': agent['id']},
      );
      if ((suites ?? const []).isEmpty) {
        final suite = await _post<Map<String, dynamic>>(
          '/api/agent-platform/evaluations/suites',
          data: {
            'agentId': agent['id'],
            'name': '${agent['name']} 发布门禁',
            'description': '移动端创建的真实 Agent 评估门禁',
            'config': const <String, dynamic>{},
            'gatePolicy': const {
              'taskSuccessRate': 0.8,
              'toolAccuracy': 0.8,
              'falseSuccessRate': 0.1,
              'recoveryRate': 0.8,
            },
          },
        );
        final suiteID = (suite?['id'] ?? '').toString();
        if (suiteID.isEmpty) throw StateError('评估套件创建失败');
        await _post<dynamic>(
          '/api/agent-platform/evaluations/suites/$suiteID/cases',
          data: {
            'name': '基础可用性',
            'category': 'general',
            'input': {'message': '请用一句话说明你已准备好执行当前任务。'},
            'expected': const <String, dynamic>{},
            'safetyCritical': false,
            'weight': 1,
          },
        );
        suites = [suite];
      }
      if (!mounted) return;
      final suite = await showModalBottomSheet<Map<String, dynamic>>(
        context: context,
        backgroundColor: context.surfacePrimary,
        builder: (sheetContext) => SafeArea(
          child: ListView(
            padding: EdgeInsets.all(AppSpacing.pagePadding),
            children: [
              Text('评估门禁', style: AppTypography.sectionTitle(sheetContext)),
              ...(suites ?? const []).whereType<Map>().map(
                (item) => ListTile(
                  title: Text((item['name'] ?? '').toString()),
                  subtitle: Text((item['description'] ?? '').toString()),
                  trailing: const Icon(Icons.play_arrow),
                  onTap: () => Navigator.pop(
                    sheetContext,
                    Map<String, dynamic>.from(item),
                  ),
                ),
              ),
            ],
          ),
        ),
      );
      if (suite == null) return;
      final result = await _post<Map<String, dynamic>>(
        '/api/agent-platform/evaluations/suites/${suite['id']}/run',
        data: {
          'revisionId': agent['currentRevisionId'],
          'results': const <dynamic>[],
        },
      );
      if (mounted) {
        amitiaSnackBar(
          context,
          '评估完成：${result?['gateStatus'] ?? result?['status'] ?? 'completed'}',
        );
      }
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '评估失败：$error');
    }
  }

  void _openAgentChat(Map<String, dynamic> agent) {
    final characterID = (agent['characterId'] ?? '').toString();
    if (characterID.isEmpty) return;
    context.go('/chat?characterId=${Uri.encodeQueryComponent(characterID)}');
  }

  Future<void> _exportAgent(Map<String, dynamic> agent) async {
    final name = (agent['name'] ?? 'agent').toString().replaceAll(
      RegExp(r'[\\/:*?"<>|\r\n]+'),
      '-',
    );
    final output = await FilePicker.platform.saveFile(
      dialogTitle: '导出 Agent 可携带包',
      fileName: '$name.amitia-agent.zip',
      type: FileType.custom,
      allowedExtensions: const ['zip'],
    );
    if (output == null || output.isEmpty) return;
    try {
      final stream = await ref
          .read(backendServiceProvider)
          .getStream(
            '/api/agent-platform/agents/${agent['id']}/export-archive',
            queryParameters: const {'version': 'mobile'},
          );
      final sink = File(output).openWrite();
      await for (final chunk in stream) {
        sink.add(chunk);
      }
      await sink.close();
      if (mounted) amitiaSnackBar(context, 'Agent 可携带包已导出');
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '导出失败：$error');
    }
  }

  Future<void> _importAgent() async {
    final picked = await FilePicker.platform.pickFiles(
      dialogTitle: '导入 Agent 可携带包',
      type: FileType.custom,
      allowedExtensions: const ['zip', 'json', 'amitia-agent'],
    );
    final path = picked?.files.single.path;
    if (path == null || path.isEmpty) return;
    try {
      if (path.toLowerCase().endsWith('.zip')) {
        final form = FormData.fromMap({
          'package': await MultipartFile.fromFile(path),
        });
        await _post<dynamic>(
          '/api/agent-platform/agents/import-archive',
          data: form,
        );
      } else {
        final payload = jsonDecode(await File(path).readAsString());
        await _post<dynamic>(
          '/api/agent-platform/agents/import',
          data: payload,
        );
      }
      await _load();
      if (mounted) amitiaSnackBar(context, 'Agent 包已导入');
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '导入失败：$error');
    }
  }

  Future<void> _createTeam() async {
    final name = TextEditingController();
    final description = TextEditingController();
    final templates = await _get<List<dynamic>>(
      '/api/agent-platform/teams/templates',
    );
    if (!mounted) return;
    var selectedTemplate = '';
    final created = await showModalBottomSheet<bool>(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(
          top: Radius.circular(AppRadius.large),
        ),
      ),
      builder: (sheetContext) => StatefulBuilder(
        builder: (context, setSheetState) => Padding(
          padding: EdgeInsets.fromLTRB(
            AppSpacing.pagePadding,
            AppSpacing.lg,
            AppSpacing.pagePadding,
            MediaQuery.of(context).viewInsets.bottom + AppSpacing.lg,
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text('新建团队', style: AppTypography.sectionTitle(context)),
              SizedBox(height: AppSpacing.md),
              DropdownButtonFormField<String>(
                initialValue: selectedTemplate,
                decoration: const InputDecoration(labelText: '团队模板'),
                items: [
                  const DropdownMenuItem(value: '', child: Text('空白团队')),
                  ...(templates ?? const []).whereType<Map>().map(
                    (item) => DropdownMenuItem(
                      value: (item['id'] ?? '').toString(),
                      child: Text((item['name'] ?? '').toString()),
                    ),
                  ),
                ],
                onChanged: (value) {
                  final template = (templates ?? const [])
                      .whereType<Map>()
                      .where((item) => item['id']?.toString() == value);
                  setSheetState(() {
                    selectedTemplate = value ?? '';
                    if (template.isNotEmpty) {
                      name.text = (template.first['name'] ?? '').toString();
                      description.text = (template.first['description'] ?? '')
                          .toString();
                    }
                  });
                },
              ),
              SizedBox(height: AppSpacing.sm),
              TextField(
                controller: name,
                decoration: const InputDecoration(labelText: '团队名称'),
              ),
              SizedBox(height: AppSpacing.sm),
              TextField(
                controller: description,
                maxLines: 2,
                decoration: const InputDecoration(labelText: '团队说明'),
              ),
              SizedBox(height: AppSpacing.lg),
              FilledButton(
                onPressed: () async {
                  if (name.text.trim().isEmpty) return;
                  try {
                    await _post(
                      selectedTemplate.isEmpty
                          ? '/api/agent-platform/teams'
                          : '/api/agent-platform/teams/from-template',
                      data: {
                        if (selectedTemplate.isNotEmpty)
                          'templateId': selectedTemplate,
                        'name': name.text.trim(),
                        'description': description.text.trim(),
                        'sharedMemory': true,
                        'sharedWorkspace': true,
                      },
                    );
                    if (sheetContext.mounted) Navigator.pop(sheetContext, true);
                  } catch (error) {
                    if (sheetContext.mounted) {
                      amitiaSnackBar(sheetContext, '创建失败：$error');
                    }
                  }
                },
                child: const Text('创建团队'),
              ),
            ],
          ),
        ),
      ),
    );
    name.dispose();
    description.dispose();
    if (created == true) await _load();
  }

  Future<void> _openTeam(Map<String, dynamic> team) async {
    try {
      final detail = await _get<Map<String, dynamic>>(
        '/api/agent-platform/teams/${team['id']}',
      );
      if (!mounted || detail == null) return;
      final action = await showModalBottomSheet<String>(
        context: context,
        isScrollControlled: true,
        backgroundColor: context.surfacePrimary,
        builder: (sheetContext) => SafeArea(
          child: ListView(
            padding: EdgeInsets.all(AppSpacing.pagePadding),
            children: [
              Text(
                (team['name'] ?? '').toString(),
                style: AppTypography.sectionTitle(sheetContext),
              ),
              Text(
                (team['description'] ?? '').toString(),
                style: AppTypography.caption(sheetContext),
              ),
              SizedBox(height: AppSpacing.md),
              Wrap(
                spacing: 8,
                runSpacing: 8,
                children: [
                  ActionChip(
                    label: const Text('添加共享上下文'),
                    onPressed: () => Navigator.pop(sheetContext, 'context'),
                  ),
                  ActionChip(
                    label: const Text('加入 Agent'),
                    onPressed: () => Navigator.pop(sheetContext, 'agent'),
                  ),
                  ActionChip(
                    label: const Text('邀请人类成员'),
                    onPressed: () => Navigator.pop(sheetContext, 'invite'),
                  ),
                  ActionChip(
                    label: const Text('团队预算'),
                    onPressed: () => Navigator.pop(sheetContext, 'budget'),
                  ),
                  ActionChip(
                    label: const Text('测试路由'),
                    onPressed: () => Navigator.pop(sheetContext, 'route'),
                  ),
                ],
              ),
              SizedBox(height: AppSpacing.md),
              Text('成员', style: AppTypography.cardTitle(sheetContext)),
              ...((detail['members'] as List?) ?? const [])
                  .whereType<Map>()
                  .map(
                    (member) => ListTile(
                      contentPadding: EdgeInsets.zero,
                      title: Text(
                        '${member['memberType']} · ${member['memberId']}',
                      ),
                      trailing: Text((member['role'] ?? '').toString()),
                    ),
                  ),
              SizedBox(height: AppSpacing.md),
              Text('共享上下文', style: AppTypography.cardTitle(sheetContext)),
              ...((detail['contexts'] as List?) ?? const [])
                  .whereType<Map>()
                  .map(
                    (item) => ListTile(
                      contentPadding: EdgeInsets.zero,
                      title: Text('${item['kind']} ${item['key'] ?? ''}'),
                      subtitle: Text((item['content'] ?? '').toString()),
                    ),
                  ),
            ],
          ),
        ),
      );
      switch (action) {
        case 'context':
          await _addTeamContext(team);
          break;
        case 'agent':
          await _addAgentToTeam(team);
          break;
        case 'invite':
          await _inviteMember((team['id'] ?? '').toString());
          break;
        case 'budget':
          await _configureTeamBudget(team);
          break;
        case 'route':
          await _testTeamRoute(team);
          break;
      }
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '读取团队失败：$error');
    }
  }

  Future<void> _addTeamContext(Map<String, dynamic> team) async {
    final content = await _askText('团队共享上下文', '规则、决策或知识', maxLines: 4);
    if (content == null || content.trim().isEmpty) return;
    final kind = await _askText('上下文类型', '类型', initial: 'decision');
    if (kind == null || kind.trim().isEmpty) return;
    try {
      await _post<dynamic>(
        '/api/agent-platform/teams/${team['id']}/contexts',
        data: {
          'kind': kind.trim(),
          'key': 'mobile-${DateTime.now().millisecondsSinceEpoch}',
          'content': content.trim(),
          'visibility': 'team',
        },
      );
      if (mounted) amitiaSnackBar(context, '团队共享上下文已保存');
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '保存上下文失败：$error');
    }
  }

  Future<void> _addAgentToTeam(Map<String, dynamic> team) async {
    if (_agents.isEmpty || !mounted) return;
    final agent = await showModalBottomSheet<Map<String, dynamic>>(
      context: context,
      backgroundColor: context.surfacePrimary,
      builder: (sheetContext) => SafeArea(
        child: ListView(
          children: _agents
              .map(
                (item) => ListTile(
                  title: Text((item['name'] ?? '').toString()),
                  subtitle: Text((item['status'] ?? '').toString()),
                  onTap: () => Navigator.pop(sheetContext, item),
                ),
              )
              .toList(growable: false),
        ),
      ),
    );
    if (agent == null) return;
    try {
      await _post<dynamic>(
        '/api/agent-platform/teams/${team['id']}/members',
        data: {
          'memberType': 'agent',
          'memberId': agent['id'],
          'role': 'member',
          'responderMode': 'mention_or_lead',
          'capabilities': const ['*'],
        },
      );
      await _load();
      if (mounted) amitiaSnackBar(context, 'Agent 已加入团队');
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '加入团队失败：$error');
    }
  }

  Future<void> _inviteMember([String teamID = '']) async {
    final email = await _askText('邀请人类成员', '邮箱');
    if (email == null || email.trim().isEmpty) return;
    final role = await _askText(
      '成员角色',
      'owner/admin/operator/member/viewer',
      initial: 'member',
    );
    if (role == null || role.trim().isEmpty) return;
    try {
      final result = await _post<Map<String, dynamic>>(
        '/api/agent-platform/users/invitations',
        data: {'email': email.trim(), 'role': role.trim(), 'teamId': teamID},
      );
      if (!mounted) return;
      await showDialog<void>(
        context: context,
        builder: (dialogContext) => AlertDialog(
          title: const Text('邀请已创建'),
          content: SelectableText((result?['token'] ?? '').toString()),
          actions: [
            FilledButton(
              onPressed: () => Navigator.pop(dialogContext),
              child: const Text('关闭'),
            ),
          ],
        ),
      );
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '邀请失败：$error');
    }
  }

  Future<void> _configureTeamBudget(Map<String, dynamic> team) async {
    final value = await _askText('团队预算', '每日 Token 上限', initial: '1000000');
    final limit = int.tryParse(value ?? '');
    if (limit == null) return;
    try {
      await _post<dynamic>(
        '/api/agent-platform/budgets',
        data: {
          'scopeType': 'team',
          'scopeId': team['id'],
          'period': 'daily',
          'tokenLimit': limit,
          'costLimitMicroUsd': 2000000,
          'toolCallLimit': 500,
          'concurrencyLimit': 4,
          'warningPercent': 80,
          'hardStop': true,
        },
      );
      if (mounted) amitiaSnackBar(context, '团队预算已保存');
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '保存团队预算失败：$error');
    }
  }

  Future<void> _testTeamRoute(Map<String, dynamic> team) async {
    final message = await _askText('团队消息路由', '测试消息');
    if (message == null || message.trim().isEmpty) return;
    try {
      final result = await _post<Map<String, dynamic>>(
        '/api/agent-platform/teams/${team['id']}/route',
        data: {
          'message': message.trim(),
          'conversationId':
              'mobile-team-route-${DateTime.now().millisecondsSinceEpoch}',
        },
      );
      if (!mounted) return;
      await showDialog<void>(
        context: context,
        builder: (dialogContext) => AlertDialog(
          title: const Text('路由结果'),
          content: SingleChildScrollView(
            child: SelectableText(
              const JsonEncoder.withIndent('  ').convert(result ?? const {}),
            ),
          ),
          actions: [
            FilledButton(
              onPressed: () => Navigator.pop(dialogContext),
              child: const Text('关闭'),
            ),
          ],
        ),
      );
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '团队路由失败：$error');
    }
  }

  Future<void> _decideApproval(
    Map<String, dynamic> approval,
    String decision,
  ) async {
    try {
      await _post(
        '/api/agent-platform/approvals/${approval['id']}/decide',
        data: {
          'decision': decision,
          'grantMode': 'once',
          'reason': '',
          'expectedRevision': (approval['revision'] as num?)?.toInt() ?? 0,
        },
      );
      await _load();
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '审批失败：$error');
    }
  }

  Future<void> _resolveReconciliation(Map<String, dynamic> item) async {
    try {
      await _post(
        '/api/agent-platform/reconciliations/${item['id']}/resolve',
        data: {
          'resolution': '用户已人工核对执行结果',
          'expectedRevision': (item['revision'] as num?)?.toInt() ?? 0,
        },
      );
      await _load();
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '对账失败：$error');
    }
  }

  Future<void> _runtimeAction(
    Map<String, dynamic> runtime,
    String action,
  ) async {
    try {
      await _post(
        '/api/agent-platform/runtimes/${runtime['id']}/$action',
        data: const {},
      );
      await _load();
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '操作失败：$error');
    }
  }

  Future<void> _snapshotRuntime(Map<String, dynamic> runtime) async {
    try {
      await _post(
        '/api/agent-platform/runtimes/${runtime['id']}/snapshots',
        data: {'name': 'mobile-${DateTime.now().millisecondsSinceEpoch}'},
      );
      if (mounted) amitiaSnackBar(context, '快照已创建');
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '快照失败：$error');
    }
  }

  Future<void> _showRuntimeTerminal(Map<String, dynamic> runtime) async {
    final command = TextEditingController(text: 'pwd');
    final workingDir = TextEditingController();
    final timeout = TextEditingController(text: '60');
    String output = '';
    String? error;
    int? exitCode;
    int? durationMs;
    bool running = false;
    try {
      await showDialog<void>(
        context: context,
        builder: (dialogContext) => StatefulBuilder(
          builder: (dialogContext, setDialogState) => AlertDialog(
            title: Text('${_agentName(runtime['agentId'].toString())} 终端'),
            content: SizedBox(
              width: double.maxFinite,
              child: SingleChildScrollView(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    TextField(
                      controller: command,
                      minLines: 3,
                      maxLines: 6,
                      decoration: const InputDecoration(
                        labelText: '命令',
                        hintText: 'pwd',
                      ),
                    ),
                    SizedBox(height: AppSpacing.sm),
                    TextField(
                      controller: workingDir,
                      decoration: const InputDecoration(
                        labelText: '工作目录',
                        hintText: 'workspace 内的相对路径',
                      ),
                    ),
                    SizedBox(height: AppSpacing.sm),
                    TextField(
                      controller: timeout,
                      keyboardType: TextInputType.number,
                      decoration: const InputDecoration(labelText: '超时秒数'),
                    ),
                    SizedBox(height: AppSpacing.md),
                    Container(
                      width: double.infinity,
                      padding: EdgeInsets.all(AppSpacing.sm),
                      decoration: BoxDecoration(
                        color: context.surfaceSecondary,
                        borderRadius: AppRadius.brMedium,
                        border: Border.all(color: context.borderPrimary),
                      ),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            exitCode == null
                                ? '命令输出'
                                : '退出码 $exitCode · ${durationMs ?? 0} ms · ${runtime['provider']}',
                            style: AppTypography.caption(context),
                          ),
                          SizedBox(height: 6),
                          SelectableText(
                            error ?? (output.isEmpty ? '尚无输出' : output),
                            style: AppTypography.caption(context).copyWith(
                              color: error == null ? null : context.error,
                              fontFamily: 'monospace',
                            ),
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
              ),
            ),
            actions: [
              TextButton(
                onPressed: running ? null : () => Navigator.pop(dialogContext),
                child: const Text('关闭'),
              ),
              FilledButton(
                onPressed: running
                    ? null
                    : () async {
                        final selectedCommand = command.text.trim();
                        if (selectedCommand.isEmpty) return;
                        setDialogState(() {
                          running = true;
                          output = '';
                          error = null;
                          exitCode = null;
                          durationMs = null;
                        });
                        try {
                          final result = await _post<Map<String, dynamic>>(
                            '/api/agent-platform/runtimes/${runtime['id']}/exec',
                            data: {
                              'command': selectedCommand,
                              'workingDir': workingDir.text.trim(),
                              'timeoutSeconds':
                                  int.tryParse(timeout.text.trim()) ?? 60,
                            },
                          );
                          if (!dialogContext.mounted) return;
                          setDialogState(() {
                            output = (result?['output'] ?? '').toString();
                            exitCode = (result?['exitCode'] as num?)?.toInt();
                            durationMs = (result?['durationMs'] as num?)
                                ?.toInt();
                          });
                        } catch (caught) {
                          if (!dialogContext.mounted) return;
                          setDialogState(() => error = caught.toString());
                        } finally {
                          if (dialogContext.mounted) {
                            setDialogState(() => running = false);
                          }
                        }
                      },
                child: running
                    ? const SizedBox.square(
                        dimension: 18,
                        child: CircularProgressIndicator(strokeWidth: 2),
                      )
                    : const Text('执行'),
              ),
            ],
          ),
        ),
      );
    } finally {
      command.dispose();
      workingDir.dispose();
      timeout.dispose();
    }
  }

  Future<void> _takeOverDesktop(Map<String, dynamic> runtime) async {
    try {
      final session = await _post<Map<String, dynamic>>(
        '/api/agent-platform/host-control',
        data: {
          'agentId': runtime['agentId'],
          'capabilities': [
            'screen.capture',
            'input.mouse',
            'input.keyboard',
            'window.control',
          ],
          'permission': <String, dynamic>{},
          'expiresInSeconds': 1800,
        },
      );
      final sessionId = (session?['id'] ?? '').toString();
      if (sessionId.isEmpty) throw StateError('桌面接管会话无效');
      await _post<Map<String, dynamic>>(
        '/api/agent-platform/host-control/$sessionId/approve',
        data: const {},
      );
      if (!mounted) return;
      await showDialog<void>(
        context: context,
        builder: (context) => _HostControlDialog(sessionId: sessionId),
      );
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '桌面接管失败：$error');
    }
  }

  Future<void> _createRuntime() async {
    if (_agents.isEmpty) {
      amitiaSnackBar(context, '请先创建 Agent');
      return;
    }
    var agentId = (_agents.first['id'] ?? '').toString();
    var provider = 'isolated';
    final endpoint = TextEditingController();
    final created = await showModalBottomSheet<bool>(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(
          top: Radius.circular(AppRadius.large),
        ),
      ),
      builder: (sheetContext) => StatefulBuilder(
        builder: (context, setSheetState) => Padding(
          padding: EdgeInsets.fromLTRB(
            AppSpacing.pagePadding,
            AppSpacing.lg,
            AppSpacing.pagePadding,
            MediaQuery.of(context).viewInsets.bottom + AppSpacing.lg,
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text('创建 Agent 电脑', style: AppTypography.sectionTitle(context)),
              SizedBox(height: AppSpacing.md),
              DropdownButtonFormField<String>(
                initialValue: agentId,
                decoration: const InputDecoration(labelText: 'Agent'),
                items: _agents
                    .map(
                      (agent) => DropdownMenuItem(
                        value: agent['id'].toString(),
                        child: Text((agent['name'] ?? '').toString()),
                      ),
                    )
                    .toList(),
                onChanged: (value) {
                  if (value != null) setSheetState(() => agentId = value);
                },
              ),
              SizedBox(height: AppSpacing.sm),
              DropdownButtonFormField<String>(
                initialValue: provider,
                decoration: const InputDecoration(labelText: '运行时'),
                items: const [
                  DropdownMenuItem(value: 'local', child: Text('本机工作区')),
                  DropdownMenuItem(value: 'isolated', child: Text('隔离容器（推荐）')),
                  DropdownMenuItem(value: 'oci', child: Text('OCI 容器')),
                  DropdownMenuItem(value: 'remote', child: Text('远程电脑')),
                  DropdownMenuItem(value: 'android', child: Text('Android')),
                ],
                onChanged: (value) {
                  if (value != null) setSheetState(() => provider = value);
                },
              ),
              if (provider == 'remote') ...[
                SizedBox(height: AppSpacing.sm),
                TextField(
                  controller: endpoint,
                  keyboardType: TextInputType.url,
                  decoration: const InputDecoration(
                    labelText: '远程 Endpoint',
                    hintText: 'https://agent.example.com/runtime',
                  ),
                ),
              ],
              SizedBox(height: AppSpacing.lg),
              FilledButton(
                onPressed: () async {
                  try {
                    final runtimePayload = {
                      'agentId': agentId,
                      'provider': provider,
                      'image': ['oci', 'isolated'].contains(provider)
                          ? 'ubuntu:24.04'
                          : '',
                      'cpuQuota': 1,
                      'memoryBytes': 1024 * 1024 * 1024,
                      'diskBytes': 10 * 1024 * 1024 * 1024,
                      'processLimit': 64,
                      'idleTimeoutSeconds': 1800,
                    };
                    if (provider == 'remote') {
                      await _post(
                        '/api/agent-platform/runtimes/remote',
                        data: {
                          'runtime': runtimePayload,
                          'endpoint': endpoint.text.trim(),
                        },
                      );
                    } else {
                      await _post(
                        '/api/agent-platform/runtimes',
                        data: runtimePayload,
                      );
                    }
                    if (sheetContext.mounted) Navigator.pop(sheetContext, true);
                  } catch (error) {
                    if (context.mounted) {
                      amitiaSnackBar(context, '创建失败：$error');
                    }
                  }
                },
                child: const Text('创建电脑'),
              ),
            ],
          ),
        ),
      ),
    );
    endpoint.dispose();
    if (created == true) await _load();
  }

  Widget _teamsView() {
    return ListView(
      padding: EdgeInsets.fromLTRB(
        AppSpacing.pagePadding,
        0,
        AppSpacing.pagePadding,
        AppSpacing.pagePadding,
      ),
      children: [
        Row(
          children: [
            Expanded(
              child: OutlinedButton.icon(
                onPressed: _createTeam,
                icon: const Icon(Icons.group_add_outlined),
                label: const Text('新建团队'),
              ),
            ),
            SizedBox(width: AppSpacing.sm),
            Expanded(
              child: OutlinedButton.icon(
                onPressed: _inviteMember,
                icon: const Icon(Icons.person_add_alt_1_outlined),
                label: const Text('邀请成员'),
              ),
            ),
          ],
        ),
        SizedBox(height: AppSpacing.md),
        if (_teams.isEmpty)
          const AmitiaEmptyState(icon: Icons.groups_outlined, title: '尚未创建团队')
        else
          ..._teams.map(
            (team) => Container(
              margin: EdgeInsets.only(bottom: AppSpacing.sm),
              padding: EdgeInsets.all(AppSpacing.md),
              decoration: BoxDecoration(
                color: context.surfacePrimary,
                borderRadius: AppRadius.brMedium,
                border: Border.all(color: context.borderPrimary, width: 0.6),
              ),
              child: InkWell(
                onTap: () => _openTeam(team),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      (team['name'] ?? '').toString(),
                      style: AppTypography.cardTitle(context),
                    ),
                    SizedBox(height: 4),
                    Text(
                      (team['description'] ?? '暂无团队说明').toString(),
                      style: AppTypography.caption(context),
                    ),
                    SizedBox(height: AppSpacing.sm),
                    Wrap(
                      spacing: 8,
                      children: [
                        _metaChip(
                          team['sharedMemory'] == true ? '共享记忆' : '独立记忆',
                        ),
                        _metaChip(
                          team['sharedWorkspace'] == true ? '共享文件' : '独立文件',
                        ),
                        _metaChip('rev ${team['revision'] ?? 1}'),
                      ],
                    ),
                  ],
                ),
              ),
            ),
          ),
      ],
    );
  }

  Widget _inboxView() {
    return ListView(
      padding: EdgeInsets.fromLTRB(
        AppSpacing.pagePadding,
        0,
        AppSpacing.pagePadding,
        AppSpacing.pagePadding,
      ),
      children: [
        Text('待审批', style: AppTypography.sectionTitle(context)),
        SizedBox(height: AppSpacing.sm),
        if (_approvals.isEmpty)
          const AmitiaEmptyState(
            icon: Icons.pending_actions_outlined,
            title: '没有待审批操作',
          )
        else
          ..._approvals.map(
            (approval) => Container(
              margin: EdgeInsets.only(bottom: AppSpacing.sm),
              padding: EdgeInsets.all(AppSpacing.md),
              decoration: BoxDecoration(
                color: context.surfacePrimary,
                borderRadius: AppRadius.brMedium,
                border: Border.all(
                  color: context.warning.withValues(alpha: 0.4),
                ),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    (approval['actionSummary'] ?? '').toString(),
                    style: AppTypography.body(context),
                  ),
                  SizedBox(height: 4),
                  Text(
                    (approval['resourceSummary'] ?? approval['toolName'] ?? '')
                        .toString(),
                    style: AppTypography.caption(context),
                  ),
                  SizedBox(height: AppSpacing.sm),
                  Row(
                    children: [
                      Expanded(
                        child: OutlinedButton(
                          onPressed: () => _decideApproval(approval, 'denied'),
                          child: const Text('拒绝'),
                        ),
                      ),
                      SizedBox(width: AppSpacing.sm),
                      Expanded(
                        child: FilledButton(
                          onPressed: () =>
                              _decideApproval(approval, 'approved'),
                          child: const Text('允许一次'),
                        ),
                      ),
                    ],
                  ),
                ],
              ),
            ),
          ),
        SizedBox(height: AppSpacing.lg),
        Text('待对账', style: AppTypography.sectionTitle(context)),
        SizedBox(height: AppSpacing.sm),
        if (_reconciliations.isEmpty)
          const AmitiaEmptyState(
            icon: Icons.rule_folder_outlined,
            title: '没有待对账记录',
          )
        else
          ..._reconciliations.map(
            (item) => Container(
              margin: EdgeInsets.only(bottom: AppSpacing.sm),
              padding: EdgeInsets.all(AppSpacing.md),
              decoration: BoxDecoration(
                color: context.surfacePrimary,
                borderRadius: AppRadius.brMedium,
                border: Border.all(
                  color: context.error.withValues(alpha: 0.35),
                ),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    (item['reason'] ?? '').toString(),
                    style: AppTypography.body(context),
                  ),
                  SizedBox(height: 4),
                  Text(
                    (item['originalError'] ?? '没有附加原始错误').toString(),
                    style: AppTypography.caption(
                      context,
                    ).copyWith(color: context.error),
                  ),
                  SizedBox(height: AppSpacing.sm),
                  Align(
                    alignment: Alignment.centerRight,
                    child: OutlinedButton(
                      onPressed: () => _resolveReconciliation(item),
                      child: const Text('标记已核对'),
                    ),
                  ),
                ],
              ),
            ),
          ),
      ],
    );
  }

  Widget _runtimesView() {
    return ListView(
      padding: EdgeInsets.fromLTRB(
        AppSpacing.pagePadding,
        0,
        AppSpacing.pagePadding,
        AppSpacing.pagePadding,
      ),
      children: [
        OutlinedButton.icon(
          onPressed: _createRuntime,
          icon: const Icon(Icons.add_to_queue_outlined),
          label: const Text('创建 Agent 电脑'),
        ),
        SizedBox(height: AppSpacing.md),
        if (_runtimes.isEmpty)
          const AmitiaEmptyState(
            icon: Icons.computer_outlined,
            title: '尚未创建 Agent 电脑',
          )
        else
          ..._runtimes.map(
            (runtime) => Container(
              margin: EdgeInsets.only(bottom: AppSpacing.sm),
              padding: EdgeInsets.all(AppSpacing.md),
              decoration: BoxDecoration(
                color: context.surfacePrimary,
                borderRadius: AppRadius.brMedium,
                border: Border.all(color: context.borderPrimary, width: 0.6),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Expanded(
                        child: Text(
                          _agentName((runtime['agentId'] ?? '').toString()),
                          style: AppTypography.cardTitle(context),
                        ),
                      ),
                      _statusBadge((runtime['state'] ?? 'created').toString()),
                    ],
                  ),
                  SizedBox(height: 4),
                  Text(
                    '${runtime['provider']} · ${(runtime['cpuQuota'] ?? 1)} CPU',
                    style: AppTypography.caption(context),
                  ),
                  SizedBox(height: AppSpacing.sm),
                  Wrap(
                    spacing: 8,
                    runSpacing: 8,
                    children: [
                      if (runtime['state'] == 'paused')
                        OutlinedButton(
                          onPressed: () => _runtimeAction(runtime, 'resume'),
                          child: const Text('恢复'),
                        )
                      else
                        OutlinedButton(
                          onPressed: runtime['state'] == 'running'
                              ? () => _runtimeAction(runtime, 'pause')
                              : () => _runtimeAction(runtime, 'start'),
                          child: Text(
                            runtime['state'] == 'running' ? '暂停' : '启动',
                          ),
                        ),
                      OutlinedButton(
                        onPressed: () => _runtimeAction(runtime, 'stop'),
                        child: const Text('停止'),
                      ),
                      OutlinedButton(
                        onPressed: () => _snapshotRuntime(runtime),
                        child: const Text('快照'),
                      ),
                      OutlinedButton(
                        onPressed: runtime['state'] == 'running'
                            ? () => _showRuntimeTerminal(runtime)
                            : null,
                        child: const Text('终端'),
                      ),
                      OutlinedButton(
                        onPressed: () => _takeOverDesktop(runtime),
                        child: const Text('接管桌面'),
                      ),
                    ],
                  ),
                ],
              ),
            ),
          ),
      ],
    );
  }

  Widget _activityView() {
    if (_activity.isEmpty) {
      return const AmitiaEmptyState(icon: Icons.timeline, title: '暂无 Agent 活动');
    }
    return ListView.builder(
      padding: EdgeInsets.fromLTRB(
        AppSpacing.pagePadding,
        0,
        AppSpacing.pagePadding,
        AppSpacing.pagePadding,
      ),
      itemCount: _activity.length,
      itemBuilder: (context, index) {
        final item = _activity[index];
        return Container(
          margin: EdgeInsets.only(bottom: AppSpacing.sm),
          padding: EdgeInsets.all(AppSpacing.md),
          decoration: BoxDecoration(
            color: context.surfacePrimary,
            borderRadius: BorderRadius.circular(AppRadius.medium),
            border: Border.all(color: context.borderPrimary),
          ),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Container(
                width: 8,
                height: 8,
                margin: const EdgeInsets.only(top: 6),
                decoration: BoxDecoration(
                  shape: BoxShape.circle,
                  color: _stateColor(item['state']?.toString() ?? ''),
                ),
              ),
              SizedBox(width: AppSpacing.sm),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      (item['title'] ?? '').toString(),
                      style: AppTypography.body(context),
                    ),
                    SizedBox(height: 3),
                    Text(
                      (item['detail'] ??
                              item['waitingReason'] ??
                              item['kind'] ??
                              '')
                          .toString(),
                      style: AppTypography.caption(context),
                    ),
                    SizedBox(height: 5),
                    Text(
                      '${item['state'] ?? ''} · ${_date(item['occurredAt'])}'
                      '${(item['actorType'] ?? '').toString().isEmpty ? '' : ' · ${item['actorType']} ${item['actorId'] ?? ''}'}',
                      style: AppTypography.caption(context),
                    ),
                  ],
                ),
              ),
            ],
          ),
        );
      },
    );
  }

  Widget _metricCard(String label, String value, IconData icon) {
    return Container(
      padding: EdgeInsets.all(AppSpacing.md),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: BorderRadius.circular(AppRadius.medium),
        border: Border.all(color: context.borderPrimary),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(icon, size: 19, color: context.accentPrimary),
          SizedBox(height: AppSpacing.sm),
          Text(value, style: AppTypography.sectionTitle(context)),
          Text(label, style: AppTypography.caption(context)),
        ],
      ),
    );
  }

  Widget _statusBadge(String value) {
    final color = value == 'active' || value == 'official'
        ? const Color(0xFF208A60)
        : value == 'disabled' || value == 'revoked'
        ? context.error
        : context.accentPrimary;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(999),
      ),
      child: Text(
        value,
        style: TextStyle(
          fontSize: 10,
          color: color,
          fontWeight: FontWeight.w600,
        ),
      ),
    );
  }

  Widget _metaChip(String value) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
      decoration: BoxDecoration(
        color: context.surfaceSecondary,
        borderRadius: BorderRadius.circular(999),
      ),
      child: Text(value, style: AppTypography.caption(context)),
    );
  }

  String _archetype(Object? value) {
    return switch (value?.toString()) {
      'companion' => '陪伴型',
      'worker' => '执行型',
      'team_lead' => '团队负责人',
      final other => other ?? 'Agent',
    };
  }

  Color _stateColor(String state) {
    if (state == 'running' || state == 'completed' || state == 'passed') {
      return const Color(0xFF208A60);
    }
    if (state == 'failed' || state == 'error') return context.error;
    if (state == 'pending' || state == 'waiting') {
      return const Color(0xFFD59B22);
    }
    return context.textTertiary;
  }

  String _agentName(String id) {
    for (final agent in _agents) {
      if (agent['id']?.toString() == id) {
        return (agent['name'] ?? id).toString();
      }
    }
    return id.isEmpty ? '系统' : id;
  }

  String _date(Object? value) {
    final parsed = DateTime.tryParse(value?.toString() ?? '');
    if (parsed == null) return '未知';
    return '${parsed.month}/${parsed.day} ${parsed.hour.toString().padLeft(2, '0')}:${parsed.minute.toString().padLeft(2, '0')}';
  }
}

class _HostControlDialog extends ConsumerStatefulWidget {
  const _HostControlDialog({required this.sessionId});

  final String sessionId;

  @override
  ConsumerState<_HostControlDialog> createState() => _HostControlDialogState();
}

class _HostControlDialogState extends ConsumerState<_HostControlDialog> {
  final _keyboardController = TextEditingController();
  Timer? _timer;
  bool _busy = false;
  String? _error;
  Uint8List? _image;

  @override
  void initState() {
    super.initState();
    _refresh();
    _timer = Timer.periodic(
      const Duration(milliseconds: 1400),
      (_) => _refresh(),
    );
  }

  @override
  void dispose() {
    _timer?.cancel();
    _keyboardController.dispose();
    super.dispose();
  }

  Future<void> _refresh() async {
    if (_busy) return;
    _busy = true;
    try {
      final capture = await ref
          .read(backendServiceProvider)
          .post<Map<String, dynamic>>(
            '/api/agent-platform/host-control/${widget.sessionId}/actions',
            data: {'action': 'screen.capture'},
          );
      final encoded = (capture?['base64'] ?? '').toString();
      if (encoded.isNotEmpty && mounted) {
        setState(() {
          _image = base64Decode(encoded);
          _error = null;
        });
      }
    } catch (error) {
      if (mounted) setState(() => _error = error.toString());
    } finally {
      _busy = false;
    }
  }

  Future<void> _sendKeyboard() async {
    final text = _keyboardController.text;
    if (text.isEmpty) return;
    try {
      await ref
          .read(backendServiceProvider)
          .post<dynamic>(
            '/api/agent-platform/host-control/${widget.sessionId}/actions',
            data: {'action': 'keyboard.type', 'text': text},
          );
      _keyboardController.clear();
      await _refresh();
    } catch (error) {
      if (mounted) setState(() => _error = error.toString());
    }
  }

  Future<void> _close() async {
    _timer?.cancel();
    try {
      await ref
          .read(backendServiceProvider)
          .post<dynamic>(
            '/api/agent-platform/host-control/${widget.sessionId}/end',
            data: const {},
          );
    } catch (error) {
      if (mounted) setState(() => _error = error.toString());
    }
    if (mounted) Navigator.pop(context);
  }

  @override
  Widget build(BuildContext context) {
    return Dialog(
      insetPadding: const EdgeInsets.all(12),
      child: SizedBox(
        width: 1100,
        height: MediaQuery.sizeOf(context).height * 0.88,
        child: Column(
          children: [
            AppBar(
              automaticallyImplyLeading: false,
              title: const Text('桌面实时画面'),
              actions: [
                IconButton(
                  onPressed: _refresh,
                  tooltip: '刷新',
                  icon: const Icon(Icons.refresh),
                ),
                IconButton(
                  onPressed: _close,
                  tooltip: '关闭',
                  icon: const Icon(Icons.close),
                ),
              ],
            ),
            Expanded(
              child: ColoredBox(
                color: Colors.black,
                child: _image == null
                    ? const Center(child: CircularProgressIndicator())
                    : InteractiveViewer(
                        minScale: 0.5,
                        maxScale: 4,
                        child: Image.memory(
                          _image!,
                          fit: BoxFit.contain,
                          gaplessPlayback: true,
                        ),
                      ),
              ),
            ),
            if (_error != null)
              Padding(
                padding: const EdgeInsets.all(8),
                child: Text(
                  _error!,
                  maxLines: 3,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(color: context.error),
                ),
              ),
            Padding(
              padding: EdgeInsets.fromLTRB(
                AppSpacing.md,
                AppSpacing.sm,
                AppSpacing.md,
                MediaQuery.viewInsetsOf(context).bottom + AppSpacing.md,
              ),
              child: Row(
                children: [
                  Expanded(
                    child: TextField(
                      controller: _keyboardController,
                      decoration: const InputDecoration(hintText: '向接管桌面发送文字'),
                      onSubmitted: (_) => _sendKeyboard(),
                    ),
                  ),
                  SizedBox(width: AppSpacing.sm),
                  FilledButton(
                    onPressed: _sendKeyboard,
                    child: const Text('发送'),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

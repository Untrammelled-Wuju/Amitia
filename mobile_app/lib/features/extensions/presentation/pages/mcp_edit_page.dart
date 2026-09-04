import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class McpEditPage extends ConsumerStatefulWidget {
  final String mcpId;

  const McpEditPage({super.key, required this.mcpId});

  @override
  ConsumerState<McpEditPage> createState() => _McpEditPageState();
}

class _McpEditPageState extends ConsumerState<McpEditPage> {
  final _nameController = TextEditingController();
  final _displayNameController = TextEditingController();
  final _descriptionController = TextEditingController();
  final _endpointController = TextEditingController();
  final _commandController = TextEditingController();
  final _argsController = TextEditingController();
  final _workDirController = TextEditingController();
  final _credentialController = TextEditingController();

  int _transportIndex = 0;
  String _authType = 'none';
  bool _enabled = false;
  bool _privateNetworkConfirmed = false;
  bool _hasSampling = false;
  bool _hasTasks = false;
  bool _hasRoots = false;
  bool _hasElicitation = false;
  final Map<String, Map<String, dynamic>> _capabilityConfigurations = {};
  Map<String, dynamic>? _server;
  bool _loading = true;
  String? _error;
  bool _saving = false;

  bool get _isNew => widget.mcpId.trim().isEmpty || widget.mcpId == 'new';
  bool get _isHttp => _transportIndex == 0;

  static const _authLabels = <String, String>{
    'none': '无需认证',
    'oauth': 'OAuth',
    'bearer_token': 'Bearer Token',
    'custom_headers': '自定义 Header',
    'stdio_env': 'STDIO 环境变量',
  };

  @override
  void initState() {
    super.initState();
    _loadServer();
  }

  Future<void> _loadServer() async {
    if (_isNew) {
      if (mounted) setState(() => _loading = false);
      return;
    }
    try {
      final svc = ref.read(mcpServiceProvider);
      final data = await svc.getServer(widget.mcpId);
      if (data == null) throw StateError('MCP 服务不存在');
      final capabilities = await svc.capabilities(widget.mcpId);
      if (!mounted) return;
      _nameController.text = (data['name'] ?? '').toString();
      _displayNameController.text = (data['displayName'] ?? '').toString();
      _descriptionController.text = (data['description'] ?? '').toString();
      _endpointController.text = (data['endpoint'] ?? '').toString();
      _commandController.text = (data['command'] ?? '').toString();
      _workDirController.text = (data['workDir'] ?? '').toString();
      _argsController.text = _parseArgs(data['args']).join('\n');
      _transportIndex = (data['transport'] ?? '').toString() == 'stdio' ? 1 : 0;
      _authType = (data['authType'] ?? 'none').toString();
      if (!_authLabels.containsKey(_authType)) _authType = 'none';
      _enabled = _asBool(data['enabled']);
      _privateNetworkConfirmed = _asBool(data['privateNetworkConfirmed']);
      final capabilityState = <String, bool>{};
      _capabilityConfigurations.clear();
      for (final item in capabilities) {
        final name = (item['capability'] ?? '').toString();
        if (name.isEmpty) continue;
        capabilityState[name] = _asBool(item['enabled']);
        _capabilityConfigurations[name] = _parseConfiguration(item['configuration']);
      }
      _hasSampling = capabilityState['sampling'] ?? false;
      _hasTasks = capabilityState['tasks'] ?? false;
      _hasRoots = capabilityState['roots'] ?? false;
      _hasElicitation = capabilityState['elicitation'] ?? false;
      setState(() {
        _server = data;
        _loading = false;
      });
    } catch (e) {
      if (mounted) {
        setState(() {
          _error = e.toString();
          _loading = false;
        });
      }
    }
  }

  @override
  void dispose() {
    _nameController.dispose();
    _displayNameController.dispose();
    _descriptionController.dispose();
    _endpointController.dispose();
    _commandController.dispose();
    _argsController.dispose();
    _workDirController.dispose();
    _credentialController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return AmitiaScaffold(
        appBar: AmitiaAppBar(
          title: _isNew ? '添加 MCP 服务' : '编辑 MCP 服务',
          showBackButton: true,
          fallbackRoute: AppRoutes.extensions,
        ),
        body: const SafeArea(
          top: false,
          child: AmitiaLoadingState(message: '加载中...'),
        ),
      );
    }
    if (_error != null && !_isNew) {
      return AmitiaScaffold(
        appBar: AmitiaAppBar(
          title: '编辑 MCP 服务',
          showBackButton: true,
          fallbackRoute: AppRoutes.extensions,
        ),
        body: SafeArea(
          top: false,
          child: AmitiaErrorState(
            message: _error!,
            onRetry: () {
              setState(() {
                _loading = true;
                _error = null;
              });
              _loadServer();
            },
          ),
        ),
      );
    }

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: _isNew ? '添加 MCP 服务' : '编辑 MCP 服务',
        showBackButton: true,
        fallbackRoute: AppRoutes.extensions,
      ),
      body: SafeArea(
        top: false,
        child: SingleChildScrollView(
          padding: EdgeInsets.fromLTRB(
            AppSpacing.pagePadding,
            AppSpacing.sm,
            AppSpacing.pagePadding,
            AppSpacing.xxxl,
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _buildBasicSection(context),
              SizedBox(height: AppSpacing.sectionGap),
              _buildTransportSection(context),
              SizedBox(height: AppSpacing.sectionGap),
              _buildAuthSection(context),
              SizedBox(height: AppSpacing.sectionGap),
              _buildCapabilitySection(context),
              SizedBox(height: AppSpacing.xxl),
              Row(
                children: [
                  Expanded(
                    child: AmitiaButton(
                      label: '取消',
                      isSecondary: true,
                      onPressed: _saving ? null : () => context.pop(),
                    ),
                  ),
                  SizedBox(width: AppSpacing.sm),
                  Expanded(
                    child: AmitiaButton(
                      label: _saving ? '保存中...' : '保存',
                      icon: Icons.check,
                      onPressed: _saving ? null : _onSave,
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildBasicSection(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('基本信息', style: AppTypography.sectionTitle(context)),
        SizedBox(height: AppSpacing.md),
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _label(context, '服务名称'),
              const SizedBox(height: 6),
              AmitiaTextField(hintText: '例如 github', controller: _nameController),
              SizedBox(height: AppSpacing.md),
              _label(context, '显示名称'),
              const SizedBox(height: 6),
              AmitiaTextField(hintText: '可选', controller: _displayNameController),
              SizedBox(height: AppSpacing.md),
              _label(context, '说明'),
              const SizedBox(height: 6),
              AmitiaTextField(hintText: '可选', controller: _descriptionController),
              SizedBox(height: AppSpacing.md),
              AmitiaSwitchTile(
                title: '启用并连接',
                subtitle: '保存后允许该 MCP 服务参与连接和工具同步',
                value: _enabled,
                onChanged: (value) => setState(() => _enabled = value),
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildTransportSection(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('连接方式', style: AppTypography.sectionTitle(context)),
        SizedBox(height: AppSpacing.md),
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              AmitiaSegmentedControl(
                segments: const ['HTTP', 'STDIO'],
                selectedIndex: _transportIndex,
                onChanged: (index) => setState(() {
                  _transportIndex = index;
                  if (_isHttp && _authType == 'stdio_env') _authType = 'none';
                }),
              ),
              SizedBox(height: AppSpacing.md),
              if (_isHttp) ...[
                _label(context, 'Server URL'),
                const SizedBox(height: 6),
                AmitiaTextField(
                  hintText: 'https://example.com/mcp',
                  controller: _endpointController,
                ),
                SizedBox(height: AppSpacing.md),
                AmitiaSwitchTile(
                  title: '允许可信私网地址',
                  subtitle: '仅在确认该服务属于可信私网时启用',
                  value: _privateNetworkConfirmed,
                  onChanged: (value) => setState(() => _privateNetworkConfirmed = value),
                ),
              ] else ...[
                _label(context, '命令'),
                const SizedBox(height: 6),
                AmitiaTextField(
                  hintText: '已安装的可执行程序',
                  controller: _commandController,
                ),
                SizedBox(height: AppSpacing.md),
                _label(context, '参数'),
                const SizedBox(height: 6),
                AmitiaTextField(
                  hintText: '每行一个参数',
                  controller: _argsController,
                ),
                SizedBox(height: AppSpacing.md),
                _label(context, '工作目录'),
                const SizedBox(height: 6),
                AmitiaTextField(hintText: '可选', controller: _workDirController),
              ],
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildAuthSection(BuildContext context) {
    final options = _authLabels.keys.where((value) => _isHttp || value == 'none' || value == 'stdio_env').toList();
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('认证', style: AppTypography.sectionTitle(context)),
        SizedBox(height: AppSpacing.md),
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              DropdownButtonFormField<String>(
                value: options.contains(_authType) ? _authType : 'none',
                decoration: const InputDecoration(labelText: '认证方式'),
                items: options
                    .map((value) => DropdownMenuItem(value: value, child: Text(_authLabels[value]!)))
                    .toList(),
                onChanged: (value) {
                  if (value != null) setState(() => _authType = value);
                },
              ),
              if (_authType == 'bearer_token' || _authType == 'custom_headers' || _authType == 'stdio_env') ...[
                SizedBox(height: AppSpacing.md),
                _label(context, _authType == 'bearer_token' ? 'Token' : '凭据 JSON'),
                const SizedBox(height: 6),
                AmitiaTextField(
                  hintText: _authType == 'bearer_token'
                      ? (_isNew ? '输入 Token' : '留空保留现有 Token')
                      : (_isNew ? '{"KEY":"VALUE"}' : '留空保留现有凭据'),
                  controller: _credentialController,
                ),
              ],
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildCapabilitySection(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('客户端能力', style: AppTypography.sectionTitle(context)),
        SizedBox(height: AppSpacing.md),
        AmitiaCard(
          child: Column(
            children: [
              AmitiaSwitchTile(
                title: 'Roots',
                subtitle: '允许向该服务提供已授权目录',
                value: _hasRoots,
                onChanged: (value) => setState(() => _hasRoots = value),
              ),
              const Divider(height: 1),
              AmitiaSwitchTile(
                title: 'Sampling',
                subtitle: '允许服务请求受控模型采样',
                value: _hasSampling,
                onChanged: (value) => setState(() => _hasSampling = value),
              ),
              const Divider(height: 1),
              AmitiaSwitchTile(
                title: 'Elicitation',
                subtitle: '允许服务请求受控表单或外部 URL 确认',
                value: _hasElicitation,
                onChanged: (value) => setState(() => _hasElicitation = value),
              ),
              const Divider(height: 1),
              AmitiaSwitchTile(
                title: 'Tasks',
                subtitle: '启用异步任务查询与取消能力',
                value: _hasTasks,
                onChanged: (value) => setState(() => _hasTasks = value),
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _label(BuildContext context, String text) => Text(text, style: AppTypography.caption(context));

  Future<void> _onSave() async {
    final name = _nameController.text.trim();
    if (name.isEmpty) {
      _showMessage('请输入服务名称', error: true);
      return;
    }
    if (_isHttp && _endpointController.text.trim().isEmpty) {
      _showMessage('请输入 Server URL', error: true);
      return;
    }
    if (!_isHttp && _commandController.text.trim().isEmpty) {
      _showMessage('请输入 STDIO 命令', error: true);
      return;
    }

    dynamic credential;
    final credentialText = _credentialController.text.trim();
    if (credentialText.isNotEmpty) {
      if (_authType == 'bearer_token') {
        credential = credentialText;
      } else if (_authType == 'custom_headers' || _authType == 'stdio_env') {
        try {
          final decoded = jsonDecode(credentialText);
          if (decoded is! Map) throw const FormatException();
          credential = decoded;
        } catch (_) {
          _showMessage('凭据必须是 JSON 对象', error: true);
          return;
        }
      }
    }

    setState(() => _saving = true);
    try {
      final svc = ref.read(mcpServiceProvider);
      final payload = <String, dynamic>{
        'name': name,
        'displayName': _displayNameController.text.trim(),
        'description': _descriptionController.text.trim(),
        'transport': _isHttp ? 'streamable_http' : 'stdio',
        'endpoint': _isHttp ? _endpointController.text.trim() : '',
        'command': _isHttp ? '' : _commandController.text.trim(),
        'args': _isHttp
            ? <String>[]
            : _argsController.text
                .split('\n')
                .map((value) => value.trim())
                .where((value) => value.isNotEmpty)
                .toList(growable: false),
        'workDir': _isHttp ? '' : _workDirController.text.trim(),
        'authType': _authType,
        'enabled': _enabled,
        'source': 'manual',
        'privateNetworkConfirmed': _isHttp && _privateNetworkConfirmed,
        if (credential != null) 'credential': credential,
      };

      Map<String, dynamic>? saved;
      if (_isNew) {
        saved = await svc.createServer(payload);
      } else {
        saved = await svc.updateServer(widget.mcpId, payload);
      }
      final serverId = _isNew ? (saved?['id'] ?? '').toString() : widget.mcpId;
      if (serverId.isEmpty) throw StateError('保存响应缺少 MCP Server ID');

      await Future.wait([
        svc.setCapability(
          serverId,
          'roots',
          _hasRoots,
          configuration: _capabilityConfiguration('roots', _hasRoots),
        ),
        svc.setCapability(
          serverId,
          'sampling',
          _hasSampling,
          configuration: _capabilityConfiguration('sampling', _hasSampling),
        ),
        svc.setCapability(
          serverId,
          'elicitation',
          _hasElicitation,
          configuration: _capabilityConfiguration('elicitation', _hasElicitation),
        ),
        svc.setCapability(
          serverId,
          'tasks',
          _hasTasks,
          configuration: _capabilityConfiguration('tasks', _hasTasks),
        ),
      ]);

      if (!mounted) return;
      _showMessage(_isNew ? 'MCP 服务已添加' : 'MCP 服务配置已保存');
      context.pop(true);
    } catch (e) {
      if (mounted) _showMessage('保存失败: ${e.toString().replaceFirst('Exception: ', '')}', error: true);
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  void _showMessage(String message, {bool error = false}) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(message),
        backgroundColor: error ? context.error : context.success,
      ),
    );
  }

  Map<String, dynamic> _capabilityConfiguration(String capability, bool enabled) {
    final existing = _capabilityConfigurations[capability];
    if (existing != null && existing.isNotEmpty) return Map<String, dynamic>.from(existing);
    if (!enabled) return <String, dynamic>{};
    switch (capability) {
      case 'sampling':
        return <String, dynamic>{'maxTokens': 2048, 'timeoutSeconds': 60, 'maxConcurrent': 1};
      case 'tasks':
        return <String, dynamic>{'maxConcurrent': 4, 'maxTTLSeconds': 86400};
      default:
        return <String, dynamic>{};
    }
  }

  static Map<String, dynamic> _parseConfiguration(dynamic value) {
    if (value is Map<String, dynamic>) return Map<String, dynamic>.from(value);
    if (value is Map) return Map<String, dynamic>.from(value);
    if (value is String && value.trim().isNotEmpty) {
      try {
        final decoded = jsonDecode(value);
        if (decoded is Map) return Map<String, dynamic>.from(decoded);
      } catch (_) {}
    }
    return <String, dynamic>{};
  }

  static bool _asBool(dynamic value) {
    if (value is bool) return value;
    if (value is num) return value != 0;
    return value.toString().toLowerCase() == 'true';
  }

  static List<String> _parseArgs(dynamic value) {
    if (value is List) return value.map((item) => item.toString()).toList(growable: false);
    final raw = (value ?? '').toString().trim();
    if (raw.isEmpty) return const [];
    try {
      final decoded = jsonDecode(raw);
      if (decoded is List) return decoded.map((item) => item.toString()).toList(growable: false);
    } catch (_) {}
    return raw.split('\n').map((item) => item.trim()).where((item) => item.isNotEmpty).toList(growable: false);
  }
}

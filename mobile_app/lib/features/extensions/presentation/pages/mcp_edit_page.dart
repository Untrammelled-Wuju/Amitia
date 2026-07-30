import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../shared/models/models.dart';
import '../../../../shared/mock_data/mock_extensions.dart';

class McpEditPage extends ConsumerStatefulWidget {
  final String mcpId;

  const McpEditPage({super.key, required this.mcpId});

  @override
  ConsumerState<McpEditPage> createState() => _McpEditPageState();
}

class _McpEditPageState extends ConsumerState<McpEditPage> {
  final _nameController = TextEditingController();
  final _addressController = TextEditingController();
  int _transportIndex = 0;
  late List<MapEntry<TextEditingController, TextEditingController>> _envControllers;
  bool _hasSampling = false;
  bool _hasTasks = false;
  bool _hasRoots = false;

  McpServer? get _server {
    try {
      return MockExtensions.mcpServers.firstWhere((s) => s.id == widget.mcpId);
    } catch (_) {
      return null;
    }
  }

  @override
  void initState() {
    super.initState();
    final server = _server;
    if (server != null) {
      _nameController.text = server.name;
      _addressController.text = server.address;
      _transportIndex = server.transport.index;
      _hasSampling = server.hasSampling;
      _hasTasks = server.hasTasks;
      _hasRoots = server.hasRoots;
      _envControllers = server.envVars.entries.map((e) {
        return MapEntry(TextEditingController(text: e.key), TextEditingController(text: e.value));
      }).toList();
    } else {
      _envControllers = [];
    }
  }

  @override
  void dispose() {
    _nameController.dispose();
    _addressController.dispose();
    for (final pair in _envControllers) {
      pair.key.dispose();
      pair.value.dispose();
    }
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final isNew = _server == null;
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: isNew ? '添加 MCP 服务' : '编辑 MCP 服务',
        showBackButton: true,
      ),
      body: SafeArea(
        top: false,
        child: SingleChildScrollView(
          padding: const EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.sm, AppSpacing.pagePadding, AppSpacing.xxxl),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _buildFormSection(context),
              const SizedBox(height: AppSpacing.sectionGap),
              _buildEnvSection(context),
              const SizedBox(height: AppSpacing.sectionGap),
              _buildCapabilitySection(context),
              const SizedBox(height: AppSpacing.xxl),
              Row(
                children: [
                  Expanded(
                    child: AmitiaButton(
                      label: '取消',
                      isSecondary: true,
                      onPressed: () => context.pop(),
                    ),
                  ),
                  const SizedBox(width: AppSpacing.sm),
                  Expanded(
                    child: AmitiaButton(
                      label: '保存',
                      icon: Icons.check,
                      onPressed: _onSave,
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

  Widget _buildFormSection(BuildContext context) {
    final transports = ['STDIO', 'SSE', 'WebSocket'];
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('基本信息', style: AppTypography.sectionTitle(context)),
        const SizedBox(height: AppSpacing.md),
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('服务名称', style: AppTypography.caption(context)),
              const SizedBox(height: 6),
              AmitiaTextField(hintText: '输入服务名称', controller: _nameController),
              const SizedBox(height: AppSpacing.md),
              Text('Transport 类型', style: AppTypography.caption(context)),
              const SizedBox(height: 6),
              AmitiaSegmentedControl(
                segments: transports,
                selectedIndex: _transportIndex,
                onChanged: (i) => setState(() => _transportIndex = i),
              ),
              const SizedBox(height: AppSpacing.md),
              Text(_transportIndex == 0 ? '启动命令' : '服务地址', style: AppTypography.caption(context)),
              const SizedBox(height: 6),
              AmitiaTextField(
                hintText: _transportIndex == 0 ? '例如：filesystem-server' : '例如：https://example.com/sse',
                controller: _addressController,
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildEnvSection(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            Text('环境变量', style: AppTypography.sectionTitle(context)),
            GestureDetector(
              onTap: _addEnvRow,
              child: Container(
                padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
                decoration: BoxDecoration(
                  color: context.accentSoft,
                  borderRadius: AppRadius.brTag,
                ),
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Icon(Icons.add, size: 16, color: context.accentPrimary),
                    const SizedBox(width: 4),
                    Text('添加', style: TextStyle(fontSize: 13, color: context.accentPrimary, fontWeight: FontWeight.w500)),
                  ],
                ),
              ),
            ),
          ],
        ),
        const SizedBox(height: AppSpacing.md),
        if (_envControllers.isEmpty)
          AmitiaCard(
            child: Center(
              child: Text('暂无环境变量', style: AppTypography.caption(context)),
            ),
          )
        else
          ..._envControllers.asMap().entries.map((entry) {
            final index = entry.key;
            final pair = entry.value;
            return Padding(
              padding: const EdgeInsets.only(bottom: AppSpacing.sm),
              child: AmitiaCard(
                child: Row(
                  children: [
                    Expanded(
                      flex: 2,
                      child: AmitiaTextField(hintText: '键名', controller: pair.key),
                    ),
                    const SizedBox(width: 8),
                    Expanded(
                      flex: 3,
                      child: AmitiaTextField(hintText: '值', controller: pair.value),
                    ),
                    const SizedBox(width: 8),
                    GestureDetector(
                      onTap: () => _removeEnvRow(index),
                      child: Container(
                        width: 36,
                        height: 36,
                        decoration: BoxDecoration(
                          color: context.error.withValues(alpha: 0.1),
                          shape: BoxShape.circle,
                        ),
                        child: Icon(Icons.close, size: 18, color: context.error),
                      ),
                    ),
                  ],
                ),
              ),
            );
          }),
      ],
    );
  }

  Widget _buildCapabilitySection(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('能力配置', style: AppTypography.sectionTitle(context)),
        const SizedBox(height: AppSpacing.md),
        AmitiaCard(
          child: Column(
            children: [
              AmitiaSwitchTile(
                title: 'Sampling',
                subtitle: '允许服务端请求 LLM 采样',
                value: _hasSampling,
                onChanged: (val) => setState(() => _hasSampling = val),
              ),
              const Divider(height: 1),
              AmitiaSwitchTile(
                title: 'Tasks',
                subtitle: '启用任务管理能力',
                value: _hasTasks,
                onChanged: (val) => setState(() => _hasTasks = val),
              ),
              const Divider(height: 1),
              AmitiaSwitchTile(
                title: 'Roots',
                subtitle: '启用根目录能力',
                value: _hasRoots,
                onChanged: (val) => setState(() => _hasRoots = val),
              ),
            ],
          ),
        ),
      ],
    );
  }

  void _addEnvRow() {
    setState(() {
      _envControllers.add(MapEntry(TextEditingController(), TextEditingController()));
    });
  }

  void _removeEnvRow(int index) {
    setState(() {
      _envControllers[index].key.dispose();
      _envControllers[index].value.dispose();
      _envControllers.removeAt(index);
    });
  }

  void _onSave() {
    if (_nameController.text.trim().isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: const Text('请输入服务名称'), backgroundColor: context.error),
      );
      return;
    }
    if (_addressController.text.trim().isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: const Text('请输入地址或命令'), backgroundColor: context.error),
      );
      return;
    }
    context.pop();
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: const Text('MCP 服务配置已保存'), backgroundColor: context.success),
    );
  }
}

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../shared/models/models.dart';
import '../../../../shared/mock_data/mock_data.dart';

class WasmPage extends ConsumerStatefulWidget {
  const WasmPage({super.key});

  @override
  ConsumerState<WasmPage> createState() => _WasmPageState();
}

class _WasmPageState extends ConsumerState<WasmPage> {
  late List<WasmModule> _modules;

  @override
  void initState() {
    super.initState();
    _modules = List.from(MockKernel.wasmModules);
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: 'WASM Runtime',
        showBackButton: true,
        fallbackRoute: AppRoutes.developer,
        actions: [
          AmitiaIconButton(
            icon: Icons.add,
            onPressed: () => _showCreateDefinitionSheet(context),
            color: context.accentPrimary,
            tooltip: '新建定义',
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: Column(
          children: [
            _buildInfoCard(context),
            Expanded(
              child: _modules.isEmpty
                  ? AmitiaEmptyState(
                      icon: Icons.extension_outlined,
                      title: '暂无 WASM 模块',
                      subtitle: '点击右上角新建定义',
                    )
                  : ListView.builder(
                      padding: const EdgeInsets.only(bottom: AppSpacing.lg),
                      itemCount: _modules.length,
                      itemBuilder: (context, index) => _buildModuleCard(context, _modules[index]),
                    ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildInfoCard(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.all(AppSpacing.pagePadding),
      child: AmitiaCard(
        backgroundColor: context.accentSoft,
        border: Border.all(color: context.accentPrimary.withValues(alpha: 0.15), width: 0.5),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Icon(Icons.info_outline, size: 20, color: context.accentPrimary),
            const SizedBox(width: 8),
            Expanded(
              child: Text(
                'WASM Runtime 管理扩展内核中的 WebAssembly 模块定义。每个模块拥有独立的配额限制，确保资源使用可控。',
                style: AppTypography.caption(context),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildModuleCard(BuildContext context, WasmModule module) {
    final isLoaded = module.status == '已加载';
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.xs),
      child: AmitiaCard(
        onTap: () => _showModuleOptions(context, module),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Container(
                  width: 40,
                  height: 40,
                  decoration: BoxDecoration(
                    color: context.accentSoft,
                    borderRadius: AppRadius.brSmall,
                  ),
                  child: Icon(Icons.extension, size: 22, color: context.accentPrimary),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(module.name, style: AppTypography.cardTitle(context)),
                      const SizedBox(height: 2),
                      Text('ID: ${module.id}', style: AppTypography.label(context)),
                    ],
                  ),
                ),
                AmitiaStatusBadge(
                  label: module.status,
                  type: isLoaded ? BadgeType.success : BadgeType.neutral,
                ),
              ],
            ),
            const SizedBox(height: AppSpacing.md),
            Row(
              children: [
                Text('配额', style: AppTypography.label(context)),
                const SizedBox(width: 8),
                Expanded(
                  child: AmitiaProgressBar(progress: module.quota > 0 ? module.used / module.quota : 0),
                ),
                const SizedBox(width: 8),
                Text('${module.used}/${module.quota}', style: AppTypography.label(context).copyWith(color: context.textSecondary)),
              ],
            ),
            const SizedBox(height: AppSpacing.md),
            Row(
              children: [
                if (isLoaded)
                  Expanded(
                    child: AmitiaButton(
                      label: '卸载模块',
                      isSecondary: true,
                      icon: Icons.power_settings_new,
                      onPressed: () => _showUnloadConfirm(context, module),
                    ),
                  )
                else
                  Expanded(
                    child: AmitiaButton(
                      label: '加载模块',
                      isSecondary: true,
                      icon: Icons.download_for_offline_outlined,
                      onPressed: () => _loadModule(context, module),
                    ),
                  ),
                const SizedBox(width: AppSpacing.sm),
                Expanded(
                  child: AmitiaButton(
                    label: '删除定义',
                    isDestructive: true,
                    icon: Icons.delete_outline,
                    onPressed: () => _showDeleteConfirm(context, module),
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  void _showCreateDefinitionSheet(BuildContext context) {
    final nameController = TextEditingController();
    final quotaController = TextEditingController(text: '100');

    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(20))),
      builder: (context) {
        return Padding(
          padding: EdgeInsets.fromLTRB(20, 0, 20, MediaQuery.of(context).viewInsets.bottom + 34),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const SizedBox(height: 8),
              Center(
                child: Container(width: 40, height: 4, decoration: BoxDecoration(color: context.borderPrimary, borderRadius: BorderRadius.circular(2))),
              ),
              const SizedBox(height: 20),
              Text('新建 WASM 定义', style: AppTypography.pageTitle(context)),
              const SizedBox(height: 20),
              Text('模块名称', style: AppTypography.label(context).copyWith(color: context.textSecondary)),
              const SizedBox(height: 6),
              AmitiaTextField(
                hintText: '请输入模块名称',
                controller: nameController,
              ),
              const SizedBox(height: 16),
              Text('配额上限', style: AppTypography.label(context).copyWith(color: context.textSecondary)),
              const SizedBox(height: 6),
              AmitiaTextField(
                hintText: '请输入配额数值',
                controller: quotaController,
                keyboardType: TextInputType.number,
              ),
              const SizedBox(height: 20),
              AmitiaButton(
                label: '创建定义',
                isFullWidth: true,
                icon: Icons.check,
                onPressed: () {
                  final name = nameController.text.trim();
                  if (name.isEmpty) {
                    ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('请输入模块名称')));
                    return;
                  }
                  final quota = int.tryParse(quotaController.text.trim()) ?? 100;
                  setState(() {
                    _modules.add(WasmModule(
                      id: 'wm${_modules.length + 1}',
                      name: name,
                      status: '已卸载',
                      quota: quota,
                      used: 0,
                    ));
                  });
                  Navigator.pop(context);
                  ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('已创建定义：$name')));
                },
              ),
            ],
          ),
        );
      },
    );
  }

  void _showUnloadConfirm(BuildContext context, WasmModule module) {
    showDialog(
      context: context,
      builder: (context) {
        return AlertDialog(
          backgroundColor: context.surfacePrimary,
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('卸载模块', style: AppTypography.cardTitle(context)),
          content: Text('确定要卸载模块「${module.name}」吗？卸载后模块将从内存中移除。', style: AppTypography.body(context)),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(context),
              child: Text('取消', style: TextStyle(color: context.textSecondary)),
            ),
            TextButton(
              onPressed: () {
                setState(() {
                  final idx = _modules.indexWhere((m) => m.id == module.id);
                  if (idx >= 0) {
                    _modules[idx] = WasmModule(
                      id: module.id,
                      name: module.name,
                      status: '已卸载',
                      quota: module.quota,
                      used: 0,
                    );
                  }
                });
                Navigator.pop(context);
                ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('已卸载模块：${module.name}')));
              },
              child: Text('卸载', style: TextStyle(color: context.warning)),
            ),
          ],
        );
      },
    );
  }

  void _loadModule(BuildContext context, WasmModule module) {
    setState(() {
      final idx = _modules.indexWhere((m) => m.id == module.id);
      if (idx >= 0) {
        _modules[idx] = WasmModule(
          id: module.id,
          name: module.name,
          status: '已加载',
          quota: module.quota,
          used: 0,
        );
      }
    });
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('已加载模块：${module.name}')));
  }

  void _showDeleteConfirm(BuildContext context, WasmModule module) {
    showDialog(
      context: context,
      builder: (context) {
        return AlertDialog(
          backgroundColor: context.surfacePrimary,
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('删除定义', style: AppTypography.cardTitle(context)),
          content: Text('确定要删除「${module.name}」的定义吗？此操作不可恢复。', style: AppTypography.body(context)),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(context),
              child: Text('取消', style: TextStyle(color: context.textSecondary)),
            ),
            TextButton(
              onPressed: () {
                setState(() {
                  _modules.removeWhere((m) => m.id == module.id);
                });
                Navigator.pop(context);
                ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('已删除定义：${module.name}')));
              },
              child: Text('删除', style: TextStyle(color: context.error)),
            ),
          ],
        );
      },
    );
  }

  void _showModuleOptions(BuildContext context, WasmModule module) {
    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(20))),
      builder: (context) {
        return SafeArea(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              const SizedBox(height: 8),
              Center(
                child: Container(width: 40, height: 4, decoration: BoxDecoration(color: context.borderPrimary, borderRadius: BorderRadius.circular(2))),
              ),
              const SizedBox(height: 16),
              Padding(
                padding: const EdgeInsets.symmetric(horizontal: 20),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(module.name, style: AppTypography.pageTitle(context)),
                    const SizedBox(height: 4),
                    Text('模块 ID: ${module.id}', style: AppTypography.caption(context)),
                    const SizedBox(height: 4),
                    Text('状态: ${module.status}', style: AppTypography.caption(context)),
                    const SizedBox(height: 4),
                    Text('配额: ${module.used}/${module.quota}', style: AppTypography.caption(context)),
                  ],
                ),
              ),
              const SizedBox(height: 16),
              ListTile(
                leading: Icon(module.status == '已加载' ? Icons.power_settings_new : Icons.download_for_offline_outlined, color: context.accentPrimary),
                title: Text(module.status == '已加载' ? '卸载模块' : '加载模块'),
                onTap: () {
                  Navigator.pop(context);
                  if (module.status == '已加载') {
                    _showUnloadConfirm(context, module);
                  } else {
                    _loadModule(context, module);
                  }
                },
              ),
              ListTile(
                leading: Icon(Icons.delete_outline, color: context.error),
                title: Text('删除定义', style: TextStyle(color: context.error)),
                onTap: () {
                  Navigator.pop(context);
                  _showDeleteConfirm(context, module);
                },
              ),
              const SizedBox(height: 8),
            ],
          ),
        );
      },
    );
  }
}

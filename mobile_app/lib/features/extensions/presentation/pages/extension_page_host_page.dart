import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';

class ExtensionPageHostPage extends ConsumerStatefulWidget {
  final String pageId;

  const ExtensionPageHostPage({super.key, required this.pageId});

  @override
  ConsumerState<ExtensionPageHostPage> createState() => _ExtensionPageHostPageState();
}

class _ExtensionPageHostPageState extends ConsumerState<ExtensionPageHostPage> {
  bool _isLoading = true;
  bool _hasPermission = false;

  @override
  void initState() {
    super.initState();
    _loadPage();
  }

  void _loadPage() {
    setState(() => _isLoading = true);
    Future.delayed(const Duration(milliseconds: 800), () {
      if (mounted) {
        setState(() => _isLoading = false);
        _showPermissionDialog();
      }
    });
  }

  void _showPermissionDialog() {
    showDialog(
      context: context,
      barrierDismissible: false,
      builder: (context) => AlertDialog(
        backgroundColor: context.surfacePrimary,
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
        title: Row(
          children: [
            Icon(Icons.shield_outlined, color: context.warning, size: 24),
            const SizedBox(width: 8),
            Text('权限确认', style: AppTypography.cardTitle(context)),
          ],
        ),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('扩展页面「${widget.pageId}」请求以下权限：', style: AppTypography.bodySmall(context)),
            const SizedBox(height: 12),
            _PermissionItem(text: '读取页面数据'),
            _PermissionItem(text: '提交表单数据'),
            _PermissionItem(text: '访问扩展状态'),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () {
              Navigator.pop(context);
              setState(() => _hasPermission = false);
            },
            child: Text('拒绝', style: TextStyle(color: context.textSecondary)),
          ),
          TextButton(
            onPressed: () {
              Navigator.pop(context);
              setState(() => _hasPermission = true);
              ScaffoldMessenger.of(this.context).showSnackBar(
                SnackBar(content: const Text('权限已授予'), backgroundColor: context.success),
              );
            },
            child: Text('允许', style: TextStyle(color: context.accentPrimary)),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '扩展页面',
        showBackButton: true,
        actions: [
          AmitiaIconButton(
            icon: Icons.refresh,
            onPressed: _loadPage,
            tooltip: '刷新',
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: _isLoading
            ? const AmitiaLoadingState(message: '正在加载扩展页面...')
            : !_hasPermission
                ? _buildNoPermissionState(context)
                : SingleChildScrollView(
                    padding: const EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.sm, AppSpacing.pagePadding, AppSpacing.xxxl),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        _buildHeader(context),
                        const SizedBox(height: AppSpacing.sectionGap),
                        _buildFormSurface(context),
                        const SizedBox(height: AppSpacing.sectionGap),
                        _buildStatusSurface(context),
                        const SizedBox(height: AppSpacing.sectionGap),
                        _buildTableSurface(context),
                        const SizedBox(height: AppSpacing.sectionGap),
                        _buildUnsupportedHint(context),
                        const SizedBox(height: AppSpacing.xxl),
                        _buildActionButtons(context),
                      ],
                    ),
                  ),
      ),
    );
  }

  Widget _buildHeader(BuildContext context) {
    return AmitiaCard(
      child: Row(
        children: [
          Container(
            width: 48,
            height: 48,
            decoration: BoxDecoration(
              color: context.accentSoft,
              borderRadius: AppRadius.brSmall,
            ),
            child: Icon(Icons.dashboard_customize_outlined, size: 24, color: context.accentPrimary),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('页面 ID: ${widget.pageId}', style: AppTypography.cardTitle(context)),
                const SizedBox(height: 2),
                Text('来源扩展: 文件系统扩展', style: AppTypography.caption(context)),
                const SizedBox(height: 2),
                Row(
                  children: [
                    Icon(Icons.verified, size: 12, color: context.success),
                    const SizedBox(width: 4),
                    Text('已验证 · v1.2.0', style: AppTypography.label(context)),
                  ],
                ),
              ],
            ),
          ),
          AmitiaStatusBadge(label: '运行中', type: BadgeType.success),
        ],
      ),
    );
  }

  Widget _buildFormSurface(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Icon(Icons.edit_note, size: 18, color: context.accentPrimary),
            const SizedBox(width: 6),
            Text('表单 Surface', style: AppTypography.sectionTitle(context)),
          ],
        ),
        const SizedBox(height: AppSpacing.md),
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('配置项名称', style: AppTypography.caption(context)),
              const SizedBox(height: 6),
              AmitiaTextField(hintText: '输入配置值'),
              const SizedBox(height: AppSpacing.md),
              Text('扫描范围', style: AppTypography.caption(context)),
              const SizedBox(height: 6),
              AmitiaTextField(hintText: '选择目录路径', prefixIcon: Icon(Icons.folder_outlined, size: 20)),
              const SizedBox(height: AppSpacing.md),
              Row(
                children: [
                  Expanded(
                    child: AmitiaSwitchTile(
                      title: '递归扫描',
                      subtitle: '包含子目录',
                      value: true,
                      onChanged: (val) {},
                    ),
                  ),
                ],
              ),
              const Divider(height: 1),
              AmitiaSwitchTile(
                title: '实时监控',
                subtitle: '文件变更时自动更新',
                value: false,
                onChanged: (val) {},
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildStatusSurface(BuildContext context) {
    final statusItems = [
      {'label': '状态', 'value': '正常运行', 'color': context.success},
      {'label': '已扫描文件', 'value': '1,247', 'color': context.info},
      {'label': '新增文件', 'value': '23', 'color': context.accentPrimary},
      {'label': '异常文件', 'value': '0', 'color': context.success},
    ];
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Icon(Icons.analytics_outlined, size: 18, color: context.accentPrimary),
            const SizedBox(width: 6),
            Text('状态 Surface', style: AppTypography.sectionTitle(context)),
          ],
        ),
        const SizedBox(height: AppSpacing.md),
        AmitiaCard(
          child: Column(
            children: statusItems.map((item) => Padding(
              padding: const EdgeInsets.only(bottom: AppSpacing.sm),
              child: Row(
                children: [
                  Text(item['label'] as String, style: AppTypography.caption(context)),
                  const Spacer(),
                  Text(item['value'] as String, style: AppTypography.bodySmall(context).copyWith(fontWeight: FontWeight.w600)),
                  const SizedBox(width: 8),
                  Container(
                    width: 8,
                    height: 8,
                    decoration: BoxDecoration(
                      color: item['color'] as Color,
                      shape: BoxShape.circle,
                    ),
                  ),
                ],
              ),
            )).toList(),
          ),
        ),
      ],
    );
  }

  Widget _buildTableSurface(BuildContext context) {
    final tableData = [
      {'name': 'config.json', 'size': '2.4 KB', 'modified': '2026-07-30', 'status': '已索引'},
      {'name': 'data.csv', 'size': '15.8 KB', 'modified': '2026-07-29', 'status': '已索引'},
      {'name': 'report.pdf', 'size': '1.2 MB', 'modified': '2026-07-28', 'status': '已索引'},
      {'name': 'temp.tmp', 'size': '0.5 KB', 'modified': '2026-07-30', 'status': '待处理'},
    ];
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Icon(Icons.table_chart_outlined, size: 18, color: context.accentPrimary),
            const SizedBox(width: 6),
            Text('表格 Surface', style: AppTypography.sectionTitle(context)),
          ],
        ),
        const SizedBox(height: AppSpacing.md),
        AmitiaCard(
          padding: EdgeInsets.zero,
          child: Column(
            children: [
              Container(
                padding: const EdgeInsets.symmetric(horizontal: AppSpacing.cardPadding, vertical: 10),
                decoration: BoxDecoration(
                  color: context.surfaceSecondary,
                  borderRadius: const BorderRadius.only(
                    topLeft: Radius.circular(16),
                    topRight: Radius.circular(16),
                  ),
                ),
                child: Row(
                  children: [
                    Expanded(flex: 3, child: Text('文件名', style: AppTypography.label(context).copyWith(fontWeight: FontWeight.w600))),
                    Expanded(flex: 2, child: Text('大小', style: AppTypography.label(context).copyWith(fontWeight: FontWeight.w600))),
                    Expanded(flex: 2, child: Text('修改日期', style: AppTypography.label(context).copyWith(fontWeight: FontWeight.w600))),
                    SizedBox(width: 60, child: Text('状态', style: AppTypography.label(context).copyWith(fontWeight: FontWeight.w600))),
                  ],
                ),
              ),
              ...tableData.map((row) => Container(
                    padding: const EdgeInsets.symmetric(horizontal: AppSpacing.cardPadding, vertical: 10),
                    decoration: BoxDecoration(
                      border: Border(bottom: BorderSide(color: context.borderSecondary, width: 0.5)),
                    ),
                    child: Row(
                      children: [
                        Expanded(flex: 3, child: Text(row['name'] as String, style: AppTypography.bodySmall(context), overflow: TextOverflow.ellipsis)),
                        Expanded(flex: 2, child: Text(row['size'] as String, style: AppTypography.label(context))),
                        Expanded(flex: 2, child: Text(row['modified'] as String, style: AppTypography.label(context))),
                        SizedBox(
                          width: 60,
                          child: AmitiaStatusBadge(
                            label: row['status'] as String,
                            type: row['status'] == '已索引' ? BadgeType.success : BadgeType.warning,
                            fontSize: 10,
                          ),
                        ),
                      ],
                    ),
                  )),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildUnsupportedHint(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: context.warning.withValues(alpha: 0.08),
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.warning.withValues(alpha: 0.3), width: 1),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(Icons.info_outline, size: 18, color: context.warning),
          const SizedBox(width: 8),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('部分组件不支持', style: AppTypography.bodySmall(context).copyWith(fontWeight: FontWeight.w600, color: context.warning)),
                const SizedBox(height: 2),
                Text('该扩展页面包含图表、地图等当前版本不支持的组件类型，已用占位符替代。完整动态渲染功能将在后续版本中实现。', style: AppTypography.label(context).copyWith(color: context.warning)),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildActionButtons(BuildContext context) {
    return Row(
      children: [
        Expanded(
          child: AmitiaButton(
            label: '保存配置',
            isSecondary: true,
            icon: Icons.save_outlined,
            onPressed: () {
              ScaffoldMessenger.of(context).showSnackBar(
                SnackBar(content: const Text('配置已保存'), backgroundColor: context.success),
              );
            },
          ),
        ),
        const SizedBox(width: AppSpacing.sm),
        Expanded(
          child: AmitiaButton(
            label: '执行操作',
            icon: Icons.play_arrow,
            onPressed: () {
              ScaffoldMessenger.of(context).showSnackBar(
                SnackBar(content: const Text('操作已触发'), backgroundColor: context.accentPrimary),
              );
            },
          ),
        ),
      ],
    );
  }

  Widget _buildNoPermissionState(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(AppSpacing.xxl),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(Icons.lock_outline, size: 56, color: context.textTertiary),
            const SizedBox(height: AppSpacing.md),
            Text('权限未授予', style: AppTypography.cardTitle(context)),
            const SizedBox(height: 4),
            Text('该扩展页面需要授权才能访问', style: AppTypography.caption(context), textAlign: TextAlign.center),
            const SizedBox(height: AppSpacing.lg),
            AmitiaButton(
              label: '重新授权',
              icon: Icons.shield_outlined,
              onPressed: _showPermissionDialog,
            ),
          ],
        ),
      ),
    );
  }
}

class _PermissionItem extends StatelessWidget {
  final String text;

  const _PermissionItem({required this.text});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: Row(
        children: [
          Icon(Icons.check_circle_outline, size: 18, color: context.accentPrimary),
          const SizedBox(width: 10),
          Expanded(child: Text(text, style: AppTypography.bodySmall(context))),
        ],
      ),
    );
  }
}

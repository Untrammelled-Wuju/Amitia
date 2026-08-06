import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/models/character.dart';

class ToolboxWorkspacePage extends ConsumerStatefulWidget {
  const ToolboxWorkspacePage({super.key});

  @override
  ConsumerState<ToolboxWorkspacePage> createState() => _ToolboxWorkspacePageState();
}

class _ToolboxWorkspacePageState extends ConsumerState<ToolboxWorkspacePage> {
  List<CharacterDto> _workspaces = [];
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() { _loading = true; _error = null; });
    try {
      final svc = ref.read(characterServiceProvider);
      final characters = await svc.list();
      if (mounted) {
        setState(() { _workspaces = characters; _loading = false; });
      }
    } catch (e) {
      if (mounted) setState(() { _error = e.toString(); _loading = false; });
    }
  }

  String _timeAgo(String dateStr) {
    if (dateStr.isEmpty) return '—';
    try {
      final dt = DateTime.parse(dateStr);
      final diff = DateTime.now().difference(dt);
      if (diff.inMinutes < 60) return '${diff.inMinutes} 分钟前';
      if (diff.inHours < 24) return '${diff.inHours} 小时前';
      return '${diff.inDays} 天前';
    } catch (_) {
      return dateStr;
    }
  }

  IconData _iconForIndex(int i) {
    final icons = [Icons.work_outline, Icons.code, Icons.edit_note_outlined, Icons.school_outlined];
    return icons[i % icons.length];
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) return const AmitiaLoadingState(message: '正在加载工作区...');
    if (_error != null) return AmitiaErrorState(message: _error!, onRetry: _load);
    if (_workspaces.isEmpty) {
      return const AmitiaEmptyState(icon: Icons.work_outline, title: '暂无工作区', subtitle: '创建角色后工作区将在此展示');
    }

    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '工作区', showBackButton: true, fallbackRoute: AppRoutes.settingsToolbox),
      body: ListView.separated(
        padding: const EdgeInsets.all(AppSpacing.pagePadding),
        itemCount: _workspaces.length,
        separatorBuilder: (_, _) => const SizedBox(height: AppSpacing.md),
        itemBuilder: (context, i) {
          final w = _workspaces[i];
          final desc = w.description.isNotEmpty ? w.description : w.personality;
          final timeStr = _timeAgo(w.createdAt);
          return Container(
            padding: const EdgeInsets.all(AppSpacing.cardPadding),
            decoration: BoxDecoration(
              color: context.surfacePrimary,
              borderRadius: AppRadius.brMedium,
              border: Border.all(color: context.borderPrimary, width: 0.5),
            ),
            child: Row(
              children: [
                Container(
                  width: 44,
                  height: 44,
                  decoration: BoxDecoration(color: context.accentSoft, borderRadius: AppRadius.brSmall),
                  child: Icon(_iconForIndex(i), size: 22, color: context.accentPrimary),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(w.name, style: AppTypography.cardTitle(context)),
                      const SizedBox(height: 2),
                      Text('${desc.isNotEmpty ? desc : '工作区'} · 更新于 $timeStr', style: AppTypography.caption(context)),
                    ],
                  ),
                ),
                Icon(Icons.chevron_right, size: 20, color: context.textTertiary),
              ],
            ),
          );
        },
      ),
    );
  }
}

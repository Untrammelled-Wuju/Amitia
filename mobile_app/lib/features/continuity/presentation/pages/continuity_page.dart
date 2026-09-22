import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/models/continuity.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../widgets/continuity_editors.dart';

class ContinuityPage extends ConsumerStatefulWidget {
  const ContinuityPage({super.key});

  @override
  ConsumerState<ContinuityPage> createState() => _ContinuityPageState();
}

class _ContinuityPageState extends ConsumerState<ContinuityPage> {
  final _searchController = TextEditingController();
  List<ContinuityThreadDto> _items = const [];
  List<ContinuityThreadDto> _searchResults = const [];
  bool _loading = true;
  bool _searchVisible = false;
  bool _searchLoading = false;
  bool _refreshing = false;
  String? _error;
  String? _searchError;
  String? _statusFilter;
  Timer? _refreshTimer;
  Timer? _searchDebounce;
  int _searchRevision = 0;

  @override
  void initState() {
    super.initState();
    _load();
    _refreshTimer = Timer.periodic(const Duration(seconds: 8), (_) {
      if (!_refreshing && mounted) unawaited(_load(showLoading: false));
    });
  }

  @override
  void dispose() {
    _refreshTimer?.cancel();
    _searchDebounce?.cancel();
    _searchController.dispose();
    super.dispose();
  }

  Future<void> _load({bool showLoading = true}) async {
    if (_refreshing) return;
    _refreshing = true;
    if (showLoading && mounted) {
      setState(() {
        _loading = true;
        _error = null;
      });
    }
    try {
      final values = await ref
          .read(continuityServiceProvider)
          .list(status: _statusFilter ?? '');
      if (!mounted) return;
      setState(() {
        _items = values;
        _loading = false;
        _error = null;
      });
    } catch (error) {
      if (!mounted) return;
      setState(() {
        _error = error.toString();
        _loading = false;
      });
    } finally {
      _refreshing = false;
    }
  }

  void _toggleSearch(bool visible) {
    FocusScope.of(context).unfocus();
    _searchDebounce?.cancel();
    _searchController.clear();
    _searchRevision++;
    setState(() {
      _searchVisible = visible;
      _statusFilter = null;
      _searchResults = const [];
      _searchLoading = false;
      _searchError = null;
      if (!visible) {
        _loading = true;
        _error = null;
      }
    });
    if (!visible) unawaited(_load());
  }

  void _search(String value) {
    _searchDebounce?.cancel();
    final query = value.trim();
    if (query.isEmpty) {
      _searchRevision++;
      setState(() {
        _searchResults = const [];
        _searchLoading = false;
        _searchError = null;
      });
      return;
    }
    _searchDebounce = Timer(
      const Duration(milliseconds: 350),
      () => unawaited(_performSearch()),
    );
  }

  Future<void> _performSearch() async {
    final query = _searchController.text.trim();
    final revision = ++_searchRevision;
    if (query.isEmpty) {
      setState(() {
        _searchResults = const [];
        _searchLoading = false;
        _searchError = null;
      });
      return;
    }
    setState(() {
      _searchLoading = true;
      _searchError = null;
    });
    try {
      final values = await ref
          .read(continuityServiceProvider)
          .list(query: query);
      if (!mounted || revision != _searchRevision || !_searchVisible) return;
      setState(() {
        _searchResults = values;
        _searchLoading = false;
        _searchError = null;
      });
    } catch (error) {
      if (!mounted || revision != _searchRevision || !_searchVisible) return;
      setState(() {
        _searchResults = const [];
        _searchLoading = false;
        _searchError = error.toString();
      });
    }
  }

  Future<void> _create() async {
    final input = await showContinuityThreadEditor(context);
    if (input == null) return;
    try {
      final created = await ref.read(continuityServiceProvider).create(input);
      if (!mounted) return;
      await _load(showLoading: false);
      if (!mounted) return;
      context.push(AppRoutes.continuityDetail(created.id));
    } catch (error) {
      if (!mounted) return;
      amitiaSnackBar(context, error.toString());
    }
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '持续事项',
        navigation: AmitiaAppBarNavigation.back,
        actions: [
          AmitiaIconButton(
            icon: _searchVisible ? Icons.close : Icons.search,
            tooltip: _searchVisible ? '退出搜索' : '搜索',
            onPressed: () => _toggleSearch(!_searchVisible),
          ),
          AmitiaIconButton(
            icon: Icons.refresh,
            tooltip: '刷新',
            onPressed: _searchVisible ? _performSearch : _load,
          ),
          AmitiaIconButton(
            icon: Icons.add,
            tooltip: '新建持续事项',
            onPressed: _create,
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: _searchVisible ? _buildSearchView() : _buildBrowseView(),
      ),
    );
  }

  Widget _buildBrowseView() {
    return Column(
      children: [
        Padding(
          padding: EdgeInsets.fromLTRB(
            AppSpacing.pagePadding,
            AppSpacing.sm,
            AppSpacing.pagePadding,
            AppSpacing.sm,
          ),
          child: DropdownButtonFormField<String?>(
            initialValue: _statusFilter,
            decoration: const InputDecoration(
              labelText: '状态',
              prefixIcon: Icon(Icons.filter_list),
            ),
            items: const [
              DropdownMenuItem<String?>(value: null, child: Text('全部状态')),
              DropdownMenuItem<String?>(value: 'active', child: Text('进行中')),
              DropdownMenuItem<String?>(value: 'waiting', child: Text('等待中')),
              DropdownMenuItem<String?>(value: 'blocked', child: Text('已阻塞')),
              DropdownMenuItem<String?>(value: 'paused', child: Text('已暂停')),
              DropdownMenuItem<String?>(value: 'completed', child: Text('已完成')),
              DropdownMenuItem<String?>(value: 'cancelled', child: Text('已取消')),
            ],
            onChanged: (value) {
              setState(() => _statusFilter = value);
              unawaited(_load());
            },
          ),
        ),
        Expanded(
          child: _loading
              ? const AmitiaLoadingState(message: '加载持续事项…')
              : _error != null
              ? AmitiaErrorState(message: _error!, onRetry: _load)
              : _items.isEmpty
              ? AmitiaEmptyState(
                  icon: Icons.checklist_rtl,
                  title: '暂无持续事项',
                  subtitle: '持续事项用于跨会话跟踪进度、下一步和等待条件',
                  actionText: '新建事项',
                  onAction: _create,
                )
              : RefreshIndicator(
                  onRefresh: _load,
                  child: ListView.separated(
                    padding: EdgeInsets.fromLTRB(
                      AppSpacing.pagePadding,
                      0,
                      AppSpacing.pagePadding,
                      AppSpacing.xl,
                    ),
                    itemCount: _items.length,
                    separatorBuilder: (_, _) => SizedBox(height: AppSpacing.sm),
                    itemBuilder: (context, index) =>
                        _ContinuityCard(item: _items[index]),
                  ),
                ),
        ),
      ],
    );
  }

  Widget _buildSearchView() {
    final query = _searchController.text.trim();
    return Column(
      children: [
        Padding(
          padding: EdgeInsets.fromLTRB(
            AppSpacing.pagePadding,
            AppSpacing.md,
            AppSpacing.pagePadding,
            AppSpacing.sm,
          ),
          child: AmitiaSearchField(
            hintText: '搜索持续事项',
            controller: _searchController,
            autofocus: true,
            onChanged: _search,
          ),
        ),
        Expanded(
          child: query.isEmpty
              ? const AmitiaEmptyState(
                  icon: Icons.search,
                  title: '输入关键词',
                  subtitle: '搜索持续事项标题、目标或当前状态',
                )
              : _searchLoading
              ? const AmitiaLoadingState(message: '搜索持续事项…')
              : _searchError != null
              ? AmitiaErrorState(
                  message: _searchError!,
                  onRetry: _performSearch,
                )
              : _searchResults.isEmpty
              ? const AmitiaEmptyState(
                  icon: Icons.search_off,
                  title: '未找到相关持续事项',
                  subtitle: '尝试更换关键词',
                )
              : ListView.separated(
                  keyboardDismissBehavior:
                      ScrollViewKeyboardDismissBehavior.onDrag,
                  padding: EdgeInsets.fromLTRB(
                    AppSpacing.pagePadding,
                    0,
                    AppSpacing.pagePadding,
                    AppSpacing.xl,
                  ),
                  itemCount: _searchResults.length,
                  separatorBuilder: (_, _) => SizedBox(height: AppSpacing.sm),
                  itemBuilder: (context, index) =>
                      _ContinuityCard(item: _searchResults[index]),
                ),
        ),
      ],
    );
  }
}

class _ContinuityCard extends StatelessWidget {
  const _ContinuityCard({required this.item});

  final ContinuityThreadDto item;

  @override
  Widget build(BuildContext context) {
    final status = _statusMeta(item.status);
    final state = item.currentState.isNotEmpty
        ? item.currentState
        : item.summary.isNotEmpty
        ? item.summary
        : item.goal;
    return AmitiaCard(
      onTap: () => context.push(AppRoutes.continuityDetail(item.id)),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(
                child: Text(
                  item.title,
                  style: AppTypography.cardTitle(context),
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              SizedBox(width: AppSpacing.sm),
              AmitiaStatusBadge(label: status.label, type: status.type),
            ],
          ),
          if (state.isNotEmpty) ...[
            SizedBox(height: AppSpacing.md),
            Text(
              state,
              style: AppTypography.bodySmall(context),
              maxLines: 3,
              overflow: TextOverflow.ellipsis,
            ),
          ],
          if (item.nextAction.isNotEmpty) ...[
            SizedBox(height: AppSpacing.sm),
            Text(
              '下一步：${item.nextAction}',
              style: AppTypography.caption(context),
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
            ),
          ],
          SizedBox(height: AppSpacing.md),
          Row(
            children: [
              Icon(
                Icons.schedule_outlined,
                size: 14,
                color: context.textTertiary,
              ),
              SizedBox(width: AppSpacing.xs),
              Text(
                _formatTime(item.lastActiveAt),
                style: AppTypography.caption(context),
              ),
              const Spacer(),
              Icon(Icons.chevron_right, size: 18, color: context.textTertiary),
            ],
          ),
        ],
      ),
    );
  }
}

({String label, BadgeType type}) _statusMeta(String status) {
  switch (status) {
    case 'waiting':
      return (label: '等待中', type: BadgeType.warning);
    case 'blocked':
      return (label: '已阻塞', type: BadgeType.error);
    case 'paused':
      return (label: '已暂停', type: BadgeType.info);
    case 'completed':
      return (label: '已完成', type: BadgeType.success);
    case 'cancelled':
      return (label: '已取消', type: BadgeType.neutral);
    default:
      return (label: '进行中', type: BadgeType.accent);
  }
}

String _formatTime(DateTime? value) {
  if (value == null) return '暂无活动时间';
  final local = value.toLocal();
  String two(int number) => number.toString().padLeft(2, '0');
  return '${local.year}-${two(local.month)}-${two(local.day)} '
      '${two(local.hour)}:${two(local.minute)}';
}

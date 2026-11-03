import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/services/extension_service.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class MarketplacePage extends ConsumerStatefulWidget {
  const MarketplacePage({super.key});

  @override
  ConsumerState<MarketplacePage> createState() => _MarketplacePageState();
}

class _MarketplacePageState extends ConsumerState<MarketplacePage> {
  bool _loading = true;
  String? _error;
  String _category = '';
  String _search = '';
  String _installing = '';
  List<Map<String, dynamic>> _categories = const [];
  List<Map<String, dynamic>> _listings = const [];
  Map<String, String> _installedVersions = const {};

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final api = ref.read(backendServiceProvider);
      final values = await Future.wait<dynamic>([
        api.get<List<dynamic>>('/api/agent-platform/market/categories'),
        api.get<List<dynamic>>(
          '/api/agent-platform/market/listings',
          queryParameters: {
            if (_category.isNotEmpty) 'categoryId': _category,
            if (_search.isNotEmpty) 'q': _search,
            'platform': 'android',
          },
        ),
        ref.read(extensionServiceProvider).getExtensionCenterView(),
      ]);
      final categories = values[0] as List<dynamic>?;
      final listings = values[1] as List<dynamic>?;
      final installed = values[2] as ExtensionCenterView;
      if (!mounted) return;
      setState(() {
        _categories = (categories ?? const [])
            .whereType<Map>()
            .map((item) => Map<String, dynamic>.from(item))
            .toList(growable: false);
        _listings = (listings ?? const [])
            .whereType<Map>()
            .map((item) => Map<String, dynamic>.from(item))
            .toList(growable: false);
        _installedVersions = {
          for (final card in [
            ...installed.installed,
            ...installed.updates,
            ...installed.needsAction,
            ...installed.localImports,
            ...installed.dev,
          ])
            card.extensionId: card.version,
        };
      });
    } catch (error) {
      if (mounted) setState(() => _error = error.toString());
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  Future<void> _install(Map<String, dynamic> view) async {
    final listing = Map<String, dynamic>.from(view['listing'] as Map? ?? {});
    final id = (listing['id'] ?? '').toString();
    setState(() => _installing = id);
    try {
      await ref.read(backendServiceProvider).post<dynamic>(
        '/api/agent-platform/market/listings/${Uri.encodeComponent(id)}/install',
        data: {'version': listing['latestVersion']},
      );
      if (mounted) {
        amitiaSnackBar(context, '安装已提交，请继续完成插件预览与授权');
      }
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '安装失败：$error');
    } finally {
      if (mounted) setState(() => _installing = '');
    }
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '插件市场',
        centerTitle: true,
        navigation: AmitiaAppBarNavigation.back,
        actions: [
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
              AppSpacing.sm,
            ),
            child: TextField(
              textInputAction: TextInputAction.search,
              decoration: const InputDecoration(
                hintText: '搜索插件、扩展 ID 或发布者',
                prefixIcon: Icon(Icons.search),
              ),
              onChanged: (value) => _search = value,
              onSubmitted: (value) {
                _search = value;
                _load();
              },
            ),
          ),
          SizedBox(
            height: 42,
            child: ListView(
              padding: EdgeInsets.symmetric(
                horizontal: AppSpacing.pagePadding,
              ),
              scrollDirection: Axis.horizontal,
              children: [
                _chip('', '全部'),
                ..._categories.map(
                  (item) => _chip(
                    (item['id'] ?? '').toString(),
                    (item['name'] ?? '').toString(),
                  ),
                ),
              ],
            ),
          ),
          SizedBox(height: AppSpacing.sm),
          Expanded(
            child: _loading
                ? const Center(child: CircularProgressIndicator())
                : _error != null
                    ? Center(child: Text('加载失败：$_error'))
                    : RefreshIndicator(
                        onRefresh: _load,
                        child: _listings.isEmpty
                            ? const AmitiaEmptyState(
                                icon: Icons.storefront_outlined,
                                title: '没有匹配的插件',
                              )
                            : ListView.builder(
                                padding: EdgeInsets.fromLTRB(
                                  AppSpacing.pagePadding,
                                  0,
                                  AppSpacing.pagePadding,
                                  AppSpacing.pagePadding,
                                ),
                                itemCount: _listings.length,
                                itemBuilder: (context, index) =>
                                    _card(_listings[index]),
                              ),
                      ),
          ),
        ],
      ),
    );
  }

  Widget _chip(String value, String label) {
    return Padding(
      padding: const EdgeInsets.only(right: 8),
      child: ChoiceChip(
        label: Text(label),
        selected: _category == value,
        onSelected: (_) {
          _category = value;
          _load();
        },
      ),
    );
  }

  Widget _card(Map<String, dynamic> view) {
    final listing = Map<String, dynamic>.from(view['listing'] as Map? ?? {});
    final category = Map<String, dynamic>.from(view['category'] as Map? ?? {});
    final id = (listing['id'] ?? '').toString();
    final trust = (listing['trustLevel'] ?? '').toString();
    final installedVersion =
        _installedVersions[(listing['extensionId'] ?? '').toString()];
    final latestVersion = (listing['latestVersion'] ?? '').toString();
    final upToDate = installedVersion == latestVersion;
    return Container(
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
              Container(
                width: 44,
                height: 44,
                alignment: Alignment.center,
                decoration: BoxDecoration(
                  color: context.accentPrimary.withValues(alpha: 0.12),
                  borderRadius: BorderRadius.circular(AppRadius.small),
                ),
                child: Text(
                  (listing['displayName'] ?? 'P').toString().characters.first,
                  style: TextStyle(
                    color: context.accentPrimary,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
              SizedBox(width: AppSpacing.sm),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      (listing['displayName'] ?? '').toString(),
                      style: AppTypography.cardTitle(context),
                    ),
                    Text(
                      (listing['extensionId'] ?? '').toString(),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: AppTypography.caption(context),
                    ),
                  ],
                ),
              ),
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                decoration: BoxDecoration(
                  color: context.surfaceSecondary,
                  borderRadius: BorderRadius.circular(999),
                ),
                child: Text(trust, style: AppTypography.caption(context)),
              ),
            ],
          ),
          SizedBox(height: AppSpacing.sm),
          Text(
            (listing['summary'] ?? '').toString(),
            maxLines: 3,
            overflow: TextOverflow.ellipsis,
          ),
          SizedBox(height: AppSpacing.sm),
          Wrap(
            spacing: 8,
            runSpacing: 6,
            children: [
              _meta((category['name'] ?? listing['categoryId']).toString()),
              _meta('v${listing['latestVersion'] ?? ''}'),
              _meta('${listing['installCount'] ?? 0} 次安装'),
              _meta('安全 ${listing['securityScore'] ?? 0}'),
            ],
          ),
          SizedBox(height: AppSpacing.md),
          SizedBox(
            width: double.infinity,
            child: FilledButton(
              onPressed:
                  view['installable'] == true &&
                          !upToDate &&
                          _installing != id
                      ? () => _install(view)
                      : null,
              child: _installing == id
                  ? const SizedBox.square(
                      dimension: 16,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : Text(upToDate
                      ? '已安装'
                      : installedVersion != null
                          ? '更新'
                          : '安装'),
            ),
          ),
        ],
      ),
    );
  }

  Widget _meta(String value) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
      decoration: BoxDecoration(
        color: context.surfaceSecondary,
        borderRadius: BorderRadius.circular(999),
      ),
      child: Text(value, style: AppTypography.caption(context)),
    );
  }
}

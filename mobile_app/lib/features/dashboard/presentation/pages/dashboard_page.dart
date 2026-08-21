import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

Future<Map<String, dynamic>?> _safeGet(Ref ref, String path, {Map<String, dynamic>? query}) async {
  try {
    return await ref.read(backendServiceProvider).get<Map<String, dynamic>>(path, queryParameters: query);
  } catch (_) {
    return null;
  }
}

final dashboardOverviewProvider = FutureProvider<Map<String, dynamic>>((ref) async {
  final values = await Future.wait([
    _safeGet(ref, '/api/health'),
    _safeGet(ref, '/api/qq/status'),
    _safeGet(ref, '/api/wechat/status'),
    _safeGet(ref, '/api/security/status'),
    _safeGet(ref, '/api/usage/overview'),
    _safeGet(ref, '/api/chats/stats'),
    _safeGet(ref, '/api/logs/recent/errors', query: const {'limit': 20}),
    _safeGet(ref, '/api/imports/batches', query: const {'limit': 5}),
  ]);
  return {
    'health': values[0],
    'qq': values[1],
    'wechat': values[2],
    'security': values[3],
    'usage': values[4],
    'stats': values[5],
    'errors': values[6],
    'imports': values[7],
  };
});

class DashboardPage extends ConsumerStatefulWidget {
  const DashboardPage({super.key});

  @override
  ConsumerState<DashboardPage> createState() => _DashboardPageState();
}

class _DashboardPageState extends ConsumerState<DashboardPage> {
  int _selectedTab = 0;

  bool _connected(Map<String, dynamic>? value) {
    if (value == null) return false;
    final data = value['data'] is Map ? Map<String, dynamic>.from(value['data'] as Map) : value;
    return data['connected'] == true || data['qqOnline'] == true || data['status'] == 'online' || data['status'] == 'connected';
  }

  @override
  Widget build(BuildContext context) {
    final overview = ref.watch(dashboardOverviewProvider);
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '概览',
        navigation: AmitiaAppBarNavigation.back,
        actions: [AmitiaIconButton(icon: Icons.refresh, tooltip: '刷新', onPressed: () => ref.invalidate(dashboardOverviewProvider))],
      ),
      body: Column(children: [
        Padding(
          padding: EdgeInsets.all(AppSpacing.pagePadding),
          child: AmitiaSegmentedControl(segments: const ['运行概览', '数据概览'], selectedIndex: _selectedTab, onChanged: (value) => setState(() => _selectedTab = value)),
        ),
        Expanded(
          child: overview.when(
            loading: () => const Center(child: CircularProgressIndicator()),
            error: (error, _) => Center(child: Text('概览加载失败：$error')),
            data: (data) => _selectedTab == 0 ? _runOverview(data) : _dataOverview(data),
          ),
        ),
      ]),
    );
  }

  Widget _runOverview(Map<String, dynamic> data) {
    final health = data['health'] as Map<String, dynamic>?;
    final status = health?['status'] is Map ? Map<String, dynamic>.from(health!['status'] as Map) : <String, dynamic>{};
    final qqConnected = _connected(data['qq'] as Map<String, dynamic>?);
    final wechatConnected = _connected(data['wechat'] as Map<String, dynamic>?);
    final security = data['security'] as Map<String, dynamic>?;
    final securityStatus = (security?['status'] ?? 'unknown').toString();
    final backendRunning = status['database'] == true;
    final runtimeReady = status['orchestratorReady'] == true;
    return RefreshIndicator(
      onRefresh: () async => ref.invalidate(dashboardOverviewProvider),
      child: ListView(
        padding: EdgeInsets.fromLTRB(AppSpacing.pagePadding, 0, AppSpacing.pagePadding, AppSpacing.pagePadding),
        children: [
          AmitiaSectionHeader(title: '系统状态'),
          SizedBox(height: AppSpacing.md),
          _StatusGrid(items: [
            _StatusItem(label: '后端', value: backendRunning ? '运行中' : '异常', icon: Icons.dns_outlined, type: backendRunning ? BadgeType.success : BadgeType.error),
            _StatusItem(label: 'Agent Runtime', value: runtimeReady ? '就绪' : '未就绪', icon: Icons.auto_awesome, type: runtimeReady ? BadgeType.success : BadgeType.warning),
            _StatusItem(label: 'QQ', value: qqConnected ? '已连接' : '未连接', icon: Icons.chat_bubble_outline, type: qqConnected ? BadgeType.success : BadgeType.neutral),
            _StatusItem(label: '微信', value: wechatConnected ? '已连接' : '未连接', icon: Icons.wechat_outlined, type: wechatConnected ? BadgeType.success : BadgeType.neutral),
            _StatusItem(label: '数据库', value: status['database'] == true ? '正常' : '异常', icon: Icons.storage, type: status['database'] == true ? BadgeType.success : BadgeType.error),
            _StatusItem(label: '访问安全', value: securityStatus == 'secure' ? '安全' : securityStatus, icon: Icons.shield_outlined, type: securityStatus == 'secure' ? BadgeType.success : BadgeType.warning),
          ]),
          SizedBox(height: AppSpacing.sectionGap),
          AmitiaSectionHeader(title: '后端健康信息'),
          SizedBox(height: AppSpacing.sm),
          AmitiaCard(child: Column(children: [
            _kv('状态', status['status'] ?? health?['status'] ?? 'unknown'),
            _kv('就绪组件', status['readyCount'] ?? 0),
            _kv('Readiness', status['readinessReady'] == true ? 'ready' : 'not ready'),
            _kv('安全状态', securityStatus),
          ])),
        ],
      ),
    );
  }

  Widget _dataOverview(Map<String, dynamic> data) {
    final usage = data['usage'] as Map<String, dynamic>? ?? const {};
    final stats = data['stats'] as Map<String, dynamic>? ?? const {};
    final errorsMap = data['errors'] as Map<String, dynamic>?;
    final errors = errorsMap?['errors'] is List ? errorsMap!['errors'] as List : const [];
    final importsMap = data['imports'] as Map<String, dynamic>?;
    final imports = importsMap?['items'] is List ? importsMap!['items'] as List : const [];
    return RefreshIndicator(
      onRefresh: () async => ref.invalidate(dashboardOverviewProvider),
      child: ListView(
        padding: EdgeInsets.fromLTRB(AppSpacing.pagePadding, 0, AppSpacing.pagePadding, AppSpacing.pagePadding),
        children: [
          AmitiaSectionHeader(title: '真实使用统计'),
          SizedBox(height: AppSpacing.md),
          GridView.count(
            shrinkWrap: true,
            physics: const NeverScrollableScrollPhysics(),
            crossAxisCount: 3,
            mainAxisSpacing: AppSpacing.md,
            crossAxisSpacing: AppSpacing.md,
            childAspectRatio: 1,
            children: [
              _StatCard(label: '今日调用', value: '${usage['todayCalls'] ?? 0}', icon: Icons.bolt_outlined),
              _StatCard(label: '今日 Token', value: '${usage['todayTokens'] ?? 0}', icon: Icons.token_outlined),
              _StatCard(label: '总请求', value: '${usage['totalRequests'] ?? 0}', icon: Icons.query_stats_outlined),
              _StatCard(label: '总 Token', value: '${usage['totalTokens'] ?? 0}', icon: Icons.data_usage_outlined),
              _StatCard(label: '对话', value: '${stats['totalConversations'] ?? stats['conversationCount'] ?? 0}', icon: Icons.chat_bubble_outline),
              _StatCard(label: '今日消息', value: '${stats['todayMessages'] ?? 0}', icon: Icons.message_outlined),
            ],
          ),
          SizedBox(height: AppSpacing.sectionGap),
          AmitiaSectionHeader(title: '最近错误 (${errors.length})'),
          SizedBox(height: AppSpacing.sm),
          AmitiaCard(
            child: errors.isEmpty
                ? Text('没有读取到最近错误', style: AppTypography.caption(context))
                : Column(crossAxisAlignment: CrossAxisAlignment.start, children: errors.whereType<Map>().take(5).map((row) => Padding(
                    padding: EdgeInsets.only(bottom: AppSpacing.sm),
                    child: Text('${row['file'] ?? ''} · ${row['line'] ?? ''}', maxLines: 2, overflow: TextOverflow.ellipsis, style: AppTypography.caption(context)),
                  )).toList()),
          ),
          SizedBox(height: AppSpacing.sectionGap),
          AmitiaSectionHeader(title: '最近导入 (${imports.length})'),
          SizedBox(height: AppSpacing.sm),
          AmitiaCard(
            child: imports.isEmpty
                ? Text('暂无最近导入记录', style: AppTypography.caption(context))
                : Column(crossAxisAlignment: CrossAxisAlignment.start, children: imports.whereType<Map>().take(5).map((row) => Padding(
                    padding: EdgeInsets.only(bottom: AppSpacing.sm),
                    child: Text('${row['fileName'] ?? row['name'] ?? row['id'] ?? '导入批次'} · ${row['status'] ?? ''}', style: AppTypography.bodySmall(context)),
                  )).toList()),
          ),
          SizedBox(height: AppSpacing.xl),
        ],
      ),
    );
  }

  Widget _kv(String label, Object? value) => Padding(
        padding: EdgeInsets.symmetric(vertical: AppSpacing.xs),
        child: Row(children: [Expanded(child: Text(label, style: AppTypography.bodySmall(context))), Text('$value', style: AppTypography.bodySmall(context).copyWith(fontWeight: FontWeight.w600))]),
      );
}

class _StatusItem {
  final String label;
  final String value;
  final IconData icon;
  final BadgeType type;
  const _StatusItem({required this.label, required this.value, required this.icon, required this.type});
}

class _StatusGrid extends StatelessWidget {
  final List<_StatusItem> items;
  const _StatusGrid({required this.items});
  @override
  Widget build(BuildContext context) => GridView.count(
        shrinkWrap: true,
        physics: const NeverScrollableScrollPhysics(),
        crossAxisCount: 2,
        mainAxisSpacing: AppSpacing.md,
        crossAxisSpacing: AppSpacing.md,
        childAspectRatio: 2.35,
        children: items.map((item) => Container(
              padding: EdgeInsets.all(AppSpacing.md),
              decoration: BoxDecoration(color: context.surfacePrimary, borderRadius: AppRadius.brMedium, border: Border.all(color: context.borderPrimary, width: .5)),
              child: Row(children: [
                Icon(item.icon, color: context.accentPrimary),
                SizedBox(width: AppSpacing.sm),
                Expanded(child: Column(mainAxisAlignment: MainAxisAlignment.center, crossAxisAlignment: CrossAxisAlignment.start, children: [Text(item.label, style: AppTypography.label(context)), Text(item.value, maxLines: 1, overflow: TextOverflow.ellipsis, style: AppTypography.bodySmall(context))])),
                AmitiaStatusBadge(label: item.value, type: item.type),
              ]),
            )).toList(),
      );
}

class _StatCard extends StatelessWidget {
  final String label;
  final String value;
  final IconData icon;
  const _StatCard({required this.label, required this.value, required this.icon});
  @override
  Widget build(BuildContext context) => Container(
        padding: EdgeInsets.all(AppSpacing.md),
        decoration: BoxDecoration(color: context.surfacePrimary, borderRadius: AppRadius.brMedium, border: Border.all(color: context.borderPrimary, width: .5)),
        child: Column(mainAxisAlignment: MainAxisAlignment.center, children: [
          Icon(icon, color: context.accentPrimary),
          SizedBox(height: AppSpacing.xs),
          Text(value, maxLines: 1, overflow: TextOverflow.ellipsis, style: AppTypography.cardTitle(context)),
          Text(label, style: AppTypography.label(context)),
        ]),
      );
}

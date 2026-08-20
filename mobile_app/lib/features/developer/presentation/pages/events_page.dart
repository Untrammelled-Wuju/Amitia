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
import '../../../../core/services/providers.dart';

class EventsPage extends ConsumerStatefulWidget {
  const EventsPage({super.key});

  @override
  ConsumerState<EventsPage> createState() => _EventsPageState();
}

class _EventsPageState extends ConsumerState<EventsPage> {
  bool _loading = true;
  String? _error;
  int _selectedTab = 0;
  final _tabs = ['事件历史', '死信队列', '事件类型'];
  List<Map<String, dynamic>> _events = [];

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
      final svc = ref.read(systemServiceProvider);
      final data = await svc.diagnostics();
      if (mounted) {
        if (data != null) {
          final events = data['events'];
          if (events is List) {
            _events = events.map((e) => Map<String, dynamic>.from(e as Map)).toList();
          }
        }
        setState(() {
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

  List<Map<String, dynamic>> get _deadLetterEvents => _events.where((e) => e['status'] == '死信').toList();
  List<Map<String, dynamic>> get _historyEvents => _events.where((e) => e['status'] != '死信').toList();

  @override
  Widget build(BuildContext context) {
    if (_loading) return const AmitiaLoadingState();
    if (_error != null) return AmitiaErrorState(message: _error!, onRetry: _load);
    if (_events.isEmpty) {
      return const AmitiaEmptyState(icon: Icons.bolt, title: '暂无事件');
    }

    return AmitiaScaffold(
      appBar: const AmitiaAppBar(
        title: '事件中心',
        showBackButton: true,
        fallbackRoute: AppRoutes.developer,
      ),
      body: SafeArea(
        top: false,
        child: Column(
          children: [
            Padding(
              padding: EdgeInsets.all(AppSpacing.pagePadding),
              child: AmitiaSegmentedControl(
                segments: _tabs,
                selectedIndex: _selectedTab,
                onChanged: (i) => setState(() => _selectedTab = i),
              ),
            ),
            Expanded(
              child: _buildContent(context),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildContent(BuildContext context) {
    switch (_selectedTab) {
      case 0:
        return _buildEventList(context, _historyEvents, isDeadLetter: false);
      case 1:
        return _deadLetterEvents.isEmpty
            ? AmitiaEmptyState(icon: Icons.inbox_outlined, title: '暂无死信', subtitle: '没有处理失败的事件')
            : _buildEventList(context, _deadLetterEvents, isDeadLetter: true);
      case 2:
        return _buildEventTypeList(context);
      default:
        return const SizedBox.shrink();
    }
  }

  Widget _buildEventList(BuildContext context, List<Map<String, dynamic>> events, {required bool isDeadLetter}) {
    return ListView.builder(
      padding: EdgeInsets.only(bottom: AppSpacing.lg),
      itemCount: events.length,
      itemBuilder: (context, index) => _buildEventCard(context, events[index], isDeadLetter),
    );
  }

  Widget _buildEventCard(BuildContext context, Map<String, dynamic> event, bool isDeadLetter) {
    final type = event['type'] as String? ?? '';
    final status = event['status'] as String? ?? '';
    final timeStr = event['time'] as String? ?? '';
    final detail = event['detail'] as String?;
    final id = event['id'] as String? ?? '';

    return Padding(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.xs),
      child: AmitiaCard(
        onTap: () => _showEventDetailSheet(context, event, isDeadLetter),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Container(
                  width: 36,
                  height: 36,
                  decoration: BoxDecoration(
                    color: _statusColor(context, status).withValues(alpha: 0.1),
                    borderRadius: AppRadius.brSmall,
                  ),
                  child: Icon(Icons.bolt, size: 18, color: _statusColor(context, status)),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(type, style: AppTypography.cardTitle(context).copyWith(fontSize: 14, fontFamily: 'monospace')),
                      const SizedBox(height: 2),
                      Text(timeStr, style: AppTypography.label(context)),
                    ],
                  ),
                ),
                _buildStatusBadge(status),
              ],
            ),
            if (detail != null) ...[
              SizedBox(height: AppSpacing.sm),
              Text(detail, style: AppTypography.caption(context)),
            ],
            if (isDeadLetter) ...[
              SizedBox(height: AppSpacing.md),
              Row(
                children: [
                  Expanded(
                    child: AmitiaButton(
                      label: '重放',
                      isSecondary: true,
                      icon: Icons.replay,
                      onPressed: () => _showReplayConfirm(context, event),
                    ),
                  ),
                  SizedBox(width: AppSpacing.sm),
                  Expanded(
                    child: AmitiaButton(
                      label: '丢弃',
                      isDestructive: true,
                      icon: Icons.delete_outline,
                      onPressed: () => _showDiscardConfirm(context, event),
                    ),
                  ),
                ],
              ),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildEventTypeList(BuildContext context) {
    final types = <String, bool>{
      'message.receive': true,
      'message.send': true,
      'memory.created': true,
      'memory.updated': true,
      'hook.error': true,
      'task.completed': true,
    };

    return ListView(
      padding: EdgeInsets.symmetric(vertical: AppSpacing.lg),
      children: types.entries.map((e) {
        return Padding(
          padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: 2),
          child: AmitiaCard(
            child: Row(
              children: [
                Container(
                  width: 36,
                  height: 36,
                  decoration: BoxDecoration(
                    color: context.accentSoft,
                    borderRadius: AppRadius.brSmall,
                  ),
                  child: Icon(Icons.bolt, size: 18, color: context.accentPrimary),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(e.key, style: AppTypography.body(context).copyWith(fontFamily: 'monospace', fontSize: 14)),
                      const SizedBox(height: 2),
                      Text(e.value ? '已订阅' : '未订阅', style: AppTypography.label(context)),
                    ],
                  ),
                ),
                Switch(
                  value: e.value,
                  onChanged: (v) {
                    ScaffoldMessenger.of(context).showSnackBar(
                      SnackBar(content: Text('${v ? '已订阅' : '已取消订阅'}：${e.key}')),
                    );
                  },
                ),
              ],
            ),
          ),
        );
      }).toList(),
    );
  }

  Color _statusColor(BuildContext context, String status) {
    switch (status) {
      case '已处理':
        return context.success;
      case '死信':
        return context.error;
      default:
        return context.textSecondary;
    }
  }

  AmitiaStatusBadge _buildStatusBadge(String status) {
    switch (status) {
      case '已处理':
        return const AmitiaStatusBadge(label: '已处理', type: BadgeType.success);
      case '死信':
        return const AmitiaStatusBadge(label: '死信', type: BadgeType.error);
      default:
        return AmitiaStatusBadge(label: status, type: BadgeType.neutral);
    }
  }

  void _showEventDetailSheet(BuildContext context, Map<String, dynamic> event, bool isDeadLetter) {
    final type = event['type'] as String? ?? '';
    final status = event['status'] as String? ?? '';
    final timeStr = event['time'] as String? ?? '';
    final detail = event['detail'] as String?;
    final id = event['id'] as String? ?? '';

    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(20))),
      builder: (context) {
        return SafeArea(
          child: Padding(
            padding: const EdgeInsets.fromLTRB(20, 0, 20, 34),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const SizedBox(height: 8),
                Center(
                  child: Container(width: 40, height: 4, decoration: BoxDecoration(color: context.borderPrimary, borderRadius: BorderRadius.circular(2))),
                ),
                const SizedBox(height: 20),
                Text('事件详情', style: AppTypography.pageTitle(context)),
                const SizedBox(height: 16),
                _buildDetailRow(context, '事件类型', type),
                _buildDetailRow(context, '状态', status),
                _buildDetailRow(context, '时间', timeStr),
                _buildDetailRow(context, '事件 ID', id),
                if (detail != null)
                  _buildDetailRow(context, '详情', detail),
                const SizedBox(height: 20),
                if (isDeadLetter) ...[
                  Row(
                    children: [
                      Expanded(
                        child: AmitiaButton(
                          label: '重放',
                          icon: Icons.replay,
                          onPressed: () {
                            Navigator.pop(context);
                            _showReplayConfirm(context, event);
                          },
                        ),
                      ),
                      SizedBox(width: AppSpacing.sm),
                      Expanded(
                        child: AmitiaButton(
                          label: '丢弃',
                          isDestructive: true,
                          icon: Icons.delete_outline,
                          onPressed: () {
                            Navigator.pop(context);
                            _showDiscardConfirm(context, event);
                          },
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: 8),
                ],
                AmitiaButton(
                  label: '关闭',
                  isFullWidth: true,
                  isSecondary: true,
                  onPressed: () => Navigator.pop(context),
                ),
              ],
            ),
          ),
        );
      },
    );
  }

  Widget _buildDetailRow(BuildContext context, String label, String value) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 70,
            child: Text(label, style: AppTypography.label(context).copyWith(color: context.textTertiary)),
          ),
          Expanded(child: Text(value, style: AppTypography.body(context))),
        ],
      ),
    );
  }

  void _showReplayConfirm(BuildContext context, Map<String, dynamic> event) {
    showDialog(
      context: context,
      builder: (context) {
        return AlertDialog(
          backgroundColor: context.surfacePrimary,
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('重放事件', style: AppTypography.cardTitle(context)),
          content: Text('确定要重放事件「${event['type']}」吗？事件将被重新投递处理。', style: AppTypography.body(context)),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(context),
              child: Text('取消', style: TextStyle(color: context.textSecondary)),
            ),
            TextButton(
              onPressed: () {
                setState(() {
                  final idx = _events.indexWhere((e) => e['id'] == event['id']);
                  if (idx >= 0) {
                    _events[idx]['status'] = '已处理';
                  }
                });
                Navigator.pop(context);
                ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('已重放事件：${event['type']}')));
              },
              child: Text('重放', style: TextStyle(color: context.accentPrimary)),
            ),
          ],
        );
      },
    );
  }

  void _showDiscardConfirm(BuildContext context, Map<String, dynamic> event) {
    showDialog(
      context: context,
      builder: (context) {
        return AlertDialog(
          backgroundColor: context.surfacePrimary,
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('丢弃事件', style: AppTypography.cardTitle(context)),
          content: Text('确定要丢弃事件「${event['type']}」吗？此操作不可恢复。', style: AppTypography.body(context)),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(context),
              child: Text('取消', style: TextStyle(color: context.textSecondary)),
            ),
            TextButton(
              onPressed: () {
                setState(() {
                  _events.removeWhere((e) => e['id'] == event['id']);
                });
                Navigator.pop(context);
                ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('已丢弃事件：${event['type']}')));
              },
              child: Text('丢弃', style: TextStyle(color: context.error)),
            ),
          ],
        );
      },
    );
  }
}

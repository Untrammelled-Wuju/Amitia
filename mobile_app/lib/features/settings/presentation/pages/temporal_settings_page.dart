import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../shared/models/models.dart';
import '../../../../shared/mock_data/mock_data.dart';

class TemporalSettingsPage extends ConsumerStatefulWidget {
  const TemporalSettingsPage({super.key});

  @override
  ConsumerState<TemporalSettingsPage> createState() => _TemporalSettingsPageState();
}

class _TemporalSettingsPageState extends ConsumerState<TemporalSettingsPage> {
  bool _timeAwareness = true;
  String _timezone = 'Asia/Shanghai (UTC+8)';
  String _dateFormat = 'YYYY-MM-DD';
  late List<TimeAnchor> _anchors;

  static const _timezones = ['Asia/Shanghai (UTC+8)', 'Asia/Tokyo (UTC+9)', 'America/New_York (UTC-5)', 'Europe/London (UTC+0)'];
  static const _dateFormats = ['YYYY-MM-DD', 'DD/MM/YYYY', 'MM/DD/YYYY', 'YYYY年MM月DD日'];

  @override
  void initState() {
    super.initState();
    _anchors = List.from(MockSettings.timeAnchors);
  }

  List<TimeAnchor> get _periodicAnchors => _anchors.where((a) => a.type == '周期锚点').toList();
  List<TimeAnchor> get _specialDates => _anchors.where((a) => a.type == '特殊日期').toList();

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '时间感知设置', showBackButton: true),
      body: ListView(
        padding: const EdgeInsets.symmetric(vertical: AppSpacing.md),
        children: [
          _SectionLabel(text: '基础设置'),
          const SizedBox(height: AppSpacing.sm),
          _buildCard([
            AmitiaSwitchTile(
              title: '时间感知',
              subtitle: '让 AI 理解当前时间和时间上下文',
              value: _timeAwareness,
              onChanged: (v) => setState(() => _timeAwareness = v),
            ),
            _divider(),
            _buildDropdownTile(
              icon: Icons.public,
              title: '时区',
              value: _timezone,
              options: _timezones,
              onChanged: (v) => setState(() => _timezone = v),
            ),
            _divider(),
            _buildDropdownTile(
              icon: Icons.date_range,
              title: '日期格式',
              value: _dateFormat,
              options: _dateFormats,
              onChanged: (v) => setState(() => _dateFormat = v),
            ),
          ]),
          const SizedBox(height: AppSpacing.sectionGap),
          AmitiaSectionHeader(
            title: '周期锚点',
            actionText: '新增',
            onAction: () => _showAnchorSheet(null),
          ),
          const SizedBox(height: AppSpacing.sm),
          ..._periodicAnchors.map((a) => _buildAnchorTile(a)),
          if (_periodicAnchors.isEmpty) _buildEmpty('暂无周期锚点'),
          const SizedBox(height: AppSpacing.sectionGap),
          AmitiaSectionHeader(
            title: '特殊日期',
            actionText: '新增',
            onAction: () => _showAnchorSheet(null, isSpecial: true),
          ),
          const SizedBox(height: AppSpacing.sm),
          ..._specialDates.map((a) => _buildAnchorTile(a)),
          if (_specialDates.isEmpty) _buildEmpty('暂无特殊日期'),
          const SizedBox(height: AppSpacing.xl),
        ],
      ),
    );
  }

  Widget _buildCard(List<Widget> children) {
    return Container(
      margin: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Column(children: children),
    );
  }

  Widget _divider() {
    return Padding(
      padding: const EdgeInsets.only(left: 56),
      child: Divider(height: 1, thickness: 0.5, color: context.borderSecondary),
    );
  }

  Widget _buildAnchorTile(TimeAnchor anchor) {
    return Container(
      margin: const EdgeInsets.only(left: AppSpacing.pagePadding, right: AppSpacing.pagePadding, bottom: AppSpacing.sm),
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.lg, vertical: 12),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Row(
        children: [
          Container(
            width: 36,
            height: 36,
            decoration: BoxDecoration(color: context.accentSoft, borderRadius: AppRadius.brSmall),
            child: Icon(
              anchor.type == '周期锚点' ? Icons.repeat : Icons.cake_outlined,
              size: 18,
              color: context.accentPrimary,
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(anchor.name, style: AppTypography.body(context)),
                const SizedBox(height: 2),
                Text('${anchor.type} · ${anchor.value}', style: AppTypography.caption(context)),
              ],
            ),
          ),
          GestureDetector(
            onTap: () => _showAnchorSheet(anchor),
            child: Padding(
              padding: const EdgeInsets.all(4),
              child: Icon(Icons.edit_outlined, size: 18, color: context.textTertiary),
            ),
          ),
          const SizedBox(width: 4),
          GestureDetector(
            onTap: () => _confirmDelete(anchor),
            child: Padding(
              padding: const EdgeInsets.all(4),
              child: Icon(Icons.delete_outline, size: 18, color: context.error),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildEmpty(String text) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.lg),
      child: Center(child: Text(text, style: AppTypography.caption(context))),
    );
  }

  Widget _buildDropdownTile({
    required IconData icon,
    required String title,
    required String value,
    required List<String> options,
    required ValueChanged<String> onChanged,
  }) {
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: () => _showOptionSheet(title, options, value, onChanged),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: AppSpacing.lg, vertical: 13),
        child: Row(
          children: [
            Container(
              width: 32,
              height: 32,
              decoration: BoxDecoration(color: context.accentSoft, shape: BoxShape.circle),
              child: Icon(icon, size: 17, color: context.accentPrimary),
            ),
            const SizedBox(width: 12),
            Expanded(child: Text(title, style: AppTypography.body(context))),
            Text(value, style: AppTypography.caption(context)),
            const SizedBox(width: 4),
            Icon(Icons.chevron_right, size: 20, color: context.textTertiary),
          ],
        ),
      ),
    );
  }

  void _showOptionSheet(String title, List<String> options, String current, ValueChanged<String> onChanged) {
    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(20))),
      builder: (ctx) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Padding(padding: const EdgeInsets.all(AppSpacing.lg), child: Text(title, style: AppTypography.sectionTitle(context))),
            ...options.map((opt) => ListTile(
                  leading: Icon(opt == current ? Icons.radio_button_checked : Icons.radio_button_off,
                      size: 20, color: opt == current ? context.accentPrimary : context.textTertiary),
                  title: Text(opt, style: AppTypography.body(context)),
                  onTap: () { onChanged(opt); Navigator.pop(ctx); },
                )),
            const SizedBox(height: AppSpacing.sm),
          ],
        ),
      ),
    );
  }

  void _showAnchorSheet(TimeAnchor? existing, {bool isSpecial = false}) {
    final nameCtrl = TextEditingController(text: existing?.name ?? '');
    final valueCtrl = TextEditingController(text: existing?.value ?? '');
    bool isPeriodic = existing?.type != '特殊日期' && !isSpecial;

    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(20))),
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setSheetState) {
          return SafeArea(
            child: Padding(
              padding: EdgeInsets.fromLTRB(AppSpacing.lg, AppSpacing.lg, AppSpacing.lg, MediaQuery.of(ctx).viewInsets.bottom + AppSpacing.lg),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(existing == null ? '新增时间锚点' : '编辑时间锚点', style: AppTypography.sectionTitle(context)),
                  const SizedBox(height: AppSpacing.lg),
                  AmitiaSegmentedControl(
                    segments: const ['周期锚点', '特殊日期'],
                    selectedIndex: isPeriodic ? 0 : 1,
                    onChanged: (i) => setSheetState(() => isPeriodic = i == 0),
                  ),
                  const SizedBox(height: AppSpacing.md),
                  Text('名称', style: AppTypography.label(context)),
                  const SizedBox(height: 4),
                  AmitiaTextField(hintText: isPeriodic ? '如：起床' : '如：生日', controller: nameCtrl),
                  const SizedBox(height: AppSpacing.md),
                  Text(isPeriodic ? '时间 (HH:mm)' : '日期 (YYYY-MM-DD)', style: AppTypography.label(context)),
                  const SizedBox(height: 4),
                  AmitiaTextField(hintText: isPeriodic ? '07:00' : '1995-06-15', controller: valueCtrl),
                  const SizedBox(height: AppSpacing.lg),
                  AmitiaButton(
                    label: '保存',
                    isFullWidth: true,
                    onPressed: () {
                      if (nameCtrl.text.isEmpty || valueCtrl.text.isEmpty) return;
                      setState(() {
                        if (existing != null) {
                          final idx = _anchors.indexWhere((a) => a.id == existing.id);
                          _anchors[idx] = TimeAnchor(
                            id: existing.id,
                            name: nameCtrl.text,
                            type: isPeriodic ? '周期锚点' : '特殊日期',
                            value: valueCtrl.text,
                          );
                        } else {
                          _anchors.add(TimeAnchor(
                            id: 'ta${DateTime.now().millisecondsSinceEpoch}',
                            name: nameCtrl.text,
                            type: isPeriodic ? '周期锚点' : '特殊日期',
                            value: valueCtrl.text,
                          ));
                        }
                      });
                      Navigator.pop(ctx);
                      ScaffoldMessenger.of(context).showSnackBar(
                        SnackBar(content: Text(existing == null ? '已添加锚点' : '已更新锚点'), duration: const Duration(seconds: 1)),
                      );
                    },
                  ),
                ],
              ),
            ),
          );
        },
      ),
    );
  }

  void _confirmDelete(TimeAnchor anchor) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
        title: Text('删除锚点', style: AppTypography.cardTitle(context)),
        content: Text('确定要删除「${anchor.name}」吗？', style: AppTypography.body(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              setState(() => _anchors.removeWhere((a) => a.id == anchor.id));
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(content: Text('已删除'), duration: Duration(seconds: 1)),
              );
            },
            child: Text('删除', style: TextStyle(color: context.error)),
          ),
        ],
      ),
    );
  }
}

class _SectionLabel extends StatelessWidget {
  final String text;
  const _SectionLabel({required this.text});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.sm, AppSpacing.pagePadding, AppSpacing.sm),
      child: Text(text, style: AppTypography.caption(context)),
    );
  }
}

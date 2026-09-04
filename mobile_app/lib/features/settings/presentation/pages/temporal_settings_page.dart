import 'dart:convert';

import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/models/character.dart';
import '../../../../core/native_bridge/providers/native_bridge_relay_provider.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../shared/models/settings_models.dart';

final _temporalConfigProvider = FutureProvider<Map<String, dynamic>?>((ref) async {
  return ref.read(temporalServiceProvider).config();
});

final _temporalAnchorsProvider =
    FutureProvider<List<Map<String, dynamic>>>((ref) async {
  return ref.read(temporalServiceProvider).anchors(limit: 500);
});

final _reunionEpisodesProvider = FutureProvider.autoDispose
    .family<List<Map<String, dynamic>>, String>((ref, characterId) async {
  if (characterId.trim().isEmpty) return const <Map<String, dynamic>>[];
  return ref
      .read(temporalServiceProvider)
      .reunionEpisodes(characterId, limit: 50);
});

class TemporalSettingsPage extends ConsumerWidget {
  const TemporalSettingsPage({super.key});

  static const _timezones = <String>[
    'Asia/Shanghai',
    'Asia/Tokyo',
    'America/New_York',
    'America/Los_Angeles',
    'Europe/London',
    'Europe/Berlin',
    'Australia/Sydney',
    'UTC',
  ];

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final configAsync = ref.watch(_temporalConfigProvider);
    final anchorsAsync = ref.watch(_temporalAnchorsProvider);

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '时间感知设置',
        showBackButton: true,
        fallbackRoute: AppRoutes.settings,
      ),
      body: configAsync.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (err, _) => _LoadError(
          error: err,
          onRetry: () {
            ref.invalidate(_temporalConfigProvider);
            ref.invalidate(_temporalAnchorsProvider);
          },
        ),
        data: (config) {
          final anchors = anchorsAsync.maybeWhen(
            data: (items) => items
                .map(TimeAnchor.fromJson)
                .where((anchor) => anchor.id.isNotEmpty)
                .toList(growable: false),
            orElse: () => const <TimeAnchor>[],
          );
          return _TemporalContent(
            config: config,
            anchors: anchors,
            timezones: _timezones,
            onRefresh: () {
              ref.invalidate(_temporalConfigProvider);
              ref.invalidate(_temporalAnchorsProvider);
            },
          );
        },
      ),
    );
  }
}

class _LoadError extends StatelessWidget {
  final Object error;
  final VoidCallback onRetry;

  const _LoadError({required this.error, required this.onRetry});

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.error_outline, size: 48, color: context.textSecondary),
            const SizedBox(height: 16),
            Text(
              '加载失败: ${error.toString().replaceFirst('Exception: ', '')}',
              style: AppTypography.body(context).copyWith(color: context.error),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 16),
            AmitiaButton(label: '重试', onPressed: onRetry),
          ],
        ),
      ),
    );
  }
}

class _TemporalContent extends ConsumerStatefulWidget {
  final Map<String, dynamic>? config;
  final List<TimeAnchor> anchors;
  final List<String> timezones;
  final VoidCallback onRefresh;

  const _TemporalContent({
    this.config,
    required this.anchors,
    required this.timezones,
    required this.onRefresh,
  });

  @override
  ConsumerState<_TemporalContent> createState() => _TemporalContentState();
}

class _TemporalContentState extends ConsumerState<_TemporalContent> {
  late bool _enabled;
  late String _timezoneMode;
  late String _timezone;
  late String _locale;
  late int _weekStart;
  late String _hemisphere;
  late bool _quietHoursEnabled;
  late String _quietHoursStart;
  late String _quietHoursEnd;
  late bool _autoDetectTimezone;
  late bool _holidayAwareness;
  late bool _daypartAwareness;
  late bool _anniversaryAwareness;
  late bool _memoryResonance;
  late bool _allowSharedDateMention;
  String _pendingTimezoneSuggestion = '';
  bool _resolvingTimezoneSuggestion = false;
  late List<TimeAnchor> _anchors;
  String? _selectedReunionCharacterId;
  bool _savingProfile = false;

  static const _timezoneModeLabels = <String, String>{
    'follow_device': '跟随设备',
    'fixed': '固定时区',
  };

  static const _localeLabels = <String, String>{
    'zh-CN': '简体中文',
    'zh-TW': '繁體中文',
    'en-US': 'English (US)',
    'ja-JP': '日本語',
  };

  static const _anchorTypeLabels = <String, String>{
    'birthday': '生日',
    'anniversary': '纪念日',
    'relationship_anniversary': '关系纪念日',
    'first_meeting': '首次相识',
    'shared_memory': '共同经历',
    'holiday': '节日',
    'deadline': '截止日期',
    'appointment': '预约',
    'exam': '考试',
    'travel': '旅行',
    'work_event': '工作事件',
    'class_event': '课程事件',
    'custom': '自定义',
  };

  @override
  void initState() {
    super.initState();
    _applyConfig(widget.config);
    _anchors = List<TimeAnchor>.from(widget.anchors);
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _detectDeviceTimezoneSuggestion();
    });
  }

  @override
  void didUpdateWidget(covariant _TemporalContent oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (!identical(widget.config, oldWidget.config)) {
      _applyConfig(widget.config);
    }
    if (!identical(widget.anchors, oldWidget.anchors)) {
      _anchors = List<TimeAnchor>.from(widget.anchors);
    }
  }

  void _applyConfig(Map<String, dynamic>? config) {
    _enabled = config?['enabled'] as bool? ?? true;
    _timezoneMode = (config?['timezoneMode'] ?? 'fixed').toString();
    if (!_timezoneModeLabels.containsKey(_timezoneMode)) {
      _timezoneMode = 'fixed';
    }
    _timezone = (config?['timezone'] ?? 'Asia/Shanghai').toString();
    _locale = (config?['locale'] ?? 'zh-CN').toString();
    if (!_localeLabels.containsKey(_locale)) _locale = 'zh-CN';
    _weekStart = (config?['weekStart'] as num?)?.toInt() == 0 ? 0 : 1;
    _hemisphere = (config?['hemisphere'] ?? 'unknown').toString();
    if (!{'unknown', 'north', 'south'}.contains(_hemisphere)) _hemisphere = 'unknown';
    final quietHours = _decodeQuietHours(config?['quietHoursJson']);
    _quietHoursEnabled = quietHours['enabled'] as bool;
    _quietHoursStart = quietHours['start'] as String;
    _quietHoursEnd = quietHours['end'] as String;
    _autoDetectTimezone = config?['autoDetectTimezone'] as bool? ?? true;
    _holidayAwareness = config?['holidayAwareness'] as bool? ?? true;
    _daypartAwareness = config?['daypartAwareness'] as bool? ?? true;
    _anniversaryAwareness =
        config?['anniversaryAwareness'] as bool? ?? true;
    _memoryResonance = config?['memoryResonance'] as bool? ?? true;
    _allowSharedDateMention =
        config?['allowSharedDateMention'] as bool? ?? true;
    _pendingTimezoneSuggestion =
        (config?['pendingTimezoneSuggestion'] ?? '').toString().trim();
  }


  Future<void> _detectDeviceTimezoneSuggestion() async {
    if (!mounted || kIsWeb || !_autoDetectTimezone || _timezoneMode != 'follow_device') return;
    if (_pendingTimezoneSuggestion.isNotEmpty) return;
    final platform = switch (defaultTargetPlatform) {
      TargetPlatform.android => 'android',
      TargetPlatform.iOS => 'ios',
      TargetPlatform.windows => 'windows',
      _ => null,
    };
    if (platform == null) return;
    try {
      final dispatcher = ref.read(nativeBridgePlatformDispatcherProvider);
      final response = await dispatcher.execute(<String, dynamic>{
        'protocolVersion': 1,
        'requestId': 'temporal-timezone-${DateTime.now().microsecondsSinceEpoch}',
        'platform': platform,
        'operation': 'device.timezone.get',
        'payload': const <String, dynamic>{},
      });
      if (!const {'success', 'ok'}.contains((response['status'] ?? '').toString())) return;
      final rawResult = response['result'];
      if (rawResult is! Map) return;
      final result = Map<String, dynamic>.from(rawResult);
      final candidate = (result['ianaTimezone'] ?? '').toString().trim();
      if (candidate.isEmpty || !_looksLikeIanaTimezone(candidate) || candidate == _timezone.trim()) return;
      final profile = await ref.read(temporalServiceProvider).suggestTimezone(candidate);
      if (!mounted || profile == null) return;
      setState(() => _applyConfig(profile));
    } catch (_) {
      // Device timezone detection is advisory. Keep the user's persisted timezone
      // unchanged when native detection or backend validation is unavailable.
    }
  }

  List<TimeAnchor> get _periodicAnchors =>
      _anchors.where((anchor) => anchor.isPeriodic).toList(growable: false);

  List<TimeAnchor> get _specialAnchors => _anchors
      .where((anchor) => anchor.isSpecialDate)
      .toList(growable: false);

  List<TimeAnchor> get _otherAnchors => _anchors
      .where((anchor) => !anchor.isPeriodic && !anchor.isSpecialDate)
      .toList(growable: false);

  Future<void> _resolveTimezoneSuggestion(bool accept) async {
    if (_resolvingTimezoneSuggestion || _pendingTimezoneSuggestion.isEmpty) return;
    setState(() => _resolvingTimezoneSuggestion = true);
    try {
      final service = ref.read(temporalServiceProvider);
      final profile = accept
          ? await service.acceptTimezoneSuggestion()
          : await service.rejectTimezoneSuggestion();
      if (!mounted) return;
      if (profile != null) {
        _applyConfig(profile);
      } else {
        _pendingTimezoneSuggestion = '';
      }
      widget.onRefresh();
      _showMessage(accept ? '已接受设备时区建议' : '已拒绝设备时区建议');
    } catch (error) {
      if (mounted) _showMessage(_errorText(error), error: true);
    } finally {
      if (mounted) setState(() => _resolvingTimezoneSuggestion = false);
    }
  }

  Future<void> _saveConfig() async {
    if (_timezone.trim().isEmpty || !_looksLikeIanaTimezone(_timezone)) {
      _showMessage('请输入有效的 IANA 时区，例如 Asia/Shanghai', error: true);
      return;
    }
    setState(() => _savingProfile = true);
    try {
      await ref.read(temporalServiceProvider).updateConfig({
        'enabled': _enabled,
        'timezoneMode': _timezoneMode,
        'timezone': _timezone.trim(),
        'locale': _locale,
        'weekStart': _weekStart,
        'hemisphere': _hemisphere,
        'quietHoursJson': jsonEncode({
          'enabled': _quietHoursEnabled,
          'start': _quietHoursStart,
          'end': _quietHoursEnd,
        }),
        'autoDetectTimezone': _autoDetectTimezone,
        'holidayAwareness': _holidayAwareness,
        'daypartAwareness': _daypartAwareness,
        'anniversaryAwareness': _anniversaryAwareness,
        'memoryResonance': _memoryResonance,
        'allowSharedDateMention': _allowSharedDateMention,
        'source': 'explicit',
        'confidence': 100,
      });
      if (!mounted) return;
      widget.onRefresh();
      _showMessage('时间感知设置已保存');
    } catch (error) {
      if (!mounted) return;
      _showMessage(_errorText(error), error: true);
    } finally {
      if (mounted) setState(() => _savingProfile = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final charactersAsync = ref.watch(characterListProvider);
    final characters =
        charactersAsync.valueOrNull ?? const <CharacterDto>[];
    final reunionCharacterId = _resolveReunionCharacterId(characters);
    final reunionEpisodesAsync = reunionCharacterId.isEmpty
        ? null
        : ref.watch(_reunionEpisodesProvider(reunionCharacterId));

    return ListView(
      padding: EdgeInsets.symmetric(vertical: AppSpacing.md),
      children: [
        if (_pendingTimezoneSuggestion.isNotEmpty) ...[
          Padding(
            padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
            child: Container(
              padding: EdgeInsets.all(AppSpacing.lg),
              decoration: BoxDecoration(
                color: context.accentSoft,
                borderRadius: AppRadius.brMedium,
                border: Border.all(color: context.accentPrimary.withValues(alpha: 0.22)),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Icon(Icons.public, size: 19, color: context.accentPrimary),
                      SizedBox(width: AppSpacing.sm),
                      Expanded(child: Text('检测到时区变化', style: AppTypography.cardTitle(context))),
                    ],
                  ),
                  SizedBox(height: AppSpacing.sm),
                  Text(
                    '当前设置：$_timezone；设备建议：$_pendingTimezoneSuggestion。接受后才会写入，拒绝后会清除建议，不会自动覆盖你的选择。',
                    style: AppTypography.caption(context),
                  ),
                  SizedBox(height: AppSpacing.md),
                  Row(
                    children: [
                      Expanded(
                        child: AmitiaButtonOutline(
                          label: '拒绝',
                          onPressed: _resolvingTimezoneSuggestion ? null : () => _resolveTimezoneSuggestion(false),
                        ),
                      ),
                      SizedBox(width: AppSpacing.sm),
                      Expanded(
                        child: AmitiaButton(
                          label: _resolvingTimezoneSuggestion ? '处理中...' : '接受建议',
                          onPressed: _resolvingTimezoneSuggestion ? null : () => _resolveTimezoneSuggestion(true),
                        ),
                      ),
                    ],
                  ),
                ],
              ),
            ),
          ),
          SizedBox(height: AppSpacing.sectionGap),
        ],
        const _SectionLabel(text: '基础设置'),
        SizedBox(height: AppSpacing.sm),
        _buildCard([
          AmitiaSwitchTile(
            title: '时间感知',
            subtitle: '让 AI 使用真实时间、日期、节日与时间上下文',
            value: _enabled,
            onChanged: (value) => setState(() => _enabled = value),
          ),
          _divider(),
          _buildDropdownTile(
            icon: Icons.sync_alt,
            title: '时区模式',
            value: _timezoneMode,
            options: _timezoneModeLabels.keys.toList(growable: false),
            labels: _timezoneModeLabels,
            onChanged: (value) => setState(() => _timezoneMode = value),
          ),
          _divider(),
          _buildTimezoneTile(),
          _divider(),
          _buildDropdownTile(
            icon: Icons.language,
            title: '区域语言',
            value: _locale,
            options: _localeLabels.keys.toList(growable: false),
            labels: _localeLabels,
            onChanged: (value) => setState(() => _locale = value),
          ),
          _divider(),
          _buildDropdownTile(
            icon: Icons.calendar_view_week_outlined,
            title: '每周起始日',
            value: _weekStart.toString(),
            options: const ['1', '0'],
            labels: const {'1': '星期一', '0': '星期日'},
            onChanged: (value) => setState(() => _weekStart = int.parse(value)),
          ),
          _divider(),
          _buildDropdownTile(
            icon: Icons.public_outlined,
            title: '所在半球',
            value: _hemisphere,
            options: const ['unknown', 'north', 'south'],
            labels: const {'unknown': '未知', 'north': '北半球', 'south': '南半球'},
            onChanged: (value) => setState(() => _hemisphere = value),
          ),
          _divider(),
          _buildQuietHoursTile(),
          _divider(),
          AmitiaSwitchTile(
            title: '自动检测设备时区',
            subtitle: '后端收到设备时区时优先使用设备时区',
            value: _autoDetectTimezone,
            onChanged: (value) =>
                setState(() => _autoDetectTimezone = value),
          ),
        ]),
        SizedBox(height: AppSpacing.sectionGap),
        const _SectionLabel(text: '感知策略'),
        SizedBox(height: AppSpacing.sm),
        _buildCard([
          AmitiaSwitchTile(
            title: '节日感知',
            subtitle: '允许识别当前地区的节日和特殊日历事件',
            value: _holidayAwareness,
            onChanged: (value) =>
                setState(() => _holidayAwareness = value),
          ),
          _divider(),
          AmitiaSwitchTile(
            title: '时段感知',
            subtitle: '理解早晨、下午、深夜等日内时间段',
            value: _daypartAwareness,
            onChanged: (value) =>
                setState(() => _daypartAwareness = value),
          ),
          _divider(),
          AmitiaSwitchTile(
            title: '纪念日感知',
            subtitle: '允许时间锚点参与纪念日和周年语义',
            value: _anniversaryAwareness,
            onChanged: (value) =>
                setState(() => _anniversaryAwareness = value),
          ),
          _divider(),
          AmitiaSwitchTile(
            title: '记忆时间共振',
            subtitle: '允许时间信息影响相关记忆的检索排序',
            value: _memoryResonance,
            onChanged: (value) =>
                setState(() => _memoryResonance = value),
          ),
          _divider(),
          AmitiaSwitchTile(
            title: '允许提及共享日期',
            subtitle: '允许角色在合适语境自然提及共享日期信息',
            value: _allowSharedDateMention,
            onChanged: (value) =>
                setState(() => _allowSharedDateMention = value),
          ),
        ]),
        SizedBox(height: AppSpacing.sectionGap),
        AmitiaSectionHeader(
          title: '周期锚点',
          actionText: '新增',
          onAction: () => _showAnchorSheet(null, initialKind: 'recurring'),
        ),
        SizedBox(height: AppSpacing.sm),
        ..._periodicAnchors.map(
          (anchor) => _buildAnchorTile(anchor, characters),
        ),
        if (_periodicAnchors.isEmpty) _buildEmpty('暂无周期锚点'),
        SizedBox(height: AppSpacing.sectionGap),
        AmitiaSectionHeader(
          title: '日期锚点',
          actionText: '新增',
          onAction: () => _showAnchorSheet(null, initialKind: 'annual_date'),
        ),
        SizedBox(height: AppSpacing.sm),
        ..._specialAnchors.map(
          (anchor) => _buildAnchorTile(anchor, characters),
        ),
        if (_specialAnchors.isEmpty) _buildEmpty('暂无日期锚点'),
        if (_otherAnchors.isNotEmpty) ...[
          SizedBox(height: AppSpacing.sectionGap),
          const _SectionLabel(text: '其他时间锚点'),
          SizedBox(height: AppSpacing.sm),
          ..._otherAnchors.map(
            (anchor) => _buildAnchorTile(anchor, characters),
          ),
        ],
        SizedBox(height: AppSpacing.sectionGap),
        _buildReunionSection(
          characters: characters,
          characterId: reunionCharacterId,
          episodesAsync: reunionEpisodesAsync,
        ),
        SizedBox(height: AppSpacing.sectionGap),
        Padding(
          padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
          child: AmitiaButton(
            label: _savingProfile ? '保存中…' : '保存配置',
            icon: Icons.check,
            isFullWidth: true,
            onPressed: _savingProfile ? null : _saveConfig,
          ),
        ),
        SizedBox(height: AppSpacing.xl),
      ],
    );
  }

  List<String> get _timezoneOptions {
    final values = <String>{...widget.timezones};
    if (_timezone.trim().isNotEmpty) values.add(_timezone.trim());
    return values.toList(growable: false);
  }

  Map<String, dynamic> _decodeQuietHours(dynamic raw) {
    final defaults = <String, dynamic>{'enabled': true, 'start': '23:00', 'end': '07:00'};
    if (raw is! String || raw.trim().isEmpty) return defaults;
    try {
      final decoded = jsonDecode(raw);
      if (decoded is Map) {
        return <String, dynamic>{
          'enabled': decoded['enabled'] is bool ? decoded['enabled'] : true,
          'start': (decoded['start'] ?? '23:00').toString(),
          'end': (decoded['end'] ?? '07:00').toString(),
        };
      }
    } catch (_) {}
    return defaults;
  }

  Widget _buildQuietHoursTile() {
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: _showQuietHoursSheet,
      child: Padding(
        padding: EdgeInsets.symmetric(horizontal: AppSpacing.lg, vertical: 13),
        child: Row(
          children: [
            Container(
              width: 32,
              height: 32,
              decoration: BoxDecoration(color: context.accentSoft, shape: BoxShape.circle),
              child: Icon(Icons.bedtime_outlined, size: 17, color: context.accentPrimary),
            ),
            const SizedBox(width: 12),
            Expanded(child: Text('安静时段', style: AppTypography.body(context))),
            Text(
              _quietHoursEnabled ? '$_quietHoursStart - $_quietHoursEnd' : '关闭',
              style: AppTypography.caption(context),
            ),
            const SizedBox(width: 4),
            Icon(Icons.chevron_right, size: 20, color: context.textTertiary),
          ],
        ),
      ),
    );
  }

  Future<void> _showQuietHoursSheet() async {
    final startController = TextEditingController(text: _quietHoursStart);
    final endController = TextEditingController(text: _quietHoursEnd);
    var enabled = _quietHoursEnabled;
    final result = await showModalBottomSheet<Map<String, dynamic>>(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(20))),
      builder: (sheetContext) => StatefulBuilder(
        builder: (sheetContext, setSheetState) => SafeArea(
          child: Padding(
            padding: EdgeInsets.fromLTRB(
              AppSpacing.lg,
              AppSpacing.lg,
              AppSpacing.lg,
              MediaQuery.of(sheetContext).viewInsets.bottom + AppSpacing.lg,
            ),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('安静时段', style: AppTypography.sectionTitle(context)),
                SizedBox(height: AppSpacing.md),
                AmitiaSwitchTile(
                  title: '启用安静时段',
                  value: enabled,
                  onChanged: (value) => setSheetState(() => enabled = value),
                ),
                SizedBox(height: AppSpacing.md),
                _sheetLabel('开始时间 (HH:mm)'),
                AmitiaTextField(controller: startController, hintText: '23:00', readOnly: !enabled),
                SizedBox(height: AppSpacing.md),
                _sheetLabel('结束时间 (HH:mm)'),
                AmitiaTextField(controller: endController, hintText: '07:00', readOnly: !enabled),
                SizedBox(height: AppSpacing.lg),
                AmitiaButton(
                  label: '确定',
                  isFullWidth: true,
                  onPressed: () {
                    final start = startController.text.trim();
                    final end = endController.text.trim();
                    if (enabled && (!_validClock(start) || !_validClock(end))) {
                      _showMessage('时间格式应为 HH:mm', error: true);
                      return;
                    }
                    Navigator.pop(sheetContext, <String, dynamic>{'enabled': enabled, 'start': start, 'end': end});
                  },
                ),
              ],
            ),
          ),
        ),
      ),
    );
    startController.dispose();
    endController.dispose();
    if (result != null && mounted) {
      setState(() {
        _quietHoursEnabled = result['enabled'] == true;
        _quietHoursStart = (result['start'] ?? '23:00').toString();
        _quietHoursEnd = (result['end'] ?? '07:00').toString();
      });
    }
  }

  Widget _buildTimezoneTile() {
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: _showTimezoneSheet,
      child: Padding(
        padding: EdgeInsets.symmetric(
          horizontal: AppSpacing.lg,
          vertical: 13,
        ),
        child: Row(
          children: [
            Container(
              width: 32,
              height: 32,
              decoration: BoxDecoration(
                color: context.accentSoft,
                shape: BoxShape.circle,
              ),
              child: Icon(
                Icons.public,
                size: 17,
                color: context.accentPrimary,
              ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Text('IANA 时区', style: AppTypography.body(context)),
            ),
            Flexible(
              child: Text(
                _timezone,
                style: AppTypography.caption(context),
                overflow: TextOverflow.ellipsis,
              ),
            ),
            const SizedBox(width: 4),
            Icon(Icons.chevron_right, size: 20, color: context.textTertiary),
          ],
        ),
      ),
    );
  }

  Future<void> _showTimezoneSheet() async {
    final controller = TextEditingController(text: _timezone);
    final selected = await showModalBottomSheet<String>(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (sheetContext) => SafeArea(
        child: Padding(
          padding: EdgeInsets.fromLTRB(
            AppSpacing.lg,
            AppSpacing.lg,
            AppSpacing.lg,
            MediaQuery.of(sheetContext).viewInsets.bottom + AppSpacing.lg,
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('设置 IANA 时区', style: AppTypography.sectionTitle(context)),
              SizedBox(height: AppSpacing.md),
              AmitiaTextField(
                hintText: '例如 Asia/Shanghai',
                controller: controller,
              ),
              SizedBox(height: AppSpacing.md),
              Wrap(
                spacing: 8,
                runSpacing: 8,
                children: _timezoneOptions
                    .map(
                      (zone) => ActionChip(
                        label: Text(zone),
                        onPressed: () => controller.text = zone,
                      ),
                    )
                    .toList(growable: false),
              ),
              SizedBox(height: AppSpacing.lg),
              AmitiaButton(
                label: '使用此时区',
                isFullWidth: true,
                onPressed: () {
                  final zone = controller.text.trim();
                  if (!_looksLikeIanaTimezone(zone)) {
                    _showMessage(
                      '请输入有效的 IANA 时区，例如 Asia/Shanghai',
                      error: true,
                    );
                    return;
                  }
                  Navigator.pop(sheetContext, zone);
                },
              ),
            ],
          ),
        ),
      ),
    );
    controller.dispose();
    if (selected != null && selected.isNotEmpty && mounted) {
      setState(() => _timezone = selected);
    }
  }

  Widget _buildCard(List<Widget> children) {
    return Container(
      margin: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
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
      child: Divider(
        height: 1,
        thickness: 0.5,
        color: context.borderSecondary,
      ),
    );
  }

  Widget _buildDropdownTile({
    required IconData icon,
    required String title,
    required String value,
    required List<String> options,
    Map<String, String> labels = const <String, String>{},
    required ValueChanged<String> onChanged,
  }) {
    final display = labels[value] ?? value;
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: () => _showOptionSheet(
        title,
        options,
        value,
        onChanged,
        labels: labels,
      ),
      child: Padding(
        padding: EdgeInsets.symmetric(
          horizontal: AppSpacing.lg,
          vertical: 13,
        ),
        child: Row(
          children: [
            Container(
              width: 32,
              height: 32,
              decoration: BoxDecoration(
                color: context.accentSoft,
                shape: BoxShape.circle,
              ),
              child: Icon(icon, size: 17, color: context.accentPrimary),
            ),
            const SizedBox(width: 12),
            Expanded(child: Text(title, style: AppTypography.body(context))),
            Flexible(
              child: Text(
                display,
                style: AppTypography.caption(context),
                overflow: TextOverflow.ellipsis,
              ),
            ),
            const SizedBox(width: 4),
            Icon(Icons.chevron_right, size: 20, color: context.textTertiary),
          ],
        ),
      ),
    );
  }

  void _showOptionSheet(
    String title,
    List<String> options,
    String current,
    ValueChanged<String> onChanged, {
    Map<String, String> labels = const <String, String>{},
  }) {
    showModalBottomSheet<void>(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (ctx) => SafeArea(
        child: ListView(
          shrinkWrap: true,
          children: [
            Padding(
              padding: EdgeInsets.all(AppSpacing.lg),
              child: Text(title, style: AppTypography.sectionTitle(context)),
            ),
            ...options.map(
              (option) => ListTile(
                leading: Icon(
                  option == current
                      ? Icons.radio_button_checked
                      : Icons.radio_button_off,
                  size: 20,
                  color: option == current
                      ? context.accentPrimary
                      : context.textTertiary,
                ),
                title: Text(
                  labels[option] ?? option,
                  style: AppTypography.body(context),
                ),
                onTap: () {
                  onChanged(option);
                  Navigator.pop(ctx);
                },
              ),
            ),
            SizedBox(height: AppSpacing.sm),
          ],
        ),
      ),
    );
  }

  Widget _buildAnchorTile(
    TimeAnchor anchor,
    List<CharacterDto> characters,
  ) {
    final scopeLabel = _anchorScopeLabel(anchor, characters);
    final candidate = anchor.requiresConfirmation || anchor.status == 'candidate';
    return Container(
      margin: EdgeInsets.only(
        left: AppSpacing.pagePadding,
        right: AppSpacing.pagePadding,
        bottom: AppSpacing.sm,
      ),
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.lg, vertical: 12),
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
            decoration: BoxDecoration(
              color: candidate ? context.warning.withValues(alpha: 0.12) : context.accentSoft,
              borderRadius: AppRadius.brSmall,
            ),
            child: Icon(
              anchor.isPeriodic ? Icons.repeat : Icons.event_outlined,
              size: 18,
              color: candidate ? context.warning : context.accentPrimary,
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Expanded(
                      child: Text(
                        anchor.title.isEmpty ? '未命名锚点' : anchor.title,
                        style: AppTypography.body(context),
                      ),
                    ),
                    if (candidate)
                      Text(
                        '待确认',
                        style: AppTypography.caption(context).copyWith(
                          color: context.warning,
                        ),
                      ),
                  ],
                ),
                const SizedBox(height: 2),
                Text(
                  '${anchor.displayType} · ${anchor.displayValue}',
                  style: AppTypography.caption(context),
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                ),
                const SizedBox(height: 2),
                Text(
                  '$scopeLabel · ${_anchorTypeLabels[anchor.anchorType] ?? anchor.anchorType}',
                  style: AppTypography.caption(context).copyWith(
                    color: context.textTertiary,
                  ),
                ),
              ],
            ),
          ),
          if (candidate)
            IconButton(
              tooltip: '确认锚点',
              onPressed: () => _confirmAnchor(anchor),
              icon: Icon(
                Icons.check_circle_outline,
                size: 20,
                color: context.accentPrimary,
              ),
            ),
          IconButton(
            tooltip: '编辑',
            onPressed: () => _showAnchorSheet(anchor),
            icon: Icon(
              Icons.edit_outlined,
              size: 19,
              color: context.textTertiary,
            ),
          ),
          IconButton(
            tooltip: '删除',
            onPressed: () => _confirmDelete(anchor),
            icon: Icon(Icons.delete_outline, size: 19, color: context.error),
          ),
        ],
      ),
    );
  }

  Widget _buildEmpty(String text) {
    return Padding(
      padding: EdgeInsets.symmetric(
        horizontal: AppSpacing.pagePadding,
        vertical: AppSpacing.lg,
      ),
      child: Center(
        child: Text(text, style: AppTypography.caption(context)),
      ),
    );
  }

  Future<void> _showAnchorSheet(
    TimeAnchor? existing, {
    String? initialKind,
  }) async {
    if (existing?.timeKind == 'derived') {
      _showMessage('派生时间锚点由系统维护，不能在此直接编辑');
      return;
    }
    final characters =
        ref.read(characterListProvider).valueOrNull ?? const <CharacterDto>[];
    final titleController = TextEditingController(text: existing?.title ?? '');
    final descriptionController =
        TextEditingController(text: existing?.description ?? '');
    final dateController = TextEditingController(
      text: existing?.localDate.isNotEmpty == true
          ? existing!.localDate
          : (initialKind == 'annual_date'
              ? _todayMonthDay()
              : _todayDate()),
    );
    final timeController = TextEditingController(
      text: existing?.localTime.isNotEmpty == true ? existing!.localTime : '09:00',
    );
    final instantController = TextEditingController(text: existing?.instantAtUtc ?? '');
    final endInstantController = TextEditingController(text: existing?.endAtUtc ?? '');

    var timeKind = existing?.timeKind ?? initialKind ?? 'recurring';
    if (!{'recurring', 'annual_date', 'local_date', 'local_datetime', 'instant', 'range'}.contains(timeKind)) {
      timeKind = 'local_date';
    }
    var scopeCharacterId = existing?.characterId ?? '';
    var anchorType = existing?.anchorType ??
        (timeKind == 'annual_date' ? 'anniversary' : 'custom');
    var frequency = _frequencyFromRRule(existing?.rrule ?? '');
    var importance = (existing?.importance ?? 70).clamp(0, 100).toDouble();
    var allowPromptMention = existing?.allowPromptMention ?? true;
    var allowProactiveMention = existing?.allowProactiveMention ?? false;

    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (sheetContext) => StatefulBuilder(
        builder: (sheetContext, setSheetState) {
          final kindIndex = switch (timeKind) {
            'recurring' => 0,
            'annual_date' => 1,
            'local_date' => 2,
            'local_datetime' => 3,
            'instant' => 4,
            'range' => 5,
            _ => 2,
          };
          return SafeArea(
            child: SingleChildScrollView(
              padding: EdgeInsets.fromLTRB(
                AppSpacing.lg,
                AppSpacing.lg,
                AppSpacing.lg,
                MediaQuery.of(sheetContext).viewInsets.bottom + AppSpacing.lg,
              ),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    existing == null ? '新增时间锚点' : '编辑时间锚点',
                    style: AppTypography.sectionTitle(context),
                  ),
                  SizedBox(height: AppSpacing.lg),
                  AmitiaSegmentedControl(
                    segments: const ['周期', '每年', '单次', '日期时间', 'UTC', '范围'],
                    selectedIndex: kindIndex,
                    onChanged: (index) => setSheetState(() {
                      timeKind = switch (index) {
                        0 => 'recurring',
                        1 => 'annual_date',
                        2 => 'local_date',
                        3 => 'local_datetime',
                        4 => 'instant',
                        _ => 'range',
                      };
                      if (timeKind == 'annual_date' &&
                          dateController.text.length >= 10) {
                        dateController.text = dateController.text.substring(5);
                      }
                      if (timeKind != 'annual_date' &&
                          dateController.text.length == 5) {
                        dateController.text = '${DateTime.now().year}-${dateController.text}';
                      }
                    }),
                  ),
                  SizedBox(height: AppSpacing.md),
                  _sheetLabel('名称'),
                  AmitiaTextField(
                    hintText: timeKind == 'recurring' ? '如：每天起床' : '如：生日',
                    controller: titleController,
                  ),
                  SizedBox(height: AppSpacing.md),
                  _sheetLabel('类型'),
                  DropdownButtonFormField<String>(
                    value: anchorType,
                    isExpanded: true,
                    items: <DropdownMenuItem<String>>[
                      if (!_anchorTypeLabels.containsKey(anchorType))
                        DropdownMenuItem<String>(value: anchorType, child: Text(anchorType)),
                      ..._anchorTypeLabels.entries.map(
                        (entry) => DropdownMenuItem<String>(value: entry.key, child: Text(entry.value)),
                      ),
                    ],
                    onChanged: (value) {
                      if (value != null) {
                        setSheetState(() => anchorType = value);
                      }
                    },
                  ),
                  SizedBox(height: AppSpacing.md),
                  _sheetLabel('作用范围'),
                  DropdownButtonFormField<String>(
                    value: characters.any((item) => item.id == scopeCharacterId)
                        ? scopeCharacterId
                        : '',
                    isExpanded: true,
                    items: [
                      const DropdownMenuItem<String>(
                        value: '',
                        child: Text('用户全局'),
                      ),
                      ...characters.map(
                        (character) => DropdownMenuItem<String>(
                          value: character.id,
                          child: Text('角色 · ${character.name}'),
                        ),
                      ),
                    ],
                    onChanged: existing == null
                        ? (value) => setSheetState(
                              () => scopeCharacterId = value ?? '',
                            )
                        : null,
                  ),
                  SizedBox(height: AppSpacing.md),
                  if (timeKind == 'instant' || timeKind == 'range') ...[
                    _sheetLabel(timeKind == 'range' ? '开始 UTC (ISO 8601)' : 'UTC 瞬间 (ISO 8601)'),
                    AmitiaTextField(
                      hintText: '2026-09-04T12:00:00Z',
                      controller: instantController,
                    ),
                    if (timeKind == 'range') ...[
                      SizedBox(height: AppSpacing.md),
                      _sheetLabel('结束 UTC (ISO 8601)'),
                      AmitiaTextField(
                        hintText: '2026-09-04T13:00:00Z',
                        controller: endInstantController,
                      ),
                    ],
                  ] else ...[
                    _sheetLabel(
                      timeKind == 'annual_date'
                          ? '日期 (MM-DD)'
                          : timeKind == 'recurring'
                              ? '起始日期 (YYYY-MM-DD)'
                              : '日期 (YYYY-MM-DD)',
                    ),
                    AmitiaTextField(
                      hintText: timeKind == 'annual_date' ? '06-15' : '2026-08-22',
                      controller: dateController,
                    ),
                    SizedBox(height: AppSpacing.md),
                    _sheetLabel('时间 (HH:mm)'),
                    AmitiaTextField(hintText: '09:00', controller: timeController),
                  ],
                  if (timeKind == 'recurring') ...[
                    SizedBox(height: AppSpacing.md),
                    _sheetLabel('重复频率'),
                    DropdownButtonFormField<String>(
                      value: frequency,
                      items: const [
                        DropdownMenuItem(value: 'DAILY', child: Text('每天')),
                        DropdownMenuItem(value: 'WEEKLY', child: Text('每周')),
                        DropdownMenuItem(value: 'MONTHLY', child: Text('每月')),
                        DropdownMenuItem(value: 'YEARLY', child: Text('每年')),
                      ],
                      onChanged: (value) {
                        if (value != null) {
                          setSheetState(() => frequency = value);
                        }
                      },
                    ),
                  ],
                  SizedBox(height: AppSpacing.md),
                  _sheetLabel('说明'),
                  AmitiaTextField(
                    hintText: '可选，用于描述这个时间锚点',
                    controller: descriptionController,
                  ),
                  SizedBox(height: AppSpacing.md),
                  Row(
                    children: [
                      Expanded(
                        child: Text('重要度', style: AppTypography.body(context)),
                      ),
                      Text(
                        importance.round().toString(),
                        style: AppTypography.caption(context),
                      ),
                    ],
                  ),
                  Slider(
                    value: importance,
                    min: 0,
                    max: 100,
                    divisions: 20,
                    onChanged: (value) =>
                        setSheetState(() => importance = value),
                  ),
                  SwitchListTile.adaptive(
                    contentPadding: EdgeInsets.zero,
                    title: const Text('允许在提示词中提及'),
                    value: allowPromptMention,
                    onChanged: (value) =>
                        setSheetState(() => allowPromptMention = value),
                  ),
                  SwitchListTile.adaptive(
                    contentPadding: EdgeInsets.zero,
                    title: const Text('允许主动消息提及'),
                    value: allowProactiveMention,
                    onChanged: (value) =>
                        setSheetState(() => allowProactiveMention = value),
                  ),
                  SizedBox(height: AppSpacing.md),
                  AmitiaButton(
                    label: '保存',
                    isFullWidth: true,
                    onPressed: () async {
                      final title = titleController.text.trim();
                      final date = dateController.text.trim();
                      final time = timeController.text.trim();
                      final instantAtUtc = instantController.text.trim();
                      final endAtUtc = endInstantController.text.trim();
                      final startInstant = DateTime.tryParse(instantAtUtc);
                      final endInstant = DateTime.tryParse(endAtUtc);
                      final usesUtc = timeKind == 'instant' || timeKind == 'range';
                      final validInstant = !usesUtc || startInstant?.isUtc == true;
                      final validEnd = timeKind != 'range' ||
                          (endInstant?.isUtc == true &&
                              startInstant != null &&
                              endInstant!.isAfter(startInstant));
                      final validCivil = usesUtc || (_validAnchorDate(timeKind, date) && _validClock(time));
                      if (title.isEmpty || !validInstant || !validEnd || !validCivil) {
                        if (mounted) _showMessage('请检查名称和时间格式', error: true);
                        return;
                      }
                      final nextRRule = timeKind == 'recurring'
                          ? _replaceRRuleFrequency(existing?.rrule ?? '', frequency)
                          : '';
                      final payload = <String, dynamic>{
                        'characterId': scopeCharacterId,
                        'scopeType': scopeCharacterId.isEmpty ? 'user' : 'relationship',
                        'anchorType': anchorType,
                        'title': title,
                        'description': descriptionController.text.trim(),
                        'timeKind': timeKind,
                        'instantAtUtc': usesUtc ? instantAtUtc : '',
                        'endAtUtc': timeKind == 'range' ? endAtUtc : '',
                        'localDate': usesUtc ? '' : date,
                        'localTime': usesUtc ? '' : time,
                        'timezone': existing?.timezone.isNotEmpty == true ? existing!.timezone : _timezone,
                        'rrule': nextRRule,
                        'durationSeconds': existing?.durationSeconds ?? 0,
                        'preWindowSeconds': existing?.preWindowSeconds ?? 259200,
                        'postWindowSeconds': existing?.postWindowSeconds ?? 86400,
                        'importance': importance.round(),
                        'confidence': existing?.confidence ?? 100,
                        'sensitivityLevel': existing?.sensitivityLevel.isNotEmpty == true ? existing!.sensitivityLevel : 'internal',
                        'allowPromptMention': allowPromptMention,
                        'allowProactiveMention': allowProactiveMention,
                        'requiresConfirmation': existing?.requiresConfirmation ?? false,
                        'source': existing?.source.isNotEmpty == true
                            ? existing!.source
                            : 'manual',
                        'status': existing?.status.isNotEmpty == true
                            ? existing!.status
                            : 'active',
                      };
                      try {
                        final service = ref.read(temporalServiceProvider);
                        if (existing == null) {
                          await service.createAnchor(payload);
                        } else {
                          await service.updateAnchor(
                            existing.id,
                            payload,
                            characterId: scopeCharacterId,
                          );
                        }
                        if (!mounted || !sheetContext.mounted) return;
                        Navigator.pop(sheetContext);
                        widget.onRefresh();
                        _showMessage(existing == null ? '已添加锚点' : '已更新锚点');
                      } catch (error) {
                        if (mounted) _showMessage(_errorText(error), error: true);
                      }
                    },
                  ),
                ],
              ),
            ),
          );
        },
      ),
    );

    titleController.dispose();
    descriptionController.dispose();
    dateController.dispose();
    timeController.dispose();
    instantController.dispose();
    endInstantController.dispose();
  }

  Widget _sheetLabel(String text) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 4),
      child: Text(text, style: AppTypography.label(context)),
    );
  }

  Future<void> _confirmAnchor(TimeAnchor anchor) async {
    try {
      await ref.read(temporalServiceProvider).confirmAnchor(
            anchor.id,
            characterId: anchor.characterId,
          );
      if (!mounted) return;
      widget.onRefresh();
      _showMessage('候选锚点已确认');
    } catch (error) {
      if (mounted) _showMessage(_errorText(error), error: true);
    }
  }

  void _confirmDelete(TimeAnchor anchor) {
    showDialog<void>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
        title: Text('删除锚点', style: AppTypography.cardTitle(context)),
        content: Text(
          '确定要删除「${anchor.title.isEmpty ? '未命名锚点' : anchor.title}」吗？',
          style: AppTypography.body(context),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext),
            child: const Text('取消'),
          ),
          TextButton(
            onPressed: () async {
              Navigator.pop(dialogContext);
              try {
                await ref.read(temporalServiceProvider).deleteAnchor(
                      anchor.id,
                      characterId: anchor.characterId,
                    );
                if (!mounted) return;
                widget.onRefresh();
                _showMessage('已删除');
              } catch (error) {
                if (mounted) _showMessage(_errorText(error), error: true);
              }
            },
            child: Text('删除', style: TextStyle(color: context.error)),
          ),
        ],
      ),
    );
  }

  Widget _buildReunionSection({
    required List<CharacterDto> characters,
    required String characterId,
    required AsyncValue<List<Map<String, dynamic>>>? episodesAsync,
  }) {
    final selectedName = _characterName(characterId, characters);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
          child: Row(
            children: [
              Expanded(
                child: Text(
                  '重逢记录',
                  style: AppTypography.sectionTitle(context),
                ),
              ),
              TextButton.icon(
                onPressed: characters.isEmpty
                    ? null
                    : () => _selectReunionCharacter(characters),
                icon: const Icon(Icons.person_outline, size: 18),
                label: Text(selectedName.isEmpty ? '选择角色' : selectedName),
              ),
              IconButton(
                tooltip: '刷新',
                onPressed: characterId.isEmpty
                    ? null
                    : () => ref.invalidate(
                          _reunionEpisodesProvider(characterId),
                        ),
                icon: const Icon(Icons.refresh, size: 19),
              ),
            ],
          ),
        ),
        SizedBox(height: AppSpacing.sm),
        if (characterId.isEmpty)
          _buildEmpty('暂无可查看的角色')
        else if (episodesAsync == null)
          _buildEmpty('暂无重逢记录')
        else
          episodesAsync.when(
            loading: () => const Padding(
              padding: EdgeInsets.all(24),
              child: Center(child: CircularProgressIndicator()),
            ),
            error: (error, _) => Padding(
              padding: EdgeInsets.symmetric(
                horizontal: AppSpacing.pagePadding,
                vertical: AppSpacing.md,
              ),
              child: Text(
                '重逢记录加载失败：${_errorText(error)}',
                style: AppTypography.caption(context).copyWith(
                  color: context.error,
                ),
              ),
            ),
            data: (episodes) {
              if (episodes.isEmpty) return _buildEmpty('暂无重逢记录');
              return Column(
                children: episodes
                    .map(
                      (episode) => _buildReunionTile(
                        characterId,
                        episode,
                      ),
                    )
                    .toList(growable: false),
              );
            },
          ),
      ],
    );
  }

  Widget _buildReunionTile(
    String characterId,
    Map<String, dynamic> episode,
  ) {
    final kind = (episode['reunionKind'] ?? '').toString();
    final level = (episode['reunionLevel'] ?? '').toString();
    final status = (episode['status'] ?? '').toString();
    final detected = _displayDateTime((episode['detectedAtUtc'] ?? '').toString());
    return Container(
      margin: EdgeInsets.only(
        left: AppSpacing.pagePadding,
        right: AppSpacing.pagePadding,
        bottom: AppSpacing.sm,
      ),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: ListTile(
        leading: Container(
          width: 36,
          height: 36,
          decoration: BoxDecoration(
            color: context.accentSoft,
            borderRadius: AppRadius.brSmall,
          ),
          child: Icon(
            Icons.auto_awesome_outlined,
            size: 18,
            color: context.accentPrimary,
          ),
        ),
        title: Text(
          '${_reunionKindLabel(kind)} · ${_reunionLevelLabel(level)}',
          style: AppTypography.body(context),
        ),
        subtitle: Text(
          '${_reunionStatusLabel(status)}${detected.isEmpty ? '' : ' · $detected'}',
          style: AppTypography.caption(context),
        ),
        trailing: Icon(
          Icons.chevron_right,
          size: 20,
          color: context.textTertiary,
        ),
        onTap: () => _showReunionEpisodeDetail(
          characterId,
          (episode['id'] ?? '').toString(),
          fallback: episode,
        ),
      ),
    );
  }

  Future<void> _showReunionEpisodeDetail(
    String characterId,
    String episodeId, {
    required Map<String, dynamic> fallback,
  }) async {
    if (episodeId.isEmpty) return;
    Map<String, dynamic> detail = fallback;
    try {
      detail = await ref
              .read(temporalServiceProvider)
              .reunionEpisode(characterId, episodeId) ??
          fallback;
    } catch (error) {
      if (mounted) _showMessage(_errorText(error), error: true);
      return;
    }
    if (!mounted) return;
    await showModalBottomSheet<void>(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (sheetContext) => SafeArea(
        child: Padding(
          padding: EdgeInsets.all(AppSpacing.lg),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('重逢记录详情', style: AppTypography.sectionTitle(context)),
              SizedBox(height: AppSpacing.lg),
              _detailRow('类型', _reunionKindLabel((detail['reunionKind'] ?? '').toString())),
              _detailRow('等级', _reunionLevelLabel((detail['reunionLevel'] ?? '').toString())),
              _detailRow('状态', _reunionStatusLabel((detail['status'] ?? '').toString())),
              _detailRow('检测时间', _displayDateTime((detail['detectedAtUtc'] ?? '').toString())),
              _detailRow('关系间隔', _durationLabel(detail['relationshipGapSeconds'])),
              _detailRow('全局间隔', _durationLabel(detail['globalGapSeconds'])),
              _detailRow('预期间隔', _durationLabel(detail['expectedGapSeconds'])),
              _detailRow('连续性', _numberLabel(detail['continuityBefore'])),
              if ((detail['suppressionReason'] ?? '').toString().isNotEmpty)
                _detailRow('抑制原因', (detail['suppressionReason'] ?? '').toString()),
            ],
          ),
        ),
      ),
    );
  }

  Widget _detailRow(String label, String value) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 10),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 88,
            child: Text(label, style: AppTypography.caption(context)),
          ),
          Expanded(
            child: Text(
              value.isEmpty ? '—' : value,
              style: AppTypography.body(context),
            ),
          ),
        ],
      ),
    );
  }

  Future<void> _selectReunionCharacter(List<CharacterDto> characters) async {
    final selected = await showModalBottomSheet<String>(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (sheetContext) => SafeArea(
        child: ListView(
          shrinkWrap: true,
          children: [
            Padding(
              padding: EdgeInsets.all(AppSpacing.lg),
              child: Text('选择角色', style: AppTypography.sectionTitle(context)),
            ),
            ...characters.map(
              (character) => ListTile(
                title: Text(character.name),
                subtitle: character.description.isEmpty
                    ? null
                    : Text(
                        character.description,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                onTap: () => Navigator.pop(sheetContext, character.id),
              ),
            ),
          ],
        ),
      ),
    );
    if (selected != null && selected.isNotEmpty && mounted) {
      setState(() => _selectedReunionCharacterId = selected);
    }
  }

  String _resolveReunionCharacterId(List<CharacterDto> characters) {
    if (characters.isEmpty) return '';
    if (_selectedReunionCharacterId != null &&
        characters.any((item) => item.id == _selectedReunionCharacterId)) {
      return _selectedReunionCharacterId!;
    }
    for (final character in characters) {
      if (character.isActive == 1) return character.id;
    }
    return characters.first.id;
  }

  String _anchorScopeLabel(
    TimeAnchor anchor,
    List<CharacterDto> characters,
  ) {
    if (anchor.characterId.isEmpty) return '用户全局';
    final name = _characterName(anchor.characterId, characters);
    return name.isEmpty ? '角色锚点' : '角色 · $name';
  }

  String _characterName(String id, List<CharacterDto> characters) {
    for (final character in characters) {
      if (character.id == id) return character.name;
    }
    return '';
  }

  void _showMessage(String message, {bool error = false}) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(message),
        duration: Duration(seconds: error ? 3 : 1),
      ),
    );
  }

  static String _errorText(Object error) {
    return error.toString().replaceFirst('Exception: ', '');
  }

  static bool _looksLikeIanaTimezone(String value) {
    final trimmed = value.trim();
    return trimmed == 'UTC' ||
        (trimmed.contains('/') && !trimmed.contains(' ') && !trimmed.contains('('));
  }

  static bool _validClock(String value) {
    final match = RegExp(r'^(\d{2}):(\d{2})$').firstMatch(value.trim());
    if (match == null) return false;
    final hour = int.tryParse(match.group(1)!);
    final minute = int.tryParse(match.group(2)!);
    return hour != null &&
        minute != null &&
        hour >= 0 &&
        hour <= 23 &&
        minute >= 0 &&
        minute <= 59;
  }

  static bool _validAnchorDate(String timeKind, String value) {
    final trimmed = value.trim();
    if (timeKind == 'annual_date') {
      final match = RegExp(r'^(\d{2})-(\d{2})$').firstMatch(trimmed);
      if (match == null) return false;
      final month = int.tryParse(match.group(1)!);
      final day = int.tryParse(match.group(2)!);
      return month != null && day != null && month >= 1 && month <= 12 && day >= 1 && day <= 31;
    }
    final match = RegExp(r'^(\d{4})-(\d{2})-(\d{2})$').firstMatch(trimmed);
    if (match == null) return false;
    final year = int.tryParse(match.group(1)!);
    final month = int.tryParse(match.group(2)!);
    final day = int.tryParse(match.group(3)!);
    return year != null &&
        month != null &&
        day != null &&
        year >= 1970 &&
        month >= 1 &&
        month <= 12 &&
        day >= 1 &&
        day <= 31;
  }

  static String _replaceRRuleFrequency(String rrule, String frequency) {
    final trimmed = rrule.trim();
    if (trimmed.isEmpty) return 'FREQ=$frequency';
    final prefix = trimmed.toUpperCase().startsWith('RRULE:') ? 'RRULE:' : '';
    final body = prefix.isEmpty ? trimmed : trimmed.substring(6);
    final parts = body.split(';').where((part) => part.trim().isNotEmpty).toList();
    var replaced = false;
    for (var i = 0; i < parts.length; i++) {
      if (parts[i].toUpperCase().startsWith('FREQ=')) {
        parts[i] = 'FREQ=$frequency';
        replaced = true;
        break;
      }
    }
    if (!replaced) parts.insert(0, 'FREQ=$frequency');
    return '$prefix${parts.join(';')}';
  }

  static String _frequencyFromRRule(String rrule) {
    final match = RegExp(r'(?:^|;)FREQ=([A-Z]+)').firstMatch(
      rrule.toUpperCase().replaceFirst('RRULE:', ''),
    );
    final value = match?.group(1) ?? 'DAILY';
    return {'DAILY', 'WEEKLY', 'MONTHLY', 'YEARLY'}.contains(value)
        ? value
        : 'DAILY';
  }

  static String _todayDate() {
    final now = DateTime.now();
    return '${now.year.toString().padLeft(4, '0')}-${now.month.toString().padLeft(2, '0')}-${now.day.toString().padLeft(2, '0')}';
  }

  static String _todayMonthDay() {
    final now = DateTime.now();
    return '${now.month.toString().padLeft(2, '0')}-${now.day.toString().padLeft(2, '0')}';
  }

  static String _displayDateTime(String value) {
    if (value.trim().isEmpty) return '';
    final parsed = DateTime.tryParse(value);
    if (parsed == null) return value;
    final local = parsed.toLocal();
    return '${local.year.toString().padLeft(4, '0')}-${local.month.toString().padLeft(2, '0')}-${local.day.toString().padLeft(2, '0')} ${local.hour.toString().padLeft(2, '0')}:${local.minute.toString().padLeft(2, '0')}';
  }

  static String _durationLabel(dynamic value) {
    final seconds = value is num ? value.toDouble() : double.tryParse('$value') ?? 0;
    if (seconds <= 0) return '0';
    final days = seconds / 86400;
    if (days >= 1) return '${days.toStringAsFixed(days >= 10 ? 0 : 1)} 天';
    final hours = seconds / 3600;
    if (hours >= 1) return '${hours.toStringAsFixed(hours >= 10 ? 0 : 1)} 小时';
    return '${(seconds / 60).round()} 分钟';
  }

  static String _numberLabel(dynamic value) {
    if (value is num) return value.toStringAsFixed(2);
    return (value ?? '').toString();
  }

  static String _reunionKindLabel(String value) {
    switch (value) {
      case 'global_return':
        return '全局回归';
      case 'relationship_reconnect':
        return '关系重连';
      case 'reply_to_recent_proactive':
        return '回应主动消息';
      default:
        return value.isEmpty ? '重逢' : value;
    }
  }

  static String _reunionLevelLabel(String value) {
    switch (value) {
      case 'none':
        return '普通';
      case 'noticeable':
        return '明显';
      case 'long':
        return '较久';
      case 'extended':
        return '长时间';
      case 'dormant':
        return '久别';
      default:
        return value.isEmpty ? '未知' : value;
    }
  }

  static String _reunionStatusLabel(String value) {
    switch (value) {
      case 'pending':
        return '待处理';
      case 'claimed':
        return '处理中';
      case 'handled':
        return '已处理';
      case 'suppressed':
        return '已抑制';
      case 'expired':
        return '已过期';
      default:
        return value.isEmpty ? '未知状态' : value;
    }
  }
}

class _SectionLabel extends StatelessWidget {
  final String text;

  const _SectionLabel({required this.text});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.fromLTRB(
        AppSpacing.pagePadding,
        AppSpacing.sm,
        AppSpacing.pagePadding,
        AppSpacing.sm,
      ),
      child: Text(text, style: AppTypography.caption(context)),
    );
  }
}

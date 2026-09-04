import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class CharacterLifeRulesPage extends ConsumerStatefulWidget {
  final String characterId;
  const CharacterLifeRulesPage({super.key, required this.characterId});

  @override
  ConsumerState<CharacterLifeRulesPage> createState() => _CharacterLifeRulesPageState();
}

class _CharacterLifeRulesPageState extends ConsumerState<CharacterLifeRulesPage> {
  static const Map<String, int> _personalityDefaults = {
    'familiarity': 78,
    'formality': 22,
    'customerServiceAvoidance': 92,
    'directness': 75,
    'verbosity': 32,
    'structureLevel': 40,
    'shortSentence': 85,
    'toneWords': 45,
    'warmth': 58,
    'comfortLevel': 55,
    'preachingAvoidance': 88,
    'companionship': 55,
    'boundary': 85,
    'dependencyAvoidance': 85,
    'execution': 75,
    'explanationDepth': 55,
    'judgment': 75,
    'clarification': 35,
    'rationality': 50,
    'humor': 40,
    'initiative': 50,
    'teasing': 30,
    'patience': 60,
    'intimacyExpression': 25,
    'flirtiness': 0,
    'romanticTone': 0,
    'suggestivenessAvoidance': 100,
    'intimacyBoundary': 90,
  };

  static const Map<String, String> _personalityLabels = {
    'familiarity': '熟悉感',
    'formality': '正式度',
    'customerServiceAvoidance': '客服腔抑制',
    'directness': '直接程度',
    'verbosity': '回复长度',
    'structureLevel': '结构化程度',
    'shortSentence': '短句程度',
    'toneWords': '语气词使用',
    'warmth': '亲和度',
    'comfortLevel': '安抚强度',
    'preachingAvoidance': '说教抑制',
    'companionship': '陪伴感',
    'boundary': '边界感',
    'dependencyAvoidance': '依赖引导抑制',
    'execution': '执行力',
    'explanationDepth': '解释深度',
    'judgment': '判断力',
    'clarification': '追问倾向',
    'rationality': '理性程度',
    'humor': '幽默感',
    'initiative': '主动性',
    'teasing': '吐槽程度',
    'patience': '耐心',
    'intimacyExpression': '亲近表达',
    'flirtiness': '暧昧倾向',
    'romanticTone': '浪漫语气',
    'suggestivenessAvoidance': '暗示性内容抑制',
    'intimacyBoundary': '亲密边界',
  };

  static const Map<String, String> _lifestyleLabels = {
    'punctualityTendency': '守时倾向',
    'earlyPrepareTendency': '提前准备',
    'selfDisciplineTendency': '自律程度',
    'sleepinessTendency': '嗜睡程度',
    'randomnessTendency': '随机性',
    'activityEnergy': '活动精力',
    'socialEnergy': '社交精力',
    'careTendency': '关心倾向',
    'dailyShareTendency': '日常分享倾向',
  };

  final _promptController = TextEditingController();
  final _bedController = TextEditingController();
  final _wakeController = TextEditingController();
  final _selfReferenceController = TextEditingController();
  final _addressingController = TextEditingController();
  final _workStartController = TextEditingController();
  final _workEndController = TextEditingController();
  final _workDaysController = TextEditingController();
  final _lunchStartController = TextEditingController();
  final _lunchEndController = TextEditingController();
  final _commuteMinController = TextEditingController();
  final _commuteMaxController = TextEditingController();
  final _prepareMinController = TextEditingController();
  final _prepareMaxController = TextEditingController();
  final _overtimeMinController = TextEditingController();
  final _overtimeMaxController = TextEditingController();

  bool _loading = true;
  bool _saving = false;
  String? _error;
  bool _timeAwareness = true;
  bool _sleepEnabled = true;
  bool _sleepReplyEnabled = false;
  String _sleepReplyMode = 'NO_REPLY';
  String _gender = 'UNSPECIFIED';
  String _pronoun = 'TA';
  int _genderExpression = 30;
  String _lifeIdentity = 'CUSTOM';
  Map<String, dynamic> _personalityConfig = <String, dynamic>{};
  Map<String, dynamic> _lifestyle = <String, dynamic>{};
  Map<String, dynamic> _workProfile = <String, dynamic>{};
  Map<String, dynamic> _relationshipTime = <String, dynamic>{};
  bool _relationshipTimeAvailable = true;
  List<Map<String, dynamic>> _fixedEvents = const [];
  List<Map<String, dynamic>> _specialEvents = const [];

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void dispose() {
    for (final controller in [
      _promptController,
      _bedController,
      _wakeController,
      _selfReferenceController,
      _addressingController,
      _workStartController,
      _workEndController,
      _workDaysController,
      _lunchStartController,
      _lunchEndController,
      _commuteMinController,
      _commuteMaxController,
      _prepareMinController,
      _prepareMaxController,
      _overtimeMinController,
      _overtimeMaxController,
    ]) {
      controller.dispose();
    }
    super.dispose();
  }

  Map<String, dynamic> _decodeConfig(dynamic raw) {
    if (raw is Map) return Map<String, dynamic>.from(raw);
    if (raw is String && raw.trim().isNotEmpty) {
      try {
        final decoded = jsonDecode(raw);
        if (decoded is Map) return Map<String, dynamic>.from(decoded);
      } catch (_) {}
    }
    return <String, dynamic>{};
  }

  bool _bool(dynamic value, {bool fallback = false}) {
    if (value is bool) return value;
    if (value is num) return value != 0;
    return fallback;
  }

  int _int(dynamic value, int fallback) => value is num ? value.round() : int.tryParse('$value') ?? fallback;

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final characterService = ref.read(characterDetailServiceProvider);
      final companion = ref.read(companionServiceProvider);
      final temporal = ref.read(temporalServiceProvider);
      Map<String, dynamic>? relationshipTime;
      var relationshipTimeAvailable = true;
      try {
        relationshipTime = await temporal.relationshipTimeSettings(widget.characterId);
      } catch (_) {
        relationshipTimeAvailable = false;
      }
      final values = await Future.wait<dynamic>([
        characterService.character(widget.characterId),
        characterService.roleProfile(characterId: widget.characterId),
        companion.fixedEvents(characterId: widget.characterId),
        companion.specialEvents(characterId: widget.characterId),
        companion.sleepSetting(characterId: widget.characterId),
        companion.lifestyleTendency(characterId: widget.characterId),
        companion.workProfile(characterId: widget.characterId),
        temporal.characterProfile(widget.characterId),
      ]);
      final character = values[0] as Map<String, dynamic>? ?? <String, dynamic>{};
      final role = values[1] as Map<String, dynamic>? ?? <String, dynamic>{};
      final sleep = values[4] as Map<String, dynamic>? ?? <String, dynamic>{};
      final temporalProfile = values[7] as Map<String, dynamic>? ?? <String, dynamic>{};
      final personality = <String, dynamic>{..._personalityDefaults, ..._decodeConfig(character['personalityConfig'])};
      if (!mounted) return;
      setState(() {
        _promptController.text = (character['basePrompt'] ?? '').toString();
        _personalityConfig = personality;
        _fixedEvents = (values[2] as List<Map<String, dynamic>>?) ?? const [];
        _specialEvents = (values[3] as List<Map<String, dynamic>>?) ?? const [];
        _bedController.text = (sleep['bedTime'] ?? '23:00').toString();
        _wakeController.text = (sleep['wakeTime'] ?? '07:00').toString();
        _sleepEnabled = _bool(sleep['enabled'], fallback: true);
        _sleepReplyEnabled = _bool(sleep['sleepReplyEnabled']);
        _sleepReplyMode = (sleep['sleepReplyMode'] ?? 'NO_REPLY').toString();
        _timeAwareness = _bool(temporalProfile['enabled'], fallback: true);
        _gender = (role['gender'] ?? character['gender'] ?? 'UNSPECIFIED').toString();
        _pronoun = (role['pronoun'] ?? character['pronoun'] ?? 'TA').toString();
        _genderExpression = _int(role['genderExpression'] ?? character['genderExpression'], 30).clamp(0, 100);
        _selfReferenceController.text = (role['selfReference'] ?? character['selfReference'] ?? '我').toString();
        _addressingController.text = (role['userAddressingStyle'] ?? character['userAddressingStyle'] ?? '').toString();
        _lifeIdentity = (role['lifeIdentity'] ?? character['lifeIdentity'] ?? 'CUSTOM').toString();
        _lifestyle = Map<String, dynamic>.from((values[5] as Map<String, dynamic>?) ?? const {});
        _workProfile = Map<String, dynamic>.from((values[6] as Map<String, dynamic>?) ?? const {});
        _relationshipTimeAvailable = relationshipTimeAvailable;
        _relationshipTime = Map<String, dynamic>.from(relationshipTime ?? const {});
        _workStartController.text = (_workProfile['workStartTime'] ?? '09:00').toString();
        _workEndController.text = (_workProfile['workEndTime'] ?? '18:00').toString();
        _workDaysController.text = (_workProfile['workDays'] ?? 'MON,TUE,WED,THU,FRI').toString();
        _lunchStartController.text = (_workProfile['lunchBreakStartTime'] ?? '12:00').toString();
        _lunchEndController.text = (_workProfile['lunchBreakEndTime'] ?? '13:30').toString();
        _commuteMinController.text = _int(_workProfile['commuteMinMinutes'], 15).toString();
        _commuteMaxController.text = _int(_workProfile['commuteMaxMinutes'], 45).toString();
        _prepareMinController.text = _int(_workProfile['prepareMinMinutes'], 20).toString();
        _prepareMaxController.text = _int(_workProfile['prepareMaxMinutes'], 60).toString();
        _overtimeMinController.text = _int(_workProfile['overtimeMinMinutes'], 30).toString();
        _overtimeMaxController.text = _int(_workProfile['overtimeMaxMinutes'], 180).toString();
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _error = e.toString();
        _loading = false;
      });
    }
  }

  Future<void> _save({bool resetLifestyle = false}) async {
    if (_saving) return;
    setState(() => _saving = true);
    try {
      final characterService = ref.read(characterDetailServiceProvider);
      final companion = ref.read(companionServiceProvider);
      final temporal = ref.read(temporalServiceProvider);
      final currentTemporal = await temporal.characterProfile(widget.characterId) ?? <String, dynamic>{};
      final updatedTemporal = Map<String, dynamic>.from(currentTemporal)
        ..['enabled'] = _timeAwareness
        ..['source'] = 'explicit'
        ..['confidence'] = 100;
      final work = Map<String, dynamic>.from(_workProfile)
        ..['workStartTime'] = _workStartController.text.trim()
        ..['workEndTime'] = _workEndController.text.trim()
        ..['workDays'] = _workDaysController.text.trim()
        ..['lunchBreakStartTime'] = _lunchStartController.text.trim()
        ..['lunchBreakEndTime'] = _lunchEndController.text.trim()
        ..['commuteMinMinutes'] = _int(_commuteMinController.text, 15)
        ..['commuteMaxMinutes'] = _int(_commuteMaxController.text, 45)
        ..['prepareMinMinutes'] = _int(_prepareMinController.text, 20)
        ..['prepareMaxMinutes'] = _int(_prepareMaxController.text, 60)
        ..['overtimeMinMinutes'] = _int(_overtimeMinController.text, 30)
        ..['overtimeMaxMinutes'] = _int(_overtimeMaxController.text, 180);
      var lifestyle = Map<String, dynamic>.from(_lifestyle)..['manuallyConfigured'] = true;
      final saveOperations = <Future<dynamic>>[
        characterService.updateCharacter(widget.characterId, {
          'basePrompt': _promptController.text.trim(),
          'personalityConfig': _personalityConfig,
          'lifeIdentity': _lifeIdentity,
        }),
        characterService.updateRoleProfile({
          'gender': _gender,
          'pronoun': _pronoun,
          'selfReference': _selfReferenceController.text.trim().isEmpty ? '我' : _selfReferenceController.text.trim(),
          'userAddressingStyle': _addressingController.text.trim().isEmpty ? null : _addressingController.text.trim(),
          'genderExpression': _genderExpression,
          'lifeIdentity': _lifeIdentity,
        }, characterId: widget.characterId),
        companion.updateSleepSetting({
          'enabled': _sleepEnabled,
          'bedTime': _bedController.text.trim(),
          'wakeTime': _wakeController.text.trim(),
          'sleepReplyEnabled': _sleepReplyEnabled,
          'sleepReplyMode': _sleepReplyMode,
        }, characterId: widget.characterId),
        if (resetLifestyle)
          companion.resetLifestyleTendency(characterId: widget.characterId).then((value) {
            if (value != null) lifestyle = Map<String, dynamic>.from(value);
          })
        else
          companion.updateLifestyleTendency(lifestyle, characterId: widget.characterId),
        companion.updateWorkProfile(work, characterId: widget.characterId),
        temporal.updateCharacterProfile(widget.characterId, updatedTemporal),
      ];
      if (_relationshipTimeAvailable) {
        saveOperations.add(temporal.updateRelationshipTimeSettings(widget.characterId, _relationshipTime));
      }
      await Future.wait(saveOperations);
      if (!mounted) return;
      setState(() {
        _workProfile = work;
        _lifestyle = lifestyle;
      });
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(resetLifestyle ? '角色生活规则已恢复默认' : '角色生活规则已保存')));
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('保存失败：$e')));
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  Future<void> _resetToDefaults() async {
    setState(() {
      _promptController.clear();
      _personalityConfig = Map<String, dynamic>.from(_personalityDefaults);
      _timeAwareness = true;
      _sleepEnabled = true;
      _sleepReplyEnabled = false;
      _sleepReplyMode = 'NO_REPLY';
      _bedController.text = '23:00';
      _wakeController.text = '07:00';
      _gender = 'UNSPECIFIED';
      _pronoun = 'TA';
      _genderExpression = 30;
      _selfReferenceController.text = '我';
      _addressingController.clear();
      _lifeIdentity = 'CUSTOM';
      _lifestyle = {for (final key in _lifestyleLabels.keys) key: 50, 'manuallyConfigured': false};
      _workProfile = <String, dynamic>{
        'enabled': false,
        'replyMode': 'SHORT_REPLY',
        'allowOvertime': false,
        'overtimeProbability': 10,
        'overtimeReplyMode': 'SHORT_REPLY',
        'delayedReplyEnabled': false,
        'commuteHomeShareEnabled': true,
        'commuteHomeShareProbability': 60,
      };
      _workStartController.text = '09:00';
      _workEndController.text = '18:00';
      _workDaysController.text = 'MON,TUE,WED,THU,FRI';
      _lunchStartController.text = '12:00';
      _lunchEndController.text = '13:30';
      _commuteMinController.text = '15';
      _commuteMaxController.text = '45';
      _prepareMinController.text = '20';
      _prepareMaxController.text = '60';
      _overtimeMinController.text = '30';
      _overtimeMaxController.text = '180';
      _relationshipTime = {
        'enabled': true,
        'reunionEnabled': true,
        'sensitivity': 'balanced',
        'allowMemoryRecall': true,
        'allowRelationshipAge': true,
        'allowReunionMention': true,
        'allowProactiveReference': true,
        'maxMentionSentences': 1,
      };
    });
    await _save(resetLifestyle: true);
  }

  Future<void> _editEvent({Map<String, dynamic>? event, required bool special}) async {
    final title = TextEditingController(text: (event?['title'] ?? '').toString());
    final start = TextEditingController(text: (event?[special ? 'startDate' : 'startTime'] ?? (special ? '' : '09:00')).toString());
    final end = TextEditingController(text: (event?[special ? 'endDate' : 'endTime'] ?? (special ? '' : '10:00')).toString());
    final result = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(event == null ? '新增${special ? '特殊事件' : '固定日程'}' : '编辑${special ? '特殊事件' : '固定日程'}'),
        content: Column(mainAxisSize: MainAxisSize.min, children: [
          TextField(controller: title, decoration: const InputDecoration(labelText: '名称')),
          TextField(controller: start, decoration: InputDecoration(labelText: special ? '开始日期 YYYY-MM-DD' : '开始时间 HH:mm')),
          TextField(controller: end, decoration: InputDecoration(labelText: special ? '结束日期 YYYY-MM-DD' : '结束时间 HH:mm')),
        ]),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context), child: const Text('取消')),
          FilledButton(
            onPressed: () => Navigator.pop(context, special
                ? {'title': title.text.trim(), 'startDate': start.text.trim(), 'endDate': end.text.trim(), 'enabled': _bool(event?['enabled'], fallback: true)}
                : {'title': title.text.trim(), 'startTime': start.text.trim(), 'endTime': end.text.trim(), 'enabled': _bool(event?['enabled'], fallback: true)}),
            child: const Text('保存'),
          ),
        ],
      ),
    );
    title.dispose();
    start.dispose();
    end.dispose();
    if (result == null || (result['title'] as String?)?.isEmpty == true) return;
    final companion = ref.read(companionServiceProvider);
    if (special) {
      if (event == null) {
        await companion.createSpecialEvent(result, characterId: widget.characterId);
      } else {
        await companion.updateSpecialEvent('${event['id']}', result, characterId: widget.characterId);
      }
    } else {
      if (event == null) {
        await companion.createFixedEvent(result, characterId: widget.characterId);
      } else {
        await companion.updateFixedEvent('${event['id']}', result, characterId: widget.characterId);
      }
    }
    await _load();
  }

  Future<void> _toggleFixedEvent(Map<String, dynamic> event) async {
    final id = (event['id'] ?? '').toString();
    if (id.isEmpty) return;
    try {
      await ref.read(companionServiceProvider).toggleFixedEventEnabled(id);
      await _load();
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('固定日程启停失败：$e')));
      }
    }
  }

  Future<void> _deleteEvent(Map<String, dynamic> event, {required bool special}) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('确认删除'),
        content: Text('确定删除“${event['title'] ?? '未命名'}”？'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context, false), child: const Text('取消')),
          FilledButton(onPressed: () => Navigator.pop(context, true), child: const Text('删除')),
        ],
      ),
    );
    if (confirmed != true) return;
    final companion = ref.read(companionServiceProvider);
    if (special) {
      await companion.deleteSpecialEvent('${event['id']}', characterId: widget.characterId);
    } else {
      await companion.deleteFixedEvent('${event['id']}', characterId: widget.characterId);
    }
    await _load();
  }

  Widget _relationshipMentionSentenceControl() {
    final raw = _relationshipTime['maxMentionSentences'];
    final parsed = raw is num ? raw.toInt() : int.tryParse(raw?.toString() ?? '') ?? 1;
    final value = parsed.clamp(0, 2).toInt();
    return ListTile(
      contentPadding: EdgeInsets.zero,
      title: const Text('最多重逢表达句数'),
      subtitle: const Text('设为 0 时保留重逢识别，但不生成重逢表达。'),
      trailing: DropdownButton<int>(
        value: value,
        items: const [
          DropdownMenuItem(value: 0, child: Text('0')),
          DropdownMenuItem(value: 1, child: Text('1')),
          DropdownMenuItem(value: 2, child: Text('2')),
        ],
        onChanged: (next) {
          if (next != null) setState(() => _relationshipTime['maxMentionSentences'] = next);
        },
      ),
    );
  }

  Widget _section(String title, Widget child) => Padding(
        padding: EdgeInsets.only(bottom: AppSpacing.sectionGap),
        child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
          AmitiaSectionHeader(title: title),
          SizedBox(height: AppSpacing.sm),
          AmitiaCard(child: child),
        ]),
      );

  Widget _numberField(TextEditingController controller, String label) => TextField(
        controller: controller,
        keyboardType: TextInputType.number,
        decoration: InputDecoration(labelText: label),
      );

  Widget _slider(String label, int value, ValueChanged<int> onChanged) => Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(children: [Expanded(child: Text(label)), Text('$value')]),
          Slider(value: value.clamp(0, 100).toDouble(), min: 0, max: 100, divisions: 100, onChanged: (v) => onChanged(v.round())),
        ],
      );

  Widget _eventsSection(String title, List<Map<String, dynamic>> events, {required bool special}) => _section(
        '$title (${events.length})',
        Column(
          children: [
            Align(alignment: Alignment.centerRight, child: OutlinedButton.icon(onPressed: () => _editEvent(special: special), icon: const Icon(Icons.add), label: const Text('新增'))),
            if (events.isEmpty)
              Padding(padding: const EdgeInsets.all(12), child: Text('当前角色暂无$title', style: AppTypography.caption(context)))
            else
              ...events.map((event) => ListTile(
                    contentPadding: EdgeInsets.zero,
                    leading: Icon(special ? Icons.event_note_outlined : Icons.schedule_outlined),
                    title: Text((event['title'] ?? '未命名').toString()),
                    subtitle: Text(special
                        ? '${event['startDate'] ?? '—'} - ${event['endDate'] ?? '—'}'
                        : '${event['startTime'] ?? '—'} - ${event['endTime'] ?? '—'}'),
                    trailing: Wrap(
                      spacing: 4,
                      crossAxisAlignment: WrapCrossAlignment.center,
                      children: [
                        if (!special)
                          Switch(
                            value: _bool(event['enabled'], fallback: true),
                            onChanged: (_) => _toggleFixedEvent(event),
                          ),
                        IconButton(icon: const Icon(Icons.edit_outlined), tooltip: '编辑', onPressed: () => _editEvent(event: event, special: special)),
                        IconButton(icon: const Icon(Icons.delete_outline), tooltip: '删除', onPressed: () => _deleteEvent(event, special: special)),
                      ],
                    ),
                  )),
          ],
        ),
      );

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '生活规则',
        showBackButton: true,
        fallbackRoute: AppRoutes.characters,
        actions: [
          AmitiaIconButton(icon: Icons.refresh, tooltip: '刷新', onPressed: _load),
          AmitiaIconButton(icon: Icons.restart_alt, tooltip: '恢复默认并保存', onPressed: _resetToDefaults),
        ],
      ),
      body: SafeArea(
        top: false,
        child: _loading
            ? const Center(child: CircularProgressIndicator())
            : _error != null
                ? Center(child: Text('加载失败：$_error'))
                : ListView(
                    padding: EdgeInsets.all(AppSpacing.pagePadding),
                    children: [
                      _section('角色 Prompt', AmitiaTextField(controller: _promptController, maxLines: 7, hintText: '角色基础 Prompt')),
                      _section(
                        '角色身份',
                        Column(children: [
                          DropdownButtonFormField<String>(
                            value: const ['UNSPECIFIED', 'MALE', 'FEMALE', 'NON_BINARY', 'CUSTOM'].contains(_gender) ? _gender : 'CUSTOM',
                            decoration: const InputDecoration(labelText: '性别'),
                            items: const [
                              DropdownMenuItem(value: 'UNSPECIFIED', child: Text('不强调性别')),
                              DropdownMenuItem(value: 'MALE', child: Text('男生')),
                              DropdownMenuItem(value: 'FEMALE', child: Text('女生')),
                              DropdownMenuItem(value: 'NON_BINARY', child: Text('非二元')),
                              DropdownMenuItem(value: 'CUSTOM', child: Text('自定义')),
                            ],
                            onChanged: (value) => setState(() {
                              _gender = value ?? 'UNSPECIFIED';
                              if (_gender == 'MALE') _pronoun = '他';
                              if (_gender == 'FEMALE') _pronoun = '她';
                              if (_gender == 'NON_BINARY' || _gender == 'UNSPECIFIED') _pronoun = 'TA';
                            }),
                          ),
                          DropdownButtonFormField<String>(
                            value: const ['TA', '他', '她'].contains(_pronoun) ? _pronoun : 'TA',
                            decoration: const InputDecoration(labelText: '代词'),
                            items: const [DropdownMenuItem(value: 'TA', child: Text('TA')), DropdownMenuItem(value: '他', child: Text('他')), DropdownMenuItem(value: '她', child: Text('她'))],
                            onChanged: (value) => setState(() => _pronoun = value ?? 'TA'),
                          ),
                          TextField(controller: _selfReferenceController, decoration: const InputDecoration(labelText: '自称')),
                          TextField(controller: _addressingController, decoration: const InputDecoration(labelText: '用户称呼风格')),
                          _slider('性别表达强度', _genderExpression, (v) => setState(() => _genderExpression = v)),
                          DropdownButtonFormField<String>(
                            value: const ['SCHOOL', 'WORK', 'UNEMPLOYED', 'HOME', 'CUSTOM'].contains(_lifeIdentity) ? _lifeIdentity : 'CUSTOM',
                            decoration: const InputDecoration(labelText: '生活场景'),
                            items: const [
                              DropdownMenuItem(value: 'SCHOOL', child: Text('上学')),
                              DropdownMenuItem(value: 'WORK', child: Text('工作')),
                              DropdownMenuItem(value: 'UNEMPLOYED', child: Text('待业')),
                              DropdownMenuItem(value: 'HOME', child: Text('居家')),
                              DropdownMenuItem(value: 'CUSTOM', child: Text('自定义')),
                            ],
                            onChanged: (value) => setState(() => _lifeIdentity = value ?? 'CUSTOM'),
                          ),
                        ]),
                      ),
                      _section(
                        '性格参数',
                        ExpansionTile(
                          tilePadding: EdgeInsets.zero,
                          title: Text('完整 ${_personalityDefaults.length} 项性格配置'),
                          subtitle: const Text('与桌面端使用同一 personalityConfig'),
                          children: _personalityDefaults.keys
                              .map((key) => _slider(_personalityLabels[key] ?? key, _int(_personalityConfig[key], _personalityDefaults[key]!), (value) => setState(() => _personalityConfig[key] = value)))
                              .toList(),
                        ),
                      ),
                      _section(
                        '生活倾向',
                        ExpansionTile(
                          tilePadding: EdgeInsets.zero,
                          title: const Text('生活习惯参数'),
                          children: _lifestyleLabels.keys
                              .map((key) => _slider(_lifestyleLabels[key]!, _int(_lifestyle[key], 50), (value) => setState(() => _lifestyle[key] = value)))
                              .toList(),
                        ),
                      ),
                      _section(
                        '时间与睡眠',
                        Column(children: [
                          SwitchListTile(contentPadding: EdgeInsets.zero, title: const Text('时间感知'), subtitle: const Text('使用当前角色 Temporal Profile'), value: _timeAwareness, onChanged: (v) => setState(() => _timeAwareness = v)),
                          SwitchListTile(contentPadding: EdgeInsets.zero, title: const Text('睡眠规则'), value: _sleepEnabled, onChanged: (v) => setState(() => _sleepEnabled = v)),
                          Row(children: [Expanded(child: TextField(controller: _bedController, decoration: const InputDecoration(labelText: '入睡时间'))), const SizedBox(width: 12), Expanded(child: TextField(controller: _wakeController, decoration: const InputDecoration(labelText: '起床时间')))]),
                          SwitchListTile(
                            contentPadding: EdgeInsets.zero,
                            title: const Text('睡眠时回复'),
                            subtitle: const Text('与桌面端使用同一 SleepSetting'),
                            value: _sleepReplyEnabled,
                            onChanged: (v) => setState(() {
                              _sleepReplyEnabled = v;
                              if (!v && _sleepReplyMode == 'SHORT_SLEEPY_REPLY') {
                                _sleepReplyMode = 'NO_REPLY';
                              }
                            }),
                          ),
                          DropdownButtonFormField<String>(
                            value: (_sleepReplyEnabled
                                    ? const ['NO_REPLY', 'SYSTEM_NOTICE', 'SHORT_SLEEPY_REPLY']
                                    : const ['NO_REPLY', 'SYSTEM_NOTICE'])
                                .contains(_sleepReplyMode)
                                ? _sleepReplyMode
                                : 'NO_REPLY',
                            decoration: InputDecoration(
                              labelText: _sleepReplyEnabled ? '睡眠回复模式' : '关闭时行为',
                            ),
                            items: [
                              const DropdownMenuItem(value: 'NO_REPLY', child: Text('不回复')),
                              const DropdownMenuItem(value: 'SYSTEM_NOTICE', child: Text('系统提示正在睡觉')),
                              if (_sleepReplyEnabled)
                                const DropdownMenuItem(value: 'SHORT_SLEEPY_REPLY', child: Text('简短困倦回复')),
                            ],
                            onChanged: (value) => setState(() => _sleepReplyMode = value ?? 'NO_REPLY'),
                          ),
                        ]),
                      ),
                      _section(
                        '关系时间',
                        !_relationshipTimeAvailable
                            ? const Text('当前后端未启用 Relationship Time 功能；其余角色设置仍可正常使用。')
                            : Column(children: [
                          SwitchListTile(contentPadding: EdgeInsets.zero, title: const Text('启用关系时间'), value: _bool(_relationshipTime['enabled'], fallback: true), onChanged: (v) => setState(() => _relationshipTime['enabled'] = v)),
                          SwitchListTile(contentPadding: EdgeInsets.zero, title: const Text('重逢感知'), value: _bool(_relationshipTime['reunionEnabled'], fallback: true), onChanged: (v) => setState(() => _relationshipTime['reunionEnabled'] = v)),
                          SwitchListTile(contentPadding: EdgeInsets.zero, title: const Text('允许回忆记忆'), value: _bool(_relationshipTime['allowMemoryRecall'], fallback: true), onChanged: (v) => setState(() => _relationshipTime['allowMemoryRecall'] = v)),
                          SwitchListTile(contentPadding: EdgeInsets.zero, title: const Text('允许提及关系时长'), value: _bool(_relationshipTime['allowRelationshipAge'], fallback: true), onChanged: (v) => setState(() => _relationshipTime['allowRelationshipAge'] = v)),
                          SwitchListTile(contentPadding: EdgeInsets.zero, title: const Text('允许提及重逢'), value: _bool(_relationshipTime['allowReunionMention'], fallback: true), onChanged: (v) => setState(() => _relationshipTime['allowReunionMention'] = v)),
                          SwitchListTile(contentPadding: EdgeInsets.zero, title: const Text('允许主动引用关系时间'), value: _bool(_relationshipTime['allowProactiveReference'], fallback: true), onChanged: (v) => setState(() => _relationshipTime['allowProactiveReference'] = v)),
                          _relationshipMentionSentenceControl(),
                          DropdownButtonFormField<String>(
                            value: const ['conservative', 'balanced', 'expressive'].contains(_relationshipTime['sensitivity']) ? _relationshipTime['sensitivity'] as String : 'balanced',
                            decoration: const InputDecoration(labelText: '敏感度'),
                            items: const [DropdownMenuItem(value: 'conservative', child: Text('保守')), DropdownMenuItem(value: 'balanced', child: Text('平衡')), DropdownMenuItem(value: 'expressive', child: Text('表达丰富'))],
                            onChanged: (v) => setState(() => _relationshipTime['sensitivity'] = v ?? 'balanced'),
                          ),
                        ]),
                      ),
                      _section(
                        '工作规则',
                        Column(children: [
                          SwitchListTile(contentPadding: EdgeInsets.zero, title: const Text('启用工作状态'), value: _bool(_workProfile['enabled']), onChanged: (v) => setState(() => _workProfile['enabled'] = v)),
                          TextField(controller: _workDaysController, decoration: const InputDecoration(labelText: '工作日', hintText: 'MON,TUE,WED,THU,FRI')),
                          Row(children: [Expanded(child: TextField(controller: _workStartController, decoration: const InputDecoration(labelText: '开始时间'))), const SizedBox(width: 12), Expanded(child: TextField(controller: _workEndController, decoration: const InputDecoration(labelText: '结束时间')))]),
                          Row(children: [Expanded(child: TextField(controller: _lunchStartController, decoration: const InputDecoration(labelText: '午休开始'))), const SizedBox(width: 12), Expanded(child: TextField(controller: _lunchEndController, decoration: const InputDecoration(labelText: '午休结束')))]),
                          Row(children: [Expanded(child: _numberField(_commuteMinController, '通勤最短（分钟）')), const SizedBox(width: 12), Expanded(child: _numberField(_commuteMaxController, '通勤最长（分钟）'))]),
                          Row(children: [Expanded(child: _numberField(_prepareMinController, '准备最短（分钟）')), const SizedBox(width: 12), Expanded(child: _numberField(_prepareMaxController, '准备最长（分钟）'))]),
                          DropdownButtonFormField<String>(
                            value: const ['NO_REPLY', 'SHORT_REPLY', 'NORMAL_REPLY'].contains(_workProfile['replyMode']) ? _workProfile['replyMode'] as String : 'SHORT_REPLY',
                            decoration: const InputDecoration(labelText: '工作时回复模式'),
                            items: const [
                              DropdownMenuItem(value: 'NO_REPLY', child: Text('不回复')),
                              DropdownMenuItem(value: 'SHORT_REPLY', child: Text('简短回复')),
                              DropdownMenuItem(value: 'NORMAL_REPLY', child: Text('正常回复')),
                            ],
                            onChanged: (value) => setState(() => _workProfile['replyMode'] = value ?? 'SHORT_REPLY'),
                          ),
                          SwitchListTile(contentPadding: EdgeInsets.zero, title: const Text('允许加班'), value: _bool(_workProfile['allowOvertime']), onChanged: (v) => setState(() => _workProfile['allowOvertime'] = v)),
                          if (_bool(_workProfile['allowOvertime'])) ...[
                            _slider('加班概率', _int(_workProfile['overtimeProbability'], 10), (v) => setState(() => _workProfile['overtimeProbability'] = v)),
                            Row(children: [Expanded(child: _numberField(_overtimeMinController, '加班最短（分钟）')), const SizedBox(width: 12), Expanded(child: _numberField(_overtimeMaxController, '加班最长（分钟）'))]),
                            DropdownButtonFormField<String>(
                              value: const ['NO_REPLY', 'SHORT_REPLY', 'NORMAL_REPLY'].contains(_workProfile['overtimeReplyMode']) ? _workProfile['overtimeReplyMode'] as String : 'SHORT_REPLY',
                              decoration: const InputDecoration(labelText: '加班回复模式'),
                              items: const [
                                DropdownMenuItem(value: 'NO_REPLY', child: Text('不回复')),
                                DropdownMenuItem(value: 'SHORT_REPLY', child: Text('简短回复')),
                                DropdownMenuItem(value: 'NORMAL_REPLY', child: Text('正常回复')),
                              ],
                              onChanged: (value) => setState(() => _workProfile['overtimeReplyMode'] = value ?? 'SHORT_REPLY'),
                            ),
                          ],
                          SwitchListTile(contentPadding: EdgeInsets.zero, title: const Text('延迟回复'), value: _bool(_workProfile['delayedReplyEnabled']), onChanged: (v) => setState(() => _workProfile['delayedReplyEnabled'] = v)),
                          SwitchListTile(contentPadding: EdgeInsets.zero, title: const Text('下班后主动分享'), value: _bool(_workProfile['commuteHomeShareEnabled'], fallback: true), onChanged: (v) => setState(() => _workProfile['commuteHomeShareEnabled'] = v)),
                          if (_bool(_workProfile['commuteHomeShareEnabled'], fallback: true))
                            _slider('下班分享概率', _int(_workProfile['commuteHomeShareProbability'], 60), (v) => setState(() => _workProfile['commuteHomeShareProbability'] = v)),
                        ]),
                      ),
                      _eventsSection('固定日程', _fixedEvents, special: false),
                      _eventsSection('特殊事件', _specialEvents, special: true),
                      AmitiaButton(label: _saving ? '保存中...' : '保存全部修改', icon: Icons.save_outlined, isFullWidth: true, onPressed: _saving ? null : _save),
                      SizedBox(height: AppSpacing.sectionGap),
                    ],
                  ),
      ),
    );
  }
}

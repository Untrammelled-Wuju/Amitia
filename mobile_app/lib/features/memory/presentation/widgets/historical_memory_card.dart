import 'package:flutter/material.dart';
import '../../../../app/theme/app_colors.dart';
import '../historical_memory_style.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../owned_memory_format.dart';
import '../pages/owned_memory_timeline.dart';

class HistoricalMemoryCard extends StatelessWidget {
  const HistoricalMemoryCard({
    super.key,
    required this.row,
    required this.kind,
    required this.onDetail,
    this.onEdit,
    this.onDelete,
    this.manager = false,
    this.timeline = false,
  });
  final Map<String, dynamic> row;
  final String kind;
  final VoidCallback onDetail;
  final VoidCallback? onEdit;
  final VoidCallback? onDelete;
  final bool manager;
  final bool timeline;

  @override
  Widget build(BuildContext context) {
    final record = _HistoricalRecord(row, kind);
    if (timeline) return _buildTimelineItem(context, record);
    if (kind == 'profile') return _buildProfileCard(context, record);
    if (kind == 'episodic') return _buildEpisodicCard(context, record);
    if (manager) return _buildManagerCard(context, record);
    return _buildMemoryCard(context, record);
  }

  bool get _batchMode => false;

  String _getCategory(_HistoricalRecord record) => record.category;
  String _getProfileFact(_HistoricalRecord record) => record.content;
  Widget _buildMemoryCard(BuildContext context, _HistoricalRecord memory) {
    return HistoricalMemorySurface(
      borderRadius: AppRadius.brMedium,
      padding: const EdgeInsets.all(AppSpacing.cardPadding),
      onTap: onDetail,
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Expanded(
                      child: Text(
                        memory.content,
                        style: AppTypography.body(context),
                      ),
                    ),
                  ],
                ),
                SizedBox(height: AppSpacing.sm),
                Row(
                  children: [
                    HistoricalMemoryBadge(
                      label: _importanceIntToString(memory.importance),
                      type: _importanceToBadgeType(memory.importance),
                    ),
                    SizedBox(width: AppSpacing.sm),
                    Flexible(
                      child: Text(
                        memory.type,
                        style: AppTypography.label(context),
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
                    SizedBox(width: AppSpacing.sm),
                    Text(
                      _formatTimeString(memory.createdAt),
                      style: AppTypography.label(context),
                    ),
                  ],
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildManagerCard(BuildContext context, _HistoricalRecord memory) {
    final isSelected = row['id'] == null;
    return HistoricalMemorySurface(
      borderRadius: AppRadius.brMedium,
      padding: const EdgeInsets.all(AppSpacing.cardPadding),
      border: Border.all(
        color: _batchMode && isSelected
            ? context.accentPrimary
            : context.borderPrimary,
        width: _batchMode && isSelected ? 1.5 : 0.5,
      ),
      onTap: onDetail,
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (_batchMode)
            Padding(
              padding: EdgeInsets.only(top: 2, right: AppSpacing.sm),
              child: Icon(
                isSelected ? Icons.check_circle : Icons.radio_button_unchecked,
                size: 20,
                color: isSelected
                    ? context.accentPrimary
                    : context.textTertiary,
              ),
            ),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Expanded(
                      child: Text(
                        memory.content,
                        style: AppTypography.body(context),
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
                  ],
                ),
                SizedBox(height: AppSpacing.sm),
                Row(
                  children: [
                    HistoricalMemoryBadge(
                      label: _importanceIntToString(memory.importance),
                      type: _importanceToBadgeType(memory.importance),
                    ),
                    SizedBox(width: AppSpacing.sm),
                    HistoricalMemoryBadge(
                      label: memory.type,
                      type: BadgeType.neutral,
                    ),
                    SizedBox(width: AppSpacing.sm),
                    Text(memory.status, style: AppTypography.label(context)),
                    const Spacer(),
                    Text(
                      _formatTimeString(memory.createdAt),
                      style: AppTypography.label(context),
                    ),
                  ],
                ),
                if (!_batchMode) ...[
                  SizedBox(height: AppSpacing.sm),
                  Row(
                    children: [
                      if (onEdit != null)
                        GestureDetector(
                          onTap: onEdit,
                          child: _buildMiniButton(
                            context,
                            '编辑',
                            context.accentPrimary,
                          ),
                        ),
                      SizedBox(width: AppSpacing.sm),
                      if (onDelete != null)
                        GestureDetector(
                          onTap: onDelete,
                          child: _buildMiniButton(context, '删除', context.error),
                        ),
                    ],
                  ),
                ],
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildMiniButton(BuildContext context, String label, Color color) {
    return Container(
      padding: EdgeInsets.symmetric(horizontal: 10, vertical: 3),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.1),
        borderRadius: AppRadius.brTag,
      ),
      child: Text(label, style: TextStyle(fontSize: 12, color: color)),
    );
  }

  Widget _buildProfileCard(BuildContext context, _HistoricalRecord profile) {
    final category = _getCategory(profile);
    final fact = _getProfileFact(profile);
    return HistoricalMemorySurface(
      borderRadius: AppRadius.brMedium,
      padding: const EdgeInsets.all(AppSpacing.cardPadding),
      onTap: onDetail,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                width: 40,
                height: 40,
                decoration: BoxDecoration(
                  color: _getCategoryColor(
                    context,
                    category,
                  ).withValues(alpha: 0.12),
                  borderRadius: AppRadius.brSmall,
                ),
                child: Icon(
                  _getCategoryIcon(category),
                  size: 20,
                  color: _getCategoryColor(context, category),
                ),
              ),
              SizedBox(width: AppSpacing.md),
              Expanded(child: Text(fact, style: AppTypography.body(context))),
            ],
          ),
          SizedBox(height: AppSpacing.sm),
          Row(
            children: [
              HistoricalMemoryBadge(
                label: category,
                type: _getCategoryBadge(category),
              ),
              SizedBox(width: AppSpacing.sm),
              Icon(Icons.source, size: 12, color: context.textTertiary),
              SizedBox(width: 2),
              Text(
                profile.gender.isNotEmpty ? profile.gender : '系统生成',
                style: AppTypography.label(context),
              ),
              const Spacer(),
              Text(
                _formatDateString(profile.createdAt),
                style: AppTypography.label(context),
              ),
            ],
          ),
          SizedBox(height: AppSpacing.sm),
          Row(
            children: [
              if (onEdit != null)
                GestureDetector(
                  onTap: onEdit,
                  child: Container(
                    padding: EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                    decoration: BoxDecoration(
                      color: context.accentSoft,
                      borderRadius: AppRadius.brTag,
                    ),
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(
                          Icons.edit_outlined,
                          size: 14,
                          color: context.accentPrimary,
                        ),
                        SizedBox(width: 4),
                        Text(
                          '编辑',
                          style: TextStyle(
                            fontSize: 12,
                            color: context.accentPrimary,
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
              SizedBox(width: AppSpacing.sm),
              if (onDelete != null)
                GestureDetector(
                  onTap: onDelete,
                  child: Container(
                    padding: EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                    decoration: BoxDecoration(
                      color: context.error.withValues(alpha: 0.1),
                      borderRadius: AppRadius.brTag,
                    ),
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(
                          Icons.delete_outline,
                          size: 14,
                          color: context.error,
                        ),
                        SizedBox(width: 4),
                        Text(
                          '删除',
                          style: TextStyle(fontSize: 12, color: context.error),
                        ),
                      ],
                    ),
                  ),
                ),
            ],
          ),
        ],
      ),
    );
  }

  Color _getCategoryColor(BuildContext context, String category) {
    switch (category) {
      case '事实':
        return context.accentPrimary;
      case '偏好':
        return context.info;
      case '习惯':
        return context.success;
      case '关系':
        return context.warning;
      default:
        return context.textSecondary;
    }
  }

  IconData _getCategoryIcon(String category) {
    switch (category) {
      case '事实':
        return Icons.fact_check_outlined;
      case '偏好':
        return Icons.thumb_up_alt_outlined;
      case '习惯':
        return Icons.repeat;
      case '关系':
        return Icons.people_outline;
      default:
        return Icons.person_outline;
    }
  }

  BadgeType _getCategoryBadge(String category) {
    switch (category) {
      case '事实':
        return BadgeType.accent;
      case '偏好':
        return BadgeType.info;
      case '习惯':
        return BadgeType.success;
      case '关系':
        return BadgeType.warning;
      default:
        return BadgeType.neutral;
    }
  }

  String _formatDateString(String timeStr) {
    if (timeStr.isEmpty) return '';
    try {
      final date = DateTime.parse(timeStr);
      return '${date.month}/${date.day}';
    } catch (_) {
      return timeStr;
    }
  }

  Widget _buildEpisodicCard(BuildContext context, _HistoricalRecord memory) {
    return HistoricalMemorySurface(
      borderRadius: AppRadius.brMedium,
      padding: const EdgeInsets.all(AppSpacing.cardPadding),
      onTap: onDetail,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                width: 44,
                height: 44,
                decoration: BoxDecoration(
                  color: _getEmotionColor(
                    context,
                    memory.emotion,
                  ).withValues(alpha: 0.12),
                  borderRadius: AppRadius.brSmall,
                ),
                child: Icon(
                  Icons.psychology_outlined,
                  size: 22,
                  color: _getEmotionColor(context, memory.emotion),
                ),
              ),
              SizedBox(width: AppSpacing.md),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      memory.summary.isNotEmpty ? memory.summary : memory.title,
                      style: AppTypography.cardTitle(context),
                    ),
                    SizedBox(height: 2),
                    Text(
                      _formatTimeString(
                        memory.timestamp.isNotEmpty
                            ? memory.timestamp
                            : memory.createdAt,
                      ),
                      style: AppTypography.caption(context),
                    ),
                  ],
                ),
              ),
              HistoricalMemoryBadge(
                label: memory.emotion,
                type: _getEmotionBadge(memory.emotion),
              ),
            ],
          ),
          SizedBox(height: AppSpacing.sm),
          Text(
            memory.content,
            style: AppTypography.caption(context),
            maxLines: 2,
            overflow: TextOverflow.ellipsis,
          ),
          SizedBox(height: AppSpacing.sm),
          Row(
            children: [
              GestureDetector(
                onTap: onDetail,
                child: Container(
                  padding: EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                  decoration: BoxDecoration(
                    color: context.accentSoft,
                    borderRadius: AppRadius.brTag,
                  ),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(
                        Icons.visibility_outlined,
                        size: 14,
                        color: context.accentPrimary,
                      ),
                      SizedBox(width: 4),
                      Text(
                        '详情',
                        style: TextStyle(
                          fontSize: 12,
                          color: context.accentPrimary,
                        ),
                      ),
                    ],
                  ),
                ),
              ),
              const Spacer(),
              if (onDelete != null)
                GestureDetector(
                  onTap: onDelete,
                  child: Container(
                    padding: EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                    decoration: BoxDecoration(
                      color: context.error.withValues(alpha: 0.1),
                      borderRadius: AppRadius.brTag,
                    ),
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(
                          Icons.delete_outline,
                          size: 14,
                          color: context.error,
                        ),
                        SizedBox(width: 4),
                        Text(
                          '删除',
                          style: TextStyle(fontSize: 12, color: context.error),
                        ),
                      ],
                    ),
                  ),
                ),
            ],
          ),
        ],
      ),
    );
  }

  Color _getEmotionColor(BuildContext context, String emotion) {
    switch (emotion) {
      case '愉快':
      case '开心':
      case '兴奋':
        return context.success;
      case '关心':
        return context.info;
      case '满足':
        return context.accentPrimary;
      case '悲伤':
      case '难过':
        return context.error;
      default:
        return context.warning;
    }
  }

  BadgeType _getEmotionBadge(String emotion) {
    switch (emotion) {
      case '愉快':
      case '开心':
      case '兴奋':
        return BadgeType.success;
      case '关心':
        return BadgeType.info;
      case '满足':
        return BadgeType.accent;
      default:
        return BadgeType.warning;
    }
  }

  Widget _buildTimelineItem(BuildContext context, _HistoricalRecord entry) {
    return Padding(
      padding: EdgeInsets.only(bottom: AppSpacing.sm),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 42,
            child: Padding(
              padding: EdgeInsets.only(top: 14),
              child: Text(
                '${entry.time.hour.toString().padLeft(2, '0')}:${entry.time.minute.toString().padLeft(2, '0')}',
                style: AppTypography.label(context),
                textAlign: TextAlign.center,
              ),
            ),
          ),
          Container(
            width: 18,
            margin: EdgeInsets.only(top: 14),
            child: Container(
              width: 10,
              height: 10,
              decoration: BoxDecoration(
                color: _getTypeColor(context, entry.type),
                shape: BoxShape.circle,
                border: Border.all(color: context.surfacePrimary, width: 2),
              ),
            ),
          ),
          SizedBox(width: AppSpacing.xs),
          Expanded(
            child: HistoricalMemorySurface(
              borderRadius: AppRadius.brMedium,
              padding: const EdgeInsets.all(AppSpacing.cardPadding),
              border: Border.all(
                color: entry.isImportant
                    ? context.warning.withValues(alpha: 0.3)
                    : context.borderPrimary,
                width: entry.isImportant ? 1 : 0.5,
              ),
              onTap: onDetail,
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Icon(
                        _getTypeIcon(entry.type),
                        size: 16,
                        color: _getTypeColor(context, entry.type),
                      ),
                      SizedBox(width: AppSpacing.xs),
                      Expanded(
                        child: Text(
                          entry.title,
                          style: AppTypography.cardTitle(context),
                        ),
                      ),
                      if (entry.isImportant)
                        Padding(
                          padding: EdgeInsets.only(left: AppSpacing.xs),
                          child: Icon(
                            Icons.star,
                            size: 14,
                            color: context.warning,
                          ),
                        ),
                      HistoricalMemoryBadge(
                        label: entry.type,
                        type: _getTypeBadge(entry.type),
                      ),
                    ],
                  ),
                  SizedBox(height: AppSpacing.xs),
                  Text(
                    entry.description,
                    style: AppTypography.caption(context),
                  ),
                  SizedBox(height: AppSpacing.sm),
                  Row(
                    children: [
                      if (entry.characterId.isNotEmpty)
                        Text(
                          '角色：${entry.characterName}',
                          style: AppTypography.label(context),
                        ),
                      if (entry.isImportant) ...[
                        SizedBox(width: AppSpacing.sm),
                        HistoricalMemoryBadge(
                          label: '重要',
                          type: BadgeType.warning,
                        ),
                      ],
                      const Spacer(),
                      GestureDetector(
                        onTap: onDetail,
                        child: Container(
                          padding: EdgeInsets.symmetric(
                            horizontal: 8,
                            vertical: 3,
                          ),
                          decoration: BoxDecoration(
                            color: context.accentSoft,
                            borderRadius: AppRadius.brTag,
                          ),
                          child: Row(
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              Icon(
                                Icons.memory,
                                size: 12,
                                color: context.accentPrimary,
                              ),
                              SizedBox(width: 4),
                              Text(
                                '记忆详情',
                                style: TextStyle(
                                  fontSize: 11,
                                  color: context.accentPrimary,
                                ),
                              ),
                            ],
                          ),
                        ),
                      ),
                    ],
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }

  Color _getTypeColor(BuildContext context, String type) {
    switch (type) {
      case '对话':
        return context.accentPrimary;
      case '主动任务':
        return context.info;
      case '记忆形成':
        return context.success;
      case '情绪':
        return context.warning;
      case '关系':
        return context.error;
      case '行为':
        return context.accentSecondary;
      default:
        return context.textSecondary;
    }
  }

  IconData _getTypeIcon(String type) {
    switch (type) {
      case '对话':
        return Icons.chat_bubble_outline;
      case '主动任务':
        return Icons.notifications_active_outlined;
      case '记忆形成':
        return Icons.memory;
      case '情绪':
        return Icons.mood;
      case '关系':
        return Icons.favorite_outline;
      case '行为':
        return Icons.analytics_outlined;
      default:
        return Icons.circle_outlined;
    }
  }

  BadgeType _getTypeBadge(String type) {
    switch (type) {
      case '对话':
        return BadgeType.accent;
      case '主动任务':
        return BadgeType.info;
      case '记忆形成':
        return BadgeType.success;
      case '情绪':
        return BadgeType.warning;
      case '关系':
        return BadgeType.error;
      default:
        return BadgeType.neutral;
    }
  }

  String _importanceIntToString(int importance) {
    if (importance >= 80) return '高';
    if (importance >= 60) return '较高';
    if (importance >= 40) return '中';
    return '低';
  }

  BadgeType _importanceToBadgeType(int importance) {
    if (importance >= 80) return BadgeType.error;
    if (importance >= 60) return BadgeType.warning;
    if (importance >= 40) return BadgeType.info;
    return BadgeType.neutral;
  }

  String _formatTimeString(String timeStr) {
    if (timeStr.isEmpty) return '';
    try {
      final time = DateTime.parse(timeStr);
      final now = DateTime.now();
      final diff = now.difference(time);
      if (diff.inHours < 1) return '刚刚';
      if (diff.inDays == 0) return '${diff.inHours}小时前';
      if (diff.inDays < 7) return '${diff.inDays}天前';
      return '${time.month}/${time.day}';
    } catch (_) {
      return timeStr;
    }
  }
}

class _HistoricalRecord {
  const _HistoricalRecord(this.row, this.kind);
  final Map<String, dynamic> row;
  final String kind;
  Map<String, dynamic> get body => ownedMemoryBody(row);
  Map get data => body['content'] is Map ? body['content'] as Map : body;
  String get content => historicalMemoryPreview(row, kind: kind);
  String get title => ownedMemoryTitle(row);
  String get summary => (data['summary'] ?? '').toString();
  String get description => content;
  String get characterId => (row['roleId'] ?? '').toString();
  String get characterName =>
      row['roleId'] == '@global-memory' ? '全局记忆' : '角色记忆';
  String get category => switch ((data['memoryType'] ??
          data['category'] ??
          body['memoryType'] ??
          '')
      .toString()) {
    'preference' || 'hobby' || '偏好' => '偏好',
    'habit' || '习惯' => '习惯',
    'relationship' || '关系' => '关系',
    _ => '事实',
  };
  String get type => kind == 'episodic'
      ? '情景记忆'
      : kind == 'profile'
      ? '用户画像'
      : category == '关系'
      ? '关系记忆'
      : '长期记忆';
  int get importance {
    final value = body['importance'] ?? data['importance'];
    final number = value is num ? value.toInt() : int.tryParse('$value') ?? 0;
    return number <= 10 ? number * 10 : number;
  }

  bool get isImportant => importance >= 80;
  String get status => body['allowContextUse'] == false
      ? '已停用'
      : body['archivedAt'] != null
      ? '已归档'
      : '已保存';
  String get emotion => (data['emotion'] ?? '平静').toString();
  String get gender => (data['source'] ?? body['source'] ?? '').toString();
  DateTime get time =>
      ownedMemoryResourceTime(row)?.toLocal() ??
      DateTime.fromMillisecondsSinceEpoch(0);
  String get createdAt => ownedMemoryResourceTime(row)?.toIso8601String() ?? '';
  String get timestamp => createdAt;
}

String historicalMemoryCategory(Map<String, dynamic> row) =>
    _HistoricalRecord(row, "profile").category;

String historicalMemoryPreview(Map<String, dynamic> row, {String? kind}) {
  final content = ownedMemoryContent(row);
  if (content is String) return content.trim();
  if (content is Map) {
    final keys = kind == 'profile'
        ? const ['attributeValue', 'fieldValue', 'value', 'text', 'summary']
        : const ['value', 'text', 'event', 'summary', 'content'];
    for (final key in keys) {
      final value = content[key];
      if (value is String && value.trim().isNotEmpty) return value.trim();
    }
  }
  return ownedMemoryText(row, kind: kind);
}

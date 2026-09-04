class Reminder {
  final String id;
  final String title;
  final String description;
  final DateTime time;
  final bool isCompleted;
  final bool isEnabled;
  final String category;
  final bool isToday;

  Reminder({
    required this.id,
    required this.title,
    required this.description,
    required this.time,
    this.isCompleted = false,
    this.isEnabled = true,
    this.category = '日常',
    this.isToday = false,
  });
}

class EmoteItem {
  final String id;
  final String name;
  final String meaning;
  final String group;
  final String? characterId;
  final bool isEnabled;
  final int sendProbability;
  final String emoji;

  EmoteItem({
    required this.id,
    required this.name,
    required this.meaning,
    required this.group,
    this.characterId,
    this.isEnabled = true,
    this.sendProbability = 50,
    this.emoji = '😊',
  });
}

class EmoteGroup {
  final String id;
  final String name;
  final int count;

  EmoteGroup({required this.id, required this.name, this.count = 0});
}

class EpisodicMemory {
  final String id;
  final DateTime time;
  final String location;
  final List<String> participants;
  final String emotion;
  final String summary;
  final String detail;

  EpisodicMemory({
    required this.id,
    required this.time,
    required this.location,
    required this.participants,
    required this.emotion,
    required this.summary,
    required this.detail,
  });
}

class MemoryGraphNode {
  final String id;
  final String label;
  final String type;
  final String category;
  final double x;
  final double y;

  MemoryGraphNode({
    required this.id,
    required this.label,
    required this.type,
    this.category = '记忆',
    this.x = 0,
    this.y = 0,
  });
}

class MemoryGraphEdge {
  final String source;
  final String target;
  final String relation;

  MemoryGraphEdge({
    required this.source,
    required this.target,
    required this.relation,
  });
}

class MemoryTimelineEntry {
  final String id;
  final DateTime time;
  final String type;
  final String title;
  final String description;
  final String? characterId;
  final bool isImportant;

  MemoryTimelineEntry({
    required this.id,
    required this.time,
    required this.type,
    required this.title,
    required this.description,
    this.characterId,
    this.isImportant = false,
  });
}

class UserProfile {
  final String id;
  final String category;
  final String fact;
  final double confidence;
  final String source;
  final DateTime updated;

  UserProfile({
    required this.id,
    required this.category,
    required this.fact,
    this.confidence = 0.8,
    required this.source,
    required this.updated,
  });
}

class WorldBookEntry {
  final String id;
  final String keyword;
  final String content;
  final int priority;
  final bool isEnabled;
  final String category;

  WorldBookEntry({
    required this.id,
    required this.keyword,
    required this.content,
    this.priority = 5,
    this.isEnabled = true,
    this.category = '默认',
  });
}

class ChatLogConversation {
  final String id;
  final String title;
  final String characterId;
  final String channel;
  final int messageCount;
  final DateTime lastTime;

  ChatLogConversation({
    required this.id,
    required this.title,
    required this.characterId,
    required this.channel,
    required this.messageCount,
    required this.lastTime,
  });
}

class ChatLogMessage {
  final String id;
  final String role;
  final String content;
  final DateTime time;
  final String? context;

  ChatLogMessage({
    required this.id,
    required this.role,
    required this.content,
    required this.time,
    this.context,
  });
}

class ImportBatch {
  final String id;
  final String source;
  final int messageCount;
  final DateTime importTime;
  final String status;

  ImportBatch({
    required this.id,
    required this.source,
    required this.messageCount,
    required this.importTime,
    this.status = '已完成',
  });
}

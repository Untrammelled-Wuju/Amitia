class CharacterVoiceConfig {
  final String id;
  final String name;
  final String preset;
  final double speed;
  final double pitch;
  final double volume;
  final bool isCurrent;

  CharacterVoiceConfig({
    required this.id,
    required this.name,
    required this.preset,
    this.speed = 1.0,
    this.pitch = 1.0,
    this.volume = 0.8,
    this.isCurrent = false,
  });
}

class FixedSchedule {
  final String id;
  final String title;
  final String startTime;
  final String endTime;
  final bool repeat;
  final String category;

  FixedSchedule({
    required this.id,
    required this.title,
    required this.startTime,
    required this.endTime,
    this.repeat = true,
    this.category = '日常',
  });
}

class SpecialState {
  final String id;
  final String name;
  final String description;
  final bool isActive;

  SpecialState({
    required this.id,
    required this.name,
    required this.description,
    this.isActive = false,
  });
}

class PsycheState {
  final String emotion;
  final int intensity;
  final int stability;
  final String influence;
  final String relationshipStatus;
  final DateTime time;

  PsycheState({
    required this.emotion,
    required this.intensity,
    required this.stability,
    required this.influence,
    required this.relationshipStatus,
    required this.time,
  });
}

class TimelineEvent {
  final String id;
  final DateTime time;
  final String type;
  final String title;
  final String description;
  final String? emotion;

  TimelineEvent({
    required this.id,
    required this.time,
    required this.type,
    required this.title,
    required this.description,
    this.emotion,
  });
}

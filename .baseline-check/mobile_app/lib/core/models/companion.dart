class CompanionStateDto {
  final String state;
  final bool isSleeping;
  final String currentActivity;
  final String nextActivity;
  final String wakeTime;
  final String sleepTime;

  CompanionStateDto({
    this.state = '',
    this.isSleeping = false,
    this.currentActivity = '',
    this.nextActivity = '',
    this.wakeTime = '',
    this.sleepTime = '',
  });

  factory CompanionStateDto.fromJson(Map<String, dynamic> json) {
    return CompanionStateDto(
      state: json['state'] as String? ?? '',
      isSleeping: json['isSleeping'] as bool? ?? false,
      currentActivity: json['currentActivity'] as String? ?? '',
      nextActivity: json['nextActivity'] as String? ?? '',
      wakeTime: json['wakeTime'] as String? ?? '',
      sleepTime: json['sleepTime'] as String? ?? '',
    );
  }
}

class LifeStateDto {
  final String mood;
  final int energy;
  final int social;
  final int hunger;
  final String location;

  LifeStateDto({
    this.mood = '',
    this.energy = 100,
    this.social = 100,
    this.hunger = 100,
    this.location = '',
  });

  factory LifeStateDto.fromJson(Map<String, dynamic> json) {
    return LifeStateDto(
      mood: json['mood'] as String? ?? '',
      energy: json['energy'] as int? ?? 100,
      social: json['social'] as int? ?? 100,
      hunger: json['hunger'] as int? ?? 100,
      location: json['location'] as String? ?? '',
    );
  }
}

class ScheduleEventDto {
  final String id;
  final String title;
  final String startTime;
  final String endTime;
  final String type;
  final int enabled;

  ScheduleEventDto({
    required this.id,
    this.title = '',
    this.startTime = '',
    this.endTime = '',
    this.type = '',
    this.enabled = 1,
  });

  factory ScheduleEventDto.fromJson(Map<String, dynamic> json) {
    return ScheduleEventDto(
      id: (json['id'] ?? '').toString(),
      title: json['title'] as String? ?? '',
      startTime: json['startTime'] as String? ?? '',
      endTime: json['endTime'] as String? ?? '',
      type: json['type'] as String? ?? '',
      enabled: json['enabled'] as int? ?? 1,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'title': title,
      'startTime': startTime,
      'endTime': endTime,
      'type': type,
      'enabled': enabled,
    };
  }
}

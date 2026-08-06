class ProfileDto {
  final String id;
  final String name;
  final int age;
  final String gender;
  final String occupation;
  final String personality;
  final String background;
  final String createdAt;

  ProfileDto({
    required this.id,
    this.name = '',
    this.age = 0,
    this.gender = '',
    this.occupation = '',
    this.personality = '',
    this.background = '',
    this.createdAt = '',
  });

  factory ProfileDto.fromJson(Map<String, dynamic> json) {
    return ProfileDto(
      id: (json['id'] ?? '').toString(),
      name: json['name'] as String? ?? '',
      age: json['age'] as int? ?? 0,
      gender: json['gender'] as String? ?? '',
      occupation: json['occupation'] as String? ?? '',
      personality: json['personality'] as String? ?? '',
      background: json['background'] as String? ?? '',
      createdAt: json['createdAt'] as String? ?? '',
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'name': name,
      'age': age,
      'gender': gender,
      'occupation': occupation,
      'personality': personality,
      'background': background,
    };
  }
}

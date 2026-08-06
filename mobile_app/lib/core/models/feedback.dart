class FeedbackDto {
  final String id;
  final String messageId;
  final int rating;
  final String comment;
  final String createdAt;

  FeedbackDto({
    required this.id,
    this.messageId = '',
    this.rating = 0,
    this.comment = '',
    this.createdAt = '',
  });

  factory FeedbackDto.fromJson(Map<String, dynamic> json) {
    return FeedbackDto(
      id: (json['id'] ?? '').toString(),
      messageId: (json['messageId'] ?? '').toString(),
      rating: json['rating'] as int? ?? 0,
      comment: json['comment'] as String? ?? '',
      createdAt: json['createdAt'] as String? ?? '',
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'messageId': messageId,
      'rating': rating,
      'comment': comment,
    };
  }
}

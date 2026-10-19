class FeedbackDto {
  final String id;
  final String messageId;
  final int rating;
  final String comment;
  final String createdAt;
  final String feedbackType;
  final String reason;

  FeedbackDto({
    required this.id,
    this.messageId = '',
    this.rating = 0,
    this.comment = '',
    this.createdAt = '',
    this.feedbackType = '',
    this.reason = '',
  });

  factory FeedbackDto.fromJson(Map<String, dynamic> json) {
    return FeedbackDto(
      id: (json['id'] ?? '').toString(),
      messageId: (json['messageId'] ?? '').toString(),
      rating: json['rating'] as int? ?? (json['feedbackType'] == 'good' ? 1 : -1),
      comment: (json['reason'] ?? json['comment'] ?? '').toString(),
      createdAt: json['createdAt'] as String? ?? '',
      feedbackType: (json['feedbackType'] ?? '').toString(),
      reason: (json['reason'] ?? '').toString(),
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'messageId': messageId,
      'rating': rating,
      'comment': comment,
      'feedbackType': feedbackType,
      'reason': reason,
    };
  }
}

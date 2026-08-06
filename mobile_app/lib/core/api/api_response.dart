class ApiResponse<T> {
  final int code;
  final String message;
  final T? data;
  final String? detail;

  ApiResponse({
    required this.code,
    required this.message,
    this.data,
    this.detail,
  });

  factory ApiResponse.fromJson(
    Map<String, dynamic> json,
    T Function(dynamic)? fromJsonT,
  ) {
    return ApiResponse<T>(
      code: json['code'] as int? ?? 0,
      message: json['message'] as String? ?? json['msg'] as String? ?? '',
      data: fromJsonT != null && json['data'] != null
          ? fromJsonT(json['data'])
          : json['data'] as T?,
      detail: json['detail'] as String?,
    );
  }

  bool get isSuccess => code == 200;
}

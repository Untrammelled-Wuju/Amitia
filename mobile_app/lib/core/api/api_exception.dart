enum ErrorSeverity { toast, banner, panel, fatal }

class ApiException implements Exception {
  final int code;
  final String message;
  final String? detail;
  final ErrorSeverity severity;
  final dynamic raw;

  ApiException({
    required this.code,
    required this.message,
    this.detail,
    this.severity = ErrorSeverity.toast,
    this.raw,
  });

  factory ApiException.fromResponse(int code, String message, {String? detail, dynamic raw}) {
    final severity = _classifySeverity(code);
    return ApiException(
      code: code,
      message: message,
      detail: detail,
      severity: severity,
      raw: raw,
    );
  }

  factory ApiException.network(String message) {
    return ApiException(
      code: 10001,
      message: message,
      severity: ErrorSeverity.toast,
    );
  }

  factory ApiException.timeout() {
    return ApiException(
      code: 10002,
      message: '请求超时',
      severity: ErrorSeverity.toast,
    );
  }

  static ErrorSeverity _classifySeverity(int code) {
    if (code == 401 || code == 700 || code == 701 || code == 20002 || code == 20003) {
      return ErrorSeverity.fatal;
    }
    if (code >= 500 && code < 600) return ErrorSeverity.panel;
    if (code >= 400 && code < 500) return ErrorSeverity.toast;
    if (code >= 600 && code < 700) return ErrorSeverity.banner;
    return ErrorSeverity.toast;
  }

  bool get isAuthError => code == 401 || code == 403 || code == 700 || code == 701 || code == 20002 || code == 20003;
}

import '../backend_transport/backend_service_api.dart';

String safeErrorMessage(Object e) {
  if (e is ServiceApiException) return e.message;
  return '操作失败，请稍后重试';
}

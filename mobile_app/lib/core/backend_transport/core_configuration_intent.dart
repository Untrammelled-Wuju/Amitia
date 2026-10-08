import 'dart:async';

class CoreConfigurationIntent {
  static final Object _zoneKey = Object();

  final int generation;
  final String? coreId;
  final String? policyRevision;
  final bool canConfigure;
  final bool Function() isCurrent;

  const CoreConfigurationIntent({
    required this.generation,
    required this.coreId,
    this.policyRevision,
    required this.canConfigure,
    required this.isCurrent,
  });

  static CoreConfigurationIntent? get current =>
      Zone.current[_zoneKey] as CoreConfigurationIntent?;

  void validateGeneration(int actualGeneration) {
    if (!canConfigure ||
        generation <= 0 ||
        generation != actualGeneration ||
        !isCurrent()) {
      throw StateError('模型配置归属或权限已变化，请关闭旧表单并重新加载');
    }
  }

  Future<T> run<T>(Future<T> Function() operation) {
    validateGeneration(generation);
    return runZoned(operation, zoneValues: {_zoneKey: this});
  }
}

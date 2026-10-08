import 'dart:async';

import '../backend_transport/core_configuration_intent.dart';
import 'core_configuration_guard.dart';

class CoreConfigurationSession {
  final CoreConfigurationGuard guard;
  final void Function(Object)? onInvalidated;
  CoreConfigurationIntent? _intent;
  Timer? _watch;
  int _epoch = 0;
  bool _checking = false;
  bool _closed = false;

  CoreConfigurationSession(this.guard, {this.onInvalidated});

  CoreConfigurationIntent? get intent => _intent;

  void invalidate(Object reason) {
    _epoch++;
    _intent = null;
    _watch?.cancel();
    _watch = null;
    if (!_closed) onInvalidated?.call(reason);
  }

  Future<T> load<T>(Future<T> Function() read) async {
    if (_closed) throw StateError('配置页面已关闭');
    final epoch = ++_epoch;
    _intent = null;
    _watch?.cancel();
    final captured = await guard.capture();
    if (_closed || epoch != _epoch) throw StateError('旧配置加载已取消');
    if (!captured.canConfigure)
      throw StateError('Core 配置由云端统一管理，只有开启统筹模式的当前 Core 管理员可以修改');
    final value = await captured.run(read);
    await guard.validate(captured);
    if (_closed || epoch != _epoch) throw StateError('旧配置加载结果已丢弃');
    _intent = captured;
    if (captured.coreId != null) {
      _watch = Timer.periodic(const Duration(seconds: 1), (_) {
        if (_checking || _closed || !identical(_intent, captured)) return;
        _checking = true;
        unawaited(
          validate(captured)
              .catchError((Object error) {
                if (identical(_intent, captured)) invalidate(error);
              })
              .whenComplete(() => _checking = false),
        );
      });
    }
    return value;
  }

  Future<void> validate(CoreConfigurationIntent? original) async {
    if (_closed || original == null || !identical(_intent, original))
      throw StateError('配置范围已变化，请重新打开原配置');
    try {
      await guard.validate(original);
    } catch (error) {
      if (identical(_intent, original)) invalidate(error);
      rethrow;
    }
    if (_closed || !identical(_intent, original)) throw StateError('原配置操作已取消');
  }

  Future<T> write<T>(
    CoreConfigurationIntent? original,
    Future<T> Function() operation,
  ) async {
    await validate(original);
    final value = await original!.run(operation);
    await validate(original);
    return value;
  }

  void close() {
    _closed = true;
    _epoch++;
    _intent = null;
    _watch?.cancel();
  }
}

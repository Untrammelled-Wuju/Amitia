import 'dart:async';

enum ExtensionManagementTarget {
  extensionCenter,
  gameCenter,
  DesktopPetCenter,
}

abstract interface class ExtensionViewInvalidator {
  void invalidateExtension(String extensionId);
  void invalidateTarget(ExtensionManagementTarget target);
  void invalidateAllTargetsForExtension(String extensionId);
  Stream<ExtensionInvalidationEvent> get changes;
  void dispose();
}

sealed class ExtensionInvalidationEvent {}

class ExtensionInvalidated extends ExtensionInvalidationEvent {
  final String extensionId;
  ExtensionInvalidated(this.extensionId);
}

class TargetInvalidated extends ExtensionInvalidationEvent {
  final ExtensionManagementTarget target;
  TargetInvalidated(this.target);
}

class AllTargetsInvalidatedForExtension extends ExtensionInvalidationEvent {
  final String extensionId;
  AllTargetsInvalidatedForExtension(this.extensionId);
}

class ExtensionViewInvalidatorImpl implements ExtensionViewInvalidator {
  final _controller = StreamController<ExtensionInvalidationEvent>.broadcast();

  @override
  Stream<ExtensionInvalidationEvent> get changes => _controller.stream;

  @override
  void invalidateExtension(String extensionId) {
    _controller.add(ExtensionInvalidated(extensionId));
  }

  @override
  void invalidateTarget(ExtensionManagementTarget target) {
    _controller.add(TargetInvalidated(target));
  }

  @override
  void invalidateAllTargetsForExtension(String extensionId) {
    _controller.add(AllTargetsInvalidatedForExtension(extensionId));
  }

  @override
  void dispose() {
    _controller.close();
  }
}

import '../models/schema_ui_types.dart';

class ActionInvocation {
  final String actionId;
  final String target;
  final Map<String, dynamic>? input;
  final String extensionId;
  final String contributionId;
  final Map<String, dynamic> ownerIdentity;

  const ActionInvocation({
    required this.actionId,
    required this.target,
    this.input,
    required this.extensionId,
    required this.contributionId,
    required this.ownerIdentity,
  });

  Map<String, dynamic> toJson() {
    return {
      'actionId': actionId,
      'target': target,
      if (input != null) 'input': input,
      'owner': ownerIdentity,
    };
  }
}

class ActionDispatcher {
  final void Function(ActionInvocation) onDispatch;

  const ActionDispatcher({required this.onDispatch});

  /// The canonical Schema UI contract validates declared actions on the host.
  /// Clients must not maintain a second target whitelist because doing so can
  /// reject actions that are valid on another platform or added by the host.
  bool isAllowed(String target) => target.trim().isNotEmpty;

  void dispatch({
    required SchemaUIActionBinding action,
    required String extensionId,
    required String contributionId,
    required String? moduleId,
    List<String>? permissions,
  }) {
    if (!isAllowed(action.target)) return;

    final ownerIdentity = <String, dynamic>{
      'extensionId': extensionId,
      'contributionId': contributionId,
      if (moduleId != null) 'moduleId': moduleId,
      if (permissions != null) 'permissions': permissions,
    };

    onDispatch(ActionInvocation(
      actionId: action.actionId,
      target: action.target,
      input: action.input,
      extensionId: extensionId,
      contributionId: contributionId,
      ownerIdentity: ownerIdentity,
    ));
  }
}

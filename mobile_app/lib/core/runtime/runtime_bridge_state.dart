enum RuntimeBridgeState {
  unavailable,
  notInstalled,
  stopped,
  installing,
  starting,
  ready,
  stopping,
  failed;

  static RuntimeBridgeState fromNative(String? value) {
    if (value == null) return RuntimeBridgeState.unavailable;
    switch (value) {
      case 'NOT_INSTALLED':
        return RuntimeBridgeState.notInstalled;
      case 'STOPPED':
        return RuntimeBridgeState.stopped;
      case 'INSTALLING':
        return RuntimeBridgeState.installing;
      case 'STARTING':
        return RuntimeBridgeState.starting;
      case 'READY':
      case 'DEGRADED':
        return RuntimeBridgeState.ready;
      case 'STOPPING':
        return RuntimeBridgeState.stopping;
      case 'INSTALLED':
        return RuntimeBridgeState.stopped;
      case 'VERIFYING':
      case 'REPAIRING':
        return RuntimeBridgeState.installing;
      case 'FAILED':
      case 'CORRUPTED':
        return RuntimeBridgeState.failed;
      case 'UNKNOWN':
      default:
        return RuntimeBridgeState.unavailable;
    }
  }
}

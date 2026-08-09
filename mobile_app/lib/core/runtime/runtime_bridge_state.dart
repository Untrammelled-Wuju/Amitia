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
        return RuntimeBridgeState.ready;
      case 'STOPPING':
        return RuntimeBridgeState.stopping;
      case 'FAILED':
      case 'CORRUPTED':
        return RuntimeBridgeState.failed;
      case 'UNKNOWN':
      case 'INSTALLED':
      case 'VERIFYING':
      case 'DEGRADED':
      case 'REPAIRING':
      default:
        return RuntimeBridgeState.unavailable;
    }
  }
}

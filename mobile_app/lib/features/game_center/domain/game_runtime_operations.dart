enum GameRuntimeOperation { start, stop, restart }

Set<GameRuntimeOperation> availableGameRuntimeOperations(String state) {
  switch (state.trim().toLowerCase()) {
    case 'created':
    case 'stopped':
      return const {GameRuntimeOperation.start};
    case 'running':
    case 'degraded':
      return const {GameRuntimeOperation.stop, GameRuntimeOperation.restart};
    case 'starting':
    case 'suspended':
      return const {GameRuntimeOperation.stop};
    default:
      return const {};
  }
}

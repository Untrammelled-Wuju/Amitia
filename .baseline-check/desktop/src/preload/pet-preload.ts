/**
 * Legacy source-path compatibility shim.
 *
 * Desktop pet windows use `animation-preload.ts` as the only canonical preload
 * implementation. Keep this file free of independent IPC/channel definitions so
 * an old local import cannot silently resurrect the retired pet preload stack.
 */
import "./animation-preload";

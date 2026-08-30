/**
 * Desktop-pet Runtime v2 has exactly one transport implementation:
 * `desktop-pet/runtime/runtime-handler-v2.ts`.
 *
 * This module intentionally keeps only the renderer-neutral instance summary
 * type used by the manager.  The former RuntimeBridgeClient implementation
 * duplicated connection/session/cursor semantics and could diverge from the
 * canonical Runtime v2 handler during reconnect and recovery.
 */
export interface PetInstanceSummary {
  petInstanceId: string;
  installationId: string;
  visible: boolean;
  currentActionKey: string;
  positionX: number;
  positionY: number;
  screenId: string;
  windowWidth: number;
  windowHeight: number;
  scale: number;
}

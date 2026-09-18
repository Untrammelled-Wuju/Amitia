/**
 * Desktop-pet Runtime v1 has exactly one transport implementation:
 * `desktop-pet/runtime/runtime-handler-v1.ts`.
 *
 * This module intentionally keeps only the renderer-neutral instance summary
 * type used by the manager.  The former RuntimeBridgeClient implementation
 * duplicated connection/session/cursor semantics and could diverge from the
 * canonical Runtime v1 handler during reconnect and recovery.
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

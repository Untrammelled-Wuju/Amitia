/**
 * Legacy source-path compatibility shim.
 *
 * The former combined preload duplicated both legacy pet IPC and Runtime v2
 * animation IPC. All desktop-pet windows now use the isolated canonical
 * `animation-preload.ts` implementation.
 */
import "./animation-preload";

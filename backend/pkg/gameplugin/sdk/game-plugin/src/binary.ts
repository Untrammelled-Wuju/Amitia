/**
 * Binary streaming is intentionally not part of amitia-game-host/1.
 *
 * The host keeps an internal binary registry for lifecycle and future protocol
 * work, but public binary.register/binary.release helpers are deliberately not
 * exported until the production transport and admission path exist end to end.
 */
export {};

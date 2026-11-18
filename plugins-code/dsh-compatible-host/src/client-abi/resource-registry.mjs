function text(value) {
  return value == null ? "" : String(value).trim();
}

function protocolOf(address) {
  const match = /^dsh-resource:\/\/([^/?#]+)/.exec(text(address));
  return match?.[1] ?? "";
}

export function createDshClientResourceRegistry() {
  const providers = new Map();
  const states = new Map();

  const publish = (address, next) => {
    const current = states.get(address);
    const snapshot = Object.freeze({
      address,
      status: next.status,
      ...(Object.prototype.hasOwnProperty.call(next, "value") ? { value: next.value } : {}),
      ...(next.failure ? { failure: next.failure } : {}),
    });
    if (current?.snapshot === snapshot) return;
    states.set(address, {
      snapshot,
      listeners: current?.listeners ?? new Set(),
      controller: current?.controller,
    });
    for (const listener of Array.from(current?.listeners ?? [])) listener();
  };

  return {
    register(provider) {
      const protocol = text(provider?.protocol);
      if (!protocol) throw new Error("DSH resource provider protocol is required");
      if (typeof provider?.open !== "function") throw new Error(`DSH resource provider ${protocol} open() is required`);
      if (providers.has(protocol)) throw new Error(`DSH resource provider ${protocol} is already registered`);
      providers.set(protocol, provider);
      let active = true;
      return () => {
        if (!active) return;
        active = false;
        if (providers.get(protocol) === provider) providers.delete(protocol);
      };
    },
    getSnapshot(address) {
      const value = text(address);
      return states.get(value)?.snapshot ?? Object.freeze({ address: value, status: "none" });
    },
    subscribe(address, listener) {
      const value = text(address);
      if (!value) return () => {};
      let state = states.get(value);
      if (!state) {
        state = { snapshot: Object.freeze({ address: value, status: "none" }), listeners: new Set(), controller: undefined };
        states.set(value, state);
      }
      state.listeners.add(listener);
      return () => {
        const current = states.get(value);
        current?.listeners.delete(listener);
      };
    },
    watch(address) {
      const value = text(address);
      const provider = providers.get(protocolOf(value));
      if (!provider) {
        publish(value, { status: "none" });
        return () => {};
      }
      const controller = new AbortController();
      const current = states.get(value);
      states.set(value, {
        snapshot: Object.freeze({ address: value, status: "loading" }),
        listeners: current?.listeners ?? new Set(),
        controller,
      });
      void (async () => {
        try {
          for await (const frame of provider.open(value, { signal: controller.signal })) {
            if (controller.signal.aborted) break;
            if (frame?.ok) {
              publish(value, { status: "ready", value: frame.value });
              return;
            }
            publish(value, { status: "failed", failure: frame?.error });
            return;
          }
          if (!controller.signal.aborted) publish(value, { status: "none" });
        } catch (error) {
          if (!controller.signal.aborted) publish(value, { status: "failed", failure: error });
        }
      })();
      return () => {
        controller.abort();
        const latest = states.get(value);
        if (latest?.controller === controller) {
          states.set(value, {
            snapshot: Object.freeze({ address: value, status: "none" }),
            listeners: latest.listeners,
            controller: undefined,
          });
        }
      };
    },
    pin(address, signal) {
      const dispose = this.watch(address);
      if (signal?.aborted) {
        dispose();
        return () => {};
      }
      signal?.addEventListener?.("abort", dispose, { once: true });
      return () => {
        signal?.removeEventListener?.("abort", dispose);
        dispose();
      };
    },
    list() {
      return Array.from(providers.keys()).sort();
    },
  };
}

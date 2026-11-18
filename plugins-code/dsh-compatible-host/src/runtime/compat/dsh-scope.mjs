export class NamedEntries {
  entriesMap = new Map();

  insert(name, value) {
    if (this.entriesMap.has(name)) throw new Error(`duplicate entry ${name}`);
    this.entriesMap.set(name, value);
    return () => this.entriesMap.delete(name);
  }

  get(name) {
    return this.entriesMap.get(name);
  }

  values() {
    return this.entriesMap.values();
  }

  isEmpty() {
    return this.entriesMap.size === 0;
  }
}

export class AnonymousEntries {
  valuesList = [];

  append(value) {
    this.valuesList.push(value);
    return () => {
      const index = this.valuesList.indexOf(value);
      if (index >= 0) this.valuesList.splice(index, 1);
    };
  }

  values() {
    return this.valuesList.values();
  }

  isEmpty() {
    return this.valuesList.length === 0;
  }
}

export class ScopedLayers {
  constructor(createLayer, onChange) {
    this.createLayer = createLayer;
    this.onChange = onChange;
    this.global = createLayer(undefined);
    this.layers = new Map();
  }

  effect(_ctx, operation, options = {}) {
    const layer = this.global;
    const dispose = operation(layer);
    this.onChange?.();
    return typeof dispose === "function" ? () => {
      dispose();
      this.onChange?.();
    } : () => this.onChange?.();
  }

  chainLayers(scope) {
    return scope && this.layers.has(scope) ? [this.layers.get(scope)] : [];
  }

  merge(_scope, read) {
    const result = new Map();
    for (const entry of read(this.global)) result.set(entry[0], entry[1]);
    return result;
  }
}

export function scopeTarget() {
  return undefined;
}

export function scopeOf(context) {
  return context?.scope;
}

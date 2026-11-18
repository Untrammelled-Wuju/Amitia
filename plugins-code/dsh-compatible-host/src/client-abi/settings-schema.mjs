function cloneContainer(container, key) {
  if (Array.isArray(container)) return [...container];
  if (container && typeof container === "object") return { ...container };
  return /^\d+$/.test(key) ? [] : {};
}

function cloneSpine(root, path) {
  const result = { ...root };
  let target = result;
  for (let index = 0; index < path.length - 1; index += 1) {
    const key = path[index];
    const child = cloneContainer(Array.isArray(target) ? target[Number(key)] : target[key], path[index + 1]);
    if (Array.isArray(target)) target[Number(key)] = child;
    else target[key] = child;
    target = child;
  }
  return { result, parent: target, leaf: path.at(-1) };
}

export function createDshSettingsSchema() {
  return {
    rehydrate(serialized) {
      const value = serialized && typeof serialized === "object" ? serialized : {};
      const validate = (input) => input;
      Object.assign(validate, value);
      return validate;
    },
    validate(schema, draft) {
      try {
        if (typeof schema === "function") schema(draft);
        return undefined;
      } catch (error) {
        return error instanceof Error ? error.message : String(error);
      }
    },
    nodeAtPath(root, path) {
      let node = root;
      for (const key of path) {
        if (node === undefined) return undefined;
        if (node.type === "object") node = node.dict?.[key];
        else if (node.type === "dict" || node.type === "array") node = node.inner;
        else return undefined;
      }
      return node;
    },
    getPath(value, path) {
      let current = value;
      for (const key of path) {
        if (Array.isArray(current)) {
          current = current[Number(key)];
          continue;
        }
        if (!current || typeof current !== "object") return undefined;
        current = current[key];
      }
      return current;
    },
    hasPath(value, path) {
      if (path.length === 0) return value !== undefined;
      const parent = this.getPath(value, path.slice(0, -1));
      const key = path.at(-1);
      if (Array.isArray(parent)) return Number(key) < parent.length;
      return Boolean(parent && typeof parent === "object" && key in parent);
    },
    setPath(root, path, value) {
      if (path.length === 0) throw new Error("DSH settings setPath requires a non-empty path");
      const { result, parent, leaf } = cloneSpine(root, path);
      if (Array.isArray(parent)) parent[Number(leaf)] = value;
      else parent[leaf] = value;
      return result;
    },
    deletePath(root, path) {
      if (path.length === 0) throw new Error("DSH settings deletePath requires a non-empty path");
      if (!this.hasPath(root, path)) return root;
      const { result, parent, leaf } = cloneSpine(root, path);
      if (Array.isArray(parent)) parent.splice(Number(leaf), 1);
      else Reflect.deleteProperty(parent, leaf);
      return result;
    },
  };
}

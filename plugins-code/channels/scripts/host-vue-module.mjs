import { pathToFileURL } from "node:url"

const virtualModuleId = "\0amitia-host-vue"

function exportableName(name) {
  return /^[A-Za-z_$][A-Za-z0-9_$]*$/.test(name)
}

export function hostVueModulePlugin(vueEntry) {
  return {
    name: "amitia-host-vue-module",
    enforce: "pre",
    resolveId(source) {
      return source === "vue" ? virtualModuleId : null
    },
    async load(id) {
      if (id !== virtualModuleId) return null
      const runtime = await import(`${pathToFileURL(vueEntry).href}?amitia-host-vue=${Date.now()}`)
      const names = Object.keys(runtime).filter(exportableName).sort()
      return [
        "const host = globalThis.__AMITIA_VUE__",
        'if (!host) throw new Error("Amitia host Vue runtime is unavailable")',
        ...names.map((name) => `export const ${name} = host[${JSON.stringify(name)}]`),
      ].join("\n")
    },
  }
}

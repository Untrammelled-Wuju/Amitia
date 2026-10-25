export function clientCssPlugin(providerId) {
  const scope = `.ui-provider-module[data-ui-provider-id="${providerId}"]`
  const rootOnly = new Set(["html", "body", "#app"])
  return {
    postcssPlugin: "amitia-client-css-scope",
    Once(root) {
      root.walkRules((rule) => {
        const parent = rule.parent
        if (parent?.type === "atrule" && /keyframes$/i.test(parent.name)) return
        rule.selectors = rule.selectors.map((selector) => {
          const trimmed = selector.trim()
          if (trimmed === ":root" || rootOnly.has(trimmed)) return scope
          if (trimmed.startsWith(":root")) return scope + trimmed.slice(":root".length)
          return `${scope} ${trimmed}`
        })
      })
    },
  }
}

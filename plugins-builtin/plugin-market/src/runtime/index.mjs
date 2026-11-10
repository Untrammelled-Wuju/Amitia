const DEFAULT_CATALOG_URL = process.env.AMITIA_PLUGIN_MARKET_CATALOG_URL || "https://amitia.untrammelled.top/api/v1/market/catalog";
const CATALOG_TIMEOUT_MS = 10000;

function text(value) {
  return value == null ? "" : String(value).trim();
}

function record(value) {
  return value && typeof value === "object" && !Array.isArray(value) ? value : {};
}

function list(value) {
  return Array.isArray(value) ? value : [];
}

function uniqueOperationKey(prefix) {
  const suffix = typeof globalThis.crypto?.randomUUID === "function"
    ? globalThis.crypto.randomUUID()
    : `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`;
  return `${prefix}:${suffix}`;
}

function platformName(value) {
  const normalized = text(value).toLowerCase();
  if (["win", "win32", "win64", "windows"].includes(normalized)) return "windows";
  if (["mac", "macos", "osx", "darwin"].includes(normalized) || normalized.startsWith("darwin-")) return "macos";
  if (normalized === "linux" || normalized.startsWith("linux-")) return "linux";
  if (normalized === "android" || normalized.startsWith("android-")) return "android";
  if (normalized === "ios" || normalized.startsWith("ios-")) return "ios";
  if (normalized === "web") return "web";
  if (normalized === "all" || normalized === "*") return "all";
  return "";
}

function platformLabel(value) {
  return {
    all: "全部平台",
    windows: "Windows",
    macos: "macOS",
    linux: "Linux",
    android: "Android",
    ios: "iOS",
    web: "Web",
  }[platformName(value)] || text(value);
}

function normalizedPlatforms(values) {
  const result = [];
  const seen = new Set();
  for (const value of list(values)) {
    const normalized = platformName(value);
    if (!normalized || seen.has(normalized)) continue;
    seen.add(normalized);
    result.push(normalized);
  }
  return result;
}

function supportsPlatform(values, current) {
  const platforms = normalizedPlatforms(values);
  if (platforms.length === 0) return true;
  const target = platformName(current);
  return platforms.includes("all") || platforms.includes(target);
}

function platformSummary(values) {
  const platforms = normalizedPlatforms(values);
  if (platforms.length === 0 || platforms.includes("all")) return "支持全部平台";
  return `支持 ${platforms.map(platformLabel).join("、")}`;
}

function parseVersion(value) {
  const match = text(value).match(/^(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z.-]+))?/);
  if (!match) return null;
  return {
    parts: [Number(match[1]), Number(match[2]), Number(match[3])],
    prerelease: match[4] || "",
  };
}

function compareVersions(left, right) {
  const a = parseVersion(left);
  const b = parseVersion(right);
  if (!a || !b) return 0;
  for (let index = 0; index < 3; index += 1) {
    if (a.parts[index] !== b.parts[index]) return a.parts[index] > b.parts[index] ? 1 : -1;
  }
  if (a.prerelease === b.prerelease) return 0;
  if (!a.prerelease) return 1;
  if (!b.prerelease) return -1;
  return a.prerelease.localeCompare(b.prerelease);
}

function byteSize(value) {
  const size = Math.max(0, Number(value) || 0);
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  return `${(size / (1024 * 1024)).toFixed(1)} MB`;
}

function listText(value) {
  return list(value).map(text).filter(Boolean).join("、");
}

function issueText(value) {
  return list(value).map((issue) => {
    const item = record(issue);
    const message = text(item.message);
    const code = text(item.code);
    if (message && code) return `${message} (${code})`;
    return message || code;
  }).filter(Boolean).join("；");
}

async function installedInventory(host) {
  const response = record(await host.call("host.plugin.list", {}));
  return new Map(list(response.plugins).map((item) => {
    const plugin = record(item);
    return [text(plugin.extensionId), plugin];
  }).filter(([extensionId]) => extensionId));
}

async function readCatalog(host, input) {
  const base = text(input.catalogUrl) || DEFAULT_CATALOG_URL;
  const url = new URL(base);
  const query = text(input.q);
  const category = text(input.category);
  const platform = platformName(input.platform);
  if (query) url.searchParams.set("q", query);
  if (category) url.searchParams.set("category", category);
  if (platform && platform !== "all") url.searchParams.set("platform", platform);
  const response = record(await requestCatalog(host, url));
  const status = Number(response.status);
  const body = record(response.body);
  if (status < 200 || status >= 300) {
    throw new Error(`插件目录请求失败: upstream_error HTTP ${status} ${JSON.stringify(body)}`);
  }
  if (![0, 200].includes(Number(body.code))) {
    throw new Error(`插件目录返回错误: ${JSON.stringify(body)}`);
  }
  const data = record(body.data);
  if (text(data.schemaVersion) !== "amitia.plugin.catalog/v1") {
    throw new Error(`不支持的插件目录协议: ${text(data.schemaVersion)}`);
  }
  return {
    data,
    catalogUrl: url.toString(),
    platform,
  };
}

async function requestCatalog(host, url) {
  try {
    return await host.call("host.network.request", {
      url: url.toString(),
      method: "GET",
      timeoutMs: CATALOG_TIMEOUT_MS,
      failOnServerError: true,
    });
  } catch (error) {
    const message = error && error.message ? String(error.message) : String(error);
    throw new Error(`插件目录请求失败: ${message}`);
  }
}

async function catalog(host, input) {
  const { data, catalogUrl, platform } = await readCatalog(host, input);
  const metadata = record(data.metadata);
  const categories = list(metadata.categories).map(record);
  const categoryNames = new Map(categories.map((category) => [text(category.slug), text(category.name)]));
  const installed = await installedInventory(host);
  const currentPlatform = platform || platformName(process.platform);
  const listings = list(data.entries).map((entryValue) => {
    const entry = record(entryValue);
    const extensionId = text(entry.extensionId);
    const latestVersion = text(entry.version);
    const installedPlugin = record(installed.get(extensionId));
    const installedVersion = text(installedPlugin.version);
    const current = installed.get(extensionId) !== undefined;
    const compatibility = record(entry.compatibility);
    const supportedPlatforms = list(compatibility.platforms).map(text).filter(Boolean);
    const platformSupported = supportsPlatform(supportedPlatforms, currentPlatform);
    const categoriesValue = list(entry.categories).map(text).filter(Boolean);
    const categoryId = categoriesValue[0] || "";
    const categoryName = categoryNames.get(categoryId) || "";
    const publisher = record(entry.publisher);
    const signature = record(entry.signature);
    const packageInfo = record(entry.package);
    const displayName = text(record(entry.name).default);
    const description = text(record(entry.description).default);
    const summary = text(entry.summary);
    const packageUri = text(packageInfo.uri);
    const sha256 = text(packageInfo.sha256);
    const fileName = text(packageInfo.fileName);
    const size = Number(packageInfo.sizeBytes) || 0;
    const publisherId = text(publisher.id);
    const publisherName = text(publisher.displayName) || publisherId;
    const trustLevel = text(entry.trustLevel);
    let action = "install";
    let actionLabel = "安装";
    let actionDisabled = !platformSupported;
    let updateAvailable = false;
    if (!platformSupported) {
      action = "unsupported";
      actionLabel = "当前端不支持";
    } else if (current) {
      if (compareVersions(latestVersion, installedVersion) > 0) {
        action = "update";
        actionLabel = "更新";
        updateAvailable = true;
      } else {
        action = "current";
        actionLabel = "已安装";
        actionDisabled = true;
      }
    }
    const detailItems = [
      { label: "插件 ID", value: extensionId },
      { label: "最新版本", value: updateAvailable ? `${installedVersion} → ${latestVersion}` : latestVersion },
      { label: "分类", value: categoryName },
      { label: "发布者", value: publisherName },
      { label: "信任级别", value: trustLevel },
      { label: "签名状态", value: text(signature.status) },
      { label: "包大小", value: byteSize(size) },
      { label: "发布时间", value: text(entry.publishedAt) },
    ];
    const platformItems = [
      { label: "当前客户端", value: currentPlatform },
      { label: "支持平台", value: platformSummary(supportedPlatforms) },
    ];
    return {
      extensionId,
      displayName,
      summary,
      description,
      publisher: publisherId,
      trustLevel,
      categoryId,
      categoryName,
      latestVersion,
      installedVersion,
      packageUri: platformSupported ? packageUri : "",
      sha256,
      fileName,
      size,
      signatures: text(signature.status),
      supportedPlatforms,
      currentPlatform,
      platformSupported,
      platformSummary: platformSummary(supportedPlatforms),
      installed: current,
      updateAvailable,
      action,
      actionLabel,
      statusText: `${actionLabel} · ${platformSummary(supportedPlatforms)}`,
      actionDisabled,
      installable: platformSupported && !actionDisabled,
      releaseNotes: text(entry.releaseNotes),
      detailItems,
      platformItems,
    };
  });
  return {
    schemaVersion: text(data.schemaVersion),
    source: catalogUrl,
    currentPlatform,
    categories,
    listings,
    count: listings.length,
  };
}

async function prepare(host, input) {
  const extensionId = text(input.extensionId);
  const version = text(input.version);
  const packageUri = text(input.packageUri);
  const sha256 = text(input.sha256);
  if (!extensionId || !version || !packageUri || !sha256) {
    throw new Error("extensionId、version、packageUri 和 sha256 为必填项");
  }
  const source = {
    uri: packageUri,
    catalogId: "plugin-market",
    sourceType: "market",
    publisher: text(input.publisher),
    trustLevel: text(input.trustLevel),
    entryId: text(input.entryId),
    ...record(input.source),
  };
  const preview = record(await host.call("host.plugin.preview", {
    extensionId,
    version,
    packageUri,
    sha256,
    fileName: text(input.fileName),
    source,
  }));
  const installed = await installedInventory(host);
  const installedPlugin = record(installed.get(extensionId));
  const currentVersion = text(installedPlugin.version);
  const operationLabel = currentVersion ? "确认更新" : "确认安装";
  const requiredPermissions = list(preview.requiredPermissions).map(record);
  const confirmations = list(preview.requiredConfirmations).map(text).filter(Boolean);
  const risks = list(preview.riskFlags).map(text).filter(Boolean);
  let confirmationsText = confirmations.join("、");
  if (confirmationsText && risks.length) confirmationsText += `；风险标记：${risks.join("、")}`;
  else if (risks.length) confirmationsText = `风险标记：${risks.join("、")}`;
  return {
    ...preview,
    currentVersion,
    operationLabel,
    summary: [
      { label: "插件", value: text(preview.extensionId) || extensionId },
      { label: "版本", value: text(preview.version) || version },
      { label: "当前版本", value: currentVersion },
      { label: "发布者", value: text(preview.publisher) },
      { label: "签名", value: text(preview.signatureStatus) },
      { label: "包信任", value: text(preview.trustDecision) },
      { label: "安全审查", value: preview.securityPassed === true ? "通过" : "未通过" },
    ],
    confirmationsText,
    issuesText: issueText(preview.issues),
    permissions: requiredPermissions,
  };
}

async function install(host, input) {
  if (input.confirmed !== true) throw new Error("安装前必须获得用户明确确认");
  const extensionId = text(input.extensionId);
  const version = text(input.version);
  const sessionId = text(input.sessionId);
  if (!extensionId || !version || !sessionId) throw new Error("sessionId、extensionId 和 version 为必填项");
  const confirmations = { ...record(input.confirmations) };
  for (const item of list(input.requiredConfirmations).map(text).filter(Boolean)) confirmations[item] = true;
  return host.call("host.plugin.install", {
    extensionId,
    version,
    previewSessionId: sessionId,
    confirmations,
    idempotencyKey: text(input.idempotencyKey) || uniqueOperationKey(`plugin-market:install:${extensionId}:${version}`),
  });
}

async function uninstall(host, input) {
  if (input.confirmed !== true) throw new Error("卸载前必须获得用户明确确认");
  const extensionId = text(input.extensionId);
  if (!extensionId) throw new Error("extensionId 为必填项");
  return host.call("host.plugin.uninstall", {
    extensionId,
    idempotencyKey: text(input.idempotencyKey) || uniqueOperationKey(`plugin-market:uninstall:${extensionId}`),
  });
}

async function command(host, inputValue) {
  const input = record(inputValue);
  switch (text(input.action)) {
    case "catalog":
      return catalog(host, input);
    case "prepare":
      return prepare(host, input);
    case "install":
      return install(host, input);
    case "uninstall":
      return uninstall(host, input);
    default:
      throw new Error(`未知插件市场操作: ${text(input.action)}`);
  }
}

const pluginMarketExtension = {
  async activate(context) {
    context.handlers.bindTool("command", (input) => command(context.host, input));
    context.log.info("plugin market activated");
  },
};

if (typeof module !== "undefined" && module.exports) {
  module.exports = pluginMarketExtension;
}

if (typeof globalThis.defineExtension === "function") {
  globalThis.defineExtension(pluginMarketExtension);
}

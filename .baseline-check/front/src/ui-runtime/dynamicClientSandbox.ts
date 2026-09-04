export interface DynamicClientSandboxCode {
  html?: string;
  css?: string;
  script?: string;
  minHeight?: number;
  maxHeight?: number;
}

export const DYNAMIC_CLIENT_HTML_LIMIT = 65_536;
export const DYNAMIC_CLIENT_CSS_LIMIT = 65_536;
export const DYNAMIC_CLIENT_SCRIPT_LIMIT = 131_072;

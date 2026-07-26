export type OutputFormat = "human" | "json" | "sarif";

export type LogLevel = "debug" | "info" | "warn" | "error" | "quiet";

export interface CliLogger {
  debug(message: string, fields?: Record<string, unknown>): void;
  info(message: string, fields?: Record<string, unknown>): void;
  warn(message: string, fields?: Record<string, unknown>): void;
  error(message: string, fields?: Record<string, unknown>): void;
  setLevel(level: LogLevel): void;
  getLevel(): LogLevel;
}

export interface CliContext {
  readonly cwd: string;
  readonly args: string[];
  readonly logger: CliLogger;
  readonly format: OutputFormat;
  readonly verbose: boolean;
  readonly quiet: boolean;
}

export interface CliCommand {
  readonly name: string;
  readonly description: string;
  readonly usage?: string;
  readonly aliases?: string[];
  readonly subcommands?: CliCommand[];
  readonly options?: CliOption[];
  readonly run(ctx: CliContext, args: string[]): Promise<CliCommandResult> | CliCommandResult;
}

export interface CliOption {
  readonly name: string;
  readonly shortName?: string;
  readonly description: string;
  readonly takesValue?: boolean;
  readonly defaultValue?: string;
  readonly required?: boolean;
  readonly choices?: string[];
}

export interface CliCommandResult {
  readonly exitCode: number;
  readonly message?: string;
  readonly data?: unknown;
  readonly reports?: CliReport[];
}

export interface CliReport {
  readonly ruleId: string;
  readonly severity: "info" | "warning" | "error";
  readonly message: string;
  readonly file?: string;
  readonly line?: number;
  readonly column?: number;
  readonly details?: Record<string, unknown>;
}

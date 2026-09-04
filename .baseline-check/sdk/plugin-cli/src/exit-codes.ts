export type ExitCode =
  | 0
  | 1
  | 2
  | 3
  | 4
  | 5
  | 6
  | 7;

export const EXIT_CODES = {
  SUCCESS: 0 as ExitCode,
  VALIDATION_OR_BUILD_FAILURE: 1 as ExitCode,
  CONFIGURATION_ERROR: 2 as ExitCode,
  ENVIRONMENT_ERROR: 3 as ExitCode,
  SIGNATURE_OR_TRUST_ERROR: 4 as ExitCode,
  TEST_FAILURE: 5 as ExitCode,
  HOST_CONNECTION_ERROR: 6 as ExitCode,
  INTERNAL_CLI_ERROR: 7 as ExitCode,
} as const;

export function describeExitCode(code: ExitCode): string {
  switch (code) {
    case 0:
      return "success";
    case 1:
      return "validation/build failure";
    case 2:
      return "configuration error";
    case 3:
      return "environment error";
    case 4:
      return "signature/trust error";
    case 5:
      return "test failure";
    case 6:
      return "host connection error";
    case 7:
      return "internal CLI error";
    default:
      return "unknown";
  }
}

let authToken: string | null = null;

export function getAuthToken(): string | null {
  return authToken;
}

export function setAuthToken(token: string | null): void {
  authToken = token;
}

export function hasAuthToken(): boolean {
  return authToken !== null && authToken.length > 0;
}

export function projectCitationSnapshot<T>(
  persisted: T[] | undefined,
  finalText: { citationSnapshot?: T[] } | null | undefined,
): T[] | undefined {
  return Array.isArray(finalText?.citationSnapshot)
    ? finalText.citationSnapshot
    : persisted;
}

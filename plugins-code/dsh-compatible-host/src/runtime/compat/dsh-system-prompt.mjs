export function renderPrompt(assembly) {
  return (assembly.sections ?? []).map((section) => section.text).join("\n\n");
}

export function renderContextSnapshot(assembly) {
  return joinContextSections(renderContextSections(assembly));
}

export function renderContextSections(assembly) {
  return (assembly.contexts ?? []).map((context) => ({ name: context.name, text: context.text }));
}

export function joinContextSections(sections) {
  return sections.map((section) => section.text).join("\n\n");
}

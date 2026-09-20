/// Maps markdown fence tags to canonical TextMate grammar ids.
///
/// Internally it maintains two structures:
///  - A set of canonical grammar ids that correspond 1:1 to a bundled
///    `<id>.json.zst` TextMate grammar file (e.g.
///    `assets/grammars/python.json.zst`).
///  - A map of fence-tag aliases (e.g. `py`, `js`, `sh`) to their canonical id.
///
/// Use [resolve] to turn any fence tag into the grammar id expected by the
/// highlighter. Throws an [ArgumentError] if the tag is blank or unrecognised.
class GrammarRegistry {
  const GrammarRegistry._();

  /// Canonical grammar ids. Each one must have a matching
  /// `assets/grammars/<id>.json.zst` file bundled with the app.
  static const Set<String> _languages = {
    // Web
    'javascript', 'typescript', 'jsx', 'tsx', 'vue', 'svelte', 'wasm',
    'json', 'json5', 'jsonc', 'jsonl',
    'html', 'css', 'scss',
    // General purpose
    'python', 'java', 'c', 'cpp', 'csharp', 'go', 'rust', 'ruby', 'php',
    'kotlin', 'swift', 'dart', 'scala', 'objective-c', 'objective-cpp',
    'd', 'pascal', 'v', 'zig', 'mojo', 'odin', 'llvm', 'riscv',
    // Scripting / shell
    'shellscript', 'shellsession', 'powershell', 'bat', 'perl', 'lua', 'awk', 'cmake', 'just',
    // Infra / config
    'docker', 'yaml', 'toml', 'ini', 'nginx', 'make', 'jinja',
    // Data / query
    'sql', 'graphql', 'xml', 'csv',
    // Docs / misc
    'markdown', 'diff', 'http', 'regexp', 'abap', 'ada', 'matlab', 'verilog', 'vhdl', 'tex',
    // Functional
    'haskell', 'elixir', 'clojure', 'erlang', 'r', 'julia', 'fsharp', 'ocaml',
  };

  /// Fence-tag aliases that don't match a canonical grammar id.
  /// Only include entries where alias != canonical id.
  static const Map<String, String> _aliases = {
    // JavaScript / TypeScript
    'js': 'javascript',
    'mjs': 'javascript',
    'cjs': 'javascript',
    'ts': 'typescript',
    // Python
    'py': 'python',
    'py3': 'python',
    'python3': 'python',
    // Ruby
    'rb': 'ruby',
    // C#
    'cs': 'csharp',
    'c#': 'csharp',
    // C++
    'c++': 'cpp',
    'cc': 'cpp',
    'cxx': 'cpp',
    'h++': 'cpp',
    // Objective-C
    'objc': 'objective-c',
    'objectivec': 'objective-c',
    'objcpp': 'objective-cpp',
    'objc++': 'objective-cpp',
    'objective-c++': 'objective-cpp',
    // Kotlin
    'kt': 'kotlin',
    'kts': 'kotlin',
    // Shell
    'sh': 'shellscript',
    'bash': 'shellscript',
    'zsh': 'shellscript',
    'shell': 'shellscript',
    'console': 'shellsession',
    // PowerShell
    'ps1': 'powershell',
    'ps': 'powershell',
    'pwsh': 'powershell',
    // YAML
    'yml': 'yaml',
    // Docker
    'dockerfile': 'docker',
    // Make
    'makefile': 'make',
    'mk': 'make',
    // Markdown
    'md': 'markdown',
    // LaTeX
    'latex': 'tex',
    // Diff / patch
    'patch': 'diff',
    // SQL dialects (approximate — fall back to generic sql grammar)
    'postgres': 'sql',
    'postgresql': 'sql',
    'mysql': 'sql',
    'plsql': 'sql',
    'sqlite': 'sql',
    // GraphQL
    'gql': 'graphql',
    // HTML
    'htm': 'html',
    'html5': 'html',
    // INI
    'properties': 'ini',
    'dosini': 'ini',
    // Go / Rust
    'golang': 'go',
    'rs': 'rust',
    // Perl
    'pl': 'perl',
    // Functional
    'hs': 'haskell',
    'ex': 'elixir',
    'exs': 'elixir',
    'erl': 'erlang',
    'clj': 'clojure',
    'cljs': 'clojure',
    'jl': 'julia',
    'f#': 'fsharp',
    'fs': 'fsharp',
    'ml': 'ocaml',
    // Regexp
    'regex': 'regexp',
    // Others
    'jinja2': 'jinja',
    'j2': 'jinja',
    'justfile': 'just',
    'delphi': 'pascal',
    'vlang': 'v',
    'vlog': 'verilog',
    'sv': 'verilog',
  };

  /// Every tag that [resolve] accepts — canonical grammar ids plus all
  /// registered aliases — sorted alphabetically.
  static List<String> get supportedLanguages => [..._languages, ..._aliases.keys].toList()..sort();

  /// Resolves a markdown fence tag (e.g. `py`, `JS`, `objective-c`) to a
  /// canonical grammar id.
  ///
  /// Throws an [ArgumentError] if [tag] is blank or has no registered grammar.
  static String resolve(String tag) {
    final normalized = tag.trim().toLowerCase();
    if (normalized.isEmpty) throw ArgumentError.value(tag, 'tag', 'Tag must not be blank');

    // If tag is already a canonical grammar id, return it.
    if (_languages.contains(normalized)) {
      return normalized;
    }

    // If tag is an alias for a canonical grammar id, return it.
    final alias = _aliases[normalized];
    if (alias != null) {
      return alias;
    } else {
      throw ArgumentError.value(
        tag,
        'tag',
        'No grammar registered for "$normalized"',
      );
    }
  }
}

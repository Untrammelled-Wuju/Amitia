//! Embeds the zstd-compressed TextMate grammar files from `../assets/grammars`
//! into the binary. For each grammar we decompress it here, at build time, to
//! extract its `scopeName` so the registry can resolve cross-grammar includes
//! (e.g. `source.js` inside `html.json.zst`) without decompressing/parsing
//! every file up front. Only the compressed bytes are embedded; the runtime
//! decompresses a grammar the first time it is used.

use std::env;
use std::fs;
use std::io::Read;
use std::path::PathBuf;

fn main() {
    let manifest_dir = PathBuf::from(env::var("CARGO_MANIFEST_DIR").unwrap());
    let grammars_dir = manifest_dir.join("..").join("assets").join("grammars");
    let out_path = PathBuf::from(env::var("OUT_DIR").unwrap()).join("grammar_data.rs");

    println!("cargo:rerun-if-changed={}", grammars_dir.display());

    let mut entries: Vec<(String, String, PathBuf)> = Vec::new();
    for entry in fs::read_dir(&grammars_dir).expect("assets/grammars directory not found") {
        let path = entry.unwrap().path();
        if path.extension().and_then(|e| e.to_str()) != Some("zst") {
            continue;
        }
        // `python.json.zst` -> id `python`.
        let id = path
            .file_name()
            .unwrap()
            .to_str()
            .unwrap()
            .trim_end_matches(".json.zst")
            .to_string();
        let compressed = fs::read(&path).unwrap();
        let mut decoder = ruzstd::decoding::StreamingDecoder::new(compressed.as_slice())
            .unwrap_or_else(|e| panic!("invalid zstd frame in {id}: {e}"));
        let mut contents = String::new();
        decoder
            .read_to_string(&mut contents)
            .unwrap_or_else(|e| panic!("failed to decompress {id}: {e}"));
        let json: serde_json::Value =
            serde_json::from_str(&contents).unwrap_or_else(|e| panic!("invalid grammar {id}: {e}"));
        let scope_name = json
            .get("scopeName")
            .and_then(|v| v.as_str())
            .unwrap_or_else(|| panic!("grammar {id} has no scopeName"))
            .to_string();
        entries.push((id, scope_name, path));
    }
    assert!(!entries.is_empty(), "no .json.zst grammars found in assets/grammars");
    entries.sort();

    let mut code = String::from(
        "/// (language id, root scope name, zstd-compressed grammar JSON) for\n\
         /// every bundled grammar.\n\
         pub static RAW_GRAMMARS: &[(&str, &str, &[u8])] = &[\n",
    );
    for (id, scope, path) in &entries {
        let path = path.canonicalize().unwrap();
        let path = path.to_str().unwrap().replace('\\', "/");
        code.push_str(&format!(
            "    ({id:?}, {scope:?}, include_bytes!({path:?})),\n"
        ));
    }
    code.push_str("];\n");

    fs::write(&out_path, code).unwrap();
}

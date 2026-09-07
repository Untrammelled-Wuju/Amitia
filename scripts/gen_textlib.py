import os, re, sys

ROOT = r"D:\桌面\跟进项目\U-Ai\Amitia_提示词系统纯接入Codex任务包_v5\prompt_texts"
OUT = r"D:\桌面\跟进项目\U-Ai\backend\internal\prompt\textlib"

DISALLOWED = [
    'groupchat', 'group_chat', 'group-chat', 'sticker', 'localmodel',
    'local_model', 'local-model', 'desktopagent', 'desktop_agent',
    'desktop-agent', 'openforu', 'open_for_u', 'plugin', 'diary', 'extension'
]

def is_allowed(name):
    lower = name.lower()
    for d in DISALLOWED:
        if d in lower:
            return False
    return True

def extract_code(filepath):
    with open(filepath, 'r', encoding='utf-8') as f:
        text = f.read()
    m = re.search(r'`````\w*\r?\n(.*?)`````', text, re.DOTALL)
    if m:
        return m.group(1).rstrip()
    print(f"WARNING: no code block in {filepath}", file=sys.stderr)
    return ""

def to_const_name(filename):
    base = os.path.splitext(filename)[0]
    if base.endswith('.en'):
        base = base[:-3] + '_En'
    base = base.replace('__', '_')
    base = base.replace('.ts', '').replace('.kt', '').replace('.md', '')
    base = re.sub(r'^src_main_', '', base)
    base = re.sub(r'^core_.*?_com_lianyu_ai_', '', base)
    base = re.sub(r'^feature_.*?_com_lianyu_ai_', '', base)
    base = re.sub(r'^docs_', '', base)
    parts = base.split('_')
    result = []
    for p in parts:
        if p:
            result.append(p[0].upper() + p[1:])
    name = 'Raw' + ''.join(result)
    name = re.sub(r'[^a-zA-Z0-9_]', '', name)
    if name and name[0].isdigit():
        name = 'N' + name
    return name or 'RawUnknown'

def go_string_lit(s):
    if '`' not in s:
        return '`' + s + '`'
    parts = s.split('`')
    chunks = []
    for i, p in enumerate(parts):
        if p:
            chunks.append('`' + p + '`')
        if i < len(parts) - 1:
            chunks.append('"`"')
    return ' + '.join(chunks)

def process_dir(dirpath, srcset_name):
    consts = []
    for fname in sorted(os.listdir(dirpath)):
        if fname == '00_本目录原文提示词合集.md':
            continue
        if not is_allowed(fname):
            print(f"  SKIP (disallowed): {fname}")
            continue
        fpath = os.path.join(dirpath, fname)
        code = extract_code(fpath)
        if not code:
            continue
        name = to_const_name(fname)
        src = fname.replace('.md', '')
        consts.append((name, code, src))
        print(f"  OK: {name} <- {fname} ({len(code)} chars)")
    return consts

def write_go_file(path, srcset, consts):
    seen = {}
    lines = ['package textlib', '']
    for name, code, src in consts:
        unique = name
        if name in seen:
            seen[name] += 1
            unique = f"{name}_{seen[name]}"
        else:
            seen[name] = 0
        lines.append(f'// SourceName: {src}')
        lines.append(f'// SourceSet: {srcset}')
        lit = go_string_lit(code)
        lines.append(f'const {unique} = {lit}')
        lines.append('')
    content = '\n'.join(lines)
    with open(path, 'w', encoding='utf-8', newline='\n') as f:
        f.write(content)
    print(f"  Wrote {path} ({len(consts)} constants, {len(content)} bytes)")

def main():
    os.makedirs(OUT, exist_ok=True)

    print("=== Ackem ===")
    ackem_consts = process_dir(os.path.join(ROOT, 'ackem'), 'ackem')
    write_go_file(os.path.join(OUT, 'ackem_texts.go'), 'ackem', ackem_consts)

    print("=== LianYu ===")
    lianyu_consts = process_dir(os.path.join(ROOT, 'lianyu'), 'lianyu')
    write_go_file(os.path.join(OUT, 'lianyu_texts.go'), 'lianyu', lianyu_consts)

    print(f"\nDone: {len(ackem_consts)} ackem + {len(lianyu_consts)} lianyu constants")

if __name__ == '__main__':
    main()

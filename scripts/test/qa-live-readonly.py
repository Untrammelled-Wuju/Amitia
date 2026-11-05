import concurrent.futures
import json
import pathlib
import time
import urllib.error
import urllib.request

ROOT = pathlib.Path(__file__).resolve().parents[2]
OUT = ROOT / 'artifacts/qa-20261105'

def main():
    OUT.mkdir(parents=True, exist_ok=True)
    endpoint = json.loads((ROOT / 'AmitiaData/run/endpoint.json').read_text())
    base = f"http://127.0.0.1:{endpoint['port']}"
    token = (ROOT / 'AmitiaData/security/local-token').read_text().strip()
    def probe(path, authenticated=True):
        headers = {'X-Amitia-Local-Token': token} if authenticated else {}
        started = time.monotonic()
        request = urllib.request.Request(base + path, headers=headers)
        try:
            with urllib.request.urlopen(request, timeout=20) as response:
                status, body = response.status, response.read()
        except urllib.error.HTTPError as error:
            status, body = error.code, error.read()
        payload = json.loads(body)
        return {'path': path, 'authenticated': authenticated, 'status': status, 'code': payload.get('code'), 'seconds': round(time.monotonic()-started,3), 'error': payload if status >= 400 or payload.get('code') not in [None, 200] else None}
    paths = ['/api/health','/readyz','/api/characters','/api/reminders','/api/runtime/capabilities','/api/extension-center/view']
    results = [probe(path, auth) for path in paths for auth in [False, True]]
    with concurrent.futures.ThreadPoolExecutor(max_workers=8) as pool:
        burst = list(pool.map(lambda _: probe('/api/characters'), range(40)))
    report = {'sequential': results, 'burst': burst, 'assertions': {'authenticated_characters_read': all(x['status']==200 and x['code']==200 for x in burst), 'anonymous_characters_rejected': next(x for x in results if x['path']=='/api/characters' and not x['authenticated'])['code'] in [401,403]}}
    (OUT / 'live-readonly.json').write_text(json.dumps(report,ensure_ascii=False,indent=2))
    print(json.dumps(report['assertions'],ensure_ascii=False))
    for item in results:
        print(item['path'],item['authenticated'],item['status'])
    if not all(report['assertions'].values()):
        raise AssertionError(report['assertions'])

if __name__ == '__main__':
    main()

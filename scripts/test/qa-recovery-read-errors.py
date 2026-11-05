import json
import pathlib
import urllib.error
import urllib.request

ROOT = pathlib.Path(__file__).resolve().parents[2]


def main():
    endpoint = json.loads((ROOT / 'AmitiaData/run/endpoint.json').read_text())
    token = (ROOT / 'AmitiaData/security/local-token').read_text().strip()
    prefix = '/api/knowledge/vector-orphans/'
    missing = '00000000-0000-4000-8000-000000000001'
    cases = [(prefix + 'audits/' + missing + '/recovery-reviews?limit=' + value, 400, message)
             for value, message in [('abc', 'strconv.Atoi: parsing "abc": invalid syntax'),
                                    ('0', 'limit must be between 1 and 30: 0'),
                                    ('31', 'limit must be between 1 and 30: 31')]]
    cases.append((prefix + 'recovery-reviews/' + missing + '/plan', 404, 'record not found'))
    results = []
    for path, expected_status, message in cases:
        request = urllib.request.Request(endpoint['httpBaseURL'] + path, headers={'X-Amitia-Local-Token': token})
        try:
            response = urllib.request.urlopen(request, timeout=20)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            raw = response.read().decode()
            payload = json.loads(raw)
            results.append({'path': path, 'status': response.status, 'body': payload,
                            'passed': response.status == expected_status and payload['msg'] == message})
    output = ROOT / 'artifacts/qa-20261105-continuous/recovery-read-after.json'
    output.write_text(json.dumps(results, ensure_ascii=False, indent=2))
    assert all(result['passed'] for result in results), results
    print('4/4 passed')


if __name__ == '__main__':
    main()

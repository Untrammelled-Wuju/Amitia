import concurrent.futures
import copy
import json
import pathlib
import sys
import urllib.error
import urllib.parse
import urllib.request
import uuid

ROOT = pathlib.Path(__file__).resolve().parents[2]
OUT = ROOT / 'artifacts/qa-20261105-continuous'
STATE = OUT / 'state.json'
EVENTS = OUT / 'events.jsonl'


def call(path, body=None):
    endpoint = json.loads((ROOT / 'AmitiaData/run/endpoint.json').read_text())
    token = (ROOT / 'AmitiaData/security/local-token').read_text().strip()
    request = urllib.request.Request(endpoint['httpBaseURL'] + '/api/device-mesh/v1/business' + path,
                                    data=None if body is None else json.dumps(body).encode(),
                                    headers={'X-Amitia-Local-Token': token, 'Content-Type': 'application/json'})
    try:
        response = urllib.request.urlopen(request, timeout=30)
    except urllib.error.HTTPError as error:
        response = error
    with response:
        raw = response.read().decode()
        result = {'path': path, 'status': response.status, 'response': json.loads(raw)}
    with EVENTS.open('a') as file:
        file.write(json.dumps(result, ensure_ascii=False) + '\n')
    return result


def data(result):
    assert result['status'] == 200, result
    return result['response'].get('data', result['response'])


def listing(role):
    return data(call('/data?' + urllib.parse.urlencode({'kind': 'worldbook', 'characterId': role})))


def run(stage):
    OUT.mkdir(parents=True, exist_ok=True)
    if stage == 'setup':
        assert not STATE.exists() or json.loads(STATE.read_text()).get('cleaned'), 'existing QA state must be verified and cleaned first'
        roles = data(call('/roles'))['roles']
        assert roles, 'no available role'
        role = roles[0]['id']
        view = listing(role)
        marker = 'QA-CONTINUOUS-' + str(uuid.uuid4())
        body = {'requestId': str(uuid.uuid4()), 'characterId': role,
                'expectedExecutionScope': view['executionScope'], 'action': 'create',
                'rule': {'scope': 'role', 'matchType': 'keyword', 'matchPattern': marker,
                         'matchScope': 'full_context', 'injectContent': marker + '-initial',
                         'priority': 5, 'enabled': True}}
        state = {'role': role, 'marker': marker, 'create': body}
        STATE.write_text(json.dumps(state, ensure_ascii=False, indent=2))
        with concurrent.futures.ThreadPoolExecutor(max_workers=8) as pool:
            results = list(pool.map(lambda _: call('/worldbook/manage', body), range(8)))
        rules = [data(result)['rule'] for result in results]
        assert len({rule['id'] for rule in rules}) == 1, results
        assert all(rule['revision'] == 1 for rule in rules), results
        state['rule'] = rules[0]
        STATE.write_text(json.dumps(state, ensure_ascii=False, indent=2))
        changed = copy.deepcopy(body)
        changed['rule']['injectContent'] += '-conflicting-request'
        assert call('/worldbook/manage', changed)['status'] == 409
        assert sum(resource['id'] == rules[0]['id'] for resource in listing(role)['snapshot']['resources']) == 1
        updates = []
        for index in range(8):
            update = copy.deepcopy(body)
            update.update(requestId=str(uuid.uuid4()), action='update', expectedRevision=1)
            update['rule'] = dict(rules[0], injectContent=marker + '-winner-' + str(index))
            updates.append(update)
        with concurrent.futures.ThreadPoolExecutor(max_workers=8) as pool:
            results = list(pool.map(lambda value: call('/worldbook/manage', value), updates))
        assert sorted(result['status'] for result in results) == [200] + [409] * 7, results
        state['rule'] = data(next(result for result in results if result['status'] == 200))['rule']
        assert state['rule']['revision'] == 2, state
        STATE.write_text(json.dumps(state, ensure_ascii=False, indent=2))
        verify(state)
    elif stage in ['verify', 'cleanup']:
        state = json.loads(STATE.read_text())
        verify(state)
        if stage == 'cleanup':
            view = listing(state['role'])
            body = {'requestId': str(uuid.uuid4()), 'characterId': state['role'],
                    'expectedExecutionScope': view['executionScope'], 'action': 'delete',
                    'expectedRevision': state['rule']['revision'],
                    'rule': {'id': state['rule']['id'], 'scope': 'role'}}
            assert data(call('/worldbook/manage', body))['saved'] is True
            assert not any(resource['id'] == state['rule']['id'] for resource in listing(state['role'])['snapshot']['resources'])
            matched = data(call('/worldbook/match', {'requestId': str(uuid.uuid4()), 'characterId': state['role'],
                           'expectedExecutionScope': listing(state['role'])['executionScope'], 'text': state['marker']}))
            assert not any(match['ruleId'] == state['rule']['id'] for match in matched['matches']), matched
            state['cleaned'] = True
            STATE.write_text(json.dumps(state, ensure_ascii=False, indent=2))
    else:
        raise ValueError(stage)
    print(stage + ' passed')


def verify(state):
    view = listing(state['role'])
    rows = [resource for resource in view['snapshot']['resources'] if resource['id'] == state['rule']['id']]
    assert len(rows) == 1, rows
    assert rows[0]['revision'] == 2, rows
    assert rows[0]['body']['injectContent'] == state['rule']['injectContent'], rows
    result = data(call('/worldbook/match', {'requestId': str(uuid.uuid4()), 'characterId': state['role'],
                  'expectedExecutionScope': view['executionScope'], 'text': state['marker']}))
    assert any(match['ruleId'] == state['rule']['id'] and match['revision'] == 2 for match in result['matches']), result
    valid = dict(state['create']['rule'], matchPattern=state['marker'] + '-invalid-batch')
    invalid = dict(valid, matchType='regex', matchPattern='[')
    imported = call('/worldbook/import', {'requestId': str(uuid.uuid4()), 'characterId': state['role'],
                    'expectedExecutionScope': view['executionScope'], 'rules': [valid, invalid]})
    assert imported['status'] == 400, imported
    after = listing(state['role'])
    assert {resource['id']: resource['revision'] for resource in after['snapshot']['resources']} == {
        resource['id']: resource['revision'] for resource in view['snapshot']['resources']}, after
    for role in data(call('/roles'))['roles']:
        if role['id'] != state['role']:
            assert not any(resource['id'] == state['rule']['id'] for resource in listing(role['id'])['snapshot']['resources'])


if __name__ == '__main__':
    run(sys.argv[1])

"""Real-browser scroll regression against an isolated local test server.

Requires websocket-client. Set SCROLL_TEST_CHROME and SCROLL_TEST_LOGIN_FILE;
the latter contains local test credentials (username/password), never production.
Run with python; defaults to http://127.0.0.1:53010.
"""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import time
import urllib.request

import websocket


base = os.environ.get('SCROLL_TEST_URL', 'http://127.0.0.1:53010')
assert base.startswith(('http://127.0.0.1:', 'http://localhost:'))
credentials = json.loads(Path(os.environ['SCROLL_TEST_LOGIN_FILE']).read_text())
opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))

with tempfile.TemporaryDirectory(prefix='new-api-scroll-', dir=os.environ.get('SCROLL_TEST_TEMP'), ignore_cleanup_errors=True) as profile:
    process = subprocess.Popen([
        os.environ['SCROLL_TEST_CHROME'], '--headless=new', '--no-first-run',
        '--no-default-browser-check', '--disable-gpu', '--no-sandbox', '--remote-debugging-port=0',
        '--remote-allow-origins=*', '--user-data-dir=' + profile, 'about:blank',
    ], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    try:
        deadline = time.monotonic() + 30
        port_file = Path(profile) / 'DevToolsActivePort'
        while not port_file.exists():
            assert time.monotonic() < deadline, 'Browser did not start'
            time.sleep(0.1)
        port = port_file.read_text().splitlines()[0]
        targets = json.load(opener.open(f'http://127.0.0.1:{port}/json'))
        target = next(t for t in targets if t['type'] == 'page')
        ws = websocket.create_connection(target['webSocketDebuggerUrl'], timeout=30, suppress_origin=True, http_no_proxy=['127.0.0.1', 'localhost'])
        sequence = 0

        def call(method, params=None):
            global sequence
            sequence += 1
            ws.send(json.dumps({'id': sequence, 'method': method, 'params': params or {}}))
            while True:
                result = json.loads(ws.recv())
                if result.get('id') == sequence:
                    assert 'error' not in result, result
                    return result.get('result', {})

        def evaluate(expression):
            result = call('Runtime.evaluate', {'expression': expression, 'returnByValue': True, 'awaitPromise': True})
            assert 'exceptionDetails' not in result, result
            return result['result'].get('value')

        def wait_for(expression):
            deadline = time.monotonic() + 30
            while not evaluate(expression):
                assert time.monotonic() < deadline, expression
                time.sleep(0.1)

        call('Emulation.setDeviceMetricsOverride', {'width': 1280, 'height': 600, 'deviceScaleFactor': 1, 'mobile': False})
        call('Emulation.setEmulatedMedia', {'features': [{'name': 'prefers-reduced-motion', 'value': os.environ.get('SCROLL_TEST_MOTION', 'no-preference')}]})
        call('Page.navigate', {'url': base + '/sign-in'})
        wait_for("document.readyState === 'complete'")
        login = evaluate("fetch('/api/user/login',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(" + json.dumps({'username': credentials['username'], 'password': credentials['password']}) + ")}).then(r=>r.json()).then(r=>r.success)")
        assert login, 'Local test login failed'
        if os.environ.get('SCROLL_TEST_LANGUAGE'):
            evaluate('localStorage.setItem("i18nextLng",'+json.dumps(os.environ['SCROLL_TEST_LANGUAGE'])+')')
        for width, height in [(1280, 600), (390, 650)]:
            call('Emulation.setDeviceMetricsOverride', {'width': width, 'height': height, 'deviceScaleFactor': 1, 'mobile': width < 500})
            routes = os.environ.get('SCROLL_TEST_ROUTES', '/wallet,/department-quotas,/wallet-adjustments,/usage-analysis,/deepseek-bill-demo,/private-knowledge-base,/skill-showcase').split(',')
            for route in routes:
                call('Page.navigate', {'url': base + route})
                if os.environ.get('SCROLL_TEST_NOTE_ONLY'):
                    wait_for("(() => {const s=document.querySelector('[data-slot=sidebar-inset]');return !!s && /1 USD = 7\\.3 CNY|1 美元＝7\\.3 元人民币/.test(s.textContent)})()")
                    print(f'PASS {width}x{height} {route}: exchange rate 7.3 explanation shown')
                    continue
                wait_for("!!document.querySelector('[data-slot=sidebar-inset] h1') && document.readyState==='complete'")
                evaluate("Promise.all(document.getAnimations().filter(a=>a.effect?.getComputedTiming().iterations!==Infinity).map(a=>a.finished.catch(()=>{}))).then(()=>true)")
                if route == '/deepseek-bill-demo':
                    wait_for("!!document.querySelector('[data-slot=sidebar-inset] h2')")
                    if os.environ.get('SCROLL_TEST_SELF'):
                        assert evaluate("!document.querySelector('#bill-user') && !document.querySelector('input[name=fx]')"), 'Ordinary user sees admin controls'
                    else:
                        wait_for("!!document.querySelector('input[name=fx]')")
                        assert evaluate("Number(document.querySelector('input[name=fx]').value)>0"), 'FX editor is missing'
                    if os.environ.get('SCROLL_TEST_EXPECTED_BILL'):
                        expected=json.loads(Path(os.environ['SCROLL_TEST_EXPECTED_BILL']).read_text(encoding='utf-8'))
                        if not os.environ.get('SCROLL_TEST_SELF'):
                            wait_for("!!document.querySelector('#bill-user')")
                            evaluate("(() => {const s=document.querySelector('#bill-user');s.value="+json.dumps(str(expected['user_id']))+";s.dispatchEvent(new Event('change',{bubbles:true}));return true})()")
                        for detail in expected['details']:
                            selector='[title="'+detail['request_id']+'"]'
                            wait_for('!!document.querySelector('+json.dumps(selector)+')')
                            text=evaluate('document.querySelector('+json.dumps(selector)+').closest("tr").textContent')
                            assert detail['cny_text'] in text, (detail['request_id'],'CNY amount mismatch')
                            assert detail['usd_text'] in text, (detail['request_id'],'USD quota mismatch')
                            assert 'CNY/USD '+format(detail['fx'],'g') in text, (detail['request_id'],'FX snapshot mismatch')
                            assert 'v'+str(detail['version']) in text, (detail['request_id'],'version mismatch')
                        summary=evaluate("(() => {const h=[...document.querySelectorAll('h2')].find(h=>h.textContent.includes("+json.dumps(expected['username'])+"));return h?.nextElementSibling?.textContent ?? ''})()")
                        assert expected['total_cny_text'] in summary, 'CNY summary mismatch'
                        assert expected['input_m'] in summary and expected['output_m'] in summary, 'Token summary mismatch'
                        assert expected['quota_text'] in evaluate('document.body.textContent'), 'Actual quota summary mismatch'
                        print(f'PASS {width}x{height} bill: CNY, USD quota, tokens, FX 7/7.3 and versions match independently calculated expectations')
                # Force a long page even when the local snapshot has no rows.
                # The fixture is inside the real route root, not a replacement layout.
                result = evaluate("""(() => {
                  const outlet = document.querySelector('[data-slot=sidebar-inset]').firstElementChild;
                  const page = outlet.firstElementChild;
                  const root = page.tagName==='MAIN' && page.classList.contains('p-0') ? page.firstElementChild : page;
                  const fixture = document.createElement('div');
                  fixture.style.cssText='height:1800px;flex-shrink:0';
                  root.append(fixture);
                  const marker = document.createElement('button');
                  marker.textContent='Scroll regression bottom'; root.append(marker);
                  marker.scrollIntoView({block:'end'});
                  const box=marker.getBoundingClientRect();
                  const ancestors=[];
                  for(let e=root;e && e!==outlet.parentElement;e=e.parentElement) ancestors.push(e);
                  return {bottom:box.bottom, top:box.top, viewport:innerHeight,
                    scrolled:ancestors.some(e=>e.scrollTop>0 && ['auto','scroll'].includes(getComputedStyle(e).overflowY))};
                })()""")
                assert result['scrolled'] and result['top'] >= 0 and result['bottom'] <= height + 1, (route, width, result)
                print(f'PASS {width}x{height} {route}: bottom reachable')
            call('Page.navigate', {'url': base + '/chat'})
            wait_for("!!document.querySelector('[data-slot=sidebar-inset] main')")
            assert evaluate("(() => {const o=document.querySelector('[data-slot=sidebar-inset]').firstElementChild;return o.scrollHeight<=o.clientHeight+1})()"), 'Chat unexpectedly overflows the outer page'
            print(f'PASS {width}x{height} /chat: outer page remains fixed')
        ws.close()
    finally:
        process.terminate()
        process.wait(timeout=15)

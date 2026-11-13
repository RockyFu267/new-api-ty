#!/usr/bin/env bash
# Use only a checkpoint prepared before the PG trial. Trial billing is abandoned.
set -euo pipefail
service_root="$(realpath -- "${1:?service root required}")"
update_dir="$(realpath -- "${2:?update directory required}")"
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
[[ "$(dirname -- "$update_dir")" == "$service_root" ]]
[[ "$(basename -- "$update_dir")" =~ ^update_[0-9]{8}_[0-9]{6}$ ]]
[[ -f "$update_dir/sqlite-rollback/manifest.json" ]]
bash "$service_root/stop.sh"
python3 "$script_dir/sqlite_cutover_rollback.py" restore --service-root "$service_root" --update-dir "$update_dir"
# Do not inherit a PG DSN exported in the operator's shell: godotenv normally
# keeps existing environment variables instead of replacing them from .env.
SQL_DSN= LOG_SQL_DSN= SQLITE_PATH="$service_root/app/one-api.db" REDIS_CONN_STRING= bash "$service_root/start.sh"
python3 - "$update_dir/sqlite-rollback/manifest.json" <<'PY'
import json,sys,time,urllib.request
manifest=json.load(open(sys.argv[1]))
opener=urllib.request.build_opener(urllib.request.ProxyHandler({}))
for attempt in range(30):
    try:
        with opener.open('http://127.0.0.1:'+str(manifest['port'])+'/api/status',timeout=2) as r:
            if r.status==200 and json.load(r).get('success'):
                print('SQLite rollback healthy; trial billing was deliberately abandoned.')
                break
    except Exception:
        pass
    time.sleep(1)
else:
    raise SystemExit('Restored files, but service health failed. Inspect logs; do not start the PG trial again automatically.')
PY

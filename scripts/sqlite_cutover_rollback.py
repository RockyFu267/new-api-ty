"""Offline checkpoint/restore for a backend-only SQLite -> PG trial.

Trial billing is deliberately abandoned on restore. PostgreSQL is never edited.
The application must be stopped; configuration changes must be frozen in trial.
"""
import argparse
from contextlib import closing
import datetime as dt
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import socket
import sqlite3
import subprocess
import sys


def digest(path):
    h = hashlib.sha256()
    with path.open('rb') as source:
        for block in iter(lambda: source.read(1024 * 1024), b''):
            h.update(block)
    return h.hexdigest()


def inside(root, path):
    result = (root / path).resolve()
    if not result.is_relative_to(root) or result == root:
        raise RuntimeError('Path escapes the service directory')
    return result


def ensure_stopped(binary, port):
    try:
        with socket.create_connection(('127.0.0.1', port), timeout=1):
            raise RuntimeError('Service port is still open; stop and drain first')
    except (ConnectionRefusedError, TimeoutError):
        pass
    if os.name == 'nt':
        env = os.environ.copy()
        env['ROLLBACK_EXECUTABLE_PATH'] = str(binary)
        env['ROLLBACK_EXECUTABLE_NAME'] = binary.stem
        found = subprocess.check_output([
            'powershell', '-NoProfile', '-NonInteractive', '-Command',
            'Get-Process -Name $env:ROLLBACK_EXECUTABLE_NAME -ErrorAction SilentlyContinue | Where-Object {$_.Path -eq $env:ROLLBACK_EXECUTABLE_PATH} | Select-Object -ExpandProperty Id; exit 0'
        ], env=env, text=True).strip()
        if found:
            raise RuntimeError('Service executable is still running')
    elif Path('/proc').is_dir():
        for proc in Path('/proc').iterdir():
            if proc.name.isdigit():
                try:
                    executable = os.readlink(proc / 'exe').removesuffix(' (deleted)')
                    if Path(executable).resolve() == binary:
                        raise RuntimeError('Service executable is still running')
                except (FileNotFoundError, PermissionError, ProcessLookupError):
                    pass
    else:
        raise RuntimeError('Unsupported process inspection platform')


def configuration(path):
    values = {}
    for line in path.read_text(encoding='utf-8-sig').splitlines():
        if '=' in line and not line.lstrip().startswith('#'):
            key, value = line.split('=', 1)
            values[key.strip()] = value.strip().strip('\"\'')
    return values


def check_sqlite_config(env_path, db_path):
    values = configuration(env_path)
    if values.get('SQL_DSN') not in ('', None, 'local') or values.get('LOG_SQL_DSN'):
        raise RuntimeError('Checkpoint must use unified SQLite, not a PG/log DSN')
    if not values.get('SQLITE_PATH') or Path(values['SQLITE_PATH']).resolve() != db_path:
        raise RuntimeError('SQLITE_PATH does not match the checkpoint database')
    if values.get('REDIS_CONN_STRING'):
        raise RuntimeError('This rollback requires Redis disabled to avoid stale balances')


def sqlite_integrity(path):
    with closing(sqlite3.connect(path.as_uri() + '?mode=ro', uri=True)) as db:
        if db.execute('PRAGMA integrity_check').fetchone()[0] != 'ok':
            raise RuntimeError('SQLite integrity check failed')


def save_json(path, data):
    path.write_text(json.dumps(data, ensure_ascii=False, indent=2), encoding='utf-8')
    path.chmod(0o600)


def atomic_copy(source, target):
    temporary = target.with_name(target.name + '.rollback-next')
    shutil.copy2(source, temporary)
    os.replace(temporary, target)


def checkpoint(root, update, binary_rel, port):
    binary, env, db = inside(root, binary_rel), inside(root, 'app/.env'), inside(root, 'app/one-api.db')
    ensure_stopped(binary, port)
    check_sqlite_config(env, db)
    destination = update / 'sqlite-rollback'
    destination.mkdir(mode=0o700)  # Refuse to overwrite an existing checkpoint.
    try:
        shutil.copy2(binary, destination / 'binary.before')
        shutil.copy2(env, destination / 'env.before')
        with closing(sqlite3.connect(db.as_uri() + '?mode=ro', uri=True)) as source:
            with closing(sqlite3.connect(destination / 'one-api.db.before')) as target:
                source.backup(target)
        sqlite_integrity(destination / 'one-api.db.before')
        for name in ['binary.before', 'env.before', 'one-api.db.before']:
            (destination / name).chmod(0o600)
        manifest = {
            'version': 1, 'service_root': str(root), 'binary': binary_rel, 'port': port,
            'created_at': dt.datetime.now(dt.timezone.utc).isoformat(),
            'policy': 'discard_trial_billing_restore_precutover_sqlite',
            'configuration_changes_must_be_frozen': True,
            'binary_mode': binary.stat().st_mode & 0o777,
            'hashes': {name: digest(destination / name) for name in ['binary.before', 'env.before', 'one-api.db.before']},
        }
        save_json(destination / 'manifest.json', manifest)  # Written last: ready marker.
    except Exception:
        # Keep incomplete artifacts for inspection; never label them ready.
        raise
    return {'checkpoint_ready': True, 'directory': str(destination)}


def restore(root, update):
    source = update / 'sqlite-rollback'
    manifest = json.loads((source / 'manifest.json').read_text(encoding='utf-8'))
    if manifest['service_root'] != str(root) or manifest['version'] != 1 or manifest['policy'] != 'discard_trial_billing_restore_precutover_sqlite':
        raise RuntimeError('Checkpoint does not match this service or rollback policy')
    binary, env, db = inside(root, manifest['binary']), inside(root, 'app/.env'), inside(root, 'app/one-api.db')
    ensure_stopped(binary, manifest['port'])
    for name, expected in manifest['hashes'].items():
        if name not in ('binary.before', 'env.before', 'one-api.db.before') or digest(source / name) != expected:
            raise RuntimeError('Checkpoint hash mismatch; nothing restored')
    if set(manifest['hashes']) != {'binary.before', 'env.before', 'one-api.db.before'}:
        raise RuntimeError('Incomplete checkpoint')
    check_sqlite_config(source / 'env.before', db)
    sqlite_integrity(source / 'one-api.db.before')
    failed = update / ('failed-state-' + dt.datetime.now(dt.timezone.utc).strftime('%Y%m%d_%H%M%S_%f'))
    failed.mkdir(mode=0o700)
    for path, name in [(binary, 'binary.failed'), (env, 'env.failed')]:
        if path.is_file():
            shutil.copy2(path, failed / name)
            (failed / name).chmod(0o600)
    # Preserve SQLite plus any old WAL/SHM together. Never put an old WAL beside
    # the restored database; it could replay writes from a different snapshot.
    for suffix in ['', '-wal', '-shm', '-journal']:
        path = Path(str(db) + suffix)
        if path.exists():
            os.replace(path, failed / ('one-api.db' + suffix))
    atomic_copy(source / 'one-api.db.before', db)
    atomic_copy(source / 'binary.before', binary)
    binary.chmod(manifest['binary_mode'])
    atomic_copy(source / 'env.before', env)
    check_sqlite_config(env, db)
    sqlite_integrity(db)
    result = {'restored': True, 'service_still_stopped': True, 'trial_billing_abandoned': True,
              'postgres_untouched': True, 'failed_state_preserved': str(failed),
              'restored_hashes': {'binary.before': digest(binary), 'env.before': digest(env), 'one-api.db.before': digest(db)}}
    if result['restored_hashes'] != manifest['hashes']:
        raise RuntimeError('Restored file verification failed; do not start')
    save_json(failed / 'restore-report.json', result)
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=['checkpoint', 'restore'])
    parser.add_argument('--service-root', required=True)
    parser.add_argument('--update-dir', required=True)
    parser.add_argument('--binary', default='bin/new-api-ty')
    parser.add_argument('--port', type=int, default=53000)
    args = parser.parse_args()
    root, update = Path(args.service_root).resolve(strict=True), Path(args.update_dir).resolve(strict=True)
    if update.parent != root or not re.fullmatch(r'update_\d{8}_\d{6}', update.name) or not 1 <= args.port <= 65535:
        raise RuntimeError('Expected service-root/update_YYYYMMDD_HHMMSS and a valid port')
    result = checkpoint(root, update, args.binary, args.port) if args.action == 'checkpoint' else restore(root, update)
    print(json.dumps(result))


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        print('Rollback operation refused/failed: ' + str(error), file=sys.stderr)
        sys.exit(1)

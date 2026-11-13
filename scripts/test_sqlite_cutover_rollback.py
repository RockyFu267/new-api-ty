import json
from pathlib import Path
import socket
import sqlite3
import sys
import tempfile
import unittest

import sqlite_cutover_rollback as rollback


class SQLiteRollbackTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name).resolve()
        self.update = self.root / 'update_20261112_120000'
        self.update.mkdir()
        (self.root / 'app').mkdir()
        (self.root / 'bin').mkdir()
        self.binary = self.root / 'bin/new-api-ty'
        self.binary.write_bytes(b'original executable')
        self.binary.chmod(0o700)
        self.db = self.root / 'app/one-api.db'
        with sqlite3.connect(self.db) as db:
            db.execute('CREATE TABLE usage(amount INTEGER)')
            db.execute('INSERT INTO usage VALUES(123)')
        db.close()
        self.env = self.root / 'app/.env'
        self.env.write_text('SQL_DSN=\nSQLITE_PATH=' + str(self.db) + '\n')
        with socket.socket() as port:
            port.bind(('127.0.0.1', 0))
            self.port = port.getsockname()[1]

    def test_restore_abandons_trial_and_preserves_failed_files_and_wal(self):
        rollback.checkpoint(self.root, self.update, 'bin/new-api-ty', self.port)
        self.binary.write_bytes(b'trial executable')
        self.env.write_text('SQL_DSN=postgres://127.0.0.1:1/offline\n')
        # The current PG server need not be reachable to restore SQLite.
        Path(str(self.db) + '-wal').write_bytes(b'stale WAL must not be replayed')
        Path(str(self.db) + '-shm').write_bytes(b'stale SHM')
        result = rollback.restore(self.root, self.update)
        self.assertTrue(result['postgres_untouched'])
        self.assertTrue(result['service_still_stopped'])
        self.assertEqual(b'original executable', self.binary.read_bytes())
        self.assertEqual('', rollback.configuration(self.env)['SQL_DSN'])
        with sqlite3.connect(self.db) as db:
            self.assertEqual([(123,)], db.execute('SELECT * FROM usage').fetchall())
        db.close()
        failed = Path(result['failed_state_preserved'])
        self.assertEqual(b'trial executable', (failed / 'binary.failed').read_bytes())
        self.assertEqual(b'stale WAL must not be replayed', (failed / 'one-api.db-wal').read_bytes())

    def test_corrupt_backup_refuses_before_replacing_current_files(self):
        rollback.checkpoint(self.root, self.update, 'bin/new-api-ty', self.port)
        self.binary.write_bytes(b'trial executable')
        backup = self.update / 'sqlite-rollback/binary.before'
        backup.write_bytes(b'corrupted backup')
        before = rollback.digest(self.db)
        with self.assertRaisesRegex(RuntimeError, 'hash mismatch'):
            rollback.restore(self.root, self.update)
        self.assertEqual(b'trial executable', self.binary.read_bytes())
        self.assertEqual(before, rollback.digest(self.db))

    def test_live_port_and_executable_prevent_offline_restore(self):
        with socket.socket() as listener:
            listener.bind(('127.0.0.1', 0))
            listener.listen()
            with self.assertRaisesRegex(RuntimeError, 'port is still open'):
                rollback.ensure_stopped(self.binary, listener.getsockname()[1])
        with self.assertRaisesRegex(RuntimeError, 'executable is still running'):
            rollback.ensure_stopped(Path(sys.executable).resolve(), self.port)


if __name__ == '__main__':
    unittest.main()

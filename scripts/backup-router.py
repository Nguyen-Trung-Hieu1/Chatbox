"""Create a consistent private backup of Windows 9router data in WSL."""
import json
import os
from pathlib import Path
import shutil
import sqlite3

os.umask(0o077)
source = Path('/mnt/c/Users/PC/AppData/Roaming/9router')
destination = Path('/var/lib/chatbox1-migration/20260906/9router')
if destination.exists():
    raise SystemExit('Backup destination already exists; refusing overwrite')
destination.mkdir(parents=True, mode=0o700)
for name in ('auth', 'jwt-secret', 'machine-id'):
    entry = source / name
    if entry.is_dir():
        shutil.copytree(entry, destination / name)
    elif entry.is_file():
        shutil.copy2(entry, destination / name)
(destination / 'db').mkdir(mode=0o700)
with sqlite3.connect((source / 'db/data.sqlite').as_uri() + '?mode=ro', uri=True) as src:
    with sqlite3.connect(destination / 'db/data.sqlite') as target:
        src.backup(target)
        if target.execute('PRAGMA integrity_check').fetchone()[0] != 'ok':
            raise RuntimeError('SQLite integrity check failed')
        counts = {}
        for (name,) in target.execute("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'"):
            quoted = '"' + name.replace('"', '""') + '"'
            counts[name] = target.execute('SELECT count(*) FROM ' + quoted).fetchone()[0]
(destination.parent / '9router-counts.json').write_text(json.dumps(counts, indent=2))
print('SQLite backup integrity: OK')
print(json.dumps(counts, indent=2))

"""Stage backed-up data and a generated MongoDB credential without printing secrets."""
import os
from pathlib import Path
import secrets
import shutil
import subprocess

os.umask(0o077)
backup = Path('/var/lib/chatbox1-migration/20260906')
live = Path('/var/lib/chatbox1-data')
if live.exists():
    raise SystemExit('Live data directory exists; refusing overwrite')
live.mkdir(mode=0o700)
(live / 'mongodb').mkdir(mode=0o700)
shutil.copytree(backup / '9router', live / '9router')
shutil.copytree('/mnt/e/Chatbox1/.migration-private/mongo-20260906', backup / 'mongo')
credential = backup / 'mongodb.env'
credential.write_text('MONGO_INITDB_ROOT_USERNAME=chatbox_admin\nMONGO_INITDB_ROOT_PASSWORD=' + secrets.token_hex(32) + '\n')
subprocess.run(['k3s','kubectl','create','namespace','chatbox1'], check=True)
subprocess.run(['k3s','kubectl','-n','chatbox1','create','secret','generic','mongodb-auth','--from-env-file='+str(credential)],check=True)
print('Private data staged; database credential created.')

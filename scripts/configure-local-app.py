"""Create a scoped DB user and a private runtime Secret for the local app."""
import json
import os
from pathlib import Path
import secrets
import sqlite3
import subprocess
from urllib.parse import quote

os.umask(0o077)
private = Path('/var/lib/chatbox1-migration/20260906')
target = private / 'backend.env'
if target.exists():
    raise SystemExit('Runtime config already exists; refusing overwrite')
cfg = {}
for line in Path('/mnt/e/Chatbox1/backend/.env').read_text().splitlines():
    if '=' in line and not line.lstrip().startswith('#'):
        k,v=line.split('=',1); cfg[k.strip()]=v.strip().strip('"\'')
key=cfg.get('AI_API_KEY','')
with sqlite3.connect('file:/var/lib/chatbox1-data/9router/db/data.sqlite?mode=ro',uri=True) as db:
    if not db.execute('SELECT 1 FROM apiKeys WHERE key=? AND isActive=1',(key,)).fetchone():
        raise SystemExit('Backend API key does not match an active key in copied 9router')
password=secrets.token_hex(32)
js='const c=new Mongo("mongodb://"+encodeURIComponent(process.env.MONGO_INITDB_ROOT_USERNAME)+":"+encodeURIComponent(process.env.MONGO_INITDB_ROOT_PASSWORD)+"@127.0.0.1:27017/?authSource=admin");const d=c.getDB("Chatbox1");if(d.getUser("chatbox_app")){throw Error("App user already exists")};d.createUser('+json.dumps({'user':'chatbox_app','pwd':password,'roles':[{'role':'readWrite','db':'Chatbox1'}]})+');'
subprocess.run(['k3s','kubectl','-n','chatbox1','exec','-i','deployment/mongodb','--','mongosh','--quiet','--nodb'],input=js,text=True,check=True,stdout=subprocess.DEVNULL)
runtime={
 'MONGO_URI':'mongodb://chatbox_app:'+quote(password,safe='')+'@mongodb:27017/Chatbox1?authSource=Chatbox1',
 'MONGO_DATABASE':'Chatbox1','AI_ENDPOINT':'http://router:20128/v1/chat/completions',
 'AI_API_KEY':key,'AI_MODEL':cfg.get('AI_MODEL','cx/gpt-5.6-terra'),
 'AI_TOKEN_BUDGET':cfg.get('AI_TOKEN_BUDGET','6000'),'HTTP_ADDR':':8080',
 'CORS_ORIGIN':'http://localhost:8088','COOKIE_SECURE':'false','SESSION_COOKIE_NAME':'chatbox_k3s_session'}
if any('\n' in v or '\r' in v for v in runtime.values()): raise RuntimeError('Invalid multiline config')
target.write_text(''.join(k+'='+v+'\n' for k,v in runtime.items()))
subprocess.run(['k3s','kubectl','-n','chatbox1','create','secret','generic','chatbox-backend-config','--from-env-file='+str(target)],check=True)
print('App configuration created; copied 9router API key validated.')

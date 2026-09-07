"""Restore the private BSON backup to the NEW K3s MongoDB only."""
import json
from pathlib import Path
import subprocess

prefix = ['k3s','kubectl','-n','chatbox1','exec','deployment/mongodb','--']
auth = 'const c = new Mongo("mongodb://"+encodeURIComponent(process.env.MONGO_INITDB_ROOT_USERNAME)+":"+encodeURIComponent(process.env.MONGO_INITDB_ROOT_PASSWORD)+"@127.0.0.1:27017/?authSource=admin"); const d=c.getDB("Chatbox1"); '
def query(js):
    result = subprocess.run(prefix+['mongosh','--quiet','--nodb','--eval',auth+js],check=True,capture_output=True,text=True)
    return json.loads(result.stdout)

if query('print(JSON.stringify(d.getCollectionNames()))'):
    raise SystemExit('Destination database is not empty; refusing restore')
subprocess.run(prefix+['sh','-c','exec mongorestore --host 127.0.0.1 --username "$MONGO_INITDB_ROOT_USERNAME" --password "$MONGO_INITDB_ROOT_PASSWORD" --authenticationDatabase admin --nsInclude "Chatbox1.*" --stopOnError /migration'],check=True)
expected=json.loads(Path('/var/lib/chatbox1-migration/20260906/mongo/Chatbox1/backup-counts.json').read_text())
actual=query('print(JSON.stringify(Object.fromEntries(d.getCollectionNames().map(n=>[n,d.getCollection(n).countDocuments({})]))))')
for name,count in expected.items():
    if name != 'sessions' and actual.get(name) != count:
        raise RuntimeError('Restored document count mismatch: '+name)
indexes=query('print(JSON.stringify(Object.fromEntries(d.getCollectionNames().map(n=>[n,d.getCollection(n).getIndexes().map(i=>i.name)]))))')
for name in expected:
    metadata=json.loads(Path('/var/lib/chatbox1-migration/20260906/mongo/Chatbox1/'+name+'.metadata.json').read_text())
    if sorted(indexes[name]) != sorted(i['name'] for i in metadata['indexes']):
        raise RuntimeError('Restored index mismatch: '+name)
Path('/var/lib/chatbox1-migration/20260906/mongo-verified.json').write_text(json.dumps(actual,indent=2))
print('RESTORE VERIFIED: document counts and index names match (sessions can expire).')
print(json.dumps(actual,indent=2))

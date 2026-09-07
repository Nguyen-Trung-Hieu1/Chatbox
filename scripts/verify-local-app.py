"""Exercise the local HTTP app, then remove only the generated test account."""
import http.cookiejar
import json
import secrets
import subprocess
import urllib.request

base='http://localhost:8088'
username='deploycheck_'+secrets.token_hex(6)
password=secrets.token_hex(20)
opener=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
def request(path, data=None):
    body=None if data is None else json.dumps(data).encode()
    return opener.open(urllib.request.Request(base+path,data=body,headers={'Content-Type':'application/json'}),timeout=150)

try:
    with request('/') as r:
        assert r.status==200 and b'<html' in r.read().lower()
    request('/backend/api/register',{'username':username,'password':password}).close()
    with request('/backend/api/login',{'username':username,'password':password}) as r:
        assert json.load(r)['user']['username']==username
    with request('/backend/api/me') as r: assert json.load(r)['username']==username
    print('HTML, registration, cookie login: PASS',flush=True)
    with request('/backend/api/conversations',{'title':'Deployment verification'}) as r: conversation=json.load(r)['id']
    with request('/backend/api/chat',{'conversation_id':conversation,'content':'Reply with exactly OK. This is a deployment connectivity test.'}) as r:
        events=[json.loads(line) for line in r if line.strip()]
    failures=[e for e in events if e.get('type')=='error']
    if failures: raise RuntimeError('AI stream error: '+str(failures))
    assert any(e.get('type')=='delta' and e.get('content') for e in events),'No streamed text received'
    assert any(e.get('type')=='done' for e in events),'No completion event'
    with request('/backend/api/history?id='+conversation) as r:
        history=json.load(r)
        assert any(m['role']=='assistant' and m['content'] for m in history)
    print('AI streaming and saved chat history: PASS',flush=True)
finally:
    auth='const c=new Mongo("mongodb://"+encodeURIComponent(process.env.MONGO_INITDB_ROOT_USERNAME)+":"+encodeURIComponent(process.env.MONGO_INITDB_ROOT_PASSWORD)+"@127.0.0.1:27017/?authSource=admin");const d=c.getDB("Chatbox1");'
    cleanup='const u=d.users.findOne({username:'+json.dumps(username)+'});if(u){const ids=d.conversations.find({owner_id:u._id}).toArray().map(x=>x._id);d.messages.deleteMany({conversation_id:{$in:ids}});d.conversations.deleteMany({owner_id:u._id});d.sessions.deleteMany({user_id:u._id});d.users.deleteOne({_id:u._id});}print("Test account cleanup: PASS");'
    subprocess.run(['k3s','kubectl','-n','chatbox1','exec','deployment/mongodb','--','mongosh','--quiet','--nodb','--eval',auth+cleanup],check=True)

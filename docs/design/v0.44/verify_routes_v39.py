#!/usr/bin/env python3
"""Read-only structural checks; not a gameplay validator."""
import json
from pathlib import Path
p=Path(__file__).with_name('transmission_routes_v39.json')
d=json.loads(p.read_text())
nodes=d['nodes']; idx={n['id']:n for n in nodes}
assert len(nodes)==len(idx)==12
seen=set()
for n in nodes:
    assert n['solo'] is True and n['forced_group'] is False
    assert n['forced_pvp'] is False and n['scope']=='content_test_draft'
    assert set(n['content'])=={'martial','internal','lightness'}
    assert n['new_named_skill_initial_proficiency']==0
    for dep in n['prerequisites']['nodes']:
        assert dep in idx
    checks=n['prerequisites']['proficiency_each']
    if n['stage']!=1:
        prev=idx[n['prerequisites']['nodes'][0]]
        assert checks=={k:{'course':v,'minimum':{2:200,3:600,4:900}[n['stage']]} for k,v in prev['content'].items()}
        assert len(checks)==3
    else:
        assert not checks
for r in ('A','B','S'):
    assert sorted(n['stage'] for n in nodes if n['route']==r)==[1,2,3,4]
    assert idx[r+'4']['prerequisites']['unique_public_victory'] is False
pending=set(idx)
while pending:
    ready={i for i in pending if set(idx[i]['prerequisites']['nodes'])<=seen}
    assert ready, 'dependency cycle'
    seen|=ready; pending-=ready
assert len(d['cross_permissions'])==4
assert len(d['guest_entries'])==4
assert d['guest_entries'][1]['checks_each']=={'砺石拳':200,'守一诀':200,'折柳步':200}
assert d['guest_entries'][3]['checks_each']=={'疏雨剑':200,'守一诀':200,'折柳步':200}
assert all(g['unlocks_later_secret'] is False for g in d['guest_entries'])
md=p.with_suffix('.md').read_text()
for n in nodes:
    assert n['id'] in md and n['npc'] in md
    for name in n['content'].values(): assert name in md
print('PASS: 12 unique main nodes; 3 routes x 4 stages; separate prior-course checks; DAG; solo flags; 4 guest entries; Markdown names match.')
print('SCOPE: structural/data consistency only; no playable quest, timing, balance or game execution verified.')

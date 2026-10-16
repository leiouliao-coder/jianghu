#!/usr/bin/env python3
"""Deterministic local ledger, NOT server concurrency / economic balance proof."""
import argparse, json, copy
from pathlib import Path
P=argparse.ArgumentParser();P.add_argument('--input',required=True);P.add_argument('--output-prefix',default='work/economy_v44_run');args=P.parse_args()
c=json.loads(Path(args.input).read_text()); checks=0

def ck(condition,message):
 global checks
 checks+=1
 if not condition: raise AssertionError(message)

class World:
 def __init__(self,players,nodes=('A','B')):
  self.pools={n:c['pool_initial'] for n in nodes};self.money={a:{n:{'carry':(v if n=='A' else 0),'pending':0,'bank':0} for n in nodes} for a,v in players.items()}
  self.orders={};self.income={a:[] for a in players};self.goods={};self.material={};self.need={};self.stock={};self.consumed={};self.reserved_slots={};self.slots={};self.hour=0;self.real_hour=0;self.mint=0;self.burn=0;self.npc_paid=0;self.transferred=0;self.logs=[];self.initial=self.total()
 def total(self):return sum(self.pools.values())+sum(v for x in self.money.values() for z in x.values() for v in z.values())
 def frozen(self,node):return sum(o['reward'] for o in self.orders.values() if o['node']==node and o['state']=='open')
 def quota(self,a):return sum(v for t,v in self.income[a] if self.real_hour-24<t<=self.real_hour)+sum(o['reward'] for o in self.orders.values() if o['account']==a and o['state']=='open')
 def invariant(self):
  ck(self.total()==self.initial+self.mint-self.burn,'money conservation')
  for n in self.pools:ck(0<=self.frozen(n)<=self.pools[n]<=c['pool_cap'],'pool including frozen cap')
  for n, items in self.stock.items():
   for item, qty in items.items():ck(qty==sum(1 for g in self.goods.values() if g['node']==n and g['name']==item and g['state']=='npc-stock'),'NPC quantity equals unconsumed actual goods')
  for a in self.money:
   ck(self.quota(a)<=c['account_limit'],'account quota')
   for n in self.money[a]:ck(all(v>=0 for v in self.money[a][n].values()),'nonnegative player money')
 def snap(self,label):
  self.invariant();r={'label':label,'hour':self.hour,'real_hour':self.real_hour,'nodes':{n:{'T':self.pools[n],'F':self.frozen(n),'available':self.pools[n]-self.frozen(n)} for n in self.pools},'player_money':copy.deepcopy(self.money),'world':self.total(),'mint':self.mint,'burn':self.burn,'npc_paid':self.npc_paid,'player_transfer':self.transferred,'demand_stock':{n:dict(v) for n,v in self.stock.items()},'need':{n:dict(v) for n,v in self.need.items()},'orders':{k:v['state'] for k,v in self.orders.items()},'goods_states':{k:v['state'] for k,v in self.goods.items()}};self.logs.append(r);return r
 def refill(self,hours=1):
  for _ in range(hours):
   self.hour+=1;self.real_hour+=1
   for n in self.pools:
    d=min(c['hourly_refill'],c['pool_cap']-self.pools[n]);self.pools[n]+=d;self.mint+=d
  self.invariant()
 def maintenance(self,hours):
  # running-time clock and pool refill remain unchanged; no catch-up mint.
  before=(self.hour,self.mint,self.total());ck(hours>=0,'maintenance duration');self.real_hour+=hours;ck(before==(self.hour,self.mint,self.total()),'maintenance no mint')
 def demand(self,n,item,q):self.need.setdefault(n,{})[item]=q;self.stock.setdefault(n,{})[item]=0
 def accept(self,oid,a,n,item):
  if oid in self.orders:return 'duplicate-order'
  reward=c['rewards'][item]
  held=sum(1 for o in self.orders.values() if o['node']==n and o['item']==item and o['state']=='open')
  if self.need.get(n,{}).get(item,0)<=held:return 'no-demand'
  if self.pools[n]-self.frozen(n)<reward:return 'no-budget'
  if self.quota(a)+reward>c['account_limit']:return 'account-limit'
  k=(a,n)
  if self.slots.get(k,0)+self.reserved_slots.get(k,0)>=200:return 'no-capacity'
  self.reserved_slots[k]=self.reserved_slots.get(k,0)+1
  self.orders[oid]={'account':a,'node':n,'item':item,'reward':reward,'state':'open'};self.invariant();return 'accepted'
 def cancel(self,oid):
  o=self.orders[oid]
  if o['state']!='open':return 'not-open'
  o['state']='cancelled';k=(o['account'],o['node']);self.reserved_slots[k]-=1;self.invariant();return 'cancelled'
 def buy(self,a,n,materials):
  price=sum(c['prices'][k]*v for k,v in materials.items());ck(self.money[a][n]['carry']>=price,'buy local money sufficient')
  self.money[a][n]['carry']-=price;self.burn+=price;inv=self.material.setdefault((a,n),{})
  for k,v in materials.items():inv[k]=inv.get(k,0)+v
  self.invariant();return price
 def craft(self,a,n,r,iid):
  inv=self.material[(a,n)];ck(iid not in self.goods,'unique item id')
  for k,v in r['recipe'].items():ck(inv.get(k,0)>=v,'craft material')
  for k,v in r['recipe'].items():inv[k]-=v
  self.goods[iid]={'owner':a,'node':n,'name':r['name'],'current':100,'max':100,'state':'player'}
 def lifecycle(self,a,n,r,iid):
  self.craft(a,n,r,iid);g=self.goods[iid];trace=[{'stage':'made','current':100,'max':100}]
  for index,wear in enumerate(r['wear']):
   g['current']=max(0,g['current']-wear);trace.append({'stage':'wear','current':g['current'],'max':g['max']})
   if index<2:
    for k,v in r['repair_recipe'].items():ck(self.material[(a,n)].get(k,0)>=v,'repair material');self.material[(a,n)][k]-=v
    g['max']-=c['repair_max_loss'];g['current']=g['max'];trace.append({'stage':'repair','current':g['current'],'max':g['max']})
  ck((g['current'],g['max'])==(0,96),'two repair wear result');g['state']='recycled'
  for k,v in r['salvage'].items():self.material[(a,n)][k]=self.material[(a,n)].get(k,0)+v
  trace.append({'stage':'recycled','salvage':r['salvage']});return trace
 def deliver(self,oid,iid):
  o=self.orders[oid]
  if o['state']=='paid':return 'already-paid'
  if o['state']!='open':return 'not-open'
  g=self.goods.get(iid)
  if not g or not(g['owner']==o['account'] and g['node']==o['node'] and g['name']==o['item'] and g['state']=='player' and g['max']==100 and g['current']==100):return 'invalid-item'
  a,n=o['account'],o['node'];g['state']='npc-stock';self.need[n][o['item']]-=1;self.stock[n][o['item']]+=1
  self.pools[n]-=o['reward'];self.money[a][n]['pending']+=o['reward'];self.npc_paid+=o['reward'];o['state']='paid';self.income[a].append((self.real_hour,o['reward']))
  k=(a,n);self.reserved_slots[k]-=1;self.slots[k]=self.slots.get(k,0)+1;self.invariant();return 'paid'
 def consume(self,n,item,ids):
  q=len(ids);ck(q>0 and len(set(ids))==q,'unique actual consumed IDs')
  ck(self.stock[n][item]>=q,'NPC consumption stock')
  for iid in ids:
   g=self.goods.get(iid);ck(g is not None and g['node']==n and g['name']==item and g['state']=='npc-stock','consume only actual NPC goods')
  for iid in ids:self.goods[iid]['state']='npc-consumed'
  self.stock[n][item]-=q;self.need[n][item]+=q;self.consumed[(n,item)]=self.consumed.get((n,item),0)+q;self.invariant()
 def claim_bank(self,a,n):
  amount=self.money[a][n]['pending'];self.money[a][n]['pending']=0;self.money[a][n]['bank']+=amount;self.invariant()
 def move_money(self,a,b,n,amount):
  ck(self.money[a][n]['carry']>=amount,'transfer balance');self.money[a][n]['carry']-=amount;self.money[b][n]['carry']+=amount;self.transferred+=amount;self.invariant()

sword,cloth=c['recipes'];prices=[]
for r in (sword,cloth):
 fresh=sum(c['prices'][k]*v for k,v in r['recipe'].items());repair=sum(c['prices'][k]*v for k,v in r['repair_recipe'].items());prices.append({'item':r['name'],'fresh_cost':fresh,'repair_cost':repair,'lifecycle_cost':fresh+2*repair,'order_reward':c['rewards'][r['name']],'order_net_before_labor':c['rewards'][r['name']]-fresh})
# A: same account two roles, local starts A1000/B0; player physically carries 200 to B BEFORE fixtures, not teleport.
a=World({'solo':1000});a.money['solo']['A']['carry']=800;a.money['solo']['B']['carry']=200;a.snap('A0 已沿普通道路实际携款200抵B；存量1000非本期发行')
a.demand('A',sword['name'],1);a.demand('B',cloth['name'],1)
traces={}
for n,r,prefix in [('A',sword,'sword'),('B',cloth,'cloth')]:
 life={k:v for k,v in r['initial_inventory'].items() if k in c['prices']};both=dict(life)
 for k,v in r['recipe'].items():both[k]=both.get(k,0)+v
 a.buy('solo',n,both);a.craft('solo',n,r,prefix+'-ORDER');traces[prefix]=a.lifecycle('solo',n,r,prefix+'-SELF')
a.snap('A1 购料并制造独立订单件、自用件，两修回收完成')
ck(a.accept('s','solo','A',sword['name'])=='accepted','sword order');ck(a.accept('c','solo','B',cloth['name'])=='accepted','cloth order');a.snap('A2 接单冻结350，账号预占350')
a.refill();a.snap('A3 运行1小时两池各发行200')
ck(a.deliver('s','sword-ORDER')=='paid','sword deliver');ck(a.deliver('c','cloth-ORDER')=='paid','cloth deliver');a.snap('A4 交付：本地待领A200/B150，收入350')
a.claim_bank('solo','A');a.claim_bank('solo','B');a.snap('A5 同节点领取至银行，不跨节点不重复占额')
ck(a.burn==340 and a.mint==400 and a.total()==3060,'A money expected');ck(sum(v for n in a.money['solo'].values() for v in n.values())==1010,'A player expected')
# B requests are serialized accepted-order model (not real network races).
b=World({'p':1000,'q':1000});b.demand('A',sword['name'],1);b.snap('B0 请求顺序p先q后')
ck(b.accept('p1','p','A',sword['name'])=='accepted','first accepted');before=b.total();ck(b.accept('q1','q','A',sword['name'])=='no-demand','same last demand rejected');ck(b.total()==before,'reject no money change');b.snap('B1 p锁定唯一需求，q被拒')
b.buy('p','A',sword['recipe']);b.craft('p','A',sword,'B-S');ck(b.deliver('p1','B-S')=='paid','deliver');before=b.total();ck(b.deliver('p1','B-S')=='already-paid','replay');ck(before==b.total(),'replay no pay');b.snap('B2 p收入200；重放未重复支付')
b.consume('A',sword['name'],['B-S']);ck(b.goods['B-S']['state']=='npc-consumed','B-S truly removed from circulation');ck(b.accept('q2','q','A',sword['name'])=='accepted','real consumption restores demand');b.snap('B3 NPC实际消耗后q可接新订单')
before=(b.total(),b.mint,b.pools['A']);ck(b.cancel('q2')=='cancelled','cancel');ck(before==(b.total(),b.mint,b.pools['A']),'cancel not mint/refund double');b.snap('B4 取消仅解冻200')
b.move_money('p','q','A',30);b.snap('B5 支付玩家工匠30为转移，非回收')
# independent negative fixtures, clearly not attached to B balance.
f=World({'p':0});f.demand('A',sword['name'],2);f.demand('B',sword['name'],2)
ck(f.accept('alt1','p','A',sword['name'])=='accepted','quota 200');ck(f.accept('alt2','p','B',sword['name'])=='accepted','quota 400');ck(f.accept('alt3','p','B',sword['name'])=='account-limit','cross-role-node quota');f.cancel('alt2');ck(f.accept('alt3','p','B',sword['name'])=='accepted','cancel frees quota')
f.refill(24);ck(f.accept('alt4','p','B',sword['name'])=='account-limit','open commitments not expired by rolling window');f.snap('负例quota：未交付预占不因24h自动消失')
g=World({'p':0,'q':0});g.pools['A']=100;g.initial=g.total();g.demand('A',sword['name'],1);ck(g.accept('budget','q','A',sword['name'])=='no-budget','budget reject');g.snap('负例资金不足：不接单不冻结')
h=World({'p':0});h.pools['A']=2900;h.initial=h.total();h.demand('A',sword['name'],1);h.accept('frozen','p','A',sword['name']);h.refill();ck(h.pools['A']==3000 and h.frozen('A')==200,'frozen included in cap');h.snap('负例含冻结上限：A仅补100而非200')
k=World({'p':0});k.slots[('p','A')]=200;k.demand('A',sword['name'],1);ck(k.accept('full','p','A',sword['name'])=='no-capacity','pending capacity preflight')
# C: no players online, no existing orders; hourly snapshots.
z=World({'offline':0});z.snap('C0 无人在线且无合同')
for hour in range(24):z.refill();z.snap('C'+str(hour+1))
ck(z.mint==4000 and z.npc_paid==0 and z.total()==6000,'24h capped refill');before=z.total();z.maintenance(8);ck(z.total()==before,'downtime no catchup');z.snap('C维护8小时不补池')
# Separate offline contract fixture: goods already in legal receiving station, system scans/accepts physical goods.
o=World({'owner':100});o.demand('A',sword['name'],1);o.buy('owner','A',sword['recipe']);o.craft('owner','A',sword,'OFFLINE-S');o.accept('off','owner','A',sword['name']);o.snap('D0 离线前实物已在授权验收站，报酬冻结200')
ck(o.deliver('off','OFFLINE-S')=='paid','offline physical acceptance');o.snap('D1 系统验收，无需货主在线，200进入A待领')
# rolling income expires exactly at 24h, commitments remain until explicit termination.
rolling=copy.deepcopy(a);ck(rolling.quota('solo')==350,'rolling before');rolling.refill(24);ck(rolling.quota('solo')==0,'rolling 24h expires actual paid');rolling.snap('独立滚动额度夹具，主场景A不推进');ck(a.hour==1 and a.mint==400,'main A observation unchanged')
result={'status':'PASS','assertions':checks,'input':str(Path(args.input).resolve()),'prices':prices,'A':a.logs,'lifecycle':traces,'B':b.logs,'negative_fixtures':[f.logs,g.logs,h.logs],'rolling_fixture':rolling.logs[-1:],'C':z.logs,'D_offline_existing_contract':o.logs,'caveats':['deterministic serial ledger only; not production concurrency','all prices rewards schedules new test drafts','NPC raw stock assumed sufficient for stated purchases; no infinite supply conclusion','free no-tool gameplay exists but not an unlimited coin faucet','material consumption is not currency burn; buy NPC material burns coins','overdue/ordinary storage fees remain zero','money reserve one slot per order in fixture only; actual coin stack capacity pending']}
pre=Path(args.output_prefix);pre.with_suffix('.json').write_text(json.dumps(result,ensure_ascii=False,indent=2));print(json.dumps(result,ensure_ascii=False,indent=2))

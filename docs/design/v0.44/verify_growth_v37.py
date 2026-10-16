#!/usr/bin/env python3
"""Document arithmetic fixture, not game growth implementation."""
import json, sys
from decimal import Decimal as D
from pathlib import Path
NAMES=['臂力','根骨','身法','悟性','定力']
def ds(x): return str(x)
def run(c):
    a={k:D(str(v)) for k,v in zip(NAMES,c['before'])}; before=a.copy()
    modes=dict(zip(NAMES,c.get('modes',['提高']*5)))
    caps={k:D('100') for k in NAMES}; totalcap=D('300')
    oldskill=D(str(c.get('skill_before','0'))); skill=oldskill
    progress=D(str(c.get('progress','0'))); initial_progress=progress
    evt=c.get('event','practice'); target=c.get('target','臂力'); status=''
    if evt=='item_cap': caps[target]=D(str(c['raised_cap'])); status='仅抬单项上限；不赠点'
    elif evt=='item_total': totalcap=D(str(c['raised_total'])); status='仅抬总上限；不赠点'
    elif evt=='equipment': status='临时效果不计永久预算；有效收益上限待定'
    elif evt=='offline': status='离线不新增进度或属性'
    elif evt=='toggle': status='仅改状态，不触发属性结算'
    else:
        caps.update({k:D(str(v)) for k,v in c.get('caps',{}).items()}); totalcap=D(str(c.get('totalcap','300')))
        progress=min(D('1'),progress+D(str(c.get('earned','0'))))
        if c.get('skill_eligible',False): skill+=D(str(c.get('skill_gain','0.1')))
        if modes[target]!='提高': status='目标未设提高；属性不变，进度保留'
        elif progress<D('1'): status='训练进度不足；属性不变'
        elif a[target]+D('.1')>caps[target]: status='目标已达单项上限；不扣来源'
        else:
            need=max(D('0'),sum(a.values())+D('.1')-totalcap)
            donor=None
            if need:
                donor=next((k for k in c.get('priority',[]) if k!=target and modes[k]=='减少' and a[k]-D('.1')>=D('10')),None)
            if need and donor is None: status='无完整一步的合法来源；不扣不加'
            else:
                if donor: a[donor]-=D('.1')
                a[target]+=D('.1'); progress-=D('1'); status=('等量转移：'+donor+'→'+target) if donor else '预算未满，正常增长'
    expect=c['expect']; assert [a[k] for k in NAMES]==[D(v) for v in expect['after']],(c['id'],a,expect)
    assert ds(progress)==expect['progress'],(c['id'],progress)
    assert ds(skill)==expect['skill'],(c['id'],skill)
    assert sum(a.values())<=totalcap
    assert all(a[k]<=caps[k] for k in NAMES)
    if evt.startswith('item') or evt in ('offline','toggle','equipment'): assert a==before
    learned=c.get('learned',True); proficiency=D(str(c.get('proficiency','600')))
    threshold=D(str(c.get('trait_threshold','50')))
    result={'id':c['id'],'前':[ds(before[k]) for k in NAMES],'后':[ds(a[k]) for k in NAMES], '前总额':ds(sum(before.values())),'后总额':ds(sum(a.values())), '状态':status,'属性状态':modes,'前进度':ds(initial_progress),'后进度':ds(progress),'技能类别':c.get('skill_type','无技能结算'),'前技能':ds(oldskill),'后技能':ds(skill),'单项上限':{k:ds(v) for k,v in caps.items()},'总上限':ds(totalcap),'已学保留':learned,'武学熟练度':ds(proficiency),'当前悟性特性启用':a['悟性']>=threshold,'断言':'PASS'}
    if 'trait_active' in expect: assert result['当前悟性特性启用']==expect['trait_active']
    if 'effective' in c: result['展示临时效果']=c['effective']
    return result
if __name__=='__main__':
    args=sys.argv[1:]
    if args and args[0]=='--input': args=args[1:]
    p=Path(args[0]) if args else Path(__file__).with_name('growth_cases_v37.json')
    data=json.loads(p.read_text()); results=[run(c) for c in data['cases']]
    print(json.dumps({'范围':'文档算术夹具；不代表游戏运行、成长速度或最终平衡','属性顺序':NAMES,'结果':results,'汇总':f"PASS {len(results)}/{len(results)}"},ensure_ascii=False,indent=2))

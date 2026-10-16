#!/usr/bin/env python3
"""独立物料账模型；不代表游戏服务器、技能成长、市场或负载测试。"""
import argparse
import copy
import json
from fractions import Fraction
from pathlib import Path

p = argparse.ArgumentParser(description=__doc__)
p.add_argument('--input', type=Path, default=Path(__file__).with_name('equipment_ledger_v41.json'))
a = p.parse_args()
data = json.loads(a.input.read_text(encoding='utf-8'))
checks = 0

def check(cond, message):
    global checks
    assert cond, message
    checks += 1

def state(spec):
    return {'inventory': copy.deepcopy(spec['initial_inventory']), 'equipment': None, 'consumed': {}, 'events': {}}

def debit(s, cost):
    for k, v in cost.items():
        s['inventory'][k] -= v
        s['consumed'][k] = s['consumed'].get(k, 0) + v

def execute(s, spec, action, event_id, profession=None, confirmed=True):
    # 事件ID结果缓存仅为本模型的原子化设计演示。
    if event_id in s['events']:
        return s['events'][event_id]
    e = s['equipment']
    if action == 'manufacture':
        if e is not None:
            return '拒绝：该示例唯一ID已使用'
        cost = spec['recipe']
    elif action == 'repair':
        if not e or e['status'] == 'destroyed':
            return '拒绝：无可修装备'
        if profession != spec['profession']:
            return '拒绝：专业不符'
        if e['M'] - data['repair_max_loss'] <= 0:
            return '拒绝：最大耐久不足'
        cost = spec['repair_recipe']
    elif action == 'salvage':
        if not confirmed:
            return '拒绝：未确认回收'
        if not e or e['status'] == 'destroyed':
            return '拒绝：原件不存在'
        cost = {}
    else:
        raise ValueError(action)
    if any(s['inventory'].get(k, 0) < v for k, v in cost.items()):
        return '拒绝：材料不足'
    debit(s, cost)
    if action == 'manufacture':
        m = spec['max_durability']
        s['equipment'] = {'id': spec['id'], 'D': m, 'M': m, 'M0': m, 'status': 'usable'}
    elif action == 'repair':
        e['M'] -= data['repair_max_loss']
        e['D'] = e['M']
        e['status'] = 'usable'
    else:
        # 墓碑记录不是仍可装备/交易的物品；同事务移除原件并结算废料。
        e['status'] = 'destroyed'
        for k, v in spec['salvage'].items():
            s['inventory'][k] = s['inventory'].get(k, 0) + v
    result = '成功：' + action
    s['events'][event_id] = result
    return result

def invariant(s):
    e = s['equipment']
    check(all(v >= 0 for v in s['inventory'].values()), '库存不得负数')
    if e:
        check(0 <= e['D'] <= e['M'] <= e['M0'], '耐久层次错误')
        check(e['status'] != 'usable' or e['D'] > 0, '零耐久不得有效')

def show(label, s):
    invariant(s)
    print(label + ' ' + json.dumps(s, ensure_ascii=False, sort_keys=True))

print('范围：独立物料状态/算术模型；不验证真实背包、容量预留、网络并发或服务端实现。')
print('输入状态：' + data['status'])
for spec in data['items']:
    print('\n=== ' + spec['name'] + ' / ' + spec['id'] + ' ===')
    s = state(spec)
    show('初始', s)
    check(execute(s, spec, 'manufacture', 'make-1') == '成功：manufacture', '制造失败')
    show('制造后', s)
    for i, wear in enumerate(spec['wear']):
        e = s['equipment']
        check(wear <= e['D'], '磨损超过当前耐久')
        e['D'] -= wear
        if not e['D']:
            e['status'] = 'disabled'
        show('使用%d后' % (i + 1), s)
        if i < 2:
            check(execute(s, spec, 'repair', 'repair-%d' % i, spec['profession']) == '成功：repair', '维修失败')
            show('维修%d后' % (i + 1), s)
    check(s['equipment']['D'] == 0 and s['equipment']['M'] == 96, '最终使用状态')
    check(execute(s, spec, 'salvage', 'salvage-1') == '成功：salvage', '回收失败')
    show('回收后', s)
    check(s['equipment']['status'] == 'destroyed', '原件必须退出流通')
    check(all(s['inventory'][k] == 0 for k in spec['initial_inventory'] if k not in spec['salvage']), '原料余量错误')
    check(s['consumed'] == {k: spec['recipe'].get(k, 0) + 2 * spec['repair_recipe'].get(k, 0) for k in set(spec['recipe']) | set(spec['repair_recipe'])}, '总消耗错误')
    main = spec['main_material']
    eq = spec['scrap_equivalent']
    returned = spec['salvage'][spec['scrap']] * Fraction(eq['numerator'], eq['denominator'])
    check(returned < spec['recipe'][main], '回收不能全返制造主材')
    check(returned < s['consumed'][main], '回收不得形成主材增殖')
    print('主材当量比较：投入%s=%s，废料%s=%s，演示换算当量=%s；非可执行加工配方。' % (main, s['consumed'][main], spec['scrap'], spec['salvage'][spec['scrap']], returned))
    original = copy.deepcopy(s)
    result = execute(s, spec, 'salvage', 'salvage-duplicate')
    check(s == original and result.startswith('拒绝'), '重复回收不得改变状态')
    print('负例：新事件重复回收 -> ' + result + '；全状态不变')
    result = execute(s, spec, 'salvage', 'salvage-1')
    check(s == original and result == '成功：salvage', '同事件重放必须幂等')
    print('重放：已成功回收事件 -> 返回缓存结果；全状态不变（模型层）')

    tests = []
    x = state(spec); x['inventory'][main] = 0
    tests.append(('材料不足', x, 'manufacture', None, True))
    for title, prof, confirmed in [('错误专业', '错误专业', True), ('最大耐久为2', spec['profession'], True), ('未确认回收', None, False)]:
        x = state(spec)
        execute(x, spec, 'manufacture', 'negative-make')
        if title == '最大耐久为2':
            x['equipment'].update(D=0, M=2, status='disabled')
        tests.append((title, x, 'salvage' if title == '未确认回收' else 'repair', prof, confirmed))
    for title, x, action, prof, confirmed in tests:
        before = copy.deepcopy(x)
        result = execute(x, spec, action, 'negative', prof, confirmed)
        check(result.startswith('拒绝') and x == before, title + '应全状态不变')
        invariant(x)
        print('负例：' + title + ' -> ' + result + '；全状态不变')
print('\nPASS：%d个断言；两条主流程、每件5个拒绝分支与1个幂等重放。' % checks)
print('未计算金币价格、收入平衡、技能或属性成长；未将主材之外不同材料相加。')

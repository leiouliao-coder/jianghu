"""Create the delivery manifest and clean repository-relative ZIP (never pushes Git).
Run only after final validators and renders have completed:
python assets/source/package_sample_village.py --out /tmp/jianghu-art-sample-village.zip
"""
from pathlib import Path
import argparse, json, hashlib, zipfile, subprocess, datetime
from PIL import Image
ROOT=Path(__file__).resolve().parents[2]
REPORT=ROOT/'docs/art/sample-village-validation.json'
BASE='2d9b020f276ed87b810c0903d65dcb00a6a9e9c4'
parser=argparse.ArgumentParser();parser.add_argument('--out',default='/tmp/jianghu-art-sample-village.zip');args=parser.parse_args()
report=json.loads(REPORT.read_text());assert report['pass'],report['errors']
assets=report['assets'];runtime={a['file']:a for a in assets}
for a in assets:
    assert hashlib.sha256((ROOT/a['file']).read_bytes()).hexdigest()==a['sha256'], 'Revalidate changed GLB: '+a['file']
assert subprocess.check_output(['git','rev-parse','main'],cwd=ROOT,text=True).strip()==BASE
assert subprocess.check_output(['git','branch','--show-current'],cwd=ROOT,text=True).strip()=='art/sample-village'
assert not subprocess.check_output(['git','diff','--name-only'],cwd=ROOT,text=True).strip(), 'Existing tracked file modified unexpectedly'

MANIFEST=ROOT/'docs/art/sample-village-manifest.md';INDEX=ROOT/'docs/art/sample-village-files.json'
DOCS=['sample-village-manifest.md','sample-village-files.json','sample-village-validation.json','sample-village-godot-validation.json','character-validation.json','buildings-validation.json','props-validation.json','ground-validation.json']
def delivery_files():
    result=[]
    for folder in ['assets/concept-art/sample','assets/source','client/art_preview']:
        for p in (ROOT/folder).rglob('*'):
            if p.is_file() and '__pycache__' not in p.parts and not p.name.endswith(('.blend1','.blend2','.log','.pyc')):
                result.append(p)
    for folder in ['village','characters']:
        for p in (ROOT/'client/assets/art'/folder).iterdir():
            if p.is_file() and (p.name.endswith('.glb') or p.name.endswith('.glb.import')):result.append(p)
    result += [p for p in (ROOT/'client/assets/art/textures').iterdir() if p.is_file() and p.name.startswith('ground_') and p.name.endswith(('.png','.png.import'))]
    result += [ROOT/'docs/art'/name for name in DOCS if (ROOT/'docs/art'/name).exists()]
    return sorted(set(result),key=lambda p:p.relative_to(ROOT).as_posix())

intro='''# 河湾镇 A 区＋新手主角美术样板清单

制作日期：2026-10-16。Blender 4.3.2。交付分支：`art/sample-village`，基于 `2d9b020f276ed87b810c0903d65dcb00a6a9e9c4`。本包只增加美术、源文件、检查材料及隔离预览，不改变既有服务器地图、碰撞或游戏主入口。

## 先看实际模型

- [镇内全景，导出 GLB 重新组装渲染](../../assets/source/renders/01-village-exported-glbs.png)
- [客栈实际模型](../../assets/source/renders/02-hall-exported-glb.png)、[铁匠铺实际模型](../../assets/source/renders/03-forge-exported-glb.png)
- [主角近景](../../assets/source/renders/04-character-exported-idle.png)、[走路](../../assets/source/renders/05-character-exported-walk.png)、[0.25 秒出招](../../assets/source/renders/06-character-exported-attack-025.png)、[倒地末帧](../../assets/source/renders/07-character-exported-death-100.png)
- [建模前布局设计图](../../assets/concept-art/sample/village-layout.png)、[角色三视图](../../assets/concept-art/sample/player-turnaround.png)

以上七张实际模型图为 1280×800 Blender 渲染，直接使用最终导出的 GLB；不是 Godot 实机截图。游戏模型采用面数预算内的风格化表达，细节密度不等同于 AI 设计图。请以实际模型渲染判断样板视觉效果。

## 验证范围

- 六个 GLB：二进制 glTF 2.0、内嵌 PNG、纹理 ≤2048、面数限制、干净场景背景重导入均通过
- 两栋建筑基座占地严格 14×14 米；客栈屋檐外挑不足 1 米；原点均在占地中心地面
- 围栏严格 4×0.3×1.2 米；五个独立小道具分别 ≤2×2 米、≤3000 三角面
- 预览内 56 段围栏均通过世界坐标方向／宽高检查，四个路口宽度保持 8／8／8／6 米
- 古树冠幅 12 米，地面树根半径 1.382 米；树冠不得参与通行碰撞
- 人物高 1.749 米，独立剑对象，28 骨骼蒙皮，五个动画轨名称与零起点时长准确
- `walk`：0.4 秒根运动 1.6 米，即 4 米/秒；支撑期脚踝数值漂移低于 0.000001 米
- `attack_basic`：0.70 秒，剑尖最远接触帧在 0.25 秒；`death`：1 秒，末尾 0.15 秒保持倒地姿势
- 地面六张 PNG：2048×2048，水平和垂直对边逐像素一致；OpenGL +Y 法线
- Godot 4.6.3 Compatibility：资源导入、场景实例化、动画清单及隔离预览脚本无界面加载通过
- 未完成项：Godot 4.7 实机验收、手机性能实测、联机脚步／碰撞／命中逻辑、任务书要求的四张游戏实机截图。这里不宣称这些项目通过

量化证据见相邻 `sample-village-validation.json`、`sample-village-godot-validation.json` 及四份专项验证 JSON。

## 接入注意

坐标为 Blender X 东／Y 北／Z 上，正面 −Y；导出 glTF +Y Up，Godot 使用 `(x,z,-y)`。客栈对应服务端 A-hall，Godot 位置 `(-15,0,-15)`；铁匠铺 A-forge 为 `(15,0,15)`。布局来源和四个路口的精确宽度见 [布局 JSON](../../assets/source/sample-village-layout.json)。

`walk` 是根运动动画。应提取根运动，或消除根平移后使用服务器 4 米/秒速度，不能两者叠加。`idle` 和 `walk` 在引擎里设循环，另外三个动作不循环，死亡保持末帧。glTF 不携带游戏命中事件，需由本机接入在 0.25 秒触发判定。导入设置保留 60 fps 采样，并把 GLB 内嵌图像作为压缩内嵌资源导入。

`props_set.glb` 中 crate、barrel、lantern_pole、well、market_table 是五个独立命名根对象，采用分开展示位置；单独摆放时重置该对象的平移。建筑、围栏及树均未生成隐式游戏碰撞。重复围栏建议在游戏接入时做批处理／实例化，并在目标手机上实际测量性能。

[隔离预览](../../client/art_preview/sample_village_preview.tscn) 可用 `godot --path client res://art_preview/sample_village_preview.tscn` 启动，Tab 切换全景和人物，1–5 播放动作，R 重播。不会改动既有游戏场景。

## 运行时文件

| 文件 | 三角面 | 内嵌贴图尺寸 | 动画 | Blender | 生成方式 |
|---|---:|---|---|---|---|
'''
rows=[]
for a in assets:
    dims=sorted(set(f'{im["size"][0]}×{im["size"][1]}' for im in a['embedded_pngs']))
    animations='、'.join(f'{x["name"]} {x["duration_seconds"]:.2f}s' for x in a['animations']) or '—'
    rows.append(f'| `{a["file"]}` | {a["triangles"]:,} | {", ".join(dims)} PNG | {animations} | 4.3.2 | 原创脚本建模＋原创像素材质；建筑／人物由 AI 设计图辅助 |')
for p in sorted((ROOT/'client/assets/art/textures').glob('*.png')):
    rows.append(f'| `{p.relative_to(ROOT)}` | — | 2048×2048 PNG | — | 4.3.2 材质源 | 原创确定性无缝像素绘制／法线生成 |')
props=json.loads((ROOT/'docs/art/props-validation.json').read_text())
body='\n'.join(rows)+'''

`props_set.glb` 总计 7,028 三角面，各件分别为：木箱 1,444，木桶 584，灯笼杆 624，水井 2,908，摊位桌 1,468。每件均低于 3,000。

## 源文件、设计图、脚本和检查资料

所有 `.blend` 打包了使用中的 PNG；脚本和原始材质 PNG 一并提供，便于修改。重建命令见 [源文件说明](../../assets/source/README.md)。建模前设计图的同名 `-prompt.md` 保存原始提示词；实际模型渲染另放在源文件目录，避免混淆。

| 文件 | 三角面／类型 | 图像尺寸 | 动画 | Blender | 生成方式／用途 |
|---|---|---|---|---|---|
'''
# Reserve rows for the manifests themselves before enumerating.
MANIFEST.write_text('');INDEX.write_text('{}')
for p in delivery_files():
    rel=p.relative_to(ROOT).as_posix()
    if rel in runtime or rel.startswith('client/assets/art/textures/') and p.suffix=='.png':continue
    dim='—';tri='—';anim='—';version='—';method='制作说明／验证记录'
    if p.suffix.lower()=='.png':
        with Image.open(p) as im:dim=f'{im.width}×{im.height}'
        method='建模前 AI 辅助设计' if 'concept-art' in rel and 'props-design' not in rel else ('原创手绘结构设计' if 'props-design' in rel else '实际模型渲染' if 'renders/' in rel or p.name.startswith('character-') or 'preview' in p.name else '原创程序绘制材质 PNG')
    elif p.suffix=='.blend':tri='可编辑源场景';version='4.3.2';method='原创模型／打包材质／摄影机与灯光'
    elif p.suffix=='.py':version='4.3.2 或系统 Python';method='可复现制作／导出／验证脚本'
    elif p.suffix=='.svg':method='原创矢量设计图'
    elif p.name.endswith('-prompt.md'):method='同名设计图原始提示词／设计记录'
    elif p.suffix in ['.gd','.tscn','.uid','.import']:method='隔离预览／导入配置／引擎元数据'
    if 'character_novice.blend' in rel:tri='14,773（角色网格）';anim='idle、walk、attack_basic、hit、death'
    body+=f'| `{rel}` | {tri} | {dim} | {anim} | {version} | {method} |\n'
body+='''
## 授权与来源

全部交付模型、贴图、动画及设计图均为本任务原创建模、程序绘制或原创 AI 辅助设计，没有引入第三方模型、HDRI、贴图、笔刷或收费资源，因此没有第三方 CC0 署名项。参考是仓库已有的两张概念图。交付内容随仓库 MIT 许可提供，AI 设计图不构成独占版权或商标保证。

Godot 内嵌图像导入模式的实现依据：[官方导入器源码](https://github.com/godotengine/godot/blob/master/modules/gltf/editor/editor_scene_importer_gltf.cpp)。软件本身不作为第三方美术资源随包分发。

文件大小和 SHA-256 见 `sample-village-files.json`。该哈希索引不计算自身哈希。ZIP 包只含本样板新增的仓库相对路径，不含 `.git`、缓存、备份或普通构建日志。
'''
MANIFEST.write_text(intro+body)
files=delivery_files();index_files=[]
for p in files:
    if p==INDEX:continue
    b=p.read_bytes();assert len(b)<25*1024*1024,f'Browser file size limit exceeded: {p}'
    index_files.append({'path':p.relative_to(ROOT).as_posix(),'bytes':len(b),'sha256':hashlib.sha256(b).hexdigest()})
index={'base_main_commit':BASE,'target_branch':'art/sample-village','generated_utc':datetime.datetime.now(datetime.timezone.utc).isoformat(),'blender':'4.3.2','files_excluding_this_index':index_files,'total_files_in_archive':len(files),'total_bytes_excluding_index':sum(f['bytes'] for f in index_files),'largest_file':max(index_files,key=lambda f:f['bytes'])}
INDEX.write_text(json.dumps(index,ensure_ascii=False,indent=2)+'\n')
out=Path(args.out);out.parent.mkdir(parents=True,exist_ok=True)
with zipfile.ZipFile(out,'w',zipfile.ZIP_DEFLATED,compresslevel=6) as z:
    for p in files:z.write(p,p.relative_to(ROOT).as_posix())
with zipfile.ZipFile(out) as z:assert z.testzip() is None
print(json.dumps({'archive':str(out),'bytes':out.stat().st_size,'sha256':hashlib.sha256(out.read_bytes()).hexdigest(),'files':len(files),'largest_file':index['largest_file']},ensure_ascii=False,indent=2))

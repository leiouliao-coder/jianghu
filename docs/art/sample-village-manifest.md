# 河湾镇 A 区＋新手主角美术样板清单

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
| `client/assets/art/village/building_hall.glb` | 19,656 | 512×512 PNG | — | 4.3.2 | 原创脚本建模＋原创像素材质；建筑／人物由 AI 设计图辅助 |
| `client/assets/art/village/building_forge.glb` | 13,088 | 512×512 PNG | — | 4.3.2 | 原创脚本建模＋原创像素材质；建筑／人物由 AI 设计图辅助 |
| `client/assets/art/village/tree_old.glb` | 2,992 | 1024×1024, 512×512 PNG | — | 4.3.2 | 原创脚本建模＋原创像素材质；建筑／人物由 AI 设计图辅助 |
| `client/assets/art/village/fence_wood_4m.glb` | 1,000 | 1024×1024, 512×512 PNG | — | 4.3.2 | 原创脚本建模＋原创像素材质；建筑／人物由 AI 设计图辅助 |
| `client/assets/art/village/props_set.glb` | 7,028 | 1024×1024, 512×512 PNG | — | 4.3.2 | 原创脚本建模＋原创像素材质；建筑／人物由 AI 设计图辅助 |
| `client/assets/art/characters/player_novice.glb` | 14,773 | 128×128, 512×512 PNG | attack_basic 0.70s、death 1.00s、hit 0.30s、idle 2.00s、walk 0.40s | 4.3.2 | 原创脚本建模＋原创像素材质；建筑／人物由 AI 设计图辅助 |
| `client/assets/art/textures/ground_dirt_road.png` | — | 2048×2048 PNG | — | 4.3.2 材质源 | 原创确定性无缝像素绘制／法线生成 |
| `client/assets/art/textures/ground_dirt_road_normal.png` | — | 2048×2048 PNG | — | 4.3.2 材质源 | 原创确定性无缝像素绘制／法线生成 |
| `client/assets/art/textures/ground_grass.png` | — | 2048×2048 PNG | — | 4.3.2 材质源 | 原创确定性无缝像素绘制／法线生成 |
| `client/assets/art/textures/ground_grass_normal.png` | — | 2048×2048 PNG | — | 4.3.2 材质源 | 原创确定性无缝像素绘制／法线生成 |
| `client/assets/art/textures/ground_stone_paving.png` | — | 2048×2048 PNG | — | 4.3.2 材质源 | 原创确定性无缝像素绘制／法线生成 |
| `client/assets/art/textures/ground_stone_paving_normal.png` | — | 2048×2048 PNG | — | 4.3.2 材质源 | 原创确定性无缝像素绘制／法线生成 |

`props_set.glb` 总计 7,028 三角面，各件分别为：木箱 1,444，木桶 584，灯笼杆 624，水井 2,908，摊位桌 1,468。每件均低于 3,000。

## 源文件、设计图、脚本和检查资料

所有 `.blend` 打包了使用中的 PNG；脚本和原始材质 PNG 一并提供，便于修改。重建命令见 [源文件说明](../../assets/source/README.md)。建模前设计图的同名 `-prompt.md` 保存原始提示词；实际模型渲染另放在源文件目录，避免混淆。

| 文件 | 三角面／类型 | 图像尺寸 | 动画 | Blender | 生成方式／用途 |
|---|---|---|---|---|---|
| `assets/concept-art/sample/building-forge-front-prompt.md` | — | — | — | — | 同名设计图原始提示词／设计记录 |
| `assets/concept-art/sample/building-forge-front.png` | — | 1536×1024 | — | — | 建模前 AI 辅助设计 |
| `assets/concept-art/sample/building-forge-oblique-prompt.md` | — | — | — | — | 同名设计图原始提示词／设计记录 |
| `assets/concept-art/sample/building-forge-oblique.png` | — | 1448×1086 | — | — | 建模前 AI 辅助设计 |
| `assets/concept-art/sample/building-forge-side-prompt.md` | — | — | — | — | 同名设计图原始提示词／设计记录 |
| `assets/concept-art/sample/building-forge-side.png` | — | 1536×1024 | — | — | 建模前 AI 辅助设计 |
| `assets/concept-art/sample/building-hall-front-prompt.md` | — | — | — | — | 同名设计图原始提示词／设计记录 |
| `assets/concept-art/sample/building-hall-front.png` | — | 1564×1006 | — | — | 建模前 AI 辅助设计 |
| `assets/concept-art/sample/building-hall-oblique-prompt.md` | — | — | — | — | 同名设计图原始提示词／设计记录 |
| `assets/concept-art/sample/building-hall-oblique.png` | — | 1536×1024 | — | — | 建模前 AI 辅助设计 |
| `assets/concept-art/sample/building-hall-side-prompt.md` | — | — | — | — | 同名设计图原始提示词／设计记录 |
| `assets/concept-art/sample/building-hall-side.png` | — | 1536×1024 | — | — | 建模前 AI 辅助设计 |
| `assets/concept-art/sample/player-turnaround-prompt.md` | — | — | — | — | 同名设计图原始提示词／设计记录 |
| `assets/concept-art/sample/player-turnaround.png` | — | 1536×1024 | — | — | 建模前 AI 辅助设计 |
| `assets/concept-art/sample/props-design-prompt.md` | — | — | — | — | 同名设计图原始提示词／设计记录 |
| `assets/concept-art/sample/props-design.png` | — | 1600×1100 | — | — | 原创手绘结构设计 |
| `assets/concept-art/sample/props-design.svg` | — | — | — | — | 原创矢量设计图 |
| `assets/concept-art/sample/village-layout-prompt.md` | — | — | — | — | 同名设计图原始提示词／设计记录 |
| `assets/concept-art/sample/village-layout.png` | — | 1536×1024 | — | — | 建模前 AI 辅助设计 |
| `assets/source/README.md` | — | — | — | — | 制作说明／验证记录 |
| `assets/source/assemble_sample_village.py` | — | — | — | 4.3.2 或系统 Python | 可复现制作／导出／验证脚本 |
| `assets/source/buildings/README.md` | — | — | — | — | 制作说明／验证记录 |
| `assets/source/buildings/build_buildings.py` | — | — | — | 4.3.2 或系统 Python | 可复现制作／导出／验证脚本 |
| `assets/source/buildings/building_forge.blend` | 可编辑源场景 | — | — | 4.3.2 | 原创模型／打包材质／摄影机与灯光 |
| `assets/source/buildings/building_hall.blend` | 可编辑源场景 | — | — | 4.3.2 | 原创模型／打包材质／摄影机与灯光 |
| `assets/source/buildings/clean_sources.py` | — | — | — | 4.3.2 或系统 Python | 可复现制作／导出／验证脚本 |
| `assets/source/buildings/make_building_textures.py` | — | — | — | 4.3.2 或系统 Python | 可复现制作／导出／验证脚本 |
| `assets/source/buildings/render_exported_buildings.py` | — | — | — | 4.3.2 或系统 Python | 可复现制作／导出／验证脚本 |
| `assets/source/buildings/renders/building_forge_front.png` | — | 1400×1100 | — | — | 实际模型渲染 |
| `assets/source/buildings/renders/building_forge_oblique.png` | — | 1400×1100 | — | — | 实际模型渲染 |
| `assets/source/buildings/renders/building_forge_side.png` | — | 1400×1100 | — | — | 实际模型渲染 |
| `assets/source/buildings/renders/building_hall_front.png` | — | 1400×1100 | — | — | 实际模型渲染 |
| `assets/source/buildings/renders/building_hall_oblique.png` | — | 1400×1100 | — | — | 实际模型渲染 |
| `assets/source/buildings/renders/building_hall_side.png` | — | 1400×1100 | — | — | 实际模型渲染 |
| `assets/source/buildings/textures/brass_basecolor.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/buildings/textures/brass_normal.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/buildings/textures/brass_orm.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/buildings/textures/cloth_basecolor.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/buildings/textures/cloth_normal.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/buildings/textures/cloth_orm.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/buildings/textures/coal_basecolor.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/buildings/textures/coal_normal.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/buildings/textures/coal_orm.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/buildings/textures/ember_basecolor.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/buildings/textures/ember_normal.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/buildings/textures/ember_orm.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/buildings/textures/iron_basecolor.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/buildings/textures/iron_normal.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/buildings/textures/iron_orm.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/buildings/textures/paper_basecolor.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/buildings/textures/paper_normal.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/buildings/textures/paper_orm.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/buildings/textures/plaster_basecolor.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/buildings/textures/plaster_normal.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/buildings/textures/plaster_orm.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/buildings/textures/roof_basecolor.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/buildings/textures/roof_dark_basecolor.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/buildings/textures/roof_dark_normal.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/buildings/textures/roof_dark_orm.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/buildings/textures/roof_light_basecolor.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/buildings/textures/roof_light_normal.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/buildings/textures/roof_light_orm.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/buildings/textures/roof_normal.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/buildings/textures/roof_orm.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/buildings/textures/roof_warm_basecolor.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/buildings/textures/roof_warm_normal.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/buildings/textures/roof_warm_orm.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/buildings/textures/stone_basecolor.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/buildings/textures/stone_normal.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/buildings/textures/stone_orm.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/buildings/textures/wood_basecolor.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/buildings/textures/wood_normal.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/buildings/textures/wood_orm.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/buildings/validate_buildings.py` | — | — | — | 4.3.2 或系统 Python | 可复现制作／导出／验证脚本 |
| `assets/source/character-attack.png` | — | 720×900 | — | — | 实际模型渲染 |
| `assets/source/character-back.png` | — | 720×900 | — | — | 实际模型渲染 |
| `assets/source/character-death.png` | — | 720×900 | — | — | 实际模型渲染 |
| `assets/source/character-front.png` | — | 720×900 | — | — | 实际模型渲染 |
| `assets/source/character-preview.png` | — | 900×1100 | — | — | 实际模型渲染 |
| `assets/source/character-side.png` | — | 720×900 | — | — | 实际模型渲染 |
| `assets/source/character-walk.png` | — | 720×900 | — | — | 实际模型渲染 |
| `assets/source/character_build.py` | — | — | — | 4.3.2 或系统 Python | 可复现制作／导出／验证脚本 |
| `assets/source/character_novice.blend` | 14,773（角色网格） | — | idle、walk、attack_basic、hit、death | 4.3.2 | 原创模型／打包材质／摄影机与灯光 |
| `assets/source/character_preview.py` | — | — | — | 4.3.2 或系统 Python | 可复现制作／导出／验证脚本 |
| `assets/source/character_textures/binding_basecolor.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/binding_normal.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/binding_orm.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/boots_basecolor.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/boots_normal.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/boots_orm.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/bronze_basecolor.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/bronze_normal.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/bronze_orm.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/eye_basecolor.png` | — | 128×128 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/eye_normal.png` | — | 128×128 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/eye_orm.png` | — | 128×128 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/flax_basecolor.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/flax_normal.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/flax_orm.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/hair_basecolor.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/hair_normal.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/hair_orm.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/hair_ridge_basecolor.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/hair_ridge_normal.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/hair_ridge_orm.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/iris_basecolor.png` | — | 128×128 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/iris_normal.png` | — | 128×128 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/iris_orm.png` | — | 128×128 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/leather_basecolor.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/leather_normal.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/leather_orm.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/skin_basecolor.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/skin_normal.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/skin_orm.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/skin_shadow_basecolor.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/skin_shadow_normal.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/skin_shadow_orm.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/slate_edge_basecolor.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/slate_edge_normal.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/slate_edge_orm.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/slate_woven_basecolor.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/slate_woven_normal.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/slate_woven_orm.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/steel_basecolor.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/steel_normal.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/steel_orm.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/trouser_basecolor.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/trouser_normal.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_textures/trouser_orm.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/character_validate.py` | — | — | — | 4.3.2 或系统 Python | 可复现制作／导出／验证脚本 |
| `assets/source/fence-layout-validation.json` | — | — | — | — | 制作说明／验证记录 |
| `assets/source/ground-material-preview.png` | — | 1800×1000 | — | — | 实际模型渲染 |
| `assets/source/ground-preview.png` | — | 2196×1540 | — | — | 实际模型渲染 |
| `assets/source/ground_build_blender.py` | — | — | — | 4.3.2 或系统 Python | 可复现制作／导出／验证脚本 |
| `assets/source/ground_generate.py` | — | — | — | 4.3.2 或系统 Python | 可复现制作／导出／验证脚本 |
| `assets/source/ground_materials.blend` | 可编辑源场景 | — | — | 4.3.2 | 原创模型／打包材质／摄影机与灯光 |
| `assets/source/ground_validate_blender.py` | — | — | — | 4.3.2 或系统 Python | 可复现制作／导出／验证脚本 |
| `assets/source/package_sample_village.py` | — | — | — | 4.3.2 或系统 Python | 可复现制作／导出／验证脚本 |
| `assets/source/props/README.md` | — | — | — | — | 制作说明／验证记录 |
| `assets/source/props/generate_props.py` | — | — | — | 4.3.2 或系统 Python | 可复现制作／导出／验证脚本 |
| `assets/source/props/props-metrics.json` | — | — | — | — | 制作说明／验证记录 |
| `assets/source/props/props.blend` | 可编辑源场景 | — | — | 4.3.2 | 原创模型／打包材质／摄影机与灯光 |
| `assets/source/props/props_textures.py` | — | — | — | 4.3.2 或系统 Python | 可复现制作／导出／验证脚本 |
| `assets/source/props/renders/props-assets-detail-prompt.md` | — | — | — | — | 同名设计图原始提示词／设计记录 |
| `assets/source/props/renders/props-assets-detail.png` | — | 1800×1100 | — | — | 实际模型渲染 |
| `assets/source/props/renders/props-assets-overview-prompt.md` | — | — | — | — | 同名设计图原始提示词／设计记录 |
| `assets/source/props/renders/props-assets-overview.png` | — | 1600×1200 | — | — | 实际模型渲染 |
| `assets/source/props/textures/bark_albedo.png` | — | 1024×1024 | — | — | 原创程序绘制材质 PNG |
| `assets/source/props/textures/bark_normal.png` | — | 1024×1024 | — | — | 原创程序绘制材质 PNG |
| `assets/source/props/textures/bark_roughness.png` | — | 1024×1024 | — | — | 原创程序绘制材质 PNG |
| `assets/source/props/textures/ceramic_albedo.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/props/textures/ceramic_normal.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/props/textures/ceramic_roughness.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/props/textures/cloth_albedo.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/props/textures/cloth_normal.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/props/textures/cloth_roughness.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/props/textures/leaf_mass_0_albedo.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/props/textures/leaf_mass_0_normal.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/props/textures/leaf_mass_0_roughness.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/props/textures/leaf_mass_1_albedo.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/props/textures/leaf_mass_1_normal.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/props/textures/leaf_mass_1_roughness.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/props/textures/leaf_mass_2_albedo.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/props/textures/leaf_mass_2_normal.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/props/textures/leaf_mass_2_roughness.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/props/textures/leaf_mass_3_albedo.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/props/textures/leaf_mass_3_normal.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/props/textures/leaf_mass_3_roughness.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/props/textures/leaves_0_albedo.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/props/textures/leaves_1_albedo.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/props/textures/leaves_2_albedo.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/props/textures/leaves_3_albedo.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/props/textures/metal_albedo.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/props/textures/metal_normal.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/props/textures/metal_roughness.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/props/textures/paper_albedo.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/props/textures/paper_normal.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/props/textures/paper_roughness.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/props/textures/rope_albedo.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/props/textures/rope_normal.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/props/textures/rope_roughness.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/props/textures/stone_albedo.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/props/textures/stone_normal.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/props/textures/stone_roughness.png` | — | 512×512 | — | — | 原创程序绘制材质 PNG |
| `assets/source/props/textures/wood_albedo.png` | — | 1024×1024 | — | — | 原创程序绘制材质 PNG |
| `assets/source/props/textures/wood_light_albedo.png` | — | 1024×1024 | — | — | 原创程序绘制材质 PNG |
| `assets/source/props/textures/wood_light_normal.png` | — | 1024×1024 | — | — | 原创程序绘制材质 PNG |
| `assets/source/props/textures/wood_light_roughness.png` | — | 1024×1024 | — | — | 原创程序绘制材质 PNG |
| `assets/source/props/textures/wood_normal.png` | — | 1024×1024 | — | — | 原创程序绘制材质 PNG |
| `assets/source/props/textures/wood_roughness.png` | — | 1024×1024 | — | — | 原创程序绘制材质 PNG |
| `assets/source/props/validate_props.py` | — | — | — | 4.3.2 或系统 Python | 可复现制作／导出／验证脚本 |
| `assets/source/renders/01-village-exported-glbs.png` | — | 1280×800 | — | — | 实际模型渲染 |
| `assets/source/renders/02-hall-exported-glb.png` | — | 1280×800 | — | — | 实际模型渲染 |
| `assets/source/renders/03-forge-exported-glb.png` | — | 1280×800 | — | — | 实际模型渲染 |
| `assets/source/renders/04-character-exported-idle.png` | — | 1280×800 | — | — | 实际模型渲染 |
| `assets/source/renders/05-character-exported-walk.png` | — | 1280×800 | — | — | 实际模型渲染 |
| `assets/source/renders/06-character-exported-attack-025.png` | — | 1280×800 | — | — | 实际模型渲染 |
| `assets/source/renders/07-character-exported-death-100.png` | — | 1280×800 | — | — | 实际模型渲染 |
| `assets/source/sample-village-layout.json` | — | — | — | — | 制作说明／验证记录 |
| `assets/source/sample_village_godot_check.gd` | — | — | — | — | 隔离预览／导入配置／引擎元数据 |
| `assets/source/validate_sample_assets.py` | — | — | — | 4.3.2 或系统 Python | 可复现制作／导出／验证脚本 |
| `client/art_preview/sample_village_preview.gd` | — | — | — | — | 隔离预览／导入配置／引擎元数据 |
| `client/art_preview/sample_village_preview.gd.uid` | — | — | — | — | 隔离预览／导入配置／引擎元数据 |
| `client/art_preview/sample_village_preview.tscn` | — | — | — | — | 隔离预览／导入配置／引擎元数据 |
| `client/assets/art/characters/player_novice.glb.import` | — | — | — | — | 隔离预览／导入配置／引擎元数据 |
| `client/assets/art/textures/ground_dirt_road.png.import` | — | — | — | — | 隔离预览／导入配置／引擎元数据 |
| `client/assets/art/textures/ground_dirt_road_normal.png.import` | — | — | — | — | 隔离预览／导入配置／引擎元数据 |
| `client/assets/art/textures/ground_grass.png.import` | — | — | — | — | 隔离预览／导入配置／引擎元数据 |
| `client/assets/art/textures/ground_grass_normal.png.import` | — | — | — | — | 隔离预览／导入配置／引擎元数据 |
| `client/assets/art/textures/ground_stone_paving.png.import` | — | — | — | — | 隔离预览／导入配置／引擎元数据 |
| `client/assets/art/textures/ground_stone_paving_normal.png.import` | — | — | — | — | 隔离预览／导入配置／引擎元数据 |
| `client/assets/art/village/building_forge.glb.import` | — | — | — | — | 隔离预览／导入配置／引擎元数据 |
| `client/assets/art/village/building_hall.glb.import` | — | — | — | — | 隔离预览／导入配置／引擎元数据 |
| `client/assets/art/village/fence_wood_4m.glb.import` | — | — | — | — | 隔离预览／导入配置／引擎元数据 |
| `client/assets/art/village/props_set.glb.import` | — | — | — | — | 隔离预览／导入配置／引擎元数据 |
| `client/assets/art/village/tree_old.glb.import` | — | — | — | — | 隔离预览／导入配置／引擎元数据 |
| `docs/art/buildings-validation.json` | — | — | — | — | 制作说明／验证记录 |
| `docs/art/character-validation.json` | — | — | — | — | 制作说明／验证记录 |
| `docs/art/ground-validation.json` | — | — | — | — | 制作说明／验证记录 |
| `docs/art/props-validation.json` | — | — | — | — | 制作说明／验证记录 |
| `docs/art/sample-village-files.json` | — | — | — | — | 制作说明／验证记录 |
| `docs/art/sample-village-godot-validation.json` | — | — | — | — | 制作说明／验证记录 |
| `docs/art/sample-village-manifest.md` | — | — | — | — | 制作说明／验证记录 |
| `docs/art/sample-village-validation.json` | — | — | — | — | 制作说明／验证记录 |

## 授权与来源

全部交付模型、贴图、动画及设计图均为本任务原创建模、程序绘制或原创 AI 辅助设计，没有引入第三方模型、HDRI、贴图、笔刷或收费资源，因此没有第三方 CC0 署名项。参考是仓库已有的两张概念图。交付内容随仓库 MIT 许可提供，AI 设计图不构成独占版权或商标保证。

Godot 内嵌图像导入模式的实现依据：[官方导入器源码](https://github.com/godotengine/godot/blob/master/modules/gltf/editor/editor_scene_importer_gltf.cpp)。软件本身不作为第三方美术资源随包分发。

文件大小和 SHA-256 见 `sample-village-files.json`。该哈希索引不计算自身哈希。ZIP 包只含本样板新增的仓库相对路径，不含 `.git`、缓存、备份或普通构建日志。

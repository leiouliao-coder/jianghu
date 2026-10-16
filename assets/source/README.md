# 河湾镇 A 区美术样板源文件

本目录是原创风格化游戏资产的可修改源文件。所有运行时 GLB 内嵌 PNG 材质贴图；每份模型 `.blend` 均保留可编辑网格、材质和已打包的贴图。人物保留人形骨架、蒙皮及五个动作。美术模型采用干净的低面数风格，AI 设计图仅用于前期美术方向，不能替代实际模型预览。

## 重新生成

在仓库根目录运行。建模和导出版本为 Blender 4.3.2；系统 Python 需要 NumPy、Pillow，地面生成还需要 SciPy。可用 CPU 后台渲染，不依赖 HDRI 或第三方模型、贴图、笔刷。

```sh
# 两栋建筑
python assets/source/buildings/make_building_textures.py
blender --background --threads 6 --python assets/source/buildings/build_buildings.py
blender --background --threads 4 --python assets/source/buildings/validate_buildings.py

# 古树、围栏、五件道具
python assets/source/props/props_textures.py
blender --background --threads 6 --python assets/source/props/generate_props.py
blender --background --threads 4 --python assets/source/props/validate_props.py

# 新手角色、贴图、骨架、动作
blender --background --threads 6 --python assets/source/character_build.py
blender --background --threads 4 --python assets/source/character_validate.py

# 三组无缝地面贴图
python assets/source/ground_generate.py
blender --background --threads 4 --python assets/source/ground_build_blender.py
blender --background assets/source/ground_materials.blend --threads 4 --python assets/source/ground_validate_blender.py

# 独立检查最终 GLB，并由导出的 GLB 组装实际模型预览
blender --background --threads 4 --python assets/source/validate_sample_assets.py
blender --background --threads 6 --python assets/source/assemble_sample_village.py
```

每个专项目录、脚本及验证 JSON 保留更详细的制作和检查信息。`renders/` 是实际 GLB 重新导入后的 Blender 渲染，不是 Godot 实机截图。`../concept-art/sample/` 保存建模前设计图及提示词。

## 坐标与布局

- 1 Blender 单位 = 1 米，X 向东、Y 向北、Z 向上，建筑正门与人物正面朝 −Y
- GLB 使用 +Y Up；Godot 映射为 `(x, z, -y)`，因此正面朝 Godot +Z
- 建筑原点为占地中心、地面高度，14 × 14 米基座和实体墙边界对齐服务端
- 客栈在 Blender `(-15, 15, 0)`，铁匠铺在 `(15, -15, 0)`；精确说明见 `sample-village-layout.json`
- `props_set.glb` 的五个对象采用分开展示位置；单独摆放时重置所选根对象的平移
- 古树仅树干应参与阻挡；树冠不能生成完整网格碰撞

## 人物接入

`idle`、`walk`、`attack_basic`、`hit`、`death` 为准确的 glTF 动画名。`idle` 与 `walk` 在引擎中设置循环；其他动作不循环，`death` 结束时保持最后一帧。

`walk` 是根运动：0.4 秒位移 1.6 米，即 4 米/秒。接入时提取并消费 `root` 骨骼的根运动，或移除该轨的平移后使用服务端 4 米/秒移动。不要把根运动再叠加到已有 4 米/秒位移。足底检查基于骨架动画和静止地面，联网插值与碰撞仍需本机实机验证。

人物导入采样为 60 fps，`attack_basic` 长 0.70 秒，接触时间 0.25 秒。以秒为准设置游戏命中事件；glTF 不定义 Godot 游戏逻辑事件。独立剑对象可以按装备状态隐藏或替换。

## 隔离预览

```sh
godot --path client res://art_preview/sample_village_preview.tscn
```

Tab 切换全景／人物，1–5 播放五个动作，R 重播。该预览不连接服务器，不修改现有地图构建器、碰撞或战斗逻辑。

Godot 4.6.3 Compatibility 已做资源导入与无界面场景加载检查。Godot 4.7 实机、手机帧率、联机走路、防穿墙、命中判定及任务书要求的四张游戏实机截图，仍须本机接入验收。不要把源文件渲染作为这些验收已通过的证据。

## 许可与资源

全部模型、贴图、动画和设计图为本任务原创制作或原创生成，没有引入第三方美术资源。随仓库根目录 MIT 许可提供；AI 设计图不构成独占版权保证。运行制作工具本身不属于随包第三方美术资源。

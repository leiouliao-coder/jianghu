"""Pack the original ground paintings into an editable Blender material studio.
Run: blender --background --python assets/source/ground_build_blender.py
The source uses native Principled BSDF, texture and normal-map nodes only.
"""
from pathlib import Path
import bpy, math
from mathutils import Vector
ROOT=Path(__file__).resolve().parents[2]
SOURCE=ROOT/'assets/source'; TEX=ROOT/'client/assets/art/textures'
bpy.ops.object.select_all(action='SELECT'); bpy.ops.object.delete(use_global=False)
for block in list(bpy.data.materials): bpy.data.materials.remove(block)
for block in list(bpy.data.images):
    if block.name not in {'Render Result','Viewer Node'}: bpy.data.images.remove(block)
bpy.context.preferences.filepaths.save_version=0
scene=bpy.context.scene
scene.render.engine='CYCLES'; scene.cycles.samples=48; scene.cycles.use_denoising=False
scene.render.resolution_x=1800; scene.render.resolution_y=1000; scene.render.resolution_percentage=100
scene.view_settings.view_transform='AgX'
scene.world.color=(.22,.22,.22)
scene.world.use_nodes=True
scene.world.node_tree.nodes['Background'].inputs['Color'].default_value=(.64,.72,.80,1)
scene.world.node_tree.nodes['Background'].inputs['Strength'].default_value=.5
scene.render.image_settings.file_format='PNG'
scene.render.film_transparent=False

def solid(name,color,roughness=1):
    mat=bpy.data.materials.new(name);mat.use_nodes=True
    p=mat.node_tree.nodes.get('Principled BSDF');p.inputs['Base Color'].default_value=(*color,1);p.inputs['Roughness'].default_value=roughness
    return mat
side=solid('Swatch edge | studio only',(.048,.068,.061))
floor=solid('Studio backdrop',(.065,.083,.075))
letter=solid('Ivory lettering',(.64,.69,.59))
for i,(stem,label) in enumerate([('ground_stone_paving','RIVER STONE'),('ground_dirt_road','WORN LOAM'),('ground_grass','JADE MEADOW')]):
    mat=bpy.data.materials.new(stem);mat.use_nodes=True
    mat['authoring']='Original deterministic procedural painting, seed 41873'
    mat['tile_metres']=4.0;mat['normal_convention']='OpenGL +Y'
    nodes=mat.node_tree.nodes;links=mat.node_tree.links
    p=nodes.get('Principled BSDF');p.inputs['Roughness'].default_value=.92
    p.inputs['Metallic'].default_value=0
    uv=nodes.new('ShaderNodeTexCoord');uv.location=(-780,0)
    c=nodes.new('ShaderNodeTexImage');c.name='2048 px painted base color';c.location=(-550,150)
    c.image=bpy.data.images.load(str(TEX/f'{stem}.png'));c.image.colorspace_settings.name='sRGB';c.image.pack();c.extension='REPEAT'
    n=nodes.new('ShaderNodeTexImage');n.name='2048 px OpenGL normal';n.location=(-550,-160)
    n.image=bpy.data.images.load(str(TEX/f'{stem}_normal.png'));n.image.colorspace_settings.name='Non-Color';n.image.pack();n.extension='REPEAT'
    bump=nodes.new('ShaderNodeNormalMap');bump.location=(-220,-130);bump.inputs['Strength'].default_value=1
    links.new(uv.outputs['UV'],c.inputs['Vector']);links.new(uv.outputs['UV'],n.inputs['Vector'])
    links.new(c.outputs['Color'],p.inputs['Base Color']);links.new(n.outputs['Color'],bump.inputs['Color']);links.new(bump.outputs['Normal'],p.inputs['Normal'])
    p.location=(20,130);nodes.get('Material Output').location=(360,130)
    bpy.ops.mesh.primitive_plane_add(size=8,location=(i*9.2,0,.15));o=bpy.context.object;o.name=f'{stem} | 8m swatch | 2x2 tiles';o.data.materials.append(mat)
    for v in o.data.uv_layers.active.data: v.uv*=2
    bpy.ops.mesh.primitive_cube_add(size=1,location=(i*9.2,0,.035));o=bpy.context.object;o.name=f'{label} base | studio only';o.dimensions=(8,8,.2);bpy.ops.object.transform_apply(location=False,rotation=False,scale=True);o.data.materials.append(side)
    bevel=o.modifiers.new('Soft presentation edge','BEVEL');bevel.width=.04;bevel.segments=3
    bpy.ops.object.text_add(location=(i*9.2,-4.7,.02));text=bpy.context.object;text.name=f'{label} caption';text.data.body=label;text.data.align_x='CENTER';text.data.size=.48;text.data.extrude=0;text.data.materials.append(letter)
    bpy.ops.object.text_add(location=(i*9.2,-5.3,.02));text=bpy.context.object;text.data.body='4m tile / 2048px / OpenGL normal';text.data.align_x='CENTER';text.data.size=.21;text.data.materials.append(letter)
bpy.ops.mesh.primitive_plane_add(size=200,location=(9,0,-.08));bpy.context.object.name='Studio ground';bpy.context.object.data.materials.append(floor)
def area(name,loc,power,size,color):
    bpy.ops.object.light_add(type='AREA',location=loc);o=bpy.context.object;o.name=name;o.data.energy=power;o.data.shape='DISK';o.data.size=size;o.data.color=color;o.rotation_euler=(Vector((9,0,0))-o.location).to_track_quat('-Z','Y').to_euler()
area('Broad warm daylight',(1,-7,17),2600,14,(1,.90,.74))
area('Sky fill',(18,8,15),1900,12,(.72,.86,1))
bpy.ops.object.camera_add(location=(9.2,-18,28));cam=bpy.context.object;cam.name='Ground material inspection camera';cam.rotation_euler=(Vector((9.2,-.2,0))-cam.location).to_track_quat('-Z','Y').to_euler();cam.data.type='ORTHO';cam.data.ortho_scale=30;scene.camera=cam
note=bpy.data.texts.new('GROUND MATERIAL README')
note.write('HE WAN VILLAGE — ORIGINAL GROUND MATERIALS\n\nThree 2048x2048 base colors and tangent-space OpenGL normals.\nAll PNG images are packed into this file. No external textures, HDRIs, or brushes.\nEach material repeats every 4 metres. Each source swatch is 8x8m with UV range 0–2.\nUse sRGB for base color and Non-Color/linear for normal maps.\nGodot: StandardMaterial3D, Metallic=0, Roughness=0.92, normal_enabled=true.\nThe only image-generating source is ground_generate.py (seed 41873).\nThe normal maps are derived from periodic physical-height fields; positive green points up UV.\nOpposite PNG edges are exactly equal; ground-validation.json includes hashes and measurements.\n')
for screen in bpy.data.screens:
    for area_ in screen.areas:
        if area_.type=='VIEW_3D':
            area_.spaces.active.region_3d.view_perspective='CAMERA'
            area_.spaces.active.shading.type='MATERIAL'
bpy.ops.wm.save_as_mainfile(filepath=str(SOURCE/'ground_materials.blend'),compress=True)
scene.render.filepath=str(SOURCE/'ground-material-preview.png')
bpy.ops.render.render(write_still=True)
print('Saved packed ground_materials.blend and ground-material-preview.png')

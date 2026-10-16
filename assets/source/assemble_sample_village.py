"""Assemble and render the SHIPPED GLBs, not the concept images.
blender --background --python assets/source/assemble_sample_village.py
All transforms use Blender X-east/Y-north/Z-up metres. Does not change runtime code.
"""
import bpy, math, json, os, sys
from pathlib import Path
from mathutils import Vector
ROOT=Path(__file__).resolve().parents[2]
requested=[]
for arg in sys.argv:
    if arg.startswith('--shots='):requested=arg.split('=',1)[1].split(',')
ART=ROOT/'client/assets/art';OUT=ROOT/'assets/source/renders';OUT.mkdir(exist_ok=True)
bpy.ops.wm.read_factory_settings(use_empty=True)
S=bpy.context.scene;S.unit_settings.system='METRIC';S.unit_settings.scale_length=1;S.render.fps=60
S.render.engine='CYCLES';S.cycles.samples=64;S.cycles.use_denoising=False;S.cycles.transparent_max_bounces=32
S.render.resolution_x=1280;S.render.resolution_y=800;S.render.resolution_percentage=100
S.render.image_settings.file_format='PNG';S.view_settings.view_transform='AgX'
S.world=bpy.data.worlds.new('Soft blue daylight');S.world.use_nodes=True;S.world.node_tree.nodes['Background'].inputs[0].default_value=(.50,.62,.70,1);S.world.node_tree.nodes['Background'].inputs[1].default_value=.6

def import_asset(relative,loc=(0,0,0),name=None):
    before=set(bpy.data.objects);bpy.ops.import_scene.gltf(filepath=str(ART/relative));objects=list(set(bpy.data.objects)-before)
    roots=[o for o in objects if o.parent not in objects]
    group=bpy.data.objects.new(name or Path(relative).stem,None);S.collection.objects.link(group)
    for o in roots:o.parent=group
    group.location=loc
    return group,objects

def material(kind):
    m=bpy.data.materials.new(kind);m.use_nodes=True;n=m.node_tree.nodes;l=m.node_tree.links;p=n.get('Principled BSDF');p.inputs['Roughness'].default_value=.92
    im=n.new('ShaderNodeTexImage');im.image=bpy.data.images.load(str(ART/f'textures/ground_{kind}.png'));l.new(im.outputs['Color'],p.inputs['Base Color'])
    normal=n.new('ShaderNodeTexImage');normal.image=bpy.data.images.load(str(ART/f'textures/ground_{kind}_normal.png'));normal.image.colorspace_settings.name='Non-Color';nm=n.new('ShaderNodeNormalMap');nm.inputs['Strength'].default_value=.65;l.new(normal.outputs['Color'],nm.inputs['Color']);l.new(nm.outputs['Normal'],p.inputs['Normal']);return m

def plane(name,size,loc,mat,tile=4):
    w,h=size;me=bpy.data.meshes.new(name);me.from_pydata([(-w/2,-h/2,0),(w/2,-h/2,0),(w/2,h/2,0),(-w/2,h/2,0)],[],[(0,1,2,3)]);me.update();o=bpy.data.objects.new(name,me);S.collection.objects.link(o);o.location=loc;me.materials.append(mat);uv=me.uv_layers.new()
    for idx,v in enumerate([(0,0),(w/tile,0),(w/tile,h/tile),(0,h/tile)]):uv.data[idx].uv=v
    return o

ground=material('grass');stone=material('stone_paving');dirt=material('dirt_road')
plane('60m village grassy boundary',(60,60),(0,0,-.045),ground)
plane('50m stone plaza',(50,50),(0,0,-.022),stone)
plane('West 8m dirt approach',(12,8),(-30,0,-.012),dirt)
plane('East 8m dirt approach',(12,8),(30,0,-.012),dirt)
plane('South 8m dirt approach',(8,12),(0,-30,-.012),dirt)
plane('North 6m dirt approach',(6,12),(0,30,-.012),dirt)
hall,_=import_asset('village/building_hall.glb',(-15,15,0),'A-hall @ exact server footprint')
forge,_=import_asset('village/building_forge.glb',(15,-15,0),'A-forge @ exact server footprint')
tree,_=import_asset('village/tree_old.glb',(14,14,0),'Old tree — canopy has no collision')
fence_group,fences=import_asset('village/fence_wood_4m.glb')
fence=[o for o in fences if o.type=='MESH'][0];fence.parent=None;fence.location=(0,0,0)
# Replicate the exact 4m source; end pieces trimmed by local X scale to keep gate widths exact.
def edge_fences(axis,coord,gap):
    for lo,hi in [(-30,-gap/2),(gap/2,30)]:
        start=lo
        while start<hi-.001:
            length=min(4,hi-start);o=fence.copy();o.data=fence.data;S.collection.objects.link(o);mid=start+length/2
            o.name=f'Fence_{axis}_{coord:g}_{mid:g}';o.location=(mid,coord,0) if axis=='X' else (coord,mid,0);o.scale=(length/4,1,1);o.rotation_mode='XYZ';o.rotation_euler=(0,0,0 if axis=='X' else math.pi/2);start+=length
edge_fences('X',-29.85,8);edge_fences('X',29.85,6);edge_fences('Y',-29.85,8);edge_fences('Y',29.85,8)
# Imported glTF objects use quaternion mode: explicitly set XYZ before Euler rotation.
# Check projected WORLD extents so a silently ignored 90-degree rotation cannot recur.
bpy.context.view_layer.update()
fence_checks=[]
for o in S.objects:
    if not o.name.startswith('Fence_'):continue
    bounds=[o.matrix_world@Vector(v) for v in o.bound_box]
    extent=[max(v[i] for v in bounds)-min(v[i] for v in bounds) for i in range(3)]
    axis=o.name.split('_')[1];across=extent[1] if axis=='X' else extent[0]
    assert abs(across-.3)<.00001,(o.name,extent)
    assert abs(extent[2]-1.2)<.00001,(o.name,extent)
    fence_checks.append({'name':o.name,'world_extent_m':extent,'aligned_to_boundary':True})
(ROOT/'assets/source/fence-layout-validation.json').write_text(json.dumps({'pass':True,'fence_segments':fence_checks},indent=2)+'\n')
bpy.data.objects.remove(fence,do_unlink=True);bpy.data.objects.remove(fence_group,do_unlink=True)
prop_group,props=import_asset('village/props_set.glb')
prop_meshes={o.name:o for o in props if o.type=='MESH'}
# All decorative props are kept off the broad cardinal paths and out of the fixed building footprint.
placements={'crate':(-23.5,10,.0),'barrel':(-23.5,12,0),'lantern_pole':(-5.5,-26,0),'well':(-13,-13,0),'market_table':(-13,6,0)}
for key,o in prop_meshes.items():
    o.parent=None
    desired=next((v for k,v in placements.items() if key.startswith(k)),None)
    if desired:o.location=desired
    if key.startswith('lantern_pole'):
        for loc in [(5.5,-26,0),(-5.5,25,0),(5.5,25,0),(-26,5.5,0),(26,5.5,0),(-14,6,0)]:
            copy=o.copy();copy.data=o.data;S.collection.objects.link(copy);copy.location=loc
    if key.startswith('barrel'):
        for loc in [(23.3,-10,0),(23.3,-12,0),(-23.5,14,0)]:
            copy=o.copy();copy.data=o.data;S.collection.objects.link(copy);copy.location=loc
bpy.data.objects.remove(prop_group,do_unlink=True)
character,character_objs=import_asset('characters/player_novice.glb',(0,0,0),'Novice at spawn')
def set_action(name,t):
    for o in character_objs:
        ad=o.animation_data
        if ad:
            ad.action=None
            for tr in ad.nla_tracks:tr.mute=(tr.name!=name)
    S.frame_set(round(t*60))
set_action('idle',0)
# Soft sunlight with restrained amber forge pool.
bpy.ops.object.light_add(type='SUN',location=(-25,-30,55));sun=bpy.context.object;sun.name='Morning sun';sun.data.energy=2.1;sun.data.angle=math.radians(14);sun.rotation_euler=(math.radians(28),math.radians(-25),math.radians(-28))
bpy.ops.object.light_add(type='AREA',location=(0,-20,30));fill=bpy.context.object;fill.data.energy=1700;fill.data.shape='DISK';fill.data.size=30;fill.rotation_euler=(Vector((0,0,0))-fill.location).to_track_quat('-Z','Y').to_euler()
bpy.ops.object.camera_add();cam=bpy.context.object;cam.name='Asset presentation 1280x800';cam.data.type='ORTHO';S.camera=cam

bpy.ops.object.light_add(type='AREA',location=(0,-4,4));portrait_fill=bpy.context.object;portrait_fill.name='Portrait softbox';portrait_fill.data.energy=450;portrait_fill.data.shape='DISK';portrait_fill.data.size=5;portrait_fill.rotation_euler=(Vector((0,0,1))-portrait_fill.location).to_track_quat('-Z','Y').to_euler();portrait_fill.hide_render=True

def shot(name,eye,target,scale):
    if requested and name[:2] not in requested:return
    portrait_fill.hide_render=name[:2] not in ['04','05','06','07']
    cam.location=eye;cam.rotation_euler=(Vector(target)-cam.location).to_track_quat('-Z','Y').to_euler();cam.data.ortho_scale=scale;S.render.filepath=str(OUT/name);bpy.ops.render.render(write_still=True)
shot('01-village-exported-glbs.png',(32,-65,105),(0,0,1.0),118)
shot('02-hall-exported-glb.png',(9,-13,57),(-15,15,4),38)
shot('03-forge-exported-glb.png',(35,-43,51.5),(15,-15,2.5),34)
# Close character view uses exported mesh and exported NLA animation.
shot('04-character-exported-idle.png',(2.6,-4.2,2.1),(0,0,.90),3.5)
set_action('walk',.1);shot('05-character-exported-walk.png',(3,-5,2.4),(0,-.4,.90),3.6)
set_action('attack_basic',.25);shot('06-character-exported-attack-025.png',(3,-4.7,2.6),(0,-.35,.90),3.8)
set_action('death',1.0);shot('07-character-exported-death-100.png',(3,-4.0,3),(0,.5,.5),3.3)
set_action('idle',0)
cam.location=(32,-65,105);cam.rotation_euler=(Vector((0,0,1))-cam.location).to_track_quat('-Z','Y').to_euler();cam.data.ortho_scale=118
# Rebuildable assembly source is this script. Individual packed .blend files remain authoritative.
print('Rendered exported GLBs:',OUT)

"""Independent GLB preview rendering. Uses exact exported files, 55 degree game view.
Usage: blender -b --python render_exported_buildings.py -- building_hall [building_forge]
"""
import bpy,sys,math,os
from pathlib import Path
from mathutils import Vector
ROOT=Path(__file__).resolve().parents[3];OUT=ROOT/'client/assets/art/village';PRE=Path(__file__).resolve().parent/'renders'
names=sys.argv[sys.argv.index('--')+1:] if '--' in sys.argv else ['building_hall','building_forge']
for name in names:
    bpy.ops.wm.read_factory_settings(use_empty=True);bpy.ops.import_scene.gltf(filepath=str(OUT/(name+'.glb')))
    scene=bpy.context.scene;scene.render.engine='CYCLES';scene.cycles.samples=32;scene.cycles.use_denoising=False
    scene.render.resolution_x=1400;scene.render.resolution_y=1100;scene.render.resolution_percentage=100;scene.view_settings.view_transform='AgX'
    w=bpy.data.worlds.new('Preview_world');scene.world=w;w.use_nodes=True;w.node_tree.nodes['Background'].inputs[0].default_value=(.68,.73,.77,1);w.node_tree.nodes['Background'].inputs[1].default_value=.85
    def aim(ob,pt):ob.rotation_euler=(Vector(pt)-ob.location).to_track_quat('-Z','Y').to_euler()
    for loc,energy,size in [((-8,-10,21),2300,12),((10,5,14),1300,10)]:
        bpy.ops.object.light_add(type='AREA',location=loc);ob=bpy.context.object;ob.data.energy=energy;ob.data.size=size;aim(ob,(0,0,3))
    bpy.ops.object.camera_add(location=(20,-27,5+math.tan(math.radians(55))*math.hypot(20,27)));cam=bpy.context.object;cam.data.type='ORTHO';cam.data.ortho_scale=29;aim(cam,(0,0,5));scene.camera=cam
    bpy.ops.mesh.primitive_plane_add(size=2000,location=(0,0,-.015));m=bpy.data.materials.new('Preview_floor');m.diffuse_color=(.42,.43,.38,1);bpy.context.object.data.materials.append(m)
    for view in os.environ.get('VIEWS','oblique').split(','):
        if view=='front':
            z=4.5 if name=='building_hall' else 3
            cam.location=(0,-32,z);cam.data.ortho_scale=20;aim(cam,(0,0,z))
        elif view=='side':
            z=4.5 if name=='building_hall' else 3
            cam.location=(32,0,z);cam.data.ortho_scale=20;aim(cam,(0,0,z))
        else:
            cam.location=(20,-27,5+math.tan(math.radians(55))*math.hypot(20,27));cam.data.ortho_scale=29;aim(cam,(0,0,5))
        scene.render.filepath=str(PRE/(name+'_'+view+'.png'));bpy.ops.render.render(write_still=True)
        print('REIMPORTED_GLB_RENDER',name,scene.render.filepath)


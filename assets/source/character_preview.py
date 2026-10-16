"""Render reproducible character evidence from the packed Blender source."""
import bpy,os
from mathutils import Vector
ROOT=os.path.abspath(os.path.join(os.path.dirname(__file__),'../..'));SRC=os.path.join(ROOT,'assets/source')
bpy.ops.wm.open_mainfile(filepath=os.path.join(SRC,'character_novice.blend'))
s=bpy.context.scene;rig=bpy.data.objects['player_novice'];cam=s.camera;s.render.resolution_x=720;s.render.resolution_y=900;s.cycles.samples=40;s.cycles.use_denoising=False
for tr in rig.animation_data.nla_tracks:tr.mute=True
shots=[('front','idle',1,(0,-5,1.30),(0,0,.88),2.0),('side','idle',1,(5,0,1.30),(0,0,.88),2.0),('back','idle',1,(0,5,1.30),(0,0,.88),2.0),('walk','walk',7,(2.7,-5,2.2),(0,-.4,.80),2.1),('attack','attack_basic',16,(3,-4.3,2.5),(0,-.45,.88),2.2),('death','death',61,(3,-3,3),(0,.65,.15),2.15)]
for name,act,frame,loc,target,scale in shots:
 rig.animation_data.action=bpy.data.actions[act];s.frame_set(frame);cam.location=loc;cam.rotation_euler=(Vector(target)-cam.location).to_track_quat('-Z','Y').to_euler();cam.data.ortho_scale=scale;s.render.filepath=os.path.join(SRC,'character-'+name+'.png');bpy.ops.render.render(write_still=True)

"""Run with Blender: blender -b --python assets/source/character_validate.py.
Parses binary glTF, independently reimports, checks actual tracks and skin transforms.
"""
import bpy,struct,json,os,math
from mathutils import Vector
ROOT=os.path.abspath(os.path.join(os.path.dirname(__file__),'../..'))
file=os.path.join(ROOT,'client/assets/art/characters/player_novice.glb')
data=open(file,'rb').read(); assert data[:4]==b'glTF'
n=struct.unpack_from('<I',data,12)[0];g=json.loads(data[20:20+n]);pos=20+n
binlen,typ=struct.unpack_from('<II',data,pos);binary=data[pos+8:pos+8+binlen]
comps={5120:('b',1),5121:('B',1),5122:('h',2),5123:('H',2),5125:('I',4),5126:('f',4)};cnt={'SCALAR':1,'VEC2':2,'VEC3':3,'VEC4':4,'MAT4':16}
def accessor(i):
 a=g['accessors'][i];v=g['bufferViews'][a['bufferView']];fmt,size=comps[a['componentType']];num=cnt[a['type']];offset=v.get('byteOffset',0)+a.get('byteOffset',0);stride=v.get('byteStride',size*num)
 return [struct.unpack_from('<'+fmt*num,binary,offset+j*stride) for j in range(a['count'])]
clips=[]
for an in g.get('animations',[]):
 times=[t[0] for s in an['samplers'] for t in accessor(s['input'])];clips.append({'name':an['name'],'start':min(times),'end':max(times),'duration':max(times)-min(times),'channels':len(an['channels'])})
tris=0
for m in g['meshes']:
 for p in m['primitives']:
  tris+=g['accessors'][p['indices']]['count']//3 if 'indices'in p else g['accessors'][p['attributes']['POSITION']]['count']//3
images=[]
for im in g.get('images',[]):
 v=g['bufferViews'][im['bufferView']];b=binary[v.get('byteOffset',0):v.get('byteOffset',0)+v['byteLength']];assert b[:8]==b'\x89PNG\r\n\x1a\n';w,h=struct.unpack_from('>II',b,16);images.append({'name':im.get('name'),'mimeType':im.get('mimeType'),'width':w,'height':h,'embedded':True})
expected={'idle':2.,'walk':.4,'attack_basic':.7,'hit':.3,'death':1.}
assert {a['name'] for a in clips}==set(expected),(clips,'Unexpected clip set')
for a in clips: assert abs(a['duration']-expected[a['name']])<1e-5 and abs(a['start'])<1e-7,a
assert tris<=15000,tris
assert all(i['width']<=2048 and i['height']<=2048 for i in images)
assert len(g['skins'])>=1
assert all('JOINTS_0'in p['attributes'] and 'WEIGHTS_0'in p['attributes'] for m in g['meshes'] for p in m['primitives'])
# Read actual root motion samples in GLB, not source script assertions.
walk=next(a for a in g['animations'] if a['name']=='walk');rootidx=next(i for i,n in enumerate(g['nodes']) if n.get('name')=='root')
ch=next(c for c in walk['channels'] if c['target']['node']==rootidx and c['target']['path']=='translation');sam=walk['samplers'][ch['sampler']];times=[v[0] for v in accessor(sam['input'])];vals=accessor(sam['output']);speeds=[]
for i in range(len(times)-1):speeds.append(abs((vals[i+1][2]-vals[i][2])/(times[i+1]-times[i])))
# Coordinates converted +Y up by glTF: forward movement is +Z.
assert max(abs(v-4) for v in speeds)<1e-4,speeds
# Reimport into factory empty Blender and inspect actions/rig/meshes.
bpy.ops.wm.read_factory_settings(use_empty=True);bpy.ops.import_scene.gltf(filepath=file)
rig=next(o for o in bpy.context.scene.objects if o.type=='ARMATURE');meshes=[o for o in bpy.context.scene.objects if o.type=='MESH']
print('REIMPORT_MESHES',[(o.name,len(o.data.polygons)) for o in meshes]);assert len(g['meshes'])==2
assert any(o.name=='Sword_Independent' for o in meshes)
scene=bpy.context.scene;scene.render.fps=60
# Blender importer may attach the action through its NLA track, identify by action names.
actions={a.name.split('_')[0]:a for a in bpy.data.actions}
reimport_clips=[a.name for a in bpy.data.actions]
# Validate the native source actual world foot matrices in stance, and weapon contact.
bpy.ops.wm.open_mainfile(filepath=os.path.join(ROOT,'assets/source/character_novice.blend'));scene=bpy.context.scene;rig=bpy.data.objects['player_novice']
for tr in rig.animation_data.nla_tracks:tr.mute=True
rig.animation_data.action=bpy.data.actions['walk'];history={};residual=[];lengths=[]
for f in range(1,26):
 scene.frame_set(f);bpy.context.view_layer.update();t=(f-1)/60;p=t/.4
 for side,phase in [('L',0),('R',.5)]:
  v=rig.matrix_world@rig.pose.bones['foot.'+side].head;phase_total=p+phase;cyc=math.floor(phase_total);q=phase_total-cyc
  if q<.5-1e-7:
   key=(side,cyc)
   if key not in history:history[key]=v.copy()
   residual.append((v-history[key]).length)
rig.animation_data.action=bpy.data.actions['attack_basic'];reach=[];tip_rest=Vector((-.218,-.01,1.085))+(Vector((-.27,.025,.4))-Vector((-.218,-.01,1.085))).normalized()*.68
rest=rig.data.bones['sword'].matrix_local
for f in range(1,44):
 scene.frame_set(f);bpy.context.view_layer.update();tip=rig.pose.bones['sword'].matrix@rest.inverted()@tip_rest;reach.append((f,float(-tip.y),list(tip)))
peak=max(reach,key=lambda t:t[1]);assert peak[0]==16,peak
rig.animation_data.action=bpy.data.actions['death'];scene.frame_set(61);bpy.context.view_layer.update();head=rig.pose.bones['head'].head.copy();root=rig.pose.bones['root'].rotation_quaternion.to_euler();death_height=float(head.z)
# Every authored limb chain stays connected, including the maximum sword reach.
max_joint_gap=0.0
for action_name,end in [('walk',25),('attack_basic',43)]:
 rig.animation_data.action=bpy.data.actions[action_name]
 for f in range(1,end+1):
  scene.frame_set(f);bpy.context.view_layer.update()
  for side in ['L','R']:
   for a,b in [('upper_arm.','forearm.'),('forearm.','hand.'),('thigh.','shin.'),('shin.','foot.')]:
    gap=(rig.pose.bones[a+side].tail-rig.pose.bones[b+side].head).length;max_joint_gap=max(max_joint_gap,gap)
assert max_joint_gap<.001,max_joint_gap
rig.animation_data.action=bpy.data.actions['death'];scene.frame_set(52);bpy.context.view_layer.update();held={p.name:p.matrix.copy() for p in rig.pose.bones};scene.frame_set(61);bpy.context.view_layer.update();hold_error=max(abs(p.matrix[r][c]-held[p.name][r][c]) for p in rig.pose.bones for r in range(4) for c in range(4));assert hold_error<1e-6
path=os.path.join(ROOT,'docs/art/character-validation.json');record=json.load(open(path));record['verified_glb']={'bytes':len(data),'triangles':tris,'animations':clips,'skins':len(g['skins']),'images':images,'meshes':[m.get('name') for m in g['meshes']],'root_motion_speed_min':min(speeds),'root_motion_speed_max':max(speeds),'reimport_success':True,'reimport_actions':reimport_clips};record['verified_source_motion']={'max_stance_ankle_drift_m':max(residual),'attack_peak_extension_frame':peak[0],'attack_peak_seconds':(peak[0]-1)/60,'attack_tip_blender':peak[2],'death_head_height_m':death_height,'death_lies_horizontal':abs(rig.pose.bones['head'].y_axis.z)<.05,'death_final_hold_matrix_error':hold_error,'max_limb_joint_gap_m':max_joint_gap}
json.dump(record,open(path,'w'),indent=2);print(json.dumps({'triangles':tris,'clips':clips,'max_stance_drift':max(residual),'attack_peak':peak,'reimport':True},indent=2))

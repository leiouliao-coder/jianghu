"""Independent asset acceptance checks. Run with Blender 4.3.2:
blender --background --python assets/source/validate_sample_assets.py
Reimports every shipped GLB into a clean scene; no gameplay claims.
"""
from pathlib import Path
import json, struct, math, bpy, hashlib, tempfile
from mathutils import Vector
ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'docs/art/sample-village-validation.json'
ASSETS=[('village','building_hall',20000),('village','building_forge',20000),('village','tree_old',3000),('village','fence_wood_4m',3000),('village','props_set',15000),('characters','player_novice',15000)]
def glb(path):
    data=path.read_bytes(); magic,version,total=struct.unpack_from('<III',data)
    assert magic==0x46546c67 and version==2 and total==len(data)
    length,typ=struct.unpack_from('<II',data,12); assert typ==0x4e4f534a
    j=json.loads(data[20:20+length]); off=20+length
    n,t=struct.unpack_from('<II',data,off); b=data[off+8:off+8+n]
    return j,b

def bounds(objects):
    points=[o.matrix_world@Vector(p) for o in objects if o.type=='MESH' for p in o.bound_box]
    if not points:return None
    low=[min(p[i] for p in points) for i in range(3)]; high=[max(p[i] for p in points) for i in range(3)]
    return {'min':low,'max':high,'dimensions':[high[i]-low[i] for i in range(3)]}

def analyze(folder,name,budget):
    path=ROOT/f'client/assets/art/{folder}/{name}.glb'
    raw=path.read_bytes(); digest=hashlib.sha256(raw).hexdigest()
    snapshot=Path(tempfile.gettempdir())/('jianghu-validation-'+name+'.glb'); snapshot.write_bytes(raw)
    j,b=glb(snapshot)
    images=[]
    for img in j.get('images',[]):
        assert 'uri' not in img, f'{name} external image {img}'
        v=j['bufferViews'][img['bufferView']]; payload=b[v.get('byteOffset',0):v.get('byteOffset',0)+v['byteLength']]
        assert payload[:8]==b'\x89PNG\r\n\x1a\n',f'{name}: image is not PNG'
        w,h=struct.unpack_from('>II',payload,16); assert max(w,h)<=2048
        images.append({'name':img.get('name',''),'size':[w,h],'bytes':len(payload),'embedded':True})
    assert images, f'{name}: no embedded image'
    tri=0
    for mesh in j.get('meshes',[]):
        for p in mesh.get('primitives',[]):
            assert p.get('mode',4)==4
            tri+=(j['accessors'][p['indices']]['count'] if 'indices' in p else j['accessors'][p['attributes']['POSITION']]['count'])//3
    assert tri<=budget, f'{name}: {tri} > {budget}'
    anim=[]
    for a in j.get('animations',[]):
        lo=min(j['accessors'][s['input']]['min'][0] for s in a['samplers']);hi=max(j['accessors'][s['input']]['max'][0] for s in a['samplers'])
        anim.append({'name':a['name'],'start_seconds':lo,'end_seconds':hi,'duration_seconds':hi-lo,'channels':len(a['channels'])})
    if name=='player_novice':
        assert sorted(a['name'] for a in anim)==sorted(['idle','walk','attack_basic','hit','death'])
        times={a['name']:a['duration_seconds'] for a in anim}
        for key,dur in [('idle',2.0),('attack_basic',.7),('hit',.3),('death',1.0)]:
            assert abs(times[key]-dur)<.002,f'{key} duration {times[key]}'
        assert j.get('skins'), 'No skin'
    bpy.ops.wm.read_factory_settings(use_empty=True)
    bpy.ops.import_scene.gltf(filepath=str(snapshot))
    bpy.context.view_layer.update()
    helpers={pb.custom_shape for arm in bpy.context.scene.objects if arm.type=='ARMATURE' for pb in arm.pose.bones if pb.custom_shape}
    meshes=[o for o in bpy.context.scene.objects if o.type=='MESH' and o not in helpers]
    imported_tri=sum(len(p.vertices)-2 for o in meshes for p in o.data.polygons)
    assert imported_tri==tri,(name,tri,imported_tri)
    box=bounds(meshes)
    if name=='fence_wood_4m':
        for a,bv in zip(box['dimensions'],[4,.3,1.2]):assert abs(a-bv)<.002,(a,bv)
    if name.startswith('building_'):
        foundation=[o for o in meshes if 'foundation' in o.name.lower()]
        fb=bounds(foundation)
        assert fb and abs(fb['dimensions'][0]-14)<.002 and abs(fb['dimensions'][1]-14)<.002, (name,fb)
        assert max(abs(x) for x in box['min'][:2]+box['max'][:2])<=8.002,(name,box)
    if name=='tree_old':
        radii=[math.hypot(v.co.x,v.co.y) for o in meshes for v in o.data.vertices if v.co.z<1.0]
        assert max(radii)<=1.502,max(radii)
        assert abs(max(box['dimensions'][:2])-12)<.02,box
    roots=[{'name':o.name,'type':o.type,'location':list(o.location),'rotation':list(o.rotation_euler)} for o in bpy.context.scene.objects if not o.parent and o not in helpers]
    groups=[]
    if name=='props_set':
        for o in meshes:
            xs=[v.co.x for v in o.data.vertices];ys=[v.co.y for v in o.data.vertices];zs=[v.co.z for v in o.data.vertices]
            dims=[max(xs)-min(xs),max(ys)-min(ys),max(zs)-min(zs)]
            n=sum(len(p.vertices)-2 for p in o.data.polygons)
            assert dims[0]<=2.001 and dims[1]<=2.001 and n<=3000,(o.name,dims,n)
            groups.append({'name':o.name,'triangles':n,'local_dimensions':dims,'origin_blender':list(o.location)})
    return {'file':str(path.relative_to(ROOT)), 'bytes':len(raw),'sha256':digest,'triangles':tri,'triangle_budget':budget,'embedded_pngs':images,'animations':anim,'skins':len(j.get('skins',[])), 'joints':[len(s['joints']) for s in j.get('skins',[])], 'bounds_blender':box,'root_nodes':roots,'prop_hierarchies':groups,'blender_reimport':'PASS','importer_helper_meshes_excluded':[o.name for o in helpers],'material_count':len(j.get('materials',[])), 'extension_names':j.get('extensionsUsed',[])}

report={'blender_version':bpy.app.version_string,'method':'Independent binary glTF inspection plus clean Blender background re-import per file','assets':[],'errors':[],'godot_4_7_gameplay':'NOT RUN; local integration acceptance required'}
for folder,name,budget in ASSETS:
    try: report['assets'].append(analyze(folder,name,budget))
    except Exception as e:report['errors'].append({'file':name+'.glb','error':str(e)})
report['pass']=not report['errors'];OUT.write_text(json.dumps(report,ensure_ascii=False,indent=2))
print(json.dumps(report,ensure_ascii=False,indent=2))
if report['errors']:raise SystemExit(1)

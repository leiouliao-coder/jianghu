"""Independent background Blender re-import and glTF binary/image validation."""
import bpy,json,struct,math
from pathlib import Path
from mathutils import Vector
ROOT=Path(__file__).resolve().parents[3]; SRC=Path(__file__).resolve().parent; OUT=ROOT/'client/assets/art/village'
metrics=json.loads((SRC/'props-metrics.json').read_text());metrics['checks']={};fail=[]
for file in ['tree_old.glb','fence_wood_4m.glb','props_set.glb']:
    path=OUT/file;data=path.read_bytes();magic,ver,total=struct.unpack_from('<III',data);ln,typ=struct.unpack_from('<II',data,12);doc=json.loads(data[20:20+ln]);bln,btyp=struct.unpack_from('<II',data,20+ln);blob=data[28+ln:28+ln+bln]
    info={'binary_version':ver,'declared_length_matches':total==len(data),'all_images_embedded':all('bufferView' in x and 'uri' not in x for x in doc.get('images',[])),'images':[],'materials':[],'scene_root_count':len(doc['scenes'][doc.get('scene',0)].get('nodes',[])),'extensions_used':doc.get('extensionsUsed',[])}
    for im in doc.get('images',[]):
        bv=doc['bufferViews'][im['bufferView']];img=blob[bv.get('byteOffset',0):bv.get('byteOffset',0)+bv['byteLength']];assert img[:8]==b'\x89PNG\r\n\x1a\n',(file,'non-PNG image');w,h=struct.unpack('>II',img[16:24]);info['images'].append({'name':im.get('name'),'mimeType':im.get('mimeType'),'size':[w,h]});assert max(w,h)<=2048
    for m in doc['materials']:info['materials'].append({'name':m.get('name'),'alphaMode':m.get('alphaMode','OPAQUE'),'doubleSided':m.get('doubleSided',False),'has_base_color_texture':'baseColorTexture' in m.get('pbrMetallicRoughness',{}),'extensions':list(m.get('extensions',{}))})
    bpy.ops.object.select_all(action='SELECT');bpy.ops.object.delete(use_global=False);bpy.ops.import_scene.gltf(filepath=str(path));obs=[o for o in bpy.context.scene.objects if o.type=='MESH'];info['reimport']='PASS';info['reimported_objects']=[]
    for o in obs:
        o.data.calc_loop_triangles();vs=[v.co for v in o.data.vertices];mn=[min(v[i] for v in vs) for i in range(3)];mx=[max(v[i] for v in vs) for i in range(3)];dim=[mx[i]-mn[i] for i in range(3)];tri=len(o.data.loop_triangles);info['reimported_objects'].append({'name':o.name,'triangles':tri,'local_bounds_m':[mn,mx],'dimensions_m':dim,'location_m':list(o.location),'remaining_modifiers':len(o.modifiers),'has_uv':bool(o.data.uv_layers)})
        if tri>3000:fail.append(f'{file}/{o.name}: over 3000 triangles ({tri})')
        if abs(mn[2])>.0001:fail.append(f'{file}/{o.name}: base not at ground ({mn[2]})')
        if file=='props_set.glb' and max(dim[:2])>2.0001:fail.append(f'{o.name}: footprint exceeds 2 m')
        if file=='fence_wood_4m.glb' and any(abs(a-b)>.0001 for a,b in zip(dim,[4,.3,1.2])):fail.append('Fence dimensions not exact')
        if file=='tree_old.glb':
            base=[v for v in vs if v.z<.35];rr=max(math.hypot(v.x,v.y) for v in base);info['max_root_radius_below_0_35_m']=rr
            if rr>1.5001:fail.append(f'Tree roots exceed 1.5 m ({rr})')
    if file=='props_set.glb' and sorted(o.name for o in obs)!=['barrel','crate','lantern_pole','market_table','well']:fail.append('Props roots/names differ')
    metrics['checks'][file]=info
metrics['props_set.glb']['total_triangles']=sum(o['triangles'] for o in metrics['props_set.glb']['objects'])
metrics['validation_status']='PASS' if not fail else 'FAIL';metrics['validation_failures']=fail
metrics['visual_qa']={'source_render_overview':'assets/source/props/renders/props-assets-overview.png','source_render_detail':'assets/source/props/renders/props-assets-detail.png','qa':'Actual Blender geometry rendered; reviewed by asset author. Godot import and in-engine lighting remain for integration acceptance.'}
(ROOT/'docs/art/props-validation.json').write_text(json.dumps(metrics,indent=2));print(json.dumps({'status':metrics['validation_status'],'failures':fail,'files':list(metrics['checks'])},indent=2))
if fail:raise RuntimeError('; '.join(fail))

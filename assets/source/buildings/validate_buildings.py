"""Independent background GLB reimport and embedded-resource validation."""
import bpy,json,struct,math
from pathlib import Path
ROOT=Path(__file__).resolve().parents[3]; OUT=ROOT/'client/assets/art/village'
report_path=ROOT/'docs/art/buildings-validation.json'
report=json.loads(report_path.read_text()) if report_path.exists() else {}
for name in ['building_hall','building_forge']:
    path=OUT/(name+'.glb')
    bpy.ops.wm.read_factory_settings(use_empty=True)
    bpy.ops.import_scene.gltf(filepath=str(path))
    meshes=[o for o in bpy.context.scene.objects if o.type=='MESH']; roots=[o for o in bpy.context.scene.objects if o.parent is None]
    v=[o.matrix_world@v.co for o in meshes for v in o.data.vertices]
    bounds=[[min(x[i] for x in v) for i in range(3)],[max(x[i] for x in v) for i in range(3)]]
    tris=sum(sum(len(p.vertices)-2 for p in o.data.polygons) for o in meshes)
    raw=path.read_bytes();magic,version,length=struct.unpack('<4sII',raw[:12]);jlen,jtype=struct.unpack('<II',raw[12:20]);g=json.loads(raw[20:20+jlen])
    image_info=[]
    for im in bpy.data.images:
        if im.type=='IMAGE':image_info.append({'name':im.name,'size':list(im.size),'packed':bool(im.packed_file)})
    errors=[]
    if magic!=b'glTF' or version!=2 or length!=len(raw):errors.append('Invalid GLB header')
    if tris>20000:errors.append('Triangle budget exceeded')
    if not all('bufferView' in im and im.get('mimeType')=='image/png' for im in g.get('images',[])):errors.append('Nonembedded/nonPNG texture')
    if not all(max(im['size'])<=2048 for im in image_info):errors.append('Texture too large')
    if any(o.modifiers for o in meshes):errors.append('Unapplied modifiers')
    if abs(bounds[0][2])>1e-5:errors.append('Not ground aligned')
    if any(any(abs(a)>1e-5 for a in r.location) for r in roots):errors.append('Nonzero root origin')
    if name=='building_hall':
        structural=[o for o in meshes if o.name.startswith(('Solid_wall','Solid_corner','Hall_foundation'))]
        sv=[o.matrix_world@v.co for o in structural for v in o.data.vertices]
        footprint=[[min(x[i] for x in sv) for i in range(2)],[max(x[i] for x in sv) for i in range(2)]]
        if any(abs(footprint[k][i]-([-7,7][k]))>1e-5 for k in range(2) for i in range(2)):errors.append('Hall solid wall footprint mismatch')
        if any(abs(bounds[k][i])>8.0001 for k in range(2) for i in range(2)):errors.append('Hall eave overhang >1m')
    else:
        footprint=[b[:2] for b in bounds]
        if any(abs(footprint[k][i]-([-7,7][k]))>1e-5 for k in range(2) for i in range(2)):errors.append('Forge occupancy outside 14m')
        if not any(any(n.type=='BSDF_PRINCIPLED' and n.inputs['Emission Strength'].default_value>0 for n in m.node_tree.nodes) for m in bpy.data.materials if m.use_nodes):errors.append('Missing emissive fire')
    result=report.setdefault(name,{})
    result['reimport_validation']={'success':not errors,'errors':errors,'blender_version':bpy.app.version_string,'triangles':tris,'bounds_blender':bounds,'solid_footprint':footprint,'root_names':[o.name for o in roots],'mesh_count':len(meshes),'embedded_png_count':len(g.get('images',[])),'image_sizes':image_info,'materials_have_basecolor_normal_metallic_roughness':all('baseColorTexture' in m.get('pbrMetallicRoughness',{}) and 'normalTexture' in m and 'metallicRoughnessTexture' in m.get('pbrMetallicRoughness',{}) for m in g.get('materials',[])),'glb_version':version,'extensions_used':g.get('extensionsUsed',[]),'file_bytes':len(raw)}
    print(name,json.dumps(result['reimport_validation']))
report_path.write_text(json.dumps(report,indent=2))
if any(not v.get('reimport_validation',{}).get('success',False) for v in report.values()):raise RuntimeError('Reimport validation failed')
print('ALL_BUILDING_CHECKS_PASS')

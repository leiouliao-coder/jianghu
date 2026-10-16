"""Fresh-process packed-source validation.
Run: blender --background assets/source/ground_materials.blend --python assets/source/ground_validate_blender.py
"""
from pathlib import Path
import bpy, json, hashlib
ROOT=Path(__file__).resolve().parents[2]
report_path=ROOT/'docs/art/ground-validation.json'
report=json.loads(report_path.read_text())
images=[x for x in bpy.data.images if x.name.startswith('ground_')]
assert len(images)==6, f'Expected six images; found {len(images)}'
assert all(x.packed_file for x in images), 'An image is not packed'
assert all(tuple(x.size)==(2048,2048) for x in images)
for image in images:
    disk=ROOT/'client/assets/art/textures'/image.name
    assert hashlib.sha256(image.packed_file.data).hexdigest()==hashlib.sha256(disk.read_bytes()).hexdigest(), f'Packed image differs from PNG: {image.name}'
materials=[x for x in bpy.data.materials if x.name.startswith('ground_')]
assert len(materials)==3
for mat in materials:
    nodes=mat.node_tree.nodes
    assert any(n.type=='NORMAL_MAP' for n in nodes)
    assert sum(n.type=='TEX_IMAGE' for n in nodes)==2
    for node in [n for n in nodes if n.type=='TEX_IMAGE']:
        expected='Non-Color' if 'normal' in node.image.name else 'sRGB'
        assert node.image.colorspace_settings.name==expected
        assert node.extension=='REPEAT'
blend=ROOT/'assets/source/ground_materials.blend'
assert blend.stat().st_size<25*1024*1024
report['blender_source']={
    'file':str(blend.relative_to(ROOT)), 'version':bpy.app.version_string,
    'fresh_process_reopen':'passed', 'bytes':blend.stat().st_size,
    'packed_png_sha256_matches_files':True, 'packed_image_count':len(images), 'materials':sorted(m.name for m in materials),
    'editable_shader_nodes':'Principled BSDF, UV coordinate, image texture, tangent normal map',
    'swatch_scale':'8x8 metres with UV 0–2: four 4m tiles per swatch',
    'builder':'assets/source/ground_build_blender.py',
    'validation_script':'assets/source/ground_validate_blender.py',
    'preview':'assets/source/ground-material-preview.png',
    'tiled_inspection_sheet':'assets/source/ground-preview.png'}
report_path.write_text(json.dumps(report,indent=2)+'\n')
print(json.dumps(report['blender_source'],indent=2))

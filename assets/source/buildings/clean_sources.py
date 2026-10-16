"""Purge orphaned authoring data from the two packed source files."""
import bpy
from pathlib import Path
SRC=Path(__file__).resolve().parent
for name in ['building_hall','building_forge']:
    bpy.ops.wm.open_mainfile(filepath=str(SRC/(name+'.blend')))
    bpy.data.orphans_purge(do_local_ids=True,do_linked_ids=True,do_recursive=True)
    for image in bpy.data.images:
        if image.source=='FILE' and not image.packed_file:image.pack()
    bpy.context.scene['source_license']='Original project-authored geometry and original painted texture pixels. No third-party resources.'
    bpy.ops.wm.save_as_mainfile(filepath=str(SRC/(name+'.blend')))
    print('CLEAN_PACKED_SOURCE',name)

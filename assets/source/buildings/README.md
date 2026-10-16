# Riverbend village architecture source

Original modeled architecture for the sample village. The six design-development images and their recorded prompts are under `assets/concept-art/sample/building-*`. All geometry and painted texture pixels were created for this project. No third-party models, brush packs, image textures or HDRIs were used.

## Reproduce

1. `python assets/source/buildings/make_building_textures.py` (Python, NumPy and Pillow)
2. `blender --background --threads 4 --python assets/source/buildings/build_buildings.py` (Blender 4.3.2)
3. `blender --background --python assets/source/buildings/validate_buildings.py`
4. Optional independent review: `blender --background --threads 4 --python assets/source/buildings/render_exported_buildings.py`

The builder writes both packed `.blend` sources, both embedded-texture GLBs, three inspection views per building, and machine-readable metrics. `SKIP_RENDERS=1` skips preview rendering; `ASSET_FILTER=building_hall` or `building_forge` builds a single asset. The independent review renderer loads the actual exported GLBs and uses a 55-degree downward orthographic camera. These are inspection renders, not AI images.

## Modeling decisions

- All mesh object origins and each asset root are `(0, 0, 0)`, at the centre of the footprint on the ground. The entrance faces Blender `-Y`; GLB export uses `+Y Up`.
- The hall's solid corner walls and foundation reach exactly `-7` and `+7` metres on X and Y. Its eave trim stays within the allowed one-metre projection. Recessed windows and closed double doors occupy actual openings. The plaque is intentionally blank.
- The forge's platform and all roof, tools, fuel and workshop details stay inside the 14-by-14-metre occupied footprint. An open-front post-supported canopy is attached to the closed rear workshop. The stove includes a separate emissive ember bed and glowing coal geometry.
- Roof surfaces use parallel barrel-tile columns, clipped at the hips, with convex roll profiles, physical overlap lips, raised cap tiles, and subtle glaze colour variation. Repeated hidden/internal surfaces were removed to preserve the roof detail budget.
- Source PNGs are 512-by-512 base colour, tangent-space normal and packed ORM (occlusion/roughness/metallic) sets. They contain original deterministic painted grain, brush glazes, plaster variation, wear and tile seams. No runtime procedural texture or unsupported shader is needed.
- All exported meshes are explicitly triangulated; no live modifiers, cameras or preview floor are included in the GLBs.

The JSON in `docs/art/buildings-validation.json` records exact counts, bounds, embedded image sizes and independent Blender background reimport results. Godot import and live scene review are handled by the integrator.

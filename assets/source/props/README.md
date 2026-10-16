# Hewan old tree, fence, and village props

Original stylized Wuxia asset set, Blender 4.3.2. No third-party meshes, HDRIs, textures, brushes, or reference assets beyond the repository's supplied concept image.

## Rebuild

From the repository root:

```sh
python assets/source/props/props_textures.py
blender --background --threads 6 --python assets/source/props/generate_props.py
blender --background --threads 2 --python assets/source/props/validate_props.py
```

The texture generator needs Python 3, Pillow, and NumPy. Blender itself has no external package dependency. The generator builds and exports all three GLBs, packs the source `.blend`, and renders two real-geometry inspection images. Rendering uses CPU Cycles with no denoiser dependency. All generation is deterministic.

## Design and materials

The authored SVG shape/palette sheet predates modeling and is supplied at `assets/concept-art/sample/props-design.svg` with a PNG and brief. Furniture has individual planks, visible stave seams, edge bevels, and small iron fasteners. The masonry well has an actual open shaft, separate staggered stone blocks, coping, winch, rope, and bucket. Table ware has modeled hollow cups. The tree has a fluted curved trunk, contained buttress roots, ten reaching boughs, twenty upper twig forks, and an uneven layered crown of 27 hand-shaped painted leaf volumes, with a restrained alpha-cutout fringe.

All material nodes are portable Principled BSDF with PNG albedo, normal maps, and roughness maps, or simple material factors. No Blender-only procedural shader has to survive export. Texture dimensions are 1024² (wood and bark) and 512² (other material maps and four leaf-spray variants). The original `textures/` files are also packed into `props.blend`. No individual texture exceeds 2048².

The fringe leaf image alpha is explicitly exported as glTF `MASK` with cutoff 0.42 and `doubleSided: true`. Leaves do not use transmission, subsurface, or alpha blending. Geometry is below the 3,000-triangle budget per prop, including the entire old tree.

## Units, origins, and placement

- Blender units are metres, +Z up; GLB export converts to +Y up
- The front of every asset is Blender −Y
- Each prop's local origin is the exact center of its horizontal bounding footprint at ground level
- Tree origin is the ground-level trunk center; the asymmetric 12 m crown is visual only, and does not affect the ≤1.5 m trunk collision requirement
- Fence is exactly 4.0 × 0.3 × 1.2 m, within floating-point tolerance
- Every root object in `props_set.glb` is independently placeable and fits within 2 × 2 m

`props_set.glb` intentionally lays out its five named root objects for inspection:

| Root name | Blender display translation (m) |
|---|---|
| `crate` | −3, 0, 0 |
| `barrel` | −1.5, 0, 0 |
| `lantern_pole` | 0, 0, 0 |
| `well` | 2, 0, 0 |
| `market_table` | 0, 2, 0 |

Reset the selected root's translation when instancing it individually. These display translations are not vertices baked into the mesh. The packed `.blend` uses a second inspection layout with the tree and fence behind the small props. Camera, lights, and the studio ground are not exported.

## Validation

`docs/art/props-validation.json` contains the source and independently re-imported triangle counts, dimensions, origins, texture sizes, file sizes, root names, material alpha modes, and exact validation results. Each GLB is opened from a clean Blender scene in a separate background invocation. Embedded image bytes are checked for the PNG signature and maximum dimensions.

`renders/props-assets-overview.png` and `renders/props-assets-detail.png` are actual Blender renders, not concept illustrations. Final Godot 4.7 Compatibility import and in-engine presentation remain the integrator's acceptance step; no unperformed engine test is claimed.

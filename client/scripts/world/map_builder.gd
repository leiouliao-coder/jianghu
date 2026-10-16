class_name MapBuilder
extends RefCounted
## Builds graybox 3D geometry from the map sent with enter_world. The
## server's shared/data/map.json is the only source of the layout.

const ZONE_COLORS: Dictionary = {
	"service": Color(0.42, 0.55, 0.45),
	"pve": Color(0.52, 0.46, 0.36),
	"risk": Color(0.66, 0.24, 0.22),
}
const OBSTACLE_COLOR: Color = Color(0.30, 0.30, 0.33)
const OBSTACLE_HEIGHT: float = 4.0


static func build(map: Dictionary) -> Node3D:
	var root: Node3D = Node3D.new()
	root.name = "Map"
	for b: Variant in map.get("blocks", []):
		root.add_child(_slab("Block_" + str(b["id"]), b["rect"], ZONE_COLORS.get(b.get("zone", "pve"), Color.GRAY), 0.0))
		var label: Label3D = name_label("%s %s" % [b["id"], b.get("name", "")], 40)
		label.name = "Label_" + str(b["id"])
		var r: Array = b["rect"]
		label.position = Vector3((float(r[0]) + float(r[2])) / 2.0, 6.0, (float(r[1]) + float(r[3])) / 2.0)
		root.add_child(label)
	for road: Variant in map.get("roads", []):
		var pts: Array = road["points"]
		for i: int in range(1, pts.size()):
			root.add_child(_road_segment("Road_%s_%d" % [road["id"], i], pts[i - 1], pts[i], float(road["width"]),
				ZONE_COLORS.get(road.get("zone", "pve"), Color.GRAY)))
	for area: Variant in map.get("risk_areas", []):
		root.add_child(_slab("Risk_" + str(area["id"]), area["rect"], ZONE_COLORS["risk"], 0.02))
	for o: Variant in map.get("obstacles", []):
		var box: MeshInstance3D = _box("Obstacle_" + str(o["id"]), o["rect"], OBSTACLE_HEIGHT, OBSTACLE_COLOR)
		root.add_child(box)
	return root


## A billboard label that keeps the same on-screen size at any zoom, with
## an outline so it reads on every ground color.
static func name_label(text: String, font_size: int) -> Label3D:
	var label: Label3D = Label3D.new()
	label.text = text
	label.font_size = font_size
	label.outline_size = 10
	label.outline_modulate = Color(0, 0, 0, 0.85)
	label.billboard = BaseMaterial3D.BILLBOARD_ENABLED
	label.fixed_size = true
	label.pixel_size = 0.0012
	return label


static func _material(color: Color) -> StandardMaterial3D:
	var m: StandardMaterial3D = StandardMaterial3D.new()
	m.albedo_color = color
	return m


## A thin ground slab covering rect [x0, z0, x1, z1] at height y.
static func _slab(node_name: String, rect: Array, color: Color, y: float) -> MeshInstance3D:
	var mi: MeshInstance3D = _box(node_name, rect, 0.1, color)
	mi.position.y = y - 0.05
	return mi


static func _box(node_name: String, rect: Array, height: float, color: Color) -> MeshInstance3D:
	var size: Vector2 = Vector2(float(rect[2]) - float(rect[0]), float(rect[3]) - float(rect[1]))
	var mesh: BoxMesh = BoxMesh.new()
	mesh.size = Vector3(size.x, height, size.y)
	mesh.material = _material(color)
	var mi: MeshInstance3D = MeshInstance3D.new()
	mi.name = node_name
	mi.mesh = mesh
	mi.position = Vector3(float(rect[0]) + size.x / 2.0, height / 2.0, float(rect[1]) + size.y / 2.0)
	return mi


static func _road_segment(node_name: String, a: Array, b: Array, width: float, color: Color) -> MeshInstance3D:
	var from: Vector2 = Vector2(float(a[0]), float(a[1]))
	var to: Vector2 = Vector2(float(b[0]), float(b[1]))
	var mesh: BoxMesh = BoxMesh.new()
	mesh.size = Vector3(from.distance_to(to) + width, 0.1, width)
	mesh.material = _material(color)
	var mi: MeshInstance3D = MeshInstance3D.new()
	mi.name = node_name
	mi.mesh = mesh
	var mid: Vector2 = (from + to) / 2.0
	mi.position = Vector3(mid.x, -0.06, mid.y)
	mi.rotation.y = -(to - from).angle()
	return mi

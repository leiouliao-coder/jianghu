extends Node3D
## Optional isolated art viewer. Never replaces live map_builder or changes collision rules.
## Run: godot --path client res://art_preview/sample_village_preview.tscn
## Tab: overview/character. 1..5: idle/walk/attack_basic/hit/death. R: replay current clip.
const ART := "res://assets/art/"
var actor: Node3D
var animation_player: AnimationPlayer
var camera: Camera3D
var status: Label
var close_view := false
var selected_clip := "idle"

func _ready() -> void:
	get_window().size = Vector2i(1280, 800)
	var world := WorldEnvironment.new()
	world.environment = Environment.new()
	world.environment.background_mode = Environment.BG_COLOR
	world.environment.background_color = Color(0.54, 0.63, 0.65)
	world.environment.ambient_light_source = Environment.AMBIENT_SOURCE_COLOR
	world.environment.ambient_light_color = Color(0.73, 0.79, 0.82)
	world.environment.ambient_light_energy = 0.8
	world.environment.tonemap_mode = Environment.TONE_MAPPER_FILMIC
	add_child(world)
	var sun := DirectionalLight3D.new()
	sun.rotation_degrees = Vector3(-55, -35, 0)
	sun.light_color = Color(1.0, 0.93, 0.80)
	sun.light_energy = 1.15
	sun.shadow_enabled = true
	add_child(sun)
	_ground("grass", Vector2(60, 60), Vector3(0, -0.04, 0))
	_ground("stone_paving", Vector2(50, 50), Vector3(0, -0.02, 0))
	for side: float in [-1.0, 1.0]:
		_ground("dirt_road", Vector2(12, 8), Vector3(30 * side, -0.01, 0))
	_ground("dirt_road", Vector2(8, 12), Vector3(0, -0.01, 30))
	_ground("dirt_road", Vector2(6, 12), Vector3(0, -0.01, -30))
	_asset("village/building_hall.glb", Vector3(-15, 0, -15))
	_asset("village/building_forge.glb", Vector3(15, 0, 15))
	_asset("village/tree_old.glb", Vector3(14, 0, -14))
	_fence_edge(true, 29.85, 8)
	_fence_edge(true, -29.85, 6)
	_fence_edge(false, 29.85, 8)
	_fence_edge(false, -29.85, 8)
	var props: Node3D = _asset("village/props_set.glb", Vector3.ZERO)
	var prop_positions := {"crate": Vector3(-23.5, 0, -10), "barrel": Vector3(-23.5, 0, -12), "lantern_pole": Vector3(-5.5, 0, 26), "well": Vector3(-13, 0, 13), "market_table": Vector3(-13, 0, -6)}
	for key: String in prop_positions:
		var original: Node3D = props.find_child(key, true, false) as Node3D
		if original:
			original.position = prop_positions[key]
	actor = _asset("characters/player_novice.glb", Vector3.ZERO)
	var players := actor.find_children("*", "AnimationPlayer", true, false)
	if not players.is_empty():
		animation_player = players[0] as AnimationPlayer
		for clip: String in ["idle", "walk"]:
			if animation_player.has_animation(clip):
				animation_player.get_animation(clip).loop_mode = Animation.LOOP_LINEAR
	camera = Camera3D.new()
	camera.projection = Camera3D.PROJECTION_ORTHOGONAL
	camera.far = 250
	add_child(camera)
	camera.make_current()
	_view()
	var layer := CanvasLayer.new()
	add_child(layer)
	var panel := PanelContainer.new()
	panel.position = Vector2(18, 18)
	layer.add_child(panel)
	var margin := MarginContainer.new()
	for side: String in ["left", "right", "top", "bottom"]:
		margin.add_theme_constant_override("margin_" + side, 12)
	panel.add_child(margin)
	var text := VBoxContainer.new()
	margin.add_child(text)
	var title := Label.new()
	title.text = "RIVERBEND / A    EXPORTED ART PREVIEW"
	title.add_theme_font_size_override("font_size", 18)
	text.add_child(title)
	status = Label.new()
	text.add_child(status)
	var help := Label.new()
	help.text = "Tab  view     1 idle   2 walk   3 attack   4 hit   5 death     R replay\nArt-only inspection. No server collision or combat acceptance implied."
	help.add_theme_font_size_override("font_size", 13)
	text.add_child(help)
	_play("idle")

func _asset(file: String, at: Vector3) -> Node3D:
	var packed: PackedScene = load(ART + file) as PackedScene
	assert(packed != null, "Cannot load " + file)
	var node: Node3D = packed.instantiate() as Node3D
	add_child(node)
	node.position = at
	return node

func _ground(kind: String, size_m: Vector2, at: Vector3) -> void:
	var mesh := PlaneMesh.new()
	mesh.size = size_m
	var material := StandardMaterial3D.new()
	material.albedo_texture = load(ART + "textures/ground_" + kind + ".png")
	material.normal_enabled = true
	material.normal_texture = load(ART + "textures/ground_" + kind + "_normal.png")
	material.normal_scale = 0.65
	material.roughness = 0.92
	material.uv1_scale = Vector3(size_m.x / 4.0, size_m.y / 4.0, 1)
	mesh.material = material
	var instance := MeshInstance3D.new()
	instance.mesh = mesh
	instance.position = at
	add_child(instance)

func _fence_edge(along_x: bool, fixed: float, gap: float) -> void:
	for interval: Vector2 in [Vector2(-30, -gap / 2), Vector2(gap / 2, 30)]:
		var start: float = interval.x
		while start < interval.y - 0.001:
			var length: float = minf(4, interval.y - start)
			var midpoint: float = start + length / 2
			var position_m: Vector3 = Vector3(midpoint, 0, fixed) if along_x else Vector3(fixed, 0, midpoint)
			var fence: Node3D = _asset("village/fence_wood_4m.glb", position_m)
			fence.scale.x = length / 4
			if not along_x:
				fence.rotation.y = PI / 2
			assert(absf(fence.transform.basis.x.normalized().dot(Vector3.RIGHT if along_x else Vector3.FORWARD)) > 0.999, "Fence must follow its boundary")
			start += length

func _view() -> void:
	camera.position = Vector3(2.9, 2.3, 4.3) if close_view else Vector3(32, 105, 65)
	camera.size = 3.0 if close_view else 94.0
	camera.look_at(Vector3(0, 0.95, 0) if close_view else Vector3(0, 1, 0))

func _play(clip: String) -> void:
	selected_clip = clip
	if animation_player and animation_player.has_animation(clip):
		animation_player.stop()
		animation_player.play(clip)
		animation_player.seek(0, true)
		status.text = "Clip: %s  |  %.2f seconds  |  1 Blender unit = 1 metre" % [clip, animation_player.get_animation(clip).length]
		if clip == "walk":
			status.text += "\nRoot-motion demonstration: 1.6m / 0.4s = 4m/s; loop resets in this art viewer"
	else:
		status.text = "Missing animation: " + clip

func _unhandled_input(event: InputEvent) -> void:
	if event is InputEventKey and event.pressed and not event.echo:
		match event.keycode:
			KEY_TAB:
				close_view = not close_view
				_view()
			KEY_1: _play("idle")
			KEY_2: _play("walk")
			KEY_3: _play("attack_basic")
			KEY_4: _play("hit")
			KEY_5: _play("death")
			KEY_R: _play(selected_clip)

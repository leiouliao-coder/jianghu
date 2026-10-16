extends SceneTree
## Independent resource import/animation inventory. No server or gameplay integration.
const FILES = ["village/building_hall.glb", "village/building_forge.glb", "village/tree_old.glb", "village/fence_wood_4m.glb", "village/props_set.glb", "characters/player_novice.glb"]
var failures: Array = []
func _initialize() -> void:
	call_deferred("_run")
func _run() -> void:
	var reports: Array = []
	for path: String in FILES:
		var packed = load("res://assets/art/" + path)
		if packed == null or not packed is PackedScene:
			failures.append(path + ": PackedScene load failed")
			continue
		var scene = packed.instantiate()
		root.add_child(scene)
		var report = {"file": path, "node_count": 0, "mesh_count": 0, "skeleton_count": 0, "animations": {}}
		_walk(scene, report)
		reports.append(report)
		scene.queue_free()
		await process_frame
	var result = {"engine": Engine.get_version_info(), "renderer": ProjectSettings.get_setting("rendering/renderer/rendering_method"), "method": "Headless imported-resource load and scene instantiation. Not a rendering or gameplay test.", "assets": reports, "failures": failures}
	var path: String = OS.get_cmdline_user_args()[0] if OS.get_cmdline_user_args().size() > 0 else "user://sample-village-godot-validation.json"
	var file = FileAccess.open(path, FileAccess.WRITE)
	file.store_string(JSON.stringify(result, "  "))
	print(JSON.stringify(result))
	quit(0 if failures.is_empty() else 1)
func _walk(node: Node, report: Dictionary) -> void:
	report.node_count += 1
	if node is MeshInstance3D:
		report.mesh_count += 1
	if node is Skeleton3D:
		report.skeleton_count += 1
	if node is AnimationPlayer:
		for animation: String in node.get_animation_list():
			report.animations[animation] = node.get_animation(animation).length
	for child: Node in node.get_children():
		_walk(child, report)

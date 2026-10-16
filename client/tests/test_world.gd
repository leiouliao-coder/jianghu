extends SceneTree
## World view pieces without a server: interpolation, map building, camera
## limits, ground picking and reply handling.

var failures: int = 0

const MAP: Dictionary = {
	"blocks": [{"id": "A", "name": "河湾镇", "zone": "service", "rect": [-30, -30, 30, 30]},
		{"id": "C", "name": "矿区", "zone": "pve", "rect": [-130, -30, -70, 30]}],
	"roads": [{"id": "A-C", "zone": "pve", "width": 8, "points": [[-30, 0], [-70, 0]]},
		{"id": "L", "zone": "risk", "width": 6, "points": [[0, -30], [0, -60], [40, -60]]}],
	"risk_areas": [{"id": "F-risk", "rect": [175, -145, 225, -115]}],
	"obstacles": [{"id": "hall", "rect": [-22, -22, -8, -8]}],
}


func _initialize() -> void:
	call_deferred("_run")


func _check(condition: bool, message: String) -> void:
	if not condition:
		failures += 1
		push_error(message)


func _run() -> void:
	_test_buffer()
	_test_map()
	await _test_camera()
	_test_ground_point()
	await _test_view_and_shell()
	await _test_input_reaches_world()
	if failures == 0:
		print("PASS world view: interpolation, map, camera, picking, replies")
	quit(0 if failures == 0 else 1)


func _test_buffer() -> void:
	var b: SnapshotBuffer = SnapshotBuffer.new()
	_check(b.sample(0.0).is_empty(), "empty buffer samples nothing")
	# Server clock = local + 1000 ms. Snapshots 100 ms apart.
	b.push(2000.0, [{"id": 1, "x": 0.0, "z": 0.0, "name": "a"}], 1000.0)
	b.push(2100.0, [{"id": 1, "x": 0.4, "z": 0.0, "name": "a"}, {"id": 2, "x": 5.0, "z": 5.0}], 1100.0)
	# render time = local + offset - 100 = 1150 + 1000 - 100 = 2050 -> halfway.
	var mid: Dictionary = b.sample(1150.0)
	_check(is_equal_approx(mid[1]["pos"].x, 0.2), "interpolates halfway between snapshots: %s" % mid[1]["pos"])
	_check(mid[2]["pos"] == Vector2(5, 5), "new entity appears at its first reported position")
	var late: Dictionary = b.sample(5000.0)
	_check(late[1]["pos"] == Vector2(0.4, 0.0), "no extrapolation past the newest snapshot")
	b.push(2050.0, [{"id": 1, "x": 99.0, "z": 0.0}], 1200.0)
	_check(is_equal_approx(b.sample(5000.0)[1]["pos"].x, 0.4), "out-of-order snapshot ignored")
	b.push(2200.0, [{"id": 2, "x": 5.0, "z": 5.0}], 1200.0)
	_check(not b.sample(5000.0).has(1), "entity missing from newest snapshot is dropped")


func _test_map() -> void:
	var root: Node3D = MapBuilder.build(MAP)
	var names: Array[String] = []
	for c: Node in root.get_children():
		names.append(c.name)
	_check(names.has("Block_A") and names.has("Block_C"), "blocks built")
	_check(names.has("Road_A-C_1") and names.has("Road_L_1") and names.has("Road_L_2"), "one mesh per road segment")
	_check(names.has("Risk_F-risk") and names.has("Obstacle_hall"), "risk area and obstacle built")
	var a: MeshInstance3D = root.get_node("Block_A")
	_check(a.position.x == 0.0 and a.position.z == 0.0 and (a.mesh as BoxMesh).size.x == 60.0, "block placed by rect")
	var risk_mat: StandardMaterial3D = (root.get_node("Road_L_1") as MeshInstance3D).mesh.material
	_check(risk_mat.albedo_color == MapBuilder.ZONE_COLORS["risk"], "risk road colored as risk")
	var seg: MeshInstance3D = root.get_node("Road_L_2")
	_check(is_equal_approx(seg.position.x, 20.0) and is_equal_approx(seg.position.z, -60.0), "segment centered")
	root.free()


func _test_camera() -> void:
	var scene: Node = (load("res://scenes/world_view.tscn") as PackedScene).instantiate()
	var rig: CameraRig = scene.get_node("CameraRig").duplicate()
	scene.free()
	root.add_child(rig)
	await process_frame
	for i: int in 100:
		rig.zoom(1.0)
	_check(rig.distance == CameraRig.MAX_DISTANCE, "zoom out clamped")
	for i: int in 100:
		rig.zoom(-1.0)
	_check(rig.distance == CameraRig.MIN_DISTANCE, "zoom in clamped")
	rig.rotate_by(100.0)
	_check(rig.yaw != CameraRig.DEFAULT_YAW, "drag rotates")
	rig.reset_view()
	_check(rig.distance == CameraRig.DEFAULT_DISTANCE and rig.yaw == CameraRig.DEFAULT_YAW, "reset restores default angle")
	_check(rig.camera.position.y > 0.0 and rig.camera.position.z > 0.0, "camera above and behind the target")
	rig.queue_free()


func _test_ground_point() -> void:
	var view: GDScript = load("res://scripts/world_view.gd")
	var hit: Variant = view.ground_point(Vector3(0, 10, 10), Vector3(0, -1, -1).normalized())
	_check(hit is Vector2 and (hit as Vector2).is_equal_approx(Vector2(0, 0)), "ray hits ground below: %s" % [hit])
	_check(view.ground_point(Vector3(0, 10, 0), Vector3(0, 1, 0)) == null, "ray pointing up misses")
	_check(view.ground_point(Vector3(0, 10, 0), Vector3(1, 0, 0)) == null, "parallel ray misses")


func _test_view_and_shell() -> void:
	var shell: Control = (load("res://scenes/account.tscn") as PackedScene).instantiate()
	root.add_child(shell)
	await process_frame
	var payload: Dictionary = {"character_id": 7, "name": "剑客", "x": 3.0, "z": -2.0, "area": "A", "zone": "service", "map": MAP}
	shell.call("_open_world", payload)
	await process_frame
	var view: Node3D = shell.world_view
	_check(view != null and not shell.get_node("Margin").visible, "entering shows the world and hides the menu")
	_check(view.get_node("HUD/Panel/Box/Area").text.contains("安全区"), "HUD shows the zone")
	_check(view.get_node("MapRoot").get_child_count() == 1, "map built once")
	view.call("setup", shell.client, payload)  # reconnect path rebuilds, not duplicates
	await process_frame
	_check(view.get_node("MapRoot").get_child_count() == 1, "re-setup replaces the map")

	view._pending["m1"] = {"type": "move_to", "target": Vector2(200, -130)}
	view.call("_on_reply", "m1", {"type": "error", "payload": {"code": "risk_confirmation_required"}})
	_check(view.get_node("HUD/RiskDialog").visible, "risk confirmation asked")
	_check(view._risk_target == Vector2(200, -130), "risk target remembered for the confirmed retry")
	view._pending["m2"] = {"type": "move_to", "target": Vector2(1, 1)}
	view.call("_on_reply", "m2", {"type": "error", "payload": {"code": "unreachable"}})
	_check(view.get_node("HUD/Panel/Box/Status").text.contains("无法到达"), "unreachable explained")
	view.call("_on_reply", "not-mine", {"type": "error", "payload": {"code": "unreachable"}})

	view.call("_on_pushed", {"type": "snapshot", "server_time": 1000.0, "payload": {"entities": [
		{"id": 7, "name": "剑客", "x": 3.0, "z": -2.0, "area": "A-C", "zone": "pve"},
		{"id": 8, "name": "路人", "x": 6.0, "z": 0.0, "area": "A", "zone": "service"}]}})
	await process_frame
	_check(view.get_node("Avatars").get_child_count() == 2, "one avatar per visible character")
	_check(view.get_node("HUD/Panel/Box/Area").text.contains("野外"), "HUD follows own character's zone")

	shell.call("_on_closed", Protocol.CLOSE_KICKED, "")
	await process_frame
	_check(shell.world_view == null and shell.get_node("Margin").visible, "kick returns to the menu")
	shell.queue_free()
	await process_frame


func _button(index: MouseButton, pressed: bool, pos: Vector2) -> InputEventMouseButton:
	var e: InputEventMouseButton = InputEventMouseButton.new()
	e.button_index = index
	e.pressed = pressed
	e.position = pos
	e.global_position = pos
	return e


## Regression: the account shell is full-screen and once swallowed right and
## middle clicks, so only the wheel worked. Inject events at the window, the
## way real input arrives, instead of calling handlers directly.
func _test_input_reaches_world() -> void:
	var shell: Control = (load("res://scenes/account.tscn") as PackedScene).instantiate()
	root.add_child(shell)
	await process_frame
	shell.call("_open_world", {"character_id": 1, "x": 0.0, "z": 0.0, "area": "A", "zone": "service", "map": MAP})
	await process_frame
	var view: Node3D = shell.world_view
	var moves: Array[Vector2] = []
	view.move_requested.connect(func(target: Vector2, _risk: bool) -> void: moves.append(target))
	var center: Vector2 = Vector2(root.size) / 2.0
	root.push_input(_button(MOUSE_BUTTON_RIGHT, true, center))
	await process_frame
	_check(moves.size() == 1, "right click on the ground requests a move")
	var rig: CameraRig = view.rig
	var yaw: float = rig.yaw
	root.push_input(_button(MOUSE_BUTTON_MIDDLE, true, center))
	var motion: InputEventMouseMotion = InputEventMouseMotion.new()
	motion.position = center + Vector2(40, 0)
	motion.relative = Vector2(40, 0)
	motion.button_mask = MOUSE_BUTTON_MASK_MIDDLE
	root.push_input(motion)
	root.push_input(_button(MOUSE_BUTTON_MIDDLE, false, center))
	await process_frame
	_check(rig.yaw != yaw, "middle drag rotates the camera")
	var distance: float = rig.distance
	root.push_input(_button(MOUSE_BUTTON_WHEEL_DOWN, true, center))
	await process_frame
	_check(rig.distance != distance, "wheel zooms")
	shell.queue_free()
	await process_frame

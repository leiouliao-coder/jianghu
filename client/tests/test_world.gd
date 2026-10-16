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
	await _test_combat_hud()
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


func _snapshot(me: Dictionary, others: Array, corpses: Array, events: Array, self_info: Dictionary) -> Dictionary:
	var ents: Array = [me]
	ents.append_array(others)
	return {"entities": ents, "corpses": corpses, "events": events, "self": self_info}


## Combat client: HUD, death screen, floating text, chasing and hitting a
## clicked monster, and looting a corpse, all against a recording client.
func _test_combat_hud() -> void:
	var view: Node3D = (load("res://scenes/world_view.tscn") as PackedScene).instantiate()
	root.add_child(view)
	await process_frame
	var fake: WsClient = (load("res://tests/fake_client.gd") as GDScript).new()
	root.add_child(fake)
	view.setup(fake, {"character_id": 1, "x": 0.0, "z": 0.0, "area": "C", "zone": "pve", "map": MAP})
	var me: Dictionary = {"id": 1, "kind": "player", "name": "我", "x": 0.0, "z": 0.0, "hp": 700, "max_hp": 1000,
		"mp": 80, "sp": 60, "preset": "armed", "action": "", "dead": false, "stunned": false, "facing": 0.0}
	var bandit: Dictionary = {"id": -1, "kind": "monster", "name": "矿区山贼", "x": 6.0, "z": 0.0, "hp": 300, "max_hp": 400, "dead": false}
	var corpse: Dictionary = {"id": 55, "x": 0.0, "z": 4.0, "owner": "路人", "expires_in": 1700, "empty": false}
	var self_info: Dictionary = {"cooldowns": {"q": 3.2, "w": 0.0}, "ranges": {"basic": 2.0}, "in_combat": true}
	view.apply_snapshot(1000.0, _snapshot(me, [bandit], [corpse], [{"kind": "hit", "actor": -1, "target": 1, "value": 40}], self_info))
	await process_frame
	_check(view.get_node("HUD/Bars/Box/HP").value == 700.0, "own HP bar")
	_check(view.get_node("HUD/Bars/Box/Values").text.contains("战斗中"), "combat state shown")
	_check(view.get_node("HUD/Bars/Box/Skills/Slot_q").text == "Q 3.2", "Q cooldown shown")
	_check(view.get_node("HUD/Bars/Box/Skills/Slot_w").text == "W", "ready skill shows only its key")
	_check(view.get_node("Corpses").get_child_count() == 1, "corpse marker drawn")
	_check(view.get_node("Effects").get_child_count() == 1, "hit shows floating damage")

	# Right-click the bandit: chase while out of range...
	view.click_ground(Vector2(6.2, 0.1))
	_check(view.attack_target == -1, "clicking a monster targets it")
	view._process(0.0)
	var chase: Dictionary = fake.last("move_to")
	_check(not chase.is_empty() and is_equal_approx(float(chase["payload"]["x"]), 6.0), "out of range: walk toward it")
	_check(view.get_node("HUD/Target").visible, "target frame shown")
	# ...then attack once in reach.
	me["x"] = 4.5
	view.apply_snapshot(1100.0, _snapshot(me, [bandit], [corpse], [], self_info))
	view._next_basic_ms = 0
	view._process(0.0)
	var hit: Dictionary = fake.last("cast")
	_check(not hit.is_empty() and hit["payload"]["slot"] == "basic" and int(hit["payload"]["target"]) == -1, "in range: basic attack the target")

	# A skill pressed mid-action is buffered and fires when the action ends,
	# taking priority over the next auto-attack.
	me["action"] = "sword_basic"
	view.apply_snapshot(1150.0, _snapshot(me, [bandit], [corpse], [], self_info))
	var casts_before: int = fake.sent.filter(func(r: Dictionary) -> bool: return r["type"] == "cast").size()
	view.cast_skill("q", Vector2(1, 0))
	view._process(0.0)
	_check(fake.sent.filter(func(r: Dictionary) -> bool: return r["type"] == "cast").size() == casts_before, "no cast while acting: buffered")
	me["action"] = ""
	view.apply_snapshot(1160.0, _snapshot(me, [bandit], [corpse], [], self_info))
	view._process(0.0)
	var q: Dictionary = fake.last("cast")
	_check(q["payload"]["slot"] == "q", "buffered Q fires as soon as the action ends, before the next basic")
	view.cast_skill("w", Vector2(1, 0))
	view._on_reply(String(fake.last("cast")["id"]), {"type": "error", "payload": {"code": "busy"}})
	_check(not view.buffered.is_empty() and view.buffered["slot"] == "w", "a busy reply also buffers the skill")
	view.buffered["until"] = 0
	view._process(0.0)
	_check(view.buffered.is_empty(), "a stale buffered skill is dropped")

	# Right-click the corpse: open it, list goods, loot one.
	view.click_ground(Vector2(0.2, 3.9))
	var open: Dictionary = fake.last("corpse_open")
	_check(not open.is_empty() and int(open["payload"]["corpse_id"]) == 55, "clicking a corpse opens it")
	view._on_reply(String(open["id"]), {"type": "corpse_open.ok", "payload": {"corpse_id": 55, "coins": 30,
		"items": [{"id": "abc", "proto": "TEST-SWORD-001", "name": "测试铁剑", "qty": 1}]}})
	await process_frame
	var buttons: Array = view.get_node("HUD/Loot/Box/Items").get_children().filter(func(n: Node) -> bool: return not n.is_queued_for_deletion())
	_check(view.get_node("HUD/Loot").visible and buttons.size() == 2, "loot window lists coins and the sword")
	_check(view.get_node("HUD/Loot/Box/Title").text == "路人的遗物", "loot window names the owner")
	(buttons[1] as Button).pressed.emit()
	var take: Dictionary = fake.last("loot")
	_check(not take.is_empty() and take["payload"]["item_id"] == "abc", "pressing an item loots it")

	# Death: the death screen appears; respawning hides it.
	me["dead"] = true
	me["hp"] = 0
	view.apply_snapshot(1200.0, _snapshot(me, [bandit], [corpse], [], self_info))
	_check(view.get_node("HUD/Death").visible, "death screen when dead")
	view.get_node("HUD/Death/Box/Respawn").pressed.emit()
	var rs: Dictionary = fake.last("respawn")
	_check(not rs.is_empty(), "respawn button asks the server")
	view._on_reply(String(rs["id"]), {"type": "respawn.ok", "payload": {"area": "A"}})
	_check(not view.get_node("HUD/Death").visible, "respawned: death screen closes")

	# A corpse that disappears closes its loot window.
	view.show_loot(55, {"coins": 1, "items": []})
	view.apply_snapshot(1300.0, _snapshot(me, [], [], [], self_info))
	_check(not view.get_node("HUD/Loot").visible, "vanished corpse closes the loot window")
	view.queue_free()
	fake.queue_free()
	await process_frame

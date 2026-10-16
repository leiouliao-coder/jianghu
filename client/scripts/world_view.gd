extends Node3D
## The in-world view: draws the map, characters, monsters and corpses from
## server snapshots, and turns input into requests. The client never decides
## positions, hits or loot; it asks, and renders what the server reports.

signal leave_requested
signal move_requested(target: Vector2, accept_risk: bool)

const AREA_NAMES: Dictionary = {"service": "安全区", "pve": "野外（无强制PVP）", "risk": "风险区（可能遭受玩家攻击）"}
const SKILL_KEYS: Dictionary = {"q": "Q", "w": "W", "e": "E", "l": "空格"}
const ERROR_TEXT: Dictionary = {
	"cooldown": "招式尚在冷却。", "insufficient_resources": "内力或体力不足。", "out_of_range": "距离太远。",
	"bad_target": "不能攻击这个目标。", "busy": "正在出招或受制，无法行动。", "stunned": "眩晕中。",
	"dead": "你已身死。", "too_far": "离遗物太远。", "not_here": "东西已经不在了。", "no_space": "背包已满或负重不足。",
	"unreachable": "无法到达该处。", "rate_limited": "操作过于频繁。",
}
const PICK_RADIUS: float = 1.5  # meters around a ground click that select an entity
const COLOR_SELF: Color = Color(0.95, 0.80, 0.35)
const COLOR_PLAYER: Color = Color(0.55, 0.70, 0.95)
const COLOR_MONSTER: Color = Color(0.85, 0.35, 0.30)

var client: WsClient
var character_id: int = 0
var buffer: SnapshotBuffer = SnapshotBuffer.new()
var self_info: Dictionary = {}  # snapshot "self": cooldowns, ranges
var entities: Dictionary = {}  # id -> latest entity Dictionary
var corpses: Dictionary = {}  # id -> latest corpse Dictionary
var attack_target: int = 0  # entity we are chasing and hitting with basic attacks
var open_corpse: int = 0
var _pending: Dictionary = {}  # req_id -> {"type", ...}
var _avatars: Dictionary = {}  # id -> Node3D
var _corpse_nodes: Dictionary = {}  # id -> Node3D
var _risk_target: Vector2 = Vector2.ZERO
var _next_chase_ms: int = 0
var _next_basic_ms: int = 0
# Input buffering: a skill pressed while still acting is resent when the
# current action ends, within BUFFER_MS. The server still enforces recovery.
const BUFFER_MS: int = 600
var buffered: Dictionary = {}  # {"slot", "dir", "until"}

@onready var rig: CameraRig = $CameraRig
@onready var map_root: Node3D = $MapRoot
@onready var avatars_root: Node3D = $Avatars
@onready var corpses_root: Node3D = $Corpses
@onready var effects_root: Node3D = $Effects
@onready var area_label: Label = $HUD/Panel/Box/Area
@onready var status_label: Label = $HUD/Panel/Box/Status
@onready var leave_button: Button = $HUD/Panel/Box/Buttons/Leave
@onready var swap_button: Button = $HUD/Panel/Box/Buttons/Swap
@onready var risk_dialog: ConfirmationDialog = $HUD/RiskDialog
@onready var target_panel: Control = $HUD/Target
@onready var death_panel: Control = $HUD/Death
@onready var loot_panel: Control = $HUD/Loot
@onready var skills_box: HBoxContainer = $HUD/Bars/Box/Skills


func _ready() -> void:
	leave_button.pressed.connect(func() -> void: leave_requested.emit())
	swap_button.pressed.connect(func() -> void: _request("swap"))
	risk_dialog.confirmed.connect(func() -> void: request_move(_risk_target, true))
	$HUD/Death/Box/Respawn.pressed.connect(func() -> void: _request("respawn"))
	$HUD/Loot/Box/Close.pressed.connect(close_loot)
	for slot: String in SKILL_KEYS:
		var label: Label = Label.new()
		label.name = "Slot_" + slot
		label.custom_minimum_size = Vector2(80, 28)
		label.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
		label.text = SKILL_KEYS[slot]
		skills_box.add_child(label)


## Called with the enter_world.ok payload (also after a reconnect).
func setup(ws: WsClient, payload: Dictionary) -> void:
	if client != ws:
		client = ws
		client.pushed.connect(_on_pushed)
		client.reply_received.connect(_on_reply)
	character_id = int(payload.get("character_id", 0))
	for child: Node in map_root.get_children():
		child.queue_free()
	map_root.add_child(MapBuilder.build(payload.get("map", {})))
	var start: Vector2 = Vector2(float(payload.get("x", 0.0)), float(payload.get("z", 0.0)))
	rig.follow(Vector3(start.x, 0.0, start.y))
	_show_area(String(payload.get("area", "")), String(payload.get("zone", "")))
	death_panel.visible = bool(payload.get("dead", false))


func _request(type: String, payload: Variant = null, extra: Dictionary = {}) -> void:
	if client == null:
		return
	var req_id: String = client.request(type, payload)
	if req_id != "":
		var info: Dictionary = {"type": type}
		info.merge(extra)
		_pending[req_id] = info


func request_move(target: Vector2, accept_risk: bool = false) -> void:
	move_requested.emit(target, accept_risk)
	_request("move_to", {"x": target.x, "z": target.y, "accept_risk": accept_risk}, {"target": target})


## Intersects a camera ray with the ground plane y = 0.
static func ground_point(origin: Vector3, direction: Vector3) -> Variant:
	if absf(direction.y) < 1e-6:
		return null
	var t: float = -origin.y / direction.y
	if t <= 0.0:
		return null
	var hit: Vector3 = origin + direction * t
	return Vector2(hit.x, hit.z)


## The entity or corpse id closest to a ground point within PICK_RADIUS.
static func pick(point: Vector2, candidates: Dictionary, exclude: int) -> int:
	var best: int = 0
	var best_d: float = PICK_RADIUS
	for id: int in candidates:
		if id == exclude:
			continue
		var c: Dictionary = candidates[id]
		var d: float = point.distance_to(Vector2(float(c.get("x", 0.0)), float(c.get("z", 0.0))))
		if d <= best_d:
			best = id
			best_d = d
	return best


func _self_pos() -> Vector2:
	var me: Dictionary = entities.get(character_id, {})
	return Vector2(float(me.get("x", 0.0)), float(me.get("z", 0.0)))


func _cursor_ground() -> Variant:
	var cam: Camera3D = rig.camera
	var screen: Vector2 = get_viewport().get_mouse_position()
	return ground_point(cam.project_ray_origin(screen), cam.project_ray_normal(screen))


func _unhandled_input(event: InputEvent) -> void:
	var screen_pos: Variant = null
	if event is InputEventMouseButton:
		var mb: InputEventMouseButton = event
		if mb.button_index == MOUSE_BUTTON_RIGHT and mb.pressed:
			screen_pos = mb.position
	elif event is InputEventScreenTouch:
		# Mobile (decision Q1): a tap on the ground moves; no other touch UI yet.
		var touch: InputEventScreenTouch = event
		if touch.pressed and touch.index == 0:
			screen_pos = touch.position
	elif event.is_action_pressed("move_stop"):
		attack_target = 0
		_request("stop")
		return
	else:
		for slot: String in ["q", "w", "e", "l"]:
			if event.is_action_pressed("skill_" + slot):
				cast_toward_cursor(slot)
				get_viewport().set_input_as_handled()
				return
	if screen_pos == null:
		return
	var cam: Camera3D = rig.camera
	var hit: Variant = ground_point(cam.project_ray_origin(screen_pos), cam.project_ray_normal(screen_pos))
	if hit != null:
		click_ground(hit)
		get_viewport().set_input_as_handled()


## A right click: attack an enemy, open a corpse, or walk there.
func click_ground(point: Vector2) -> void:
	var living: Dictionary = {}
	for id: int in entities:
		if not entities[id].get("dead", false):
			living[id] = entities[id]
	var enemy: int = pick(point, living, character_id)
	if enemy != 0:
		attack_target = enemy
		_next_chase_ms = 0
		_next_basic_ms = 0
		_refresh_hud()  # show the target frame now, not on the next snapshot
		return
	var corpse: int = pick(point, corpses, 0)
	if corpse != 0:
		attack_target = 0
		_request("corpse_open", {"corpse_id": corpse}, {"corpse": corpse})
		return
	attack_target = 0
	request_move(point)


func cast_toward_cursor(slot: String) -> void:
	var cursor: Variant = _cursor_ground()
	if cursor == null:
		return
	cast_skill(slot, (cursor as Vector2) - _self_pos())


func cast_skill(slot: String, dir: Vector2) -> void:
	var me: Dictionary = entities.get(character_id, {})
	if String(me.get("action", "")) != "":
		buffered = {"slot": slot, "dir": dir, "until": Time.get_ticks_msec() + BUFFER_MS}
		return
	buffered = {}
	_request("cast", {"slot": slot, "dir": [dir.x, dir.y]}, {"slot": slot, "dir": dir})


func _drive_buffer() -> void:
	if buffered.is_empty():
		return
	if Time.get_ticks_msec() > int(buffered["until"]):
		buffered = {}
		return
	var me: Dictionary = entities.get(character_id, {})
	if String(me.get("action", "")) == "" and not me.get("stunned", false):
		var b: Dictionary = buffered
		buffered = {}
		_request("cast", {"slot": b["slot"], "dir": [b["dir"].x, b["dir"].y]}, {"slot": b["slot"], "dir": b["dir"]})


## Chase the attack target into basic-attack range, then keep hitting it.
func _drive_attack() -> void:
	if attack_target == 0:
		return
	var t: Dictionary = entities.get(attack_target, {})
	var me: Dictionary = entities.get(character_id, {})
	if t.is_empty() or t.get("dead", false) or me.is_empty() or me.get("dead", false):
		attack_target = 0
		return
	var now: int = Time.get_ticks_msec()
	var target_pos: Vector2 = Vector2(float(t["x"]), float(t["z"]))
	var reach: float = float(self_info.get("ranges", {}).get("basic", 1.5))
	if _self_pos().distance_to(target_pos) > reach - 0.2:
		if now >= _next_chase_ms:
			_next_chase_ms = now + 300
			_request("move_to", {"x": target_pos.x, "z": target_pos.y, "accept_risk": false}, {"target": target_pos, "chase": true})
	elif buffered.is_empty() and String(me.get("action", "")) == "" and not me.get("stunned", false) and now >= _next_basic_ms:
		_next_basic_ms = now + 200
		_request("cast", {"slot": "basic", "target": attack_target}, {"slot": "basic"})


func _on_pushed(envelope: Dictionary) -> void:
	var payload: Dictionary = envelope["payload"]
	match envelope["type"]:
		"snapshot":
			apply_snapshot(float(envelope.get("server_time", 0)), payload)
		"leave_cancelled":
			var reasons: Dictionary = {"moved": "移动", "acted": "出招", "attacked": "遭到攻击"}
			status_label.text = "离开已取消（%s）。" % reasons.get(payload.get("reason", ""), "")
		"left_world":
			status_label.text = "已离开江湖。"


func apply_snapshot(server_time: float, payload: Dictionary) -> void:
	var list: Array = payload.get("entities", [])
	buffer.push(server_time, list, Time.get_ticks_msec())
	entities.clear()
	for e: Variant in list:
		entities[int(e["id"])] = e
	corpses.clear()
	for c: Variant in payload.get("corpses", []):
		corpses[int(c["id"])] = c
	var me_info: Variant = payload.get("self")
	self_info = me_info if typeof(me_info) == TYPE_DICTIONARY else {}
	for ev: Variant in payload.get("events", []):
		_show_event(ev)
	_refresh_hud()
	_refresh_corpses()


func _on_reply(req_id: String, envelope: Dictionary) -> void:
	if not _pending.has(req_id):
		return
	var request: Dictionary = _pending[req_id]
	_pending.erase(req_id)
	var payload: Dictionary = envelope["payload"]
	if envelope["type"] == "error":
		var code: String = String(payload.get("code", ""))
		var chasing: bool = request.get("chase", false)
		if code == "risk_confirmation_required" and not chasing:
			_risk_target = request.get("target", Vector2.ZERO)
			risk_dialog.popup_centered()
		elif code == "busy" and request.has("dir") and buffered.is_empty():
			# Pressed during an action the client had not seen yet: buffer it.
			buffered = {"slot": request["slot"], "dir": request["dir"], "until": Time.get_ticks_msec() + BUFFER_MS}
		elif not (chasing or (request.get("slot", "") == "basic" and code == "busy")):
			status_label.text = ERROR_TEXT.get(code, "操作失败：" + code)
		return
	match String(request["type"]):
		"corpse_open":
			show_loot(int(request["corpse"]), payload)
		"loot":
			_request("corpse_open", {"corpse_id": open_corpse}, {"corpse": open_corpse})
		"respawn":
			death_panel.visible = false
			status_label.text = "已在 %s 复活。" % String(payload.get("area", ""))
		"swap":
			status_label.text = "切换中……"
		"cast":
			pass
		_:
			status_label.text = ""


func show_loot(corpse_id: int, payload: Dictionary) -> void:
	open_corpse = corpse_id
	var owner: String = String(corpses.get(corpse_id, {}).get("owner", ""))
	$HUD/Loot/Box/Title.text = ("%s的遗物" % owner) if owner != "" else "遗物"
	var items_box: VBoxContainer = $HUD/Loot/Box/Items
	for child: Node in items_box.get_children():
		child.queue_free()
	var coins: int = int(payload.get("coins", 0))
	if coins > 0:
		var b: Button = Button.new()
		b.text = "银钱 × %d" % coins
		b.pressed.connect(func() -> void: _request("loot", {"corpse_id": corpse_id, "coins": true}))
		items_box.add_child(b)
	var items: Array = payload.get("items", [])
	for it: Variant in items:
		var item: Dictionary = it
		var button: Button = Button.new()
		button.text = "%s × %d" % [String(item.get("name", item.get("proto", "?"))), int(item.get("qty", 1))]
		var item_id: String = String(item["id"])
		button.pressed.connect(func() -> void: _request("loot", {"corpse_id": corpse_id, "item_id": item_id}))
		items_box.add_child(button)
	if coins == 0 and items.is_empty():
		var empty: Label = Label.new()
		empty.text = "已经空了。"
		items_box.add_child(empty)
	loot_panel.visible = true


func close_loot() -> void:
	open_corpse = 0
	loot_panel.visible = false


func _process(_delta: float) -> void:
	var seen: Dictionary = buffer.sample(Time.get_ticks_msec())
	for id: int in seen:
		var data: Dictionary = seen[id]["data"]
		var avatar: Node3D = _avatars.get(id)
		if avatar == null:
			avatar = _make_avatar(id, data)
			_avatars[id] = avatar
			avatars_root.add_child(avatar)
		var p: Vector2 = seen[id]["pos"]
		avatar.position = Vector3(p.x, 0.0, p.y)
		_style_avatar(avatar, data)
		if id == character_id:
			rig.follow(avatar.position)
			_show_area(String(data.get("area", "")), String(data.get("zone", "")))
	for id: int in _avatars.keys():
		if not seen.has(id):
			_avatars[id].queue_free()
			_avatars.erase(id)
	_drive_buffer()
	_drive_attack()


func _refresh_hud() -> void:
	var me: Dictionary = entities.get(character_id, {})
	if not me.is_empty():
		var hp: ProgressBar = $HUD/Bars/Box/HP
		hp.max_value = float(me.get("max_hp", 1))
		hp.value = float(me.get("hp", 0))
		($HUD/Bars/Box/MP as ProgressBar).value = float(me.get("mp", 0))
		($HUD/Bars/Box/SP as ProgressBar).value = float(me.get("sp", 0))
		var weapon: String = "持剑" if me.get("preset", "") == "armed" else "空手"
		var fighting: String = " · 战斗中" if self_info.get("in_combat", false) else ""
		$HUD/Bars/Box/Values.text = "气血 %d/%d · 内力 %d · 体力 %d · %s%s" % [int(me.get("hp", 0)), int(me.get("max_hp", 0)),
			int(me.get("mp", 0)), int(me.get("sp", 0)), weapon, fighting]
		death_panel.visible = bool(me.get("dead", false))
	var cds: Dictionary = self_info.get("cooldowns", {})
	for slot: String in SKILL_KEYS:
		var label: Label = skills_box.get_node("Slot_" + slot)
		var left: float = float(cds.get(slot, 0.0))
		label.text = SKILL_KEYS[slot] if left <= 0.0 else "%s %.1f" % [SKILL_KEYS[slot], left]
	var t: Dictionary = entities.get(attack_target, {})
	target_panel.visible = not t.is_empty()
	if not t.is_empty():
		$HUD/Target/Box/Name.text = String(t.get("name", ""))
		var bar: ProgressBar = $HUD/Target/Box/HP
		bar.max_value = float(t.get("max_hp", 1))
		bar.value = float(t.get("hp", 0))


func _refresh_corpses() -> void:
	for id: int in corpses:
		var c: Dictionary = corpses[id]
		var node: Node3D = _corpse_nodes.get(id)
		if node == null:
			node = _make_corpse(String(c.get("owner", "")))
			_corpse_nodes[id] = node
			corpses_root.add_child(node)
		node.position = Vector3(float(c["x"]), 0.0, float(c["z"]))
		(node.get_node("Label") as Label3D).modulate = Color(1, 1, 1, 0.4) if c.get("empty", false) else Color.WHITE
	for id: int in _corpse_nodes.keys():
		if not corpses.has(id):
			_corpse_nodes[id].queue_free()
			_corpse_nodes.erase(id)
			if id == open_corpse:
				close_loot()


func _make_corpse(owner: String) -> Node3D:
	var node: Node3D = Node3D.new()
	var box: MeshInstance3D = MeshInstance3D.new()
	var mesh: BoxMesh = BoxMesh.new()
	mesh.size = Vector3(1.2, 0.3, 0.8)
	var mat: StandardMaterial3D = StandardMaterial3D.new()
	mat.albedo_color = Color(0.25, 0.2, 0.15)
	mesh.material = mat
	box.mesh = mesh
	box.position.y = 0.15
	node.add_child(box)
	var label: Label3D = MapBuilder.name_label("%s的遗物" % owner, 24)
	label.name = "Label"
	label.position.y = 1.0
	node.add_child(label)
	return node


func _show_event(ev: Dictionary) -> void:
	var kind: String = String(ev.get("kind", ""))
	var who: int = int(ev.get("target", 0)) if kind in ["hit", "stun", "stun_immune"] else int(ev.get("actor", 0))
	var text: String = ""
	var color: Color = Color.WHITE
	match kind:
		"hit":
			text = "-%d" % int(ev.get("value", 0))
			color = Color(1.0, 0.45, 0.35) if who == character_id else Color(1.0, 0.95, 0.6)
		"miss":
			text = "落空"
		"parry":
			text = "招架"
			color = Color(0.6, 0.9, 1.0)
		"stun":
			text = "眩晕"
			color = Color(1.0, 0.8, 0.3)
		"stun_immune":
			text = "免控"
		"death":
			text = "倒下"
			color = Color(0.8, 0.8, 0.8)
		_:
			return
	var e: Dictionary = entities.get(who, {})
	if e.is_empty():
		return
	float_text(Vector3(float(e["x"]), 2.6, float(e["z"])), text, color)


func float_text(at: Vector3, text: String, color: Color) -> void:
	var label: Label3D = MapBuilder.name_label(text, 36)
	label.modulate = color
	label.position = at
	effects_root.add_child(label)
	var tween: Tween = label.create_tween()
	tween.tween_property(label, "position:y", at.y + 1.5, 0.8)
	tween.parallel().tween_property(label, "modulate:a", 0.0, 0.8)
	tween.tween_callback(label.queue_free)


func _show_area(area: String, zone: String) -> void:
	area_label.text = "%s · %s" % [area, AREA_NAMES.get(zone, zone)]


func _make_avatar(id: int, data: Dictionary) -> Node3D:
	var root: Node3D = Node3D.new()
	root.name = "Avatar_%d" % id
	var body: MeshInstance3D = MeshInstance3D.new()
	body.name = "Body"
	var capsule: CapsuleMesh = CapsuleMesh.new()
	capsule.radius = 0.4
	capsule.height = 1.8
	var material: StandardMaterial3D = StandardMaterial3D.new()
	if id == character_id:
		material.albedo_color = COLOR_SELF
	elif data.get("kind", "") == "monster":
		material.albedo_color = COLOR_MONSTER
	else:
		material.albedo_color = COLOR_PLAYER
	capsule.material = material
	body.mesh = capsule
	body.position.y = 0.9
	root.add_child(body)
	var label: Label3D = MapBuilder.name_label(String(data.get("name", "?")), 28)
	label.name = "Name"
	label.position.y = 2.4
	root.add_child(label)
	return root


## Placeholder pose until real models arrive: lie down when dead, lean into
## an attack, and show HP and stun in the name tag.
func _style_avatar(avatar: Node3D, data: Dictionary) -> void:
	var body: Node3D = avatar.get_node("Body")
	var dead: bool = data.get("dead", false)
	body.rotation = Vector3(0, 0, PI / 2) if dead else Vector3.ZERO
	body.position.y = 0.4 if dead else 0.9
	avatar.rotation.y = -float(data.get("facing", 0.0)) + PI / 2
	if String(data.get("action", "")) != "" and not dead:
		body.rotation.x = -0.35
	var tag: Label3D = avatar.get_node("Name")
	var hp: String = "" if dead else " %d/%d" % [int(data.get("hp", 0)), int(data.get("max_hp", 0))]
	tag.text = String(data.get("name", "?")) + hp + (" 晕" if data.get("stunned", false) else "")

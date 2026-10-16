extends Node3D
## The in-world view: draws the map and characters from server snapshots and
## sends destinations. The client never decides positions; it only asks to
## move and renders what the server reports.

signal leave_requested
signal move_requested(target: Vector2, accept_risk: bool)

const AREA_NAMES: Dictionary = {"service": "安全区", "pve": "野外（无强制PVP）", "risk": "风险区（可能遭受玩家攻击）"}

var client: WsClient
var character_id: int = 0
var buffer: SnapshotBuffer = SnapshotBuffer.new()
var _pending: Dictionary = {}  # req_id -> {"type", "target"}
var _avatars: Dictionary = {}  # id -> Node3D
var _risk_target: Vector2 = Vector2.ZERO

@onready var rig: CameraRig = $CameraRig
@onready var map_root: Node3D = $MapRoot
@onready var avatars_root: Node3D = $Avatars
@onready var area_label: Label = $HUD/Panel/Box/Area
@onready var status_label: Label = $HUD/Panel/Box/Status
@onready var leave_button: Button = $HUD/Panel/Box/Leave
@onready var risk_dialog: ConfirmationDialog = $HUD/RiskDialog


func _ready() -> void:
	leave_button.pressed.connect(func() -> void: leave_requested.emit())
	risk_dialog.confirmed.connect(func() -> void: request_move(_risk_target, true))


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


func request_move(target: Vector2, accept_risk: bool = false) -> void:
	move_requested.emit(target, accept_risk)
	if client == null:
		return
	var req_id: String = client.request("move_to", {"x": target.x, "z": target.y, "accept_risk": accept_risk})
	if req_id != "":
		_pending[req_id] = {"type": "move_to", "target": target}


## Intersects a camera ray with the ground plane y = 0.
static func ground_point(origin: Vector3, direction: Vector3) -> Variant:
	if absf(direction.y) < 1e-6:
		return null
	var t: float = -origin.y / direction.y
	if t <= 0.0:
		return null
	var hit: Vector3 = origin + direction * t
	return Vector2(hit.x, hit.z)


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
	elif event.is_action_pressed("move_stop") and client != null:
		var req_id: String = client.request("stop")
		_pending[req_id] = {"type": "stop"}
	if screen_pos == null:
		return
	var cam: Camera3D = rig.camera
	var hit: Variant = ground_point(cam.project_ray_origin(screen_pos), cam.project_ray_normal(screen_pos))
	if hit != null:
		request_move(hit)
		get_viewport().set_input_as_handled()


func _on_pushed(envelope: Dictionary) -> void:
	if envelope["type"] != "snapshot":
		return
	buffer.push(float(envelope.get("server_time", 0)), envelope["payload"].get("entities", []), Time.get_ticks_msec())


func _on_reply(req_id: String, envelope: Dictionary) -> void:
	if not _pending.has(req_id):
		return
	var request: Dictionary = _pending[req_id]
	_pending.erase(req_id)
	if envelope["type"] != "error":
		status_label.text = ""
		return
	var code: String = String(envelope["payload"].get("code", ""))
	match code:
		"risk_confirmation_required":
			_risk_target = request.get("target", Vector2.ZERO)
			risk_dialog.popup_centered()
		"unreachable":
			status_label.text = "无法到达该处。"
		"rate_limited":
			status_label.text = "操作过于频繁。"
		_:
			status_label.text = "移动失败：" + code


func _process(_delta: float) -> void:
	var now: float = Time.get_ticks_msec()
	var seen: Dictionary = buffer.sample(now)
	for id: int in seen:
		var avatar: Node3D = _avatars.get(id)
		if avatar == null:
			avatar = _make_avatar(id, String(seen[id]["data"].get("name", "?")))
			_avatars[id] = avatar
			avatars_root.add_child(avatar)
		var p: Vector2 = seen[id]["pos"]
		avatar.position = Vector3(p.x, 0.0, p.y)
		if id == character_id:
			rig.follow(avatar.position)
			var data: Dictionary = seen[id]["data"]
			_show_area(String(data.get("area", "")), String(data.get("zone", "")))
	for id: int in _avatars.keys():
		if not seen.has(id):
			_avatars[id].queue_free()
			_avatars.erase(id)


func _show_area(area: String, zone: String) -> void:
	area_label.text = "%s · %s" % [area, AREA_NAMES.get(zone, zone)]


func _make_avatar(id: int, display_name: String) -> Node3D:
	var root: Node3D = Node3D.new()
	root.name = "Avatar_%d" % id
	var body: MeshInstance3D = MeshInstance3D.new()
	var capsule: CapsuleMesh = CapsuleMesh.new()
	capsule.radius = 0.4
	capsule.height = 1.8
	var material: StandardMaterial3D = StandardMaterial3D.new()
	material.albedo_color = Color(0.95, 0.80, 0.35) if id == character_id else Color(0.55, 0.70, 0.95)
	capsule.material = material
	body.mesh = capsule
	body.position.y = 0.9
	root.add_child(body)
	var label: Label3D = MapBuilder.name_label(display_name, 28)
	label.position.y = 2.4
	root.add_child(label)
	return root

extends Control
## Account screen and game shell: register, log in, list/create characters,
## then show the world view while a character is in the world. The token
## stays in memory only.

const WORLD_VIEW: PackedScene = preload("res://scenes/world_view.tscn")

const ERROR_TEXT: Dictionary = {
	"bad_credentials": "账号、密码或登录凭证不正确。",
	"invalid_name": "名字须为 2–32 个字母、数字、_ 或 -。",
	"invalid_password": "密码须为 8–128 字节。",
	"name_taken": "这个名字已被使用。",
	"character_limit": "每个账号最多 6 个角色。",
	"character_present": "该账号另一名角色仍在江湖中：请重连该角色，或等待其离场。",
	"unknown_character": "找不到该角色。",
	"not_in_world": "当前没有角色在江湖中。",
	"not_authenticated": "请先登录。",
	"internal": "服务器出错，请稍后重试。",
}

@onready var address: LineEdit = $Margin/Content/Address
@onready var name_edit: LineEdit = $Margin/Content/Name
@onready var password_edit: LineEdit = $Margin/Content/Password
@onready var register_button: Button = $Margin/Content/AuthButtons/Register
@onready var login_button: Button = $Margin/Content/AuthButtons/Login
@onready var status_label: Label = $Margin/Content/Status
@onready var characters_box: VBoxContainer = $Margin/Content/Characters
@onready var character_list: ItemList = $Margin/Content/Characters/List
@onready var new_name: LineEdit = $Margin/Content/Characters/NewName
@onready var create_button: Button = $Margin/Content/Characters/Buttons/Create
@onready var enter_button: Button = $Margin/Content/Characters/Buttons/Enter
@onready var leave_button: Button = $Margin/Content/Characters/Buttons/Leave
@onready var client: WsClient = $Client

var token: String = ""
var _pending: Dictionary = {}  # req_id -> request type
var _queued: Array[Dictionary] = []  # requests waiting for the connection
var _characters: Array[Dictionary] = []
var world_view: Node3D = null
var _in_world_id: int = 0


func _ready() -> void:
	register_button.pressed.connect(_on_register)
	login_button.pressed.connect(_on_login)
	create_button.pressed.connect(_on_create)
	enter_button.pressed.connect(_on_enter)
	leave_button.pressed.connect(func() -> void: _send("leave_world"))
	client.opened.connect(_on_opened)
	client.closed.connect(_on_closed)
	client.reply_received.connect(_on_reply)
	client.pushed.connect(_on_pushed)
	characters_box.visible = false


## Returns an error message, or "" when name and password look acceptable.
static func validate_credentials(name: String, password: String) -> String:
	var length: int = name.length()
	if length < 2 or length > 32:
		return ERROR_TEXT["invalid_name"]
	var bytes: int = password.to_utf8_buffer().size()
	if bytes < 8 or bytes > 128:
		return ERROR_TEXT["invalid_password"]
	return ""


func _credentials() -> Dictionary:
	return {"name": name_edit.text.strip_edges(), "password": password_edit.text}


func _on_register() -> void:
	var c: Dictionary = _credentials()
	var problem: String = validate_credentials(c["name"], c["password"])
	if problem != "":
		status_label.text = problem
		return
	_send("register", c)


func _on_login() -> void:
	var c: Dictionary = _credentials()
	var problem: String = validate_credentials(c["name"], c["password"])
	if problem != "":
		status_label.text = problem
		return
	_send("login", c)


func _on_create() -> void:
	var name: String = new_name.text.strip_edges()
	if name.length() < 2 or name.length() > 32:
		status_label.text = ERROR_TEXT["invalid_name"]
		return
	_send("char_create", {"name": name})


func _on_enter() -> void:
	var selected: PackedInt32Array = character_list.get_selected_items()
	if selected.is_empty():
		status_label.text = "请先选择一个角色。"
		return
	_send("enter_world", {"character_id": int(_characters[selected[0]]["id"])})


## Sends now if connected; otherwise connects and sends once open.
func _send(type: String, payload: Variant = null) -> void:
	if client.is_open():
		var req_id: String = client.request(type, payload)
		_pending[req_id] = type
		return
	_queued.append({"type": type, "payload": payload})
	var url: String = address.text.strip_edges()
	if client.connect_to(url) != OK:
		_queued.clear()
		status_label.text = "地址须以 ws:// 或 wss:// 开头，例如 ws://127.0.0.1:8080/v1/ws"
		return
	status_label.text = "正在连接 / Connecting…"


func _on_opened() -> void:
	status_label.text = "已连接 / Connected"
	if token != "" and _queued.is_empty():
		_send("resume", {"token": token})
	for item: Dictionary in _queued:
		_send(item["type"], item["payload"])
	_queued.clear()


func _on_closed(code: int, _reason: String) -> void:
	_pending.clear()
	if code == Protocol.CLOSE_KICKED:
		status_label.text = "账号已在其他设备登录，本连接已断开。"
		token = ""
		characters_box.visible = false
		_close_world()
	elif code == Protocol.CLOSE_TOO_MANY_FAILURES:
		_close_world()
		status_label.text = "登录失败次数过多，连接已断开。"
	else:
		status_label.text = "连接已断开（%d），稍后自动重连。" % code


func _on_pushed(envelope: Dictionary) -> void:
	if envelope["type"] == "kicked":
		status_label.text = "账号已在其他设备登录。"


func _on_reply(req_id: String, envelope: Dictionary) -> void:
	var type: String = String(_pending.get(req_id, ""))
	_pending.erase(req_id)
	var payload: Dictionary = envelope["payload"]
	if envelope["type"] == "error":
		var code: String = String(payload.get("code", ""))
		status_label.text = ERROR_TEXT.get(code, "请求失败：%s" % code)
		if type == "resume":
			token = ""
			characters_box.visible = false
		return
	match type:
		"register":
			status_label.text = "注册成功，请登录。"
		"login", "resume":
			if type == "login":
				token = String(payload.get("token", ""))
			status_label.text = "已登录 / Logged in"
			characters_box.visible = true
			if world_view != null and _in_world_id != 0:
				# Reconnected while in the world: take the character back.
				_send("enter_world", {"character_id": _in_world_id})
			else:
				_send("char_list")
		"char_list":
			_show_characters(payload)
		"char_create":
			new_name.text = ""
			_send("char_list")
		"enter_world":
			_open_world(payload)
		"leave_world":
			_close_world()
			status_label.text = "角色已离开江湖。"
			_send("char_list")


func _show_characters(payload: Dictionary) -> void:
	_characters.clear()
	character_list.clear()
	var present: Variant = payload.get("present")
	var present_id: int = -1
	if typeof(present) == TYPE_DICTIONARY:
		present_id = int(present.get("character_id", -1))
	for c: Variant in payload.get("characters", []):
		if typeof(c) != TYPE_DICTIONARY:
			continue
		_characters.append(c)
		var label: String = String(c.get("name", "?"))
		if int(c.get("id", 0)) == present_id:
			label += "（在江湖中）"
		character_list.add_item(label)


func _open_world(payload: Dictionary) -> void:
	_in_world_id = int(payload.get("character_id", 0))
	if world_view == null:
		world_view = WORLD_VIEW.instantiate()
		add_child(world_view)
		world_view.leave_requested.connect(func() -> void: _send("leave_world"))
	world_view.setup(client, payload)
	$Background.visible = false
	$Margin.visible = false


func _close_world() -> void:
	_in_world_id = 0
	if world_view != null:
		world_view.queue_free()
		world_view = null
	$Background.visible = true
	$Margin.visible = true

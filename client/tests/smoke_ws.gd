extends SceneTree
## Real end-to-end check against a running server with a database:
## register, log in, create and list characters, enter and leave the world,
## then a second login of the same account kicks the first connection.
## Usage: godot --headless --path client --script res://tests/smoke_ws.gd -- ws://127.0.0.1:8080/v1/ws

var url: String = "ws://127.0.0.1:8080/v1/ws"
var replies: Dictionary = {}
var closes: Array[int] = []


func _initialize() -> void:
	call_deferred("_run")


func _fail(message: String) -> void:
	push_error("FAIL " + message)
	quit(1)


func _connect(client: WsClient) -> bool:
	client.auto_reconnect = false
	client.reply_received.connect(func(id: String, env: Dictionary) -> void: replies[id] = env)
	client.closed.connect(func(code: int, _r: String) -> void: closes.append(code))
	root.add_child(client)
	if client.connect_to(url) != OK:
		return false
	var deadline: int = Time.get_ticks_msec() + 5000
	while not client.is_open() and Time.get_ticks_msec() < deadline:
		await process_frame
	return client.is_open()


func _call(client: WsClient, type: String, payload: Variant = null) -> Dictionary:
	var id: String = client.request(type, payload)
	var deadline: int = Time.get_ticks_msec() + 5000
	while not replies.has(id) and Time.get_ticks_msec() < deadline:
		await process_frame
	return replies.get(id, {"type": "timeout"})


func _expect(env: Dictionary, type: String, step: String) -> bool:
	if env.get("type") != type:
		_fail("%s: expected %s, got %s" % [step, type, JSON.stringify(env)])
		return false
	return true


func _run() -> void:
	var args: PackedStringArray = OS.get_cmdline_user_args()
	if not args.is_empty():
		url = args[0]
	var name: String = "smoke%d" % (Time.get_unix_time_from_system() * 1000.0)
	var creds: Dictionary = {"name": name, "password": "password-" + name}
	var first: WsClient = WsClient.new()
	if not await _connect(first):
		_fail("could not connect to " + url)
		return
	if not _expect(await _call(first, "register", creds), "register.ok", "register"): return
	var login: Dictionary = await _call(first, "login", creds)
	if not _expect(login, "login.ok", "login"): return
	if String(login["payload"].get("token", "")).length() < 40:
		_fail("login token missing")
		return
	var created: Dictionary = await _call(first, "char_create", {"name": "剑客" + name.right(4)})
	if not _expect(created, "char_create.ok", "char_create"): return
	var char_id: int = int(created["payload"]["id"])
	var listed: Dictionary = await _call(first, "char_list")
	if not _expect(listed, "char_list.ok", "char_list"): return
	if listed["payload"]["characters"].size() != 1:
		_fail("char_list size")
		return
	if not _expect(await _call(first, "enter_world", {"character_id": char_id}), "enter_world.ok", "enter_world"): return

	var second: WsClient = WsClient.new()
	if not await _connect(second):
		_fail("second connection")
		return
	if not _expect(await _call(second, "login", creds), "login.ok", "second login"): return
	var deadline: int = Time.get_ticks_msec() + 5000
	while not closes.has(Protocol.CLOSE_KICKED) and Time.get_ticks_msec() < deadline:
		await process_frame
	# The close code is authoritative; the preceding "kicked" push may be
	# dropped by WebSocketPeer when it arrives together with the close frame.
	if not closes.has(Protocol.CLOSE_KICKED):
		_fail("first connection was not kicked (closes=%s)" % [closes])
		return
	if not _expect(await _call(second, "enter_world", {"character_id": char_id}), "enter_world.ok", "take over character"): return
	if not _expect(await _call(second, "leave_world"), "leave_world.ok", "leave_world"): return
	second.close_connection()
	print("PASS websocket smoke: register, login, characters, enter/leave, kick on second login")
	quit(0)
